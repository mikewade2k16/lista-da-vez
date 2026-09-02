package omnichannel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store e a persistencia do modulo (schema messaging.*).
//
// REGRA DA CASA, sem excecao: TODA query filtra por account_id — inclusive as que
// recebem um id ja validado no service (defesa em profundidade, principio 2). IDs sao
// string + cast no SQL ($1::uuid); nao importamos pacote de uuid (padrao da casa).
type Store struct {
	pool                  *pgxpool.Pool
	aiDispatchV2          atomic.Bool
	historyCutoffDisabled atomic.Bool
}

// NewStore cria o Store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// SetHistoryCutoffEnforced controla somente o cutoff por instancia do P0. O zero-value e
// deliberadamente seguro: disabled=false significa filtro LIGADO. A privacidade por contato
// continua sendo aplicada mesmo durante o rollback funcional desta flag.
func (s *Store) SetHistoryCutoffEnforced(enabled bool) {
	s.historyCutoffDisabled.Store(!enabled)
}

func (s *Store) HistoryCutoffEnforced() bool {
	return !s.historyCutoffDisabled.Load()
}

type rowScanner interface {
	Scan(dest ...any) error
}

// ============================================================================
// Conversas
// ============================================================================

// conversationCols sai na ordem esperada por scanConversation. Os uuid saem como text
// (::text) para scanear em string/*string sem pacote de uuid. instance_name/display_name
// vem do LEFT JOIN na instancia (alias i) — exige o alias `c` na tabela de conversas.
const conversationCols = `c.id::text, c.instance_id::text, c.instance_scope_key,
	i.instance_name, i.display_name, c.channel, c.state, c.external_id, c.contact_id::text,
	c.contact_name, c.contact_avatar_url, c.contact_phone, c.assigned_to_id,
	c.created_at, c.updated_at, c.last_message_at`

// lastMessageCol resolve o preview da ultima mensagem como subquery ESCALAR de um jsonb
// (uma linha por conversa, sem multiplicar no JOIN), amarrada a MESMA account (defesa em
// profundidade: nunca mostra mensagem de outra conta). NULL = conversa sem mensagem.
// Usa o indice messaging_messages_conversation_created_idx — sem N+1.
func (s *Store) lastMessageCol() string {
	return `,
	(select jsonb_build_object(
		'id', m.id::text, 'content', m.content, 'messageType', m.message_type,
		'mediaUrl', m.media_url, 'direction', m.direction, 'status', m.status,
		'createdAt', m.created_at)
	 from messaging.messages m
	 where m.conversation_id = c.id and m.account_id = c.account_id` +
		s.historyVisibleMessagePredicate("m", "c") + `
	 order by m.created_at desc, m.id desc
	 limit 1)`
}

const visibleConversationFilter = ` and not exists (
	select 1 from messaging.contact_suppressions suppression
	where suppression.account_id=c.account_id and suppression.contact_id=c.contact_id
	  and suppression.is_hidden=true)`

// effectiveHistoryCutoffExpression e a unica definicao do cutoff operacional. O contato
// continua global; o cutoff da instancia vale somente para WhatsApp e pode ser desligado pela
// flag de rollback. Os aliases sao literais internos definidos nos call-sites deste pacote.
func (s *Store) effectiveHistoryCutoffExpression(conversationAlias string) string {
	contactCutoff := `(select suppression.history_cleared_at
		from messaging.contact_suppressions suppression
		where suppression.account_id=` + conversationAlias + `.account_id
		  and suppression.contact_id=` + conversationAlias + `.contact_id)`
	if !s.HistoryCutoffEnforced() {
		return `coalesce(` + contactCutoff + `, '-infinity'::timestamptz)`
	}
	instanceCutoff := `(select history_instance.history_visible_from
		from messaging.whatsapp_instances history_instance
		where ` + conversationAlias + `.channel='WHATSAPP'
		  and history_instance.account_id=` + conversationAlias + `.account_id
		  and (history_instance.id=` + conversationAlias + `.instance_id
		    or (` + conversationAlias + `.instance_id is null
		      and history_instance.instance_name=` + conversationAlias + `.instance_scope_key)))`
	return `greatest(coalesce(` + instanceCutoff + `, '-infinity'::timestamptz),
		coalesce(` + contactCutoff + `, '-infinity'::timestamptz))`
}

func (s *Store) historyVisibleMessagePredicate(messageAlias, conversationAlias string) string {
	predicate := ` and ` + messageAlias + `.created_at > ` + s.effectiveHistoryCutoffExpression(conversationAlias)
	if s.HistoryCutoffEnforced() {
		predicate += ` and not (` + messageAlias + `.origin='ai'
		  and ` + messageAlias + `.status='FAILED'
		  and ` + messageAlias + `.provider_error_code='history_reset')`
	}
	return predicate
}

// historyVisibleConversationPredicate nunca desliga a privacidade do contato. A flag de
// rollback remove apenas o cutoff da instancia. WhatsApp exige ao menos uma mensagem visivel;
// outros canais somem somente quando ha um cutoff de contato e nenhuma mensagem posterior.
func (s *Store) historyVisibleConversationPredicate(conversationAlias string) string {
	contactCutoff := `(select suppression.history_cleared_at
		from messaging.contact_suppressions suppression
		where suppression.account_id=` + conversationAlias + `.account_id
		  and suppression.contact_id=` + conversationAlias + `.contact_id)`
	contactVisible := `(` + contactCutoff + ` is null or exists (
		select 1 from messaging.messages history_contact_message
		where history_contact_message.account_id=` + conversationAlias + `.account_id
		  and history_contact_message.conversation_id=` + conversationAlias + `.id
		  and history_contact_message.created_at > ` + contactCutoff + `))`
	if !s.HistoryCutoffEnforced() {
		return ` and ` + contactVisible
	}
	return ` and ` + contactVisible + ` and (` + conversationAlias + `.channel <> 'WHATSAPP' or exists (
		select 1 from messaging.messages history_message
		where history_message.account_id=` + conversationAlias + `.account_id
		  and history_message.conversation_id=` + conversationAlias + `.id` +
		s.historyVisibleMessagePredicate("history_message", conversationAlias) + `))`
}

func (s *Store) IsMessageHistoryVisible(ctx context.Context, accountID, conversationID, messageID string) (bool, error) {
	var visible bool
	err := s.pool.QueryRow(ctx, `select exists (
		select 1 from messaging.messages history_message
		join messaging.conversations history_conversation
		  on history_conversation.account_id=history_message.account_id
		 and history_conversation.id=history_message.conversation_id
		where history_message.account_id=$1::uuid
		  and history_message.conversation_id=$2::uuid
		  and history_message.id=$3::uuid`+
		s.historyVisibleMessagePredicate("history_message", "history_conversation")+`)`,
		accountID, conversationID, messageID).Scan(&visible)
	return visible, err
}

// lockHistoryExternalEffectScope estabelece a ordem canonica usada por qualquer efeito
// externo que dependa do historico: instancia -> conversa. O lock da instancia permanece
// aberto na mesma transacao do callback, portanto um reset que chegou primeiro e sempre
// observado; se o efeito chegou primeiro, o reset espera ate o provider terminar.
//
// Conversas 0200 sem instance_id usam a chave legada account_id + instance_scope_key. A
// segunda leitura, ja com os locks, impede que rename/backfill troque o escopo entre a
// resolucao otimista e a revalidacao.
func lockHistoryExternalEffectScope(ctx context.Context, tx pgx.Tx, accountID, conversationID, conversationLock string) error {
	return lockHistoryExternalEffectScopeMode(ctx, tx, accountID, conversationID, conversationLock, false)
}

// lockHistoryExternalEffectScopeNowait e usado somente por gateways chamados de dentro de
// outro lease (n8n -> gateway). Ele nunca espera atras de um reset exclusivo: falha fechado e
// deixa a requisicao externa terminar, evitando um ciclo distribuido que o PostgreSQL nao ve.
func lockHistoryExternalEffectScopeNowait(ctx context.Context, tx pgx.Tx, accountID, conversationID, conversationLock string) error {
	return lockHistoryExternalEffectScopeMode(ctx, tx, accountID, conversationID, conversationLock, true)
}

func lockHistoryExternalEffectScopeMode(ctx context.Context, tx pgx.Tx, accountID, conversationID,
	conversationLock string, noWait bool) error {
	var channel, instanceScopeKey string
	var instanceID, resolvedInstanceID *string
	err := tx.QueryRow(ctx, `select c.channel,c.instance_id::text,c.instance_scope_key,
		coalesce(c.instance_id::text,(select history_instance.id::text
			from messaging.whatsapp_instances history_instance
			where c.channel='WHATSAPP' and history_instance.account_id=c.account_id
			  and history_instance.instance_name=c.instance_scope_key limit 1))
		from messaging.conversations c
		where c.account_id=$1::uuid and c.id=$2::uuid`, accountID, conversationID).
		Scan(&channel, &instanceID, &instanceScopeKey, &resolvedInstanceID)
	if err != nil {
		return err
	}

	var lockedInstanceID, lockedInstanceName string
	if strings.EqualFold(channel, "WHATSAPP") {
		if resolvedInstanceID != nil {
			instanceLock := " for share"
			if noWait {
				instanceLock += " nowait"
			}
			err = tx.QueryRow(ctx, `select id::text,instance_name
				from messaging.whatsapp_instances
				where account_id=$1::uuid and id=$2::uuid`+instanceLock, accountID, *resolvedInstanceID).
				Scan(&lockedInstanceID, &lockedInstanceName)
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrHistoryResetInvalidated
			}
			if err != nil {
				return err
			}
		}
	}

	lockClause := ""
	switch conversationLock {
	case "share":
		lockClause = " for share"
	case "update":
		lockClause = " for update"
	}
	var lockedChannel, lockedScopeKey string
	var lockedConversationID string
	var lockedConversationInstanceID *string
	err = tx.QueryRow(ctx, `select id::text,channel,instance_id::text,instance_scope_key
		from messaging.conversations where account_id=$1::uuid and id=$2::uuid`+lockClause,
		accountID, conversationID).
		Scan(&lockedConversationID, &lockedChannel, &lockedConversationInstanceID, &lockedScopeKey)
	if err != nil {
		return err
	}
	if !strings.EqualFold(channel, "WHATSAPP") || resolvedInstanceID == nil {
		return nil
	}
	if !strings.EqualFold(lockedChannel, "WHATSAPP") ||
		(lockedConversationInstanceID != nil && *lockedConversationInstanceID != lockedInstanceID) ||
		(lockedConversationInstanceID == nil && lockedScopeKey != lockedInstanceName) {
		return ErrHistoryResetInvalidated
	}
	return nil
}

func isHistoryEffectLockUnavailable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "55P03"
}

// WithMessageExternalEffectLease revalida uma mensagem e mantem apenas o fence compartilhado
// da instancia durante todo o efeito externo. Conversa e mensagem sao relidas sem tuple locks
// longos, preservando inbound/supersede enquanto LLM, midia ou provider estao em andamento.
// allowed=false e fail-closed: nenhum callback e executado quando a mensagem ficou oculta ou o
// escopo deixou de existir.
func (s *Store) WithMessageExternalEffectLease(ctx context.Context, accountID, conversationID, messageID string,
	effect func() error) (bool, error) {
	return s.withMessageExternalEffectLease(ctx, accountID, conversationID, messageID, false, effect)
}

// WithMessageExternalEffectLeaseNowait protege gateways internos que podem ser chamados sob um
// lease externo do n8n. Contencao com reset vira allowed=false em vez de espera circular.
func (s *Store) WithMessageExternalEffectLeaseNowait(ctx context.Context, accountID, conversationID, messageID string,
	effect func() error) (bool, error) {
	return s.withMessageExternalEffectLease(ctx, accountID, conversationID, messageID, true, effect)
}

func (s *Store) withMessageExternalEffectLease(ctx context.Context, accountID, conversationID, messageID string,
	noWait bool, effect func() error) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	lock := lockHistoryExternalEffectScope
	if noWait {
		lock = lockHistoryExternalEffectScopeNowait
	}
	if err := lock(ctx, tx, accountID, conversationID, "none"); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrHistoryResetInvalidated) || isHistoryEffectLockUnavailable(err) {
			return false, nil
		}
		return false, err
	}
	var lockedMessageID string
	err = tx.QueryRow(ctx, `select history_message.id::text
		from messaging.messages history_message
		join messaging.conversations history_conversation
		  on history_conversation.account_id=history_message.account_id
		 and history_conversation.id=history_message.conversation_id
		where history_message.account_id=$1::uuid
		  and history_message.conversation_id=$2::uuid
		  and history_message.id=$3::uuid`+
		s.historyVisibleMessagePredicate("history_message", "history_conversation"),
		accountID, conversationID, messageID).Scan(&lockedMessageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if effect != nil {
		if err := effect(); err != nil {
			return true, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return true, err
	}
	return true, nil
}

// WithForwardMessageExternalEffectLease mantem os fences das instancias de origem e destino
// em ordem deterministica enquanto o conteudo visivel da origem vira uma nova mensagem. Nenhum
// tuple lock longo e mantido, portanto forward para a mesma conversa e forwards cruzados nao
// bloqueiam o update normal da conversa de destino.
func (s *Store) WithForwardMessageExternalEffectLease(ctx context.Context, accountID, sourceConversationID,
	messageID, targetConversationID string, effect func() error) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	type scope struct {
		conversationID string
		channel        string
		instanceID     *string
	}
	scopes := make([]scope, 0, 2)
	for _, conversationID := range []string{sourceConversationID, targetConversationID} {
		var item scope
		item.conversationID = conversationID
		err := tx.QueryRow(ctx, `select c.channel,
			coalesce(c.instance_id::text,(select history_instance.id::text
				from messaging.whatsapp_instances history_instance
				where c.channel='WHATSAPP' and history_instance.account_id=c.account_id
				  and history_instance.instance_name=c.instance_scope_key limit 1))
			from messaging.conversations c
			where c.account_id=$1::uuid and c.id=$2::uuid`, accountID, conversationID).
			Scan(&item.channel, &item.instanceID)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		scopes = append(scopes, item)
	}
	instanceIDs := make([]string, 0, 2)
	seen := map[string]bool{}
	for _, item := range scopes {
		if strings.EqualFold(item.channel, "WHATSAPP") && item.instanceID != nil && !seen[*item.instanceID] {
			seen[*item.instanceID] = true
			instanceIDs = append(instanceIDs, *item.instanceID)
		}
	}
	sort.Strings(instanceIDs)
	for _, instanceID := range instanceIDs {
		var lockedID string
		if err := tx.QueryRow(ctx, `select id::text from messaging.whatsapp_instances
			where account_id=$1::uuid and id=$2::uuid for share`, accountID, instanceID).Scan(&lockedID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
	}
	for _, item := range scopes {
		if !strings.EqualFold(item.channel, "WHATSAPP") || item.instanceID == nil {
			continue
		}
		var stillMapped bool
		if err := tx.QueryRow(ctx, `select exists (select 1 from messaging.conversations c
			join messaging.whatsapp_instances history_instance
			  on history_instance.account_id=c.account_id
			 and (history_instance.id=c.instance_id or
			      (c.instance_id is null and history_instance.instance_name=c.instance_scope_key))
			where c.account_id=$1::uuid and c.id=$2::uuid and history_instance.id=$3::uuid)`,
			accountID, item.conversationID, *item.instanceID).Scan(&stillMapped); err != nil {
			return false, err
		}
		if !stillMapped {
			return false, nil
		}
	}
	var visibleMessageID string
	err = tx.QueryRow(ctx, `select history_message.id::text
		from messaging.messages history_message
		join messaging.conversations history_conversation
		  on history_conversation.account_id=history_message.account_id
		 and history_conversation.id=history_message.conversation_id
		where history_message.account_id=$1::uuid
		  and history_message.conversation_id=$2::uuid
		  and history_message.id=$3::uuid`+
		s.historyVisibleMessagePredicate("history_message", "history_conversation"),
		accountID, sourceConversationID, messageID).Scan(&visibleMessageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if effect != nil {
		if err := effect(); err != nil {
			return true, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return true, err
	}
	return true, nil
}

// conversationRow e a linha crua da conversa (colunas do banco, antes da projecao).
type conversationRow struct {
	ID                  string
	InstanceID          *string
	InstanceScopeKey    string
	InstanceName        *string
	InstanceDisplayName *string
	Channel             string
	State               string
	ExternalID          string
	ContactID           *string
	ContactName         *string
	ContactAvatarURL    *string
	ContactPhone        *string
	AssignedToID        *string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	LastMessageAt       time.Time
	// LastMessage vem do lastMessageCol como jsonb cru (padrao da casa: jsonb -> RawMessage,
	// nunca scan direto em struct). NULL/vazio = conversa sem mensagem. Quem desserializa
	// para LastMessageView e o service (view builder).
	LastMessage json.RawMessage
}

func scanConversation(row rowScanner) (conversationRow, error) {
	var c conversationRow
	err := row.Scan(&c.ID, &c.InstanceID, &c.InstanceScopeKey, &c.InstanceName,
		&c.InstanceDisplayName, &c.Channel, &c.State, &c.ExternalID, &c.ContactID,
		&c.ContactName, &c.ContactAvatarURL, &c.ContactPhone, &c.AssignedToID,
		&c.CreatedAt, &c.UpdatedAt, &c.LastMessageAt, &c.LastMessage)
	return c, err
}

// ConversationFilter filtra a listagem do inbox.
type ConversationFilter struct {
	ConversationPageFilter
	// Visibility e o resolver relacional P1B. Toda superficie HTTP o preenche.
	Visibility *VisibilityScope
}

// ListConversations devolve as conversas da account ordenadas por last_message_at DESC.
// SEM paginacao — o contrato do legado (e do front) e a lista inteira.
type conversationCursor struct {
	LastMessageAt time.Time
	ID            string
}

func encodeConversationCursor(lastMessageAt time.Time, id string) string {
	raw := lastMessageAt.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeConversationCursor(raw string) (conversationCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return conversationCursor{}, ErrInvalidBody
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 || !omnichannelUUIDPattern.MatchString(parts[1]) {
		return conversationCursor{}, ErrInvalidBody
	}
	lastMessageAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return conversationCursor{}, ErrInvalidBody
	}
	return conversationCursor{LastMessageAt: lastMessageAt, ID: parts[1]}, nil
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

func (s *Store) ListConversations(ctx context.Context, accountID string, f ConversationFilter) ([]conversationRow, error) {
	query := `select ` + conversationCols + s.lastMessageCol() + `
		from messaging.conversations c
		left join messaging.whatsapp_instances i
			on i.account_id = c.account_id and c.channel='WHATSAPP'
			and (i.id = c.instance_id or (c.instance_id is null and i.instance_name=c.instance_scope_key))
		where c.account_id = $1::uuid` + visibleConversationFilter +
		s.historyVisibleConversationPredicate("c")
	args := []any{accountID}
	if f.Visibility != nil {
		query, args = appendConversationVisibility(query, args, "c", *f.Visibility)
	}

	if strings.TrimSpace(f.InstanceID) != "" {
		args = append(args, strings.TrimSpace(f.InstanceID))
		position := strconv.Itoa(len(args))
		query += ` and (c.instance_id = $` + position + `::uuid or
			(c.channel='WHATSAPP' and c.instance_id is null and exists (
				select 1 from messaging.whatsapp_instances filter_instance
				where filter_instance.account_id=c.account_id
				  and filter_instance.id=$` + position + `::uuid
				  and filter_instance.instance_name=c.instance_scope_key)))`
	}
	if f.Channel != "" {
		args = append(args, f.Channel)
		query += " and c.channel = $" + strconv.Itoa(len(args))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		query += ` and case
			when c.state = 'closed' then 'CLOSED'
			when c.state = 'pending' then 'PENDING'
			else 'OPEN' end = $` + strconv.Itoa(len(args))
	}
	if f.QueueID != "" {
		args = append(args, f.QueueID)
		query += " and c.queue_id = $" + strconv.Itoa(len(args)) + "::uuid"
	}
	if f.ResponsibleID != "" {
		args = append(args, f.ResponsibleID)
		query += " and coalesce(c.assigned_user_id::text, c.assigned_to_id) = $" + strconv.Itoa(len(args))
	}
	if f.Search != "" {
		args = append(args, "%"+escapeLike(strings.ToLower(f.Search))+"%")
		position := strconv.Itoa(len(args))
		query += ` and (lower(coalesce(c.contact_name, '')) like $` + position + ` escape '\'
			or lower(coalesce(c.contact_phone, '')) like $` + position + ` escape '\'
			or lower(c.external_id) like $` + position + ` escape '\'
			or exists (select 1 from messaging.messages sm
				where sm.account_id = c.account_id and sm.conversation_id = c.id` +
			s.historyVisibleMessagePredicate("sm", "c") + `
				  and lower(sm.content) like $` + position + ` escape '\'))`
	}
	if f.BeforeCursor != "" {
		cursor, err := decodeConversationCursor(f.BeforeCursor)
		if err != nil {
			return nil, err
		}
		args = append(args, cursor.LastMessageAt, cursor.ID)
		query += " and (c.last_message_at, c.id) < ($" + strconv.Itoa(len(args)-1) + ", $" + strconv.Itoa(len(args)) + "::uuid)"
	}
	args = append(args, f.Limit)
	query += " order by c.last_message_at desc, c.id desc limit $" + strconv.Itoa(len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]conversationRow, 0)
	for rows.Next() {
		c, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) UpdateConversationGroupName(ctx context.Context, accountID, instanceName, externalID, name string) error {
	_, err := s.pool.Exec(ctx, `update messaging.conversations
		set contact_name=$4, updated_at=now()
		where account_id=$1::uuid and instance_scope_key=$2 and external_id=$3
		  and channel='WHATSAPP' and lower(trim(external_id)) like '%@g.us'`,
		accountID, strings.TrimSpace(instanceName), strings.TrimSpace(externalID), strings.TrimSpace(name))
	return err
}

// GetConversation devolve uma conversa da account. Conversa de OUTRA account cai no
// filtro e volta pgx.ErrNoRows -> o service traduz para ErrNotFound (404, nunca 403).
func (s *Store) GetConversation(ctx context.Context, accountID, id string) (conversationRow, error) {
	query := `select ` + conversationCols + s.lastMessageCol() + `
		from messaging.conversations c
		left join messaging.whatsapp_instances i
			on i.account_id = c.account_id and c.channel='WHATSAPP'
			and (i.id = c.instance_id or (c.instance_id is null and i.instance_name=c.instance_scope_key))
		where c.account_id = $1::uuid and c.id = $2::uuid` + visibleConversationFilter +
		s.historyVisibleConversationPredicate("c")
	return scanConversation(s.pool.QueryRow(ctx, query, accountID, id))
}

// GetConversationForCompose resolve somente os metadados necessarios para iniciar um novo
// outbound. Diferente do GET publico, ele nao exige mensagem pos-cutoff: isso permite reutilizar
// a conversa canonica de um contato depois de um reset. O preview e o last_message_at, porem,
// continuam estritamente derivados de mensagens visiveis; nenhum conteudo/timestamp historico e
// devolvido. Contact suppression continua sendo respeitada.
func (s *Store) GetConversationForCompose(ctx context.Context, accountID, id string) (conversationRow, error) {
	visibleLastMessageAt := `(select compose_message.created_at
		from messaging.messages compose_message
		where compose_message.account_id=c.account_id
		  and compose_message.conversation_id=c.id` +
		s.historyVisibleMessagePredicate("compose_message", "c") + `
		order by compose_message.created_at desc, compose_message.id desc limit 1)`
	cols := `c.id::text, c.instance_id::text, c.instance_scope_key,
		i.instance_name, i.display_name, c.channel, c.state, c.external_id, c.contact_id::text,
		c.contact_name, c.contact_avatar_url, c.contact_phone, c.assigned_to_id,
		c.created_at, c.updated_at, coalesce(` + visibleLastMessageAt + `, c.created_at)`
	query := `select ` + cols + s.lastMessageCol() + `
		from messaging.conversations c
		left join messaging.whatsapp_instances i
			on i.account_id=c.account_id and c.channel='WHATSAPP'
			and (i.id=c.instance_id or (c.instance_id is null and i.instance_name=c.instance_scope_key))
		where c.account_id=$1::uuid and c.id=$2::uuid` + visibleConversationFilter
	row, err := scanConversation(s.pool.QueryRow(ctx, query, accountID, id))
	if err != nil {
		return conversationRow{}, err
	}
	if len(row.LastMessage) == 0 || string(row.LastMessage) == "null" {
		// O compose e uma projecao efemera, nao uma entrada no inbox. Sem mensagem visivel,
		// timestamps persistidos tambem pertencem ao historico oculto e nao podem vazar.
		now := time.Now().UTC()
		row.CreatedAt = now
		row.UpdatedAt = now
		row.LastMessageAt = now
	}
	return row, nil
}

// ============================================================================
// Mensagens
// ============================================================================

// messageColsNoReply e usado somente em criacoes internas que nunca possuem reply. Ele e
// fail-closed: remove qualquer snapshot e projeta os campos de reply como NULL.
const messageColsNoReply = `m.id::text, m.account_id::text, m.conversation_id::text,
	m.sender_user_id::text, m.direction, m.message_type, m.sender_name, m.sender_avatar_url,
	m.content, m.media_url, m.media_mime_type, m.media_file_name, m.media_file_size_bytes,
	m.media_caption, m.media_duration_seconds, coalesce(m.metadata_json,'{}'::jsonb)-'replySnapshot', m.status,
	m.origin, null::text, null::text, null::text, null::text, null::text,
	m.provider_status_at, m.provider_error_code,
	case when m.message_type = 'TEXT' then ''
	  when m.media_storage_key is not null and m.media_source_kind = 'disk' then 'ready'
	  else coalesce(m.metadata_json->>'mediaState', 'pending') end,
	m.external_message_id, m.created_at, m.updated_at`

func (s *Store) visibleReplyExistsExpression(messageAlias, conversationAlias string) string {
	return `exists (select 1 from messaging.messages reply_visible
		where reply_visible.id=` + messageAlias + `.reply_to_message_id
		  and reply_visible.account_id=` + messageAlias + `.account_id
		  and reply_visible.conversation_id=` + messageAlias + `.conversation_id` +
		s.historyVisibleMessagePredicate("reply_visible", conversationAlias) + `)`
}

func (s *Store) replyValueExpression(messageAlias, conversationAlias, column string) string {
	return `(select reply_value.` + column + ` from messaging.messages reply_value
		where reply_value.id=` + messageAlias + `.reply_to_message_id
		  and reply_value.account_id=` + messageAlias + `.account_id
		  and reply_value.conversation_id=` + messageAlias + `.conversation_id` +
		s.historyVisibleMessagePredicate("reply_value", conversationAlias) + `)`
}

func (s *Store) sanitizedMessageMetadataExpression(messageAlias, conversationAlias string) string {
	return `case when coalesce(` + messageAlias + `.metadata_json,'{}'::jsonb) ? 'replySnapshot'
		and not (` + s.visibleReplyExistsExpression(messageAlias, conversationAlias) + `)
		then coalesce(` + messageAlias + `.metadata_json,'{}'::jsonb)-'replySnapshot'
		else ` + messageAlias + `.metadata_json end`
}

func (s *Store) messageCols() string {
	visibleReply := s.visibleReplyExistsExpression("m", "c")
	return `m.id::text, m.account_id::text, m.conversation_id::text,
	m.sender_user_id::text, m.direction, m.message_type, m.sender_name, m.sender_avatar_url,
	m.content, m.media_url, m.media_mime_type, m.media_file_name, m.media_file_size_bytes,
	m.media_caption, m.media_duration_seconds, ` + s.sanitizedMessageMetadataExpression("m", "c") + `, m.status,
	m.origin, case when ` + visibleReply + ` then m.reply_to_message_id::text end,
	case when ` + visibleReply + ` then m.reply_to_external_message_id end,
	` + s.replyValueExpression("m", "c", "sender_name") + `,
	` + s.replyValueExpression("m", "c", "content") + `,
	` + s.replyValueExpression("m", "c", "message_type") + `,
	m.provider_status_at, m.provider_error_code,
	case when m.message_type = 'TEXT' then ''
	  when m.media_storage_key is not null and m.media_source_kind = 'disk' then 'ready'
	  else coalesce(m.metadata_json->>'mediaState', 'pending') end,
	m.external_message_id, m.created_at, m.updated_at`
}

func scanMessage(row rowScanner) (MessageView, error) {
	var m MessageView
	var replyMessageID, replyExternalID, replySender, replyContent, replyType *string
	err := row.Scan(&m.ID, &m.TenantID, &m.ConversationID, &m.SenderUserID, &m.Direction,
		&m.MessageType, &m.SenderName, &m.SenderAvatarURL, &m.Content, &m.MediaURL,
		&m.MediaMimeType, &m.MediaFileName, &m.MediaFileSizeBytes, &m.MediaCaption,
		&m.MediaDurationSeconds, &m.MetadataJSON, &m.Status, &m.Origin, &replyMessageID,
		&replyExternalID, &replySender, &replyContent, &replyType,
		&m.ProviderStatusAt, &m.ProviderErrorCode, &m.MediaState, &m.ExternalMessageID,
		&m.CreatedAt, &m.UpdatedAt)
	if err == nil && (replyMessageID != nil || replyExternalID != nil) {
		m.ReplyTo = &ReplyToView{
			MessageID:         replyMessageID,
			ExternalMessageID: deref(replyExternalID),
			SenderName:        deref(replySender),
			Content:           deref(replyContent),
			MessageType:       deref(replyType),
		}
		fillReplySnapshot(&m)
	}
	m.CanRetryMedia = m.MediaState == "failed"
	return m, err
}

func fillReplySnapshot(m *MessageView) {
	if m == nil || m.ReplyTo == nil || len(m.MetadataJSON) == 0 {
		return
	}
	var metadata struct {
		ReplySnapshot struct {
			Content     string `json:"content"`
			MessageType string `json:"messageType"`
			Participant string `json:"participant"`
		} `json:"replySnapshot"`
	}
	if json.Unmarshal(m.MetadataJSON, &metadata) != nil {
		return
	}
	if m.ReplyTo.Content == "" {
		m.ReplyTo.Content = metadata.ReplySnapshot.Content
	}
	if m.ReplyTo.MessageType == "" {
		m.ReplyTo.MessageType = metadata.ReplySnapshot.MessageType
	}
	if m.ReplyTo.SenderName == "" {
		m.ReplyTo.SenderName = metadata.ReplySnapshot.Participant
	}
}

// hiddenFilter exclui as mensagens que o usuario apagou "para mim"
// (messaging.hidden_messages). O legado aplica isso na listagem do historico.
const hiddenFilter = ` and not exists (
	select 1 from messaging.hidden_messages h
	where h.message_id = m.id and h.user_id = $3::uuid)`

type messageCursor struct {
	CreatedAt time.Time
	ID        string
}

func encodeMessageCursor(createdAt time.Time, id string) string {
	raw := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeMessageCursor(raw string) (messageCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return messageCursor{}, ErrInvalidBody
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return messageCursor{}, ErrInvalidBody
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return messageCursor{}, ErrInvalidBody
	}
	return messageCursor{CreatedAt: createdAt, ID: parts[1]}, nil
}

// resolveBeforeMessage traduz o beforeId legado para o cursor estavel (created_at,id).
// O id permanece preso a account+conversa, logo um id de outro tenant nunca e cursor valido.
func (s *Store) resolveBeforeMessage(ctx context.Context, accountID, conversationID, beforeID string) (*messageCursor, error) {
	var cursor messageCursor
	err := s.pool.QueryRow(ctx, `select m.created_at, m.id::text from messaging.messages m
		join messaging.conversations c on c.account_id=m.account_id and c.id=m.conversation_id
		where m.account_id = $1::uuid and m.conversation_id = $2::uuid and m.id = $3::uuid`+
		s.historyVisibleMessagePredicate("m", "c"),
		accountID, conversationID, beforeID).Scan(&cursor.CreatedAt, &cursor.ID)
	switch {
	// beforeId inexistente (ou de outra conta/conversa) => sem filtro de data: a pagina
	// volta como se nao houvesse cursor. E o comportamento do legado (beforeMessage null).
	case errors.Is(err, pgx.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, err
	default:
		return &cursor, nil
	}
}

// ListMessages devolve a pagina do historico em ordem ASC (a mais antiga primeiro).
//
// O algoritmo e o do legado, replicado EXATO (divergir quebra o scroll infinito):
// resolve beforeId -> created_at, filtra created_at <, ordena DESC, take limit e
// INVERTE o array. Ver routes-message-read-list.ts:97-170.
func (s *Store) ListMessages(ctx context.Context, accountID, userID, conversationID string, f MessagePageFilter) ([]MessageView, error) {
	query := `select ` + s.messageCols() + ` from messaging.messages m
		join messaging.conversations c on c.account_id=m.account_id and c.id=m.conversation_id
		where m.account_id = $1::uuid and m.conversation_id = $2::uuid` + hiddenFilter +
		s.historyVisibleMessagePredicate("m", "c")
	args := []any{accountID, conversationID, userID}

	var before *messageCursor
	if strings.TrimSpace(f.BeforeCursor) != "" {
		decoded, err := decodeMessageCursor(f.BeforeCursor)
		if err != nil {
			return nil, err
		}
		before = &decoded
	} else if strings.TrimSpace(f.BeforeID) != "" {
		resolved, err := s.resolveBeforeMessage(ctx, accountID, conversationID, f.BeforeID)
		if err != nil {
			return nil, err
		}
		before = resolved
	}
	if before != nil {
		args = append(args, before.CreatedAt, before.ID)
		query += " and (m.created_at, m.id) < ($" + strconv.Itoa(len(args)-1) + ", $" + strconv.Itoa(len(args)) + "::uuid)"
	}
	args = append(args, f.Limit)
	query += " order by m.created_at desc, m.id desc limit $" + strconv.Itoa(len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	desc := make([]MessageView, 0, f.Limit)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		desc = append(desc, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Inverte para ASC: o front espera a mais antiga primeiro.
	out := make([]MessageView, 0, len(desc))
	for i := len(desc) - 1; i >= 0; i-- {
		out = append(out, desc[i])
	}
	return out, nil
}

// HasOlderMessage responde o `hasMore`: existe mensagem MAIS ANTIGA que a primeira da
// pagina? Nao conta as escondidas do usuario — senao o front pediria uma pagina que
// voltaria vazia e o scroll infinito travaria.
func (s *Store) HasOlderMessage(ctx context.Context, accountID, userID, conversationID string, oldest time.Time, oldestID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `select exists (
		select 1 from messaging.messages m
		join messaging.conversations c on c.account_id=m.account_id and c.id=m.conversation_id
		where m.account_id = $1::uuid and m.conversation_id = $2::uuid
		  and (m.created_at, m.id) < ($4, $5::uuid)
		  and not exists (select 1 from messaging.hidden_messages h
			where h.message_id = m.id and h.user_id = $3::uuid)`+
		s.historyVisibleMessagePredicate("m", "c")+`)`,
		accountID, conversationID, userID, oldest, oldestID).Scan(&exists)
	return exists, err
}

// GetMessage devolve uma mensagem da conversa. Filtra por account E conversa: id de
// outra conta (ou de outra conversa) volta pgx.ErrNoRows -> 404.
func (s *Store) GetMessage(ctx context.Context, accountID, userID, conversationID, messageID string) (MessageView, error) {
	query := `select ` + s.messageCols() + ` from messaging.messages m
		join messaging.conversations c on c.account_id=m.account_id and c.id=m.conversation_id
		where m.account_id = $1::uuid and m.conversation_id = $2::uuid` + hiddenFilter +
		s.historyVisibleMessagePredicate("m", "c") +
		` and m.id = $4::uuid`
	return scanMessage(s.pool.QueryRow(ctx, query, accountID, conversationID, userID, messageID))
}
