package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Service concentra as regras de negocio do modulo (camada do meio: o handler nao toca
// o banco, o Store nao conhece HTTP).
//
// Toda operacao recebe o accountID resolvido do Principal — nunca do body (principio 2).
type Service struct {
	store                 *Store
	groupMetadataResolver *groupMetadataResolver
}

// NewService cria o Service.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

func (s *Service) setGroupMetadataResolver(resolver *groupMetadataResolver) {
	s.groupMetadataResolver = resolver
}

// Caller e o contexto do chamador que as regras desta fase precisam: quem e (userID) e
// se e admin da conta (o filtro de instancia do A2 e o canViewSensitive dependem disso).
type Caller struct {
	UserID string
	// IsAdmin = papel administrativo (platform_admin/owner/director). Resolvido do
	// Principal em http.go (callerFrom), nunca do body.
	IsAdmin bool
	// IsPlatformAdmin separa operacoes de plataforma (como teto contratado) das
	// configuracoes que um administrador comum da conta pode alterar.
	IsPlatformAdmin bool
}

// translate mapeia o erro do banco para o erro do dominio. pgx.ErrNoRows vira SEMPRE
// ErrNotFound — inclusive quando a linha existe mas e de outra conta (o filtro de
// account_id a esconde). E de proposito: fora de escopo responde 404, nunca 403.
func translate(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// ============================================================================
// Conversas
// ============================================================================

// ListConversations devolve o inbox da account, ordenado por last_message_at DESC e
// SEM paginacao (contrato do legado). instanceID vazio = todas as instancias.
//
// O escopo de instancia por usuario e o A2 CORRIGIDO: o legado tem um ternario que
// devolve a mesma coisa nos dois ramos (whatsapp-instances.ts:681-683), ou seja, todo
// usuario ve tudo. Aqui o nao-admin so ve as conversas das instancias que ele alcanca.
func (s *Service) ListConversations(ctx context.Context, accountID string, caller Caller, pageFilter ConversationPageFilter) (ConversationPageView, error) {
	normalized, err := normalizeConversationFilter(pageFilter)
	if err != nil {
		return ConversationPageView{}, err
	}
	f := ConversationFilter{ConversationPageFilter: normalized}
	if !caller.IsAdmin {
		scopeKeys, err := s.accessibleScopeKeys(ctx, accountID, caller)
		if err != nil {
			return ConversationPageView{}, err
		}
		f.ScopeKeys = scopeKeys
	}
	f.Limit = normalized.Limit + 1
	rows, err := s.store.ListConversations(ctx, accountID, f)
	if err != nil {
		return ConversationPageView{}, err
	}
	hasMore := len(rows) > normalized.Limit
	if hasMore {
		rows = rows[:normalized.Limit]
	}
	if s.groupMetadataResolver != nil {
		for i := range rows {
			if !isWhatsAppGroupExternalID(rows[i].ExternalID) {
				continue
			}
			if name, resolveErr := s.groupMetadataResolver.resolve(ctx, accountID, rows[i].InstanceScopeKey, rows[i].ExternalID); resolveErr == nil && name != "" {
				rows[i].ContactName = &name
				_ = s.store.UpdateConversationGroupName(ctx, accountID, rows[i].InstanceScopeKey, rows[i].ExternalID, name)
			}
		}
	}
	out := make([]ConversationView, 0, len(rows))
	for _, row := range rows {
		view, err := conversationView(row)
		if err != nil {
			return ConversationPageView{}, err
		}
		out = append(out, view)
	}
	nextCursor := ""
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		nextCursor = encodeConversationCursor(last.LastMessageAt, last.ID)
	}
	return ConversationPageView{Conversations: out, HasMore: hasMore, NextCursor: nextCursor}, nil
}

var omnichannelUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func normalizeConversationFilter(f ConversationPageFilter) (ConversationPageFilter, error) {
	f.Search = strings.Join(strings.Fields(strings.TrimSpace(f.Search)), " ")
	if len([]rune(f.Search)) > 120 {
		return ConversationPageFilter{}, ErrInvalidBody
	}
	f.Channel = strings.ToUpper(strings.TrimSpace(f.Channel))
	if f.Channel == "ALL" {
		f.Channel = ""
	}
	if f.Channel != "" && f.Channel != "WHATSAPP" && f.Channel != "INSTAGRAM" {
		return ConversationPageFilter{}, ErrInvalidBody
	}
	f.Status = strings.ToUpper(strings.TrimSpace(f.Status))
	if f.Status == "ALL" {
		f.Status = ""
	}
	if f.Status != "" && f.Status != "OPEN" && f.Status != "PENDING" && f.Status != "CLOSED" {
		return ConversationPageFilter{}, ErrInvalidBody
	}
	for _, id := range []string{f.InstanceID, f.QueueID, f.ResponsibleID} {
		if strings.TrimSpace(id) != "" && !omnichannelUUIDPattern.MatchString(strings.TrimSpace(id)) {
			return ConversationPageFilter{}, ErrInvalidBody
		}
	}
	f.InstanceID = strings.TrimSpace(f.InstanceID)
	f.QueueID = strings.TrimSpace(f.QueueID)
	f.ResponsibleID = strings.TrimSpace(f.ResponsibleID)
	switch {
	case f.Limit <= 0:
		f.Limit = defaultConversationLimit
	case f.Limit > maxConversationLimit:
		f.Limit = maxConversationLimit
	}
	return f, nil
}

// accessibleScopeKeys devolve os instance_scope_key que o usuario alcanca (A2). Lista
// vazia = o usuario nao alcanca instancia nenhuma; devolvemos um scope impossivel para
// a query nao virar "sem filtro" (fail-close: sem acesso => inbox vazio, nao inbox
// inteiro). O gate de dado DEFINITIVO e queue_members e chega na F8 (canonico §5.2).
func (s *Service) accessibleScopeKeys(ctx context.Context, accountID string, caller Caller) ([]string, error) {
	instances, err := s.store.ListInstances(ctx, accountID, InstanceFilter{
		ActiveOnly:        true,
		ResponsibleUserID: caller.UserID,
	})
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(instances))
	for _, i := range instances {
		keys = append(keys, i.InstanceName)
	}
	if len(keys) == 0 {
		return []string{}, nil
	}
	return keys, nil
}

// conversationView monta a view do front a partir da linha crua: projeta state -> status
// e desserializa o preview da ultima mensagem.
func conversationView(row conversationRow) (ConversationView, error) {
	scopeKey := row.InstanceScopeKey
	// instanceName cai para o scope key quando nao ha instancia no join — o legado faz
	// `instance?.instanceName ?? instanceScopeKey ?? null` (realtime.ts:42).
	instanceName := row.InstanceName
	if instanceName == nil && scopeKey != "" {
		instanceName = &scopeKey
	}

	var lastMessage *LastMessageView
	if len(row.LastMessage) > 0 && string(row.LastMessage) != "null" {
		var lm LastMessageView
		if err := json.Unmarshal(row.LastMessage, &lm); err != nil {
			return ConversationView{}, err
		}
		lastMessage = &lm
	}

	return ConversationView{
		ID:                  row.ID,
		InstanceID:          row.InstanceID,
		InstanceScopeKey:    &scopeKey,
		InstanceName:        instanceName,
		InstanceDisplayName: row.InstanceDisplayName,
		Channel:             row.Channel,
		Status:              projectStatus(row.State),
		AIStatus:            projectAIStatus(row.State),
		ExternalID:          row.ExternalID,
		ContactID:           row.ContactID,
		ContactName:         row.ContactName,
		ContactAvatarURL:    row.ContactAvatarURL,
		ContactPhone:        row.ContactPhone,
		AssignedToID:        row.AssignedToID,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
		LastMessageAt:       row.LastMessageAt,
		LastMessage:         lastMessage,
	}, nil
}

// ============================================================================
// Mensagens
// ============================================================================

// ListMessages devolve a pagina do historico. A conversa e resolvida ANTES (escopo:
// conversa de outra conta => 404) e a paginacao e `limit` + `beforeId`, nao cursor.
func (s *Service) ListMessages(ctx context.Context, accountID string, caller Caller, conversationID string, f MessagePageFilter) (MessagePageView, error) {
	if err := s.assertConversationScope(ctx, accountID, caller, conversationID); err != nil {
		return MessagePageView{}, err
	}
	f.Limit = normalizeLimit(f.Limit)

	messages, err := s.store.ListMessages(ctx, accountID, caller.UserID, conversationID, f)
	if err != nil {
		return MessagePageView{}, translate(err)
	}

	// hasMore = existe mensagem MAIS ANTIGA que a primeira da pagina (a pagina ja veio
	// em ASC, entao a primeira e a mais antiga). Pagina vazia = nao ha mais nada atras.
	hasMore := false
	if len(messages) > 0 {
		hasMore, err = s.store.HasOlderMessage(ctx, accountID, caller.UserID, conversationID, messages[0].CreatedAt, messages[0].ID)
		if err != nil {
			return MessagePageView{}, err
		}
	}
	nextCursor := ""
	if hasMore && len(messages) > 0 {
		nextCursor = encodeMessageCursor(messages[0].CreatedAt, messages[0].ID)
	}
	return MessagePageView{ConversationID: conversationID, Messages: messages, HasMore: hasMore, NextCursor: nextCursor}, nil
}

// GetMessage devolve uma mensagem da conversa (escopo validado antes; a query do Store
// filtra por account TAMBEM — defesa em profundidade).
func (s *Service) GetMessage(ctx context.Context, accountID string, caller Caller, conversationID, messageID string) (MessageView, error) {
	if err := s.assertConversationScope(ctx, accountID, caller, conversationID); err != nil {
		return MessageView{}, err
	}
	m, err := s.store.GetMessage(ctx, accountID, caller.UserID, conversationID, messageID)
	if err != nil {
		return MessageView{}, translate(err)
	}
	return m, nil
}

// assertConversationScope garante que a conversa existe, e da account e que o usuario a
// alcanca (A2). Qualquer falha vira ErrNotFound -> 404: nunca 403, que confirmaria a
// existencia do recurso (enumeration).
func (s *Service) assertConversationScope(ctx context.Context, accountID string, caller Caller, conversationID string) error {
	hidden, err := s.store.IsConversationHidden(ctx, accountID, conversationID)
	if err != nil {
		return err
	}
	if hidden {
		return ErrNotFound
	}
	row, err := s.store.GetConversation(ctx, accountID, conversationID)
	if err != nil {
		return translate(err)
	}
	if caller.IsAdmin {
		return nil
	}
	keys, err := s.accessibleScopeKeys(ctx, accountID, caller)
	if err != nil {
		return err
	}
	for _, k := range keys {
		if k == row.InstanceScopeKey {
			return nil
		}
	}
	return ErrNotFound
}

// normalizeLimit aplica o contrato do front: 1..200, default 100. Fora da faixa nao e
// erro no legado (o zod cai no default) — replicamos: 0/negativo => default; acima do
// teto => teto.
func normalizeLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultMessageLimit
	case limit > maxMessageLimit:
		return maxMessageLimit
	default:
		return limit
	}
}
