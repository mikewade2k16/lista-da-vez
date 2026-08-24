package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// Persistencia do chat com memoria (WAVE 4, contrato D1). Conversas e mensagens no
// schema calendar.chat_* (migration 0191). TUDO account-scoped (WHERE account_id):
// conta A nunca le/escreve conversa ou mensagem de B (defesa em profundidade, mesmo
// alem do gate de membership do RequireAuthWithAccount). Espelha o padrao account-
// scoped + soft-delete + order by created_at do tasks.repository_postgres_collab.

// ChatConversation e uma conversa do chat do calendario (calendar.chat_conversations).
// ScopeClientID e ponteiro (nullable): preenchido so quando ScopeMode == chatScopeClient.
// CreatedByName so e preenchido pelo ListConversations (join em core.users); vazio nas
// demais leituras.
type ChatConversation struct {
	ID              string
	AccountID       string
	CreatedByUserID string
	CreatedByName   string
	Title           string
	EntrySurface    string  // calendar | meta_ads | global; origem, nao autorizacao
	ScopeMode       string  // 'client' | 'all'
	ScopeClientID   *string // null quando scope_mode='all'
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ChatConversationInput sao os campos de criacao de uma conversa (ja normalizados
// pelo service via validateScope). ScopeClientID vazio = coluna null.
type ChatConversationInput struct {
	Title         string
	EntrySurface  string
	ScopeMode     string
	ScopeClientID string
}

// ChatMessage e uma mensagem persistida de uma conversa (calendar.chat_messages).
// Role e 'user' | 'assistant'. A memoria do LLM (contrato D4) le as ultimas N destas.
type ChatMessage struct {
	ID             string
	ConversationID string
	Role           string
	Content        string
	Proposal       *ChatProposal
	ProposalStatus string
	// Proposals (WAVE 5.1, multi-tarefa) = lista de propostas com status proprio por item.
	// Fonte da verdade a partir da 0195; Proposal/ProposalStatus (singular) so p/ retrocompat.
	Proposals      []StoredProposal
	CalendarItems  []AIContextEvent
	Resources      []AssistantResource
	ContextModules []string
	CreatedAt      time.Time
}

type ChatMessageInput struct {
	// ID e opcional. O Assistente 360 o pregera quando precisa criar uma
	// proposta Meta idempotente antes de persistir o card; vazio usa UUID do banco.
	ID             string
	Role           string
	Content        string
	Proposal       *ChatProposal
	ProposalStatus string
	Proposals      []StoredProposal
	CalendarItems  []AIContextEvent
	Resources      []AssistantResource
	ContextModules []string
}

// chatConversationStore e a fatia de persistencia do chat com memoria consumida pelo
// Service. Embutida em calendarStore (service.go). IsAgencyOfAccount resolve a
// visibilidade org-aware (espelho de auth.account_checker / core.store_postgres) para
// o resolveChatAccess (chat_access.go) — sem confiar em nada do body.
type chatConversationStore interface {
	CreateConversation(ctx context.Context, accountID, createdByUserID string, in ChatConversationInput) (ChatConversation, error)
	GetConversation(ctx context.Context, id, accountID string) (ChatConversation, error)
	ListConversations(ctx context.Context, accountID, requesterUserID string, isAgency bool) ([]ChatConversation, error)
	SoftDeleteConversation(ctx context.Context, id, accountID string) error
	// TouchConversation sobe updated_at e titula pela 1a pergunta (so se sem titulo);
	// AppendMessage NAO move updated_at, por isso o bump explicito (contrato D4).
	TouchConversation(ctx context.Context, accountID, conversationID, titleIfEmpty string) error
	AppendMessage(ctx context.Context, accountID, conversationID string, in ChatMessageInput) (ChatMessage, error)
	GetMessage(ctx context.Context, accountID, conversationID, messageID string) (ChatMessage, error)
	SetProposalStatus(ctx context.Context, accountID, conversationID, messageID, proposalID, status string) (ChatMessage, error)
	ListLastMessages(ctx context.Context, accountID, conversationID string, limit int) ([]ChatMessage, error)
	ListMessages(ctx context.Context, accountID, conversationID string) ([]ChatMessage, error)
	IsAgencyOfAccount(ctx context.Context, accountID, userID string) (bool, error)
}

// chatConversationCols e a lista base de colunas da conversa na ordem de scanChatConversation.
// scope_client_id::text sai como *string (nullable). account/created_by como text (::text).
const chatConversationCols = `id::text, account_id::text, created_by_user_id::text, title,
	entry_surface, scope_mode, scope_client_id::text, created_at, updated_at`

func scanChatConversation(row rowScanner) (ChatConversation, error) {
	var c ChatConversation
	err := row.Scan(&c.ID, &c.AccountID, &c.CreatedByUserID, &c.Title,
		&c.EntrySurface, &c.ScopeMode, &c.ScopeClientID, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func scanChatMessage(row rowScanner) (ChatMessage, error) {
	var m ChatMessage
	var proposalRaw, proposalsRaw, itemsRaw, resourcesRaw, contextModulesRaw json.RawMessage
	err := row.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &proposalRaw,
		&m.ProposalStatus, &proposalsRaw, &itemsRaw, &resourcesRaw, &contextModulesRaw, &m.CreatedAt)
	if err == nil {
		if len(proposalRaw) > 0 && string(proposalRaw) != "null" {
			var proposal ChatProposal
			if json.Unmarshal(proposalRaw, &proposal) == nil {
				m.Proposal = &proposal
			}
		}
		_ = json.Unmarshal(proposalsRaw, &m.Proposals)
		if m.Proposals == nil {
			m.Proposals = []StoredProposal{}
		}
		// Retrocompat: mensagem antiga sem `proposals` mas com `proposal` singular vira lista de
		// 1 (id '0'), preservando o status. O backfill da 0195 ja cobre as existentes; isto e a
		// rede de seguranca para qualquer linha nao migrada.
		if len(m.Proposals) == 0 && m.Proposal != nil {
			status := m.ProposalStatus
			if status == "" || status == "none" {
				status = "pending"
			}
			m.Proposals = []StoredProposal{{
				ID: "0", Action: "create", Kind: m.Proposal.Kind, Fields: m.Proposal.Fields, Status: status,
			}}
		}
		_ = json.Unmarshal(itemsRaw, &m.CalendarItems)
		if m.CalendarItems == nil {
			m.CalendarItems = []AIContextEvent{}
		}
		_ = json.Unmarshal(resourcesRaw, &m.Resources)
		m.Resources = sanitizeAssistantResources(m.Resources)
		_ = json.Unmarshal(contextModulesRaw, &m.ContextModules)
		m.ContextModules = sanitizeAssistantContextModules(m.ContextModules)
	}
	return m, err
}

const chatMessageCols = `id::text, conversation_id::text, role, content,
	proposal, proposal_status, proposals, calendar_items, resources, context_modules, created_at`

// CreateConversation insere uma conversa na account. Escopo por account_id + o dono
// (created_by_user_id) vem SEMPRE do Principal, nunca do body.
func (s *Store) CreateConversation(ctx context.Context, accountID, createdByUserID string, in ChatConversationInput) (ChatConversation, error) {
	const q = `
		insert into calendar.chat_conversations
			(account_id, created_by_user_id, title, entry_surface, scope_mode, scope_client_id)
		values ($1::uuid, $2::uuid, $3, $4, $5, $6::uuid)
		returning ` + chatConversationCols
	return scanChatConversation(s.pool.QueryRow(ctx, q,
		accountID, createdByUserID, in.Title, in.EntrySurface, in.ScopeMode, nullUUID(in.ScopeClientID)))
}

// GetConversation le uma conversa VIVA no escopo da account. Fora do escopo (outra
// conta) ou apagada => pgx.ErrNoRows (o service colapsa em ErrNotFound/404, sem vazar
// existencia). A checagem de dono-ou-agencia fica no service (authorizeConversation).
func (s *Store) GetConversation(ctx context.Context, id, accountID string) (ChatConversation, error) {
	const q = `select ` + chatConversationCols + `
		from calendar.chat_conversations
		where id = $1::uuid and account_id = $2::uuid and deleted_at is null`
	return scanChatConversation(s.pool.QueryRow(ctx, q, id, accountID))
}

// ListConversations lista as conversas vivas da account. isAgency=true => TODAS da
// conta (com o nome do autor, via left join core.users); isAgency=false (cliente-side)
// => so as created_by = requesterUserID. Ordem: mais recentes primeiro (updated_at desc).
func (s *Store) ListConversations(ctx context.Context, accountID, requesterUserID string, isAgency bool) ([]ChatConversation, error) {
	q := `select c.id::text, c.account_id::text, c.created_by_user_id::text, c.title,
			c.entry_surface, c.scope_mode, c.scope_client_id::text, c.created_at, c.updated_at,
			coalesce(nullif(trim(u.display_name), ''), u.email, '')
		from calendar.chat_conversations c
		left join core.users u on u.id = c.created_by_user_id
		where c.account_id = $1::uuid and c.deleted_at is null`
	args := []any{accountID}
	if !isAgency {
		args = append(args, requesterUserID)
		q += " and c.created_by_user_id = $" + strconv.Itoa(len(args)) + "::uuid"
	}
	q += " order by c.updated_at desc, c.created_at desc"

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ChatConversation, 0)
	for rows.Next() {
		var c ChatConversation
		if err := rows.Scan(&c.ID, &c.AccountID, &c.CreatedByUserID, &c.Title,
			&c.EntrySurface, &c.ScopeMode, &c.ScopeClientID, &c.CreatedAt, &c.UpdatedAt, &c.CreatedByName); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// TouchConversation sobe o updated_at da conversa (para ela subir no list) e a titula
// pela 1a pergunta SO quando ela ainda esta sem titulo (title vazio), no escopo da
// account. Chamado apos gravar a resposta da IA (contrato D4). AppendMessage NAO move o
// updated_at da conversa; por isso este bump explicito. Conversa de outra conta / apagada
// => nenhuma linha (ErrNotFound). titleIfEmpty ja vem trim/truncado do service.
func (s *Store) TouchConversation(ctx context.Context, accountID, conversationID, titleIfEmpty string) error {
	const q = `update calendar.chat_conversations
		set title = case when title = '' then $3 else title end,
			updated_at = now()
		where id = $1::uuid and account_id = $2::uuid and deleted_at is null`
	tag, err := s.pool.Exec(ctx, q, conversationID, accountID, titleIfEmpty)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SoftDeleteConversation marca a conversa como apagada (deleted_at) no escopo da
// account. Nenhuma linha casada (outra conta / ja apagada) => pgx.ErrNoRows. A regra
// dono-ou-agencia e aplicada antes pelo service (authorizeConversation).
func (s *Store) SoftDeleteConversation(ctx context.Context, id, accountID string) error {
	const q = `update calendar.chat_conversations
		set deleted_at = now(), updated_at = now()
		where id = $1::uuid and account_id = $2::uuid and deleted_at is null`
	tag, err := s.pool.Exec(ctx, q, id, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// AppendMessage grava uma mensagem amarrando-a a uma conversa VIVA da MESMA account
// (insert ... select ... where account_id + deleted_at is null): conversa de outra
// conta ou apagada => nenhuma linha (ErrNotFound). Espelha o AddComment do tasks.
func (s *Store) AppendMessage(ctx context.Context, accountID, conversationID string, in ChatMessageInput) (ChatMessage, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ChatMessage{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	proposal, err := json.Marshal(in.Proposal)
	if err != nil {
		return ChatMessage{}, err
	}
	proposals := in.Proposals
	if proposals == nil {
		proposals = []StoredProposal{}
	}
	proposalsRaw, err := json.Marshal(proposals)
	if err != nil {
		return ChatMessage{}, err
	}
	items, err := json.Marshal(in.CalendarItems)
	if err != nil {
		return ChatMessage{}, err
	}
	resourcesRaw, err := json.Marshal(sanitizeAssistantResources(in.Resources))
	if err != nil {
		return ChatMessage{}, err
	}
	contextModulesRaw, err := json.Marshal(sanitizeAssistantContextModules(in.ContextModules))
	if err != nil {
		return ChatMessage{}, err
	}
	status := in.ProposalStatus
	if status == "" {
		status = "none"
	}
	const q = `
		insert into calendar.chat_messages
			(id, conversation_id, account_id, role, content, proposal, proposal_status, proposals, calendar_items, resources, context_modules)
		select coalesce(nullif($3, '')::uuid, gen_random_uuid()), c.id, c.account_id,
			$4, $5, $6::jsonb, $7, $8::jsonb, $9::jsonb, $10::jsonb, $11::jsonb
		from calendar.chat_conversations c
		where c.id = $1::uuid and c.account_id = $2::uuid and c.deleted_at is null
		returning ` + chatMessageCols
	msg, err := scanChatMessage(tx.QueryRow(ctx, q, conversationID, accountID,
		in.ID, in.Role, in.Content, proposal, status, proposalsRaw, items, resourcesRaw, contextModulesRaw))
	if errors.Is(err, pgx.ErrNoRows) {
		return ChatMessage{}, ErrNotFound
	}
	if err != nil {
		return ChatMessage{}, err
	}
	if err := seedChatProposalExecutions(ctx, tx, accountID, conversationID, msg); err != nil {
		return ChatMessage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ChatMessage{}, err
	}
	return msg, nil
}

func (s *Store) GetMessage(
	ctx context.Context,
	accountID, conversationID, messageID string,
) (ChatMessage, error) {
	const q = `select ` + chatMessageCols + `
		from calendar.chat_messages
		where id = $1::uuid and account_id = $2::uuid and conversation_id = $3::uuid`
	msg, err := scanChatMessage(s.pool.QueryRow(ctx, q, messageID, accountID, conversationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ChatMessage{}, ErrNotFound
	}
	return msg, err
}

// SetProposalStatus resolve UMA proposta (por id, dentro do array `proposals`) de forma
// idempotente: so muda o elemento cujo status ainda e 'pending', preservando a ordem
// (with ordinality). Nenhum elemento pending com esse id (ja resolvido / inexistente /
// conta errada) => pgx.ErrNoRows => ErrNotFound. account + conversation barram acesso cruzado.
func (s *Store) SetProposalStatus(ctx context.Context, accountID, conversationID, messageID, proposalID, status string) (ChatMessage, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ChatMessage{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const q = `update calendar.chat_messages m
		set proposals = coalesce((
			select jsonb_agg(
				case when elem->>'id' = $4 then jsonb_set(elem, '{status}', to_jsonb($5::text)) else elem end
				order by ord
			)
			from jsonb_array_elements(m.proposals) with ordinality as t(elem, ord)
		), '[]'::jsonb)
		where m.id = $1::uuid and m.account_id = $2::uuid and m.conversation_id = $3::uuid
		  and exists (
			select 1 from jsonb_array_elements(m.proposals) e
			where e->>'id' = $4 and e->>'status' = 'pending'
		  )
		returning ` + chatMessageCols
	msg, err := scanChatMessage(tx.QueryRow(ctx, q, messageID, accountID, conversationID, proposalID, status))
	if errors.Is(err, pgx.ErrNoRows) {
		return ChatMessage{}, ErrNotFound
	}
	if err != nil {
		return ChatMessage{}, err
	}
	if status == "rejected" {
		const rejectExecution = `update calendar.chat_proposal_executions
			set status = 'rejected', rejected_at = now(), completed_at = now(), updated_at = now()
			where account_id = $1::uuid and conversation_id = $2::uuid
			  and message_id = $3::uuid and proposal_id = $4 and status = 'pending'`
		if _, err := tx.Exec(ctx, rejectExecution, accountID, conversationID, messageID, proposalID); err != nil {
			return ChatMessage{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ChatMessage{}, err
	}
	return msg, nil
}

// ListLastMessages devolve as ULTIMAS `limit` mensagens da conversa (memoria do LLM,
// contrato D4) em ordem CRONOLOGICA (asc) para virar o historico. Escopo por account_id
// (defesa em profundidade). limit <= 0 = sem teto.
func (s *Store) ListLastMessages(ctx context.Context, accountID, conversationID string, limit int) ([]ChatMessage, error) {
	inner := `select m.id, m.conversation_id, m.role, m.content, m.proposal,
		m.proposal_status, m.proposals, m.calendar_items, m.resources, m.context_modules, m.created_at
		from calendar.chat_messages m
		where m.account_id = $1::uuid and m.conversation_id = $2::uuid
		order by m.created_at desc, m.id desc`
	args := []any{accountID, conversationID}
	if limit > 0 {
		args = append(args, limit)
		inner += " limit $" + strconv.Itoa(len(args))
	}
	q := `select id::text, conversation_id::text, role, content, proposal,
		proposal_status, proposals, calendar_items, resources, context_modules, created_at
		from (` + inner + `) recent
		order by recent.created_at asc, recent.id asc`
	return s.queryChatMessages(ctx, q, args...)
}

// ListMessages devolve TODAS as mensagens da conversa em ordem cronologica (asc),
// no escopo da account. Usado pelo GET da conversa (contrato D3).
func (s *Store) ListMessages(ctx context.Context, accountID, conversationID string) ([]ChatMessage, error) {
	const q = `select ` + chatMessageCols + `
		from calendar.chat_messages
		where account_id = $1::uuid and conversation_id = $2::uuid
		order by created_at asc, id asc`
	return s.queryChatMessages(ctx, q, accountID, conversationID)
}

// queryChatMessages executa uma query de mensagens e escaneia a lista (helper comum
// de ListLastMessages/ListMessages).
func (s *Store) queryChatMessages(ctx context.Context, q string, args ...any) ([]ChatMessage, error) {
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ChatMessage, 0)
	for rows.Next() {
		m, err := scanChatMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// IsAgencyOfAccount resolve, no banco, se o usuario e "agencia" para a account: e
// platform_admin (core.users.is_platform_admin) OU agency_owner em core.organization_users
// da org da account. Espelha auth/account_checker.go e core/store_postgres.go (fonte da
// verdade da visibilidade org-aware). SQL 100% parametrizado ($1=accountID, $2=userID).
func (s *Store) IsAgencyOfAccount(ctx context.Context, accountID, userID string) (bool, error) {
	const q = `
		select
			exists (
				select 1 from core.users u
				where u.id = $2::uuid and u.is_active = true and u.is_platform_admin = true
			)
			or exists (
				select 1
				from core.accounts a
				join core.organization_users ou on ou.organization_id = a.organization_id
				where a.id = $1::uuid
				  and ou.user_id = $2::uuid
				  and ou.org_role = 'agency_owner'
			)`
	var isAgency bool
	err := s.pool.QueryRow(ctx, q, accountID, userID).Scan(&isAgency)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return isAgency, err
}
