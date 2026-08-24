package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// Conversas persistidas do chat com memoria (WAVE 4, contratos D3/D4). Camada service das
// rotas /v1/calendar/chat/conversations* e /chat/scope + os helpers do ask (alvo/escopo/
// titulo/contexto). Acesso SEMPRE resolvido server-side pela permissao (resolveChatAccess/
// IsAgencyOfAccount + authorizeConversation): conversa/cliente fora do visivel => 404.

// ChatAskResult e a resposta do ChatAsk (contrato D4): a resposta da IA + a conversa
// (id + titulo) para o front sincronizar o estado da conversa persistida.
type ChatAskResult struct {
	Answer         string
	ConversationID string
	Title          string
	Surface        string
	Capabilities   []AssistantCapability
	// Message carrega as propostas (multi-tarefa, WAVE 5.1) no campo Proposals: o front le
	// dali. A IA propoe; o usuario confirma cada uma e cria pela API autenticada.
	Message ChatMessageView
	// AIError (WAVE 5) = a IA nao respondeu (503/cota/chave/vazio). O front mostra o estado
	// "IA fora do ar" (visual distinto) em vez de um balao normal; a mensagem nao e persistida.
	AIError bool
}

// ChatConversationSummary e o item lean do list de conversas (contrato D3): sem as
// mensagens (o GET da conversa carrega o historico). createdByName vem do join em
// core.users (list) ou do Principal (create).
type ChatConversationSummary struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Surface         string    `json:"surface"`
	ScopeMode       string    `json:"scopeMode"`
	ScopeClientID   string    `json:"scopeClientId"`
	CreatedByUserID string    `json:"createdByUserId"`
	CreatedByName   string    `json:"createdByName"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// ChatMessageView e a projecao de uma mensagem no GET da conversa (contrato D3).
// Proposals (WAVE 5.1) e a lista autoritativa (multi-tarefa); proposal/proposalStatus
// (singular) ficam so p/ retrocompat de front antigo.
type ChatMessageView struct {
	ID             string              `json:"id"`
	Role           string              `json:"role"`
	Content        string              `json:"content"`
	Proposal       *ChatProposal       `json:"proposal,omitempty"`
	ProposalStatus string              `json:"proposalStatus"`
	Proposals      []StoredProposal    `json:"proposals"`
	CalendarItems  []AIContextEvent    `json:"calendarItems"`
	Resources      []AssistantResource `json:"resources"`
	CreatedAt      time.Time           `json:"createdAt"`
}

type UpdateChatProposalRequest struct {
	Status string `json:"status"`
}

// ChatConversationDetail e a conversa + suas mensagens em ordem cronologica (GET, D3).
type ChatConversationDetail struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	Surface       string            `json:"surface"`
	ScopeMode     string            `json:"scopeMode"`
	ScopeClientID string            `json:"scopeClientId"`
	Messages      []ChatMessageView `json:"messages"`
}

// ChatScopeClient e um cliente visivel (id+name) para o SELECT de escopo (contrato D3).
type ChatScopeClient struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ChatScopeView alimenta o SELECT de escopo do front (contrato D3): canSelect (mostra o
// select), lockedClientId (cliente travado do cliente-side, quando o select fica escondido)
// e a lista de clientes visiveis. Tudo resolvido server-side pela permissao.
type ChatScopeView struct {
	CanSelect      bool              `json:"canSelect"`
	LockedClientID string            `json:"lockedClientId"`
	Clients        []ChatScopeClient `json:"clients"`
}

// CreateChatConversationRequest e o body do POST /chat/conversations (contrato D3). O
// escopo e normalizado server-side (validateScope) — o body nunca decide sozinho.
type CreateChatConversationRequest struct {
	Surface       string `json:"surface"`
	ScopeMode     string `json:"scopeMode"`
	ScopeClientID string `json:"scopeClientId"`
	Title         string `json:"title"`
}

// chatTarget e o alvo resolvido de um ask: a conversa (zero-value quando ainda NAO
// materializada, caso novo) + o escopo ja normalizado contra o acesso atual + se ja existe.
type chatTarget struct {
	conv     ChatConversation
	mode     string
	clientID string
	surface  string
	existing bool
}

// resolveChatTarget resolve a conversa alvo + o escopo normalizado do ask (contrato D4),
// SEM materializar a conversa nova (o ChatAsk so cria depois de validar a IA, evitando
// conversa orfa). Conversa existente: valida dono-ou-agencia (senao ErrNotFound/404) e
// REVALIDA o escopo SALVO contra o acesso ATUAL (nunca da contexto de cliente que o
// usuario nao pode mais ver). Conversa nova: valida o escopo do body (scopeClientId com
// fallback no clientId legado). Nao confia no body para decidir cliente/modo.
func (s *Service) resolveChatTarget(ctx context.Context, access ChatAccess, accountID, userID string, req ChatAskRequest) (chatTarget, error) {
	if id := strings.TrimSpace(req.ConversationID); id != "" {
		conv, err := s.authorizeConversation(ctx, access, accountID, id, userID)
		if err != nil {
			return chatTarget{}, err
		}
		// Revalida o escopo SALVO contra o acesso ATUAL: se o usuario nao pode mais ver o
		// cliente/'all' da conversa, NEGA (404) — nao reescreve pro locked (o que replicaria o
		// historico de um cliente fora do acesso ao LLM). Fecha o vazamento por perda de acesso.
		if !access.canAccessSavedScope(conv.ScopeMode, ptrToStr(conv.ScopeClientID)) {
			return chatTarget{}, ErrNotFound
		}
		surface, err := immutableAssistantConversationSurface(conv.EntrySurface, req.Surface)
		if err != nil {
			return chatTarget{}, err
		}
		return chatTarget{conv: conv, mode: conv.ScopeMode, clientID: normalizeUUID(ptrToStr(conv.ScopeClientID)), surface: surface, existing: true}, nil
	}
	scopeClient := firstNonEmpty(strings.TrimSpace(req.ScopeClientID), strings.TrimSpace(req.ClientID))
	mode, clientID, err := access.validateScope(req.ScopeMode, scopeClient)
	if err != nil {
		return chatTarget{}, err
	}
	surface := AssistantSurfaceCalendar
	if strings.TrimSpace(req.Surface) != "" {
		surface, err = normalizeAssistantSurface(req.Surface)
		if err != nil {
			return chatTarget{}, err
		}
	}
	return chatTarget{mode: mode, clientID: clientID, surface: surface, existing: false}, nil
}

// buildChatContext monta o bloco context do payload conforme o escopo (contrato D4): em
// 'client' reusa BuildAIContext (agregado C9 SEM account, via chatContextFrom); em 'all'
// usa BuildAIContextAll (resumo lean de cada cliente visivel + eventos/nota do mes). O
// chat tambem recebe as tasks reais do board configurado para poder propor CRUD por ID.
// O retorno e `any` porque as duas formas tem shapes diferentes no payload.
func (s *Service) buildChatContext(ctx context.Context, accountID string, principal auth.Principal, access ChatAccess, mode, clientID, month string, policy assistantContextPolicy) (any, error) {
	names := access.clientNameByID()
	calendarAccountID := access.calendarAccountID(accountID)
	if mode == chatScopeAll {
		block := AIContextAll{
			Scope:    chatScopeAll,
			Month:    month,
			Clients:  make([]AIContextClientLean, 0, len(access.VisibleClientIDs)),
			Holidays: []Holiday{},
			Events:   []AIContextEvent{},
		}
		if policy.calendar {
			var err error
			block, err = s.BuildAIContextAll(ctx, calendarAccountID, access.VisibleClientIDs, month)
			if err != nil {
				return nil, err
			}
		} else {
			for _, visibleClientID := range capClientIDs(access.VisibleClientIDs, maxContextClients) {
				block.Clients = append(block.Clients, AIContextClientLean{ID: visibleClientID, Name: names[visibleClientID]})
			}
		}
		// Nomeia todo cliente VISIVEL (nao so os com evento/perfil): loadAccountNames deixa o
		// resto sem nome e a IA nao conseguia cita-lo ("faltou Duby/Bari"). Fonte = select de
		// escopo (permission-scoped); nenhum nome novo vaza.
		fillLeanClientNames(block.Clients, names)
		if policy.tasks {
			block.Tasks = s.chatTasksContext(ctx, accountID, principal, "", access.VisibleClientIDs)
		}
		if policy.users {
			block.People = s.chatPeopleContext(ctx, accountID)
		}
		if policy.calendar && policy.tasks {
			block.ContentOperations = s.readContentOperations(ctx, accountID, principal, access.VisibleClientIDs)
			block.MonthNotes = appendContentOperationsReadOnly(block.MonthNotes, block.ContentOperations)
		}
		return block, nil
	}
	block := calendarChatContext{
		Client:   &planClient{ID: clientID, Name: names[clientID]},
		Month:    month,
		Holidays: []Holiday{},
		Events:   []AIContextEvent{},
		Plans:    []AIContextPlan{},
	}
	if policy.calendar {
		aic, err := s.BuildAIContext(ctx, calendarAccountID, clientID, month)
		if err != nil {
			return nil, err
		}
		block = chatContextFrom(aic)
	}
	// Cliente unico em foco: se veio sem nome (sem evento/perfil), usa o nome do select.
	if block.Client != nil && strings.TrimSpace(block.Client.Name) == "" {
		if name := strings.TrimSpace(names[block.Client.ID]); name != "" {
			block.Client.Name = name
		}
	}
	if policy.tasks {
		block.Tasks = s.chatTasksContext(ctx, accountID, principal, clientID, []string{clientID})
	}
	if policy.users {
		block.People = s.chatPeopleContext(ctx, accountID)
	}
	if policy.calendar && policy.tasks {
		block.ContentOperations = s.readContentOperations(ctx, accountID, principal, []string{clientID})
		block.MonthNotes = appendContentOperationsReadOnly(block.MonthNotes, block.ContentOperations)
	}
	// Planos sao metadata da conta INTEIRA (todos os clientes); so a agencia ve. Cliente-side
	// (ou usuario subset) NAO recebe metadata (mes/status/provider) de planos de clientes que
	// nao pode ver — WAVE 4, achado da revisao.
	if !access.IsAgency {
		block.Plans = nil
	}
	return block, nil
}

// O workflow atual ja serializa monthNotes. O bloco tipado continua separado no JSON,
// mas esta copia explicitamente read-only garante que o Crow o enxergue sem lhe dar
// qualquer participacao no disparo ou na escrita dos alertas.
func appendContentOperationsReadOnly(monthNotes string, brief any) string {
	if brief == nil {
		return monthNotes
	}
	raw, err := json.Marshal(brief)
	if err != nil || len(raw) == 0 {
		return monthNotes
	}
	prefix := "[RESUMO OPERACIONAL SOMENTE LEITURA — calculado por Tasks + Calendário; não é uma anotação e o Crow não dispara alertas] "
	if strings.TrimSpace(monthNotes) == "" {
		return prefix + string(raw)
	}
	return monthNotes + "\n\n" + prefix + string(raw)
}

func (s *Service) readContentOperations(ctx context.Context, accountID string, principal auth.Principal, clientIDs []string) any {
	if s.contentOperationsProvider == nil {
		return nil
	}
	brief, err := s.contentOperationsProvider(ctx, accountID, principal, clientIDs)
	if err != nil {
		return nil
	}
	return brief
}

// deriveChatTitle deriva o titulo da conversa da 1a pergunta (contrato D4): colapsa
// espacos e trunca em maxChatTitleRunes. Vazio quando a pergunta e so espaco.
func deriveChatTitle(question string) string {
	return truncateRunes(strings.Join(strings.Fields(question), " "), maxChatTitleRunes)
}

// ptrToStr desreferencia um *string (nil => "").
func ptrToStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ListChatConversations lista as conversas visiveis ao principal (contrato D3): agencia
// ve TODAS da conta; cliente-side so as suas (created_by = ele). So precisa de IsAgency
// (nao dos clientes visiveis), entao resolve direto no store (lean).
func (s *Service) ListChatConversations(ctx context.Context, accountID string, principal auth.Principal) ([]ChatConversationSummary, error) {
	account := strings.TrimSpace(accountID)
	access, err := s.resolveChatAccess(ctx, principal, account)
	if err != nil {
		return nil, err
	}
	convs, err := s.store.ListConversations(ctx, account, principal.UserID, access.IsAgency)
	if err != nil {
		return nil, err
	}
	out := make([]ChatConversationSummary, 0, len(convs))
	surfaceAccess := make(map[string]error, 3)
	for _, c := range convs {
		if !access.canAccessSavedScope(c.ScopeMode, ptrToStr(c.ScopeClientID)) {
			continue
		}
		surface := strings.TrimSpace(c.EntrySurface)
		accessErr, checked := surfaceAccess[surface]
		if !checked {
			_, accessErr = s.resolveConversationCapabilities(ctx, account, surface, principal)
			surfaceAccess[surface] = accessErr
		}
		if accessErr != nil {
			if isAssistantConversationAccessDenied(accessErr) {
				continue
			}
			return nil, accessErr
		}
		out = append(out, summaryFrom(c, c.CreatedByName))
	}
	return out, nil
}

// GetChatConversation devolve a conversa + mensagens (contrato D3): dono OU agencia; fora
// disso => ErrNotFound/404 (nao vaza existencia de conversa alheia).
func (s *Service) GetChatConversation(ctx context.Context, accountID, id string, principal auth.Principal) (ChatConversationDetail, error) {
	account := strings.TrimSpace(accountID)
	// Acesso COMPLETO (IsAgency + VisibleClientIDs) para revalidar o escopo salvo, nao so o
	// dono/agencia (WAVE 4, achado da revisao): quem perdeu acesso ao cliente da conversa nao
	// pode reler o historico dele.
	access, err := s.resolveChatAccess(ctx, principal, account)
	if err != nil {
		return ChatConversationDetail{}, err
	}
	conv, err := s.authorizeConversation(ctx, access, account, id, principal.UserID)
	if err != nil {
		return ChatConversationDetail{}, err
	}
	if !access.canAccessSavedScope(conv.ScopeMode, ptrToStr(conv.ScopeClientID)) {
		return ChatConversationDetail{}, ErrNotFound
	}
	capabilities, err := s.resolveConversationCapabilities(ctx, account, conv.EntrySurface, principal)
	if err != nil {
		if isAssistantConversationAccessDenied(err) {
			return ChatConversationDetail{}, ErrNotFound
		}
		return ChatConversationDetail{}, err
	}
	msgs, err := s.store.ListMessages(ctx, account, conv.ID)
	if err != nil {
		return ChatConversationDetail{}, err
	}
	msgs = filterChatMessagesForCapabilities(msgs, capabilities)
	msgs, err = hydrateMetaActionProposals(ctx, account, msgs, s.metaAssistantActionStatusProvider)
	if err != nil {
		return ChatConversationDetail{}, err
	}
	return ChatConversationDetail{
		ID:            conv.ID,
		Title:         conv.Title,
		Surface:       conv.EntrySurface,
		ScopeMode:     conv.ScopeMode,
		ScopeClientID: ptrToStr(conv.ScopeClientID),
		Messages:      toMessageViews(msgs),
	}, nil
}

// CreateChatConversation cria uma conversa vazia (contrato D3): o escopo do body e
// normalizado server-side (validateScope) — cliente fora do visivel => ErrInvalidClient
// (404), 'all' so p/ quem tem select. createdByName vem do Principal (sem re-consultar).
func (s *Service) CreateChatConversation(ctx context.Context, accountID string, principal auth.Principal, req CreateChatConversationRequest) (ChatConversationSummary, error) {
	account := strings.TrimSpace(accountID)
	access, err := s.resolveChatAccess(ctx, principal, account)
	if err != nil {
		return ChatConversationSummary{}, err
	}
	mode, clientID, err := access.validateScope(req.ScopeMode, req.ScopeClientID)
	if err != nil {
		return ChatConversationSummary{}, err
	}
	surface := AssistantSurfaceCalendar
	if strings.TrimSpace(req.Surface) != "" {
		surface, err = normalizeAssistantSurface(req.Surface)
		if err != nil {
			return ChatConversationSummary{}, err
		}
	}
	if _, err = s.resolveConversationCapabilities(ctx, account, surface, principal); err != nil {
		return ChatConversationSummary{}, err
	}
	conv, err := s.store.CreateConversation(ctx, account, principal.UserID, ChatConversationInput{
		Title:         strings.TrimSpace(req.Title),
		EntrySurface:  surface,
		ScopeMode:     mode,
		ScopeClientID: clientID,
	})
	if err != nil {
		return ChatConversationSummary{}, err
	}
	return summaryFrom(conv, principalDisplayName(principal)), nil
}

// DeleteChatConversation faz soft-delete da conversa (contrato D3): dono OU agencia;
// fora disso => ErrNotFound/404 (nao vaza existencia).
func (s *Service) DeleteChatConversation(ctx context.Context, accountID, id string, principal auth.Principal) error {
	account := strings.TrimSpace(accountID)
	access, err := s.resolveChatAccess(ctx, principal, account)
	if err != nil {
		return err
	}
	conv, err := s.authorizeConversation(ctx, access, account, id, principal.UserID)
	if err != nil {
		return err
	}
	if !access.canAccessSavedScope(conv.ScopeMode, ptrToStr(conv.ScopeClientID)) {
		return ErrNotFound
	}
	if _, err = s.resolveConversationCapabilities(ctx, account, conv.EntrySurface, principal); err != nil {
		if isAssistantConversationAccessDenied(err) {
			return ErrNotFound
		}
		return err
	}
	messages, err := s.store.ListMessages(ctx, account, conv.ID)
	if err != nil {
		return err
	}
	if conversationHasPendingMetaActions(messages) {
		if s.metaAssistantConversationCancelProvider == nil {
			return ErrMetaActionNotSucceeded
		}
		if err := s.metaAssistantConversationCancelProvider(
			ctx, account, principal.UserID, conv.ID,
		); err != nil {
			return err
		}
	}
	return mapNotFound(s.store.SoftDeleteConversation(ctx, strings.TrimSpace(id), account))
}

// UpdateChatProposal registra a decisao explicita do usuario sobre UMA proposta (por
// proposalID, multi-tarefa WAVE 5.1). O cartao continua na mensagem e apenas troca de
// estado; conversa e account sao reautorizadas antes do update. Idempotente no store
// (so muda item ainda 'pending'); item ja resolvido / inexistente => ErrNotFound.
func (s *Service) UpdateChatProposal(ctx context.Context, accountID, conversationID, messageID, proposalID string, principal auth.Principal, req UpdateChatProposalRequest) (ChatMessageView, error) {
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "accepted" && status != "rejected" {
		return ChatMessageView{}, ErrInvalidProposalStatus
	}
	account := strings.TrimSpace(accountID)
	access, err := s.resolveChatAccess(ctx, principal, account)
	if err != nil {
		return ChatMessageView{}, err
	}
	conv, err := s.authorizeConversation(ctx, access, account, conversationID, principal.UserID)
	if err != nil || !access.canAccessSavedScope(conv.ScopeMode, ptrToStr(conv.ScopeClientID)) {
		return ChatMessageView{}, ErrNotFound
	}
	capabilities, err := s.resolveConversationCapabilities(ctx, account, conv.EntrySurface, principal)
	if err != nil {
		if isAssistantConversationAccessDenied(err) {
			return ChatMessageView{}, ErrNotFound
		}
		return ChatMessageView{}, err
	}
	messageID = strings.TrimSpace(messageID)
	proposalID = strings.TrimSpace(proposalID)
	current, getErr := s.store.GetMessage(ctx, account, conv.ID, messageID)
	if getErr != nil {
		return ChatMessageView{}, mapNotFound(getErr)
	}
	proposal, isMetaAction := metaActionProposalFromMessage(current, proposalID)
	if proposal == nil {
		return ChatMessageView{}, ErrNotFound
	}
	// Compatibilidade segura: o PATCH apenas registra rejeicao de cards locais.
	// Aceite local sem efeito autoritativo era o contrato legado que permitia ao
	// front mutar primeiro e apenas "pintar" o card depois. Meta conserva este
	// caminho porque o seu receipt duravel e validado logo abaixo.
	if status == "accepted" && !isMetaAction {
		return ChatMessageView{}, ErrProposalExecutionUnavailable
	}
	if isMetaAction {
		meta := proposal.Fields.MetaAction
		if meta == nil || !uuidRe.MatchString(strings.TrimSpace(meta.ActionProposalID)) {
			return ChatMessageView{}, ErrMetaActionNotSucceeded
		}
		switch status {
		case "accepted":
			if assistantCapabilityMode(capabilities, "meta_ads") != assistantModeWrite ||
				!canManageMetaActions(principal) {
				return ChatMessageView{}, ErrForbidden
			}
			if s.metaAssistantActionStatusProvider == nil {
				return ChatMessageView{}, ErrMetaActionNotSucceeded
			}
			result, statusErr := s.metaAssistantActionStatusProvider(
				ctx, account, strings.TrimSpace(meta.ActionProposalID),
			)
			if statusErr != nil {
				return ChatMessageView{}, statusErr
			}
			if normalizeMetaActionStatus(result.Status) != "succeeded" {
				return ChatMessageView{}, ErrMetaActionNotSucceeded
			}
		case "rejected":
			if proposal.Status == "rejected" {
				if s.metaAssistantActionStatusProvider == nil {
					return ChatMessageView{}, ErrMetaActionNotSucceeded
				}
				result, statusErr := s.metaAssistantActionStatusProvider(
					ctx, account, strings.TrimSpace(meta.ActionProposalID),
				)
				if statusErr != nil {
					return ChatMessageView{}, statusErr
				}
				if normalizeMetaActionStatus(result.Status) != "cancelled" {
					return ChatMessageView{}, ErrMetaActionNotSucceeded
				}
				return messageViewFrom(current), nil
			}
			if proposal.Status != "pending" || s.metaAssistantActionCancelProvider == nil {
				return ChatMessageView{}, ErrMetaActionNotSucceeded
			}
			result, cancelErr := s.metaAssistantActionCancelProvider(ctx, MetaAssistantActionLifecycleRequest{
				AccountID: account, ActorUserID: principal.UserID,
				ConversationID: conv.ID, MessageID: messageID,
				ActionProposalID: strings.TrimSpace(meta.ActionProposalID),
				IdempotencyKey:   "assistant-cancel:" + messageID + ":" + strings.TrimSpace(meta.ActionProposalID),
			})
			if cancelErr != nil {
				return ChatMessageView{}, cancelErr
			}
			if normalizeMetaActionStatus(result.Status) != "cancelled" {
				return ChatMessageView{}, ErrMetaActionNotSucceeded
			}
		}
	}
	message, err := s.store.SetProposalStatus(ctx, account, conv.ID,
		messageID, proposalID, status)
	if err != nil {
		if !errors.Is(err, ErrNotFound) && !errors.Is(err, pgx.ErrNoRows) {
			return ChatMessageView{}, err
		}
		// Replay idempotente: se a primeira resposta PATCH se perdeu, devolve a
		// mensagem ja resolvida em vez de induzir o front a repetir a mutacao.
		current, getErr := s.store.GetMessage(ctx, account, conv.ID, messageID)
		if getErr != nil {
			return ChatMessageView{}, mapNotFound(getErr)
		}
		proposal, _ := metaActionProposalFromMessage(current, proposalID)
		if proposal == nil || proposal.Status != status {
			return ChatMessageView{}, ErrNotFound
		}
		return messageViewFrom(current), nil
	}
	return messageViewFrom(message), nil
}

func conversationHasPendingMetaActions(messages []ChatMessage) bool {
	for _, message := range messages {
		for _, proposal := range message.Proposals {
			if proposal.Status == "pending" && proposal.Kind == "metaAction" &&
				proposal.Fields.MetaAction != nil &&
				uuidRe.MatchString(strings.TrimSpace(proposal.Fields.MetaAction.ActionProposalID)) {
				return true
			}
		}
	}
	return false
}

// ChatScope alimenta o SELECT de escopo do front (contrato D3): canSelect + lockedClientId
// (derivados do acesso) + a lista NOMEADA de clientes visiveis. Uma UNICA ida ao tenants
// scope via resolveChatContext (acesso + nomes juntos).
func (s *Service) ChatScope(ctx context.Context, accountID string, principal auth.Principal) (ChatScopeView, error) {
	access, clients, err := s.resolveChatContext(ctx, principal, strings.TrimSpace(accountID))
	if err != nil {
		return ChatScopeView{}, err
	}
	return ChatScopeView{
		CanSelect:      access.canSelectScope(),
		LockedClientID: access.lockedClientID(),
		Clients:        clients,
	}, nil
}

// summaryFrom projeta uma ChatConversation no item lean do list (contrato D3). name e o
// autor ja resolvido (join no list, Principal no create).
func summaryFrom(c ChatConversation, name string) ChatConversationSummary {
	return ChatConversationSummary{
		ID:              c.ID,
		Title:           c.Title,
		Surface:         c.EntrySurface,
		ScopeMode:       c.ScopeMode,
		ScopeClientID:   ptrToStr(c.ScopeClientID),
		CreatedByUserID: c.CreatedByUserID,
		CreatedByName:   name,
		UpdatedAt:       c.UpdatedAt,
	}
}

// toMessageViews projeta as mensagens persistidas na view do GET da conversa (contrato D3).
func toMessageViews(msgs []ChatMessage) []ChatMessageView {
	out := make([]ChatMessageView, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, messageViewFrom(m))
	}
	return out
}

func messageViewFrom(m ChatMessage) ChatMessageView {
	items := m.CalendarItems
	if items == nil {
		items = []AIContextEvent{}
	}
	proposals := m.Proposals
	if proposals == nil {
		proposals = []StoredProposal{}
	}
	proposals = proposalExecutionViews(proposals, items)
	resources := sanitizeAssistantResources(m.Resources)
	return ChatMessageView{ID: m.ID, Role: m.Role, Content: m.Content, Proposal: m.Proposal,
		ProposalStatus: m.ProposalStatus, Proposals: proposals, CalendarItems: items,
		Resources: resources, CreatedAt: m.CreatedAt}
}

// principalDisplayName resolve um rotulo do autor a partir do Principal (nome > email >
// userId) para o createdByName da conversa recem-criada, sem re-consultar o banco.
func principalDisplayName(principal auth.Principal) string {
	if name := strings.TrimSpace(principal.DisplayName); name != "" {
		return name
	}
	if email := strings.TrimSpace(principal.Email); email != "" {
		return email
	}
	return principal.UserID
}
