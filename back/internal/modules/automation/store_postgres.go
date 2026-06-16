package automation

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store e a persistencia do modulo (schema automation.*).
type Store struct {
	pool *pgxpool.Pool
}

// NewStore cria o Store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// GetOrCreateDefault retorna a automacao default da account (+ seu channel WAHA),
// criando ambos se ainda nao existirem. V0: 1 automacao "Tony" por account na
// sessao "default".
func (s *Store) GetOrCreateDefault(ctx context.Context, accountID string) (Automation, Channel, error) {
	a, err := s.getDefaultAutomation(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		a, err = s.createDefaultAutomation(ctx, accountID)
	}
	if err != nil {
		return Automation{}, Channel{}, err
	}

	ch, err := s.getChannel(ctx, a.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		ch, err = s.createChannel(ctx, a)
	}
	if err != nil {
		return Automation{}, Channel{}, err
	}
	return a, ch, nil
}

func (s *Store) getDefaultAutomation(ctx context.Context, accountID string) (Automation, error) {
	const q = `select id, account_id, type, name, slug, status, created_at, updated_at
		from automation.automations
		where account_id = $1 and slug = $2`
	return scanAutomation(s.pool.QueryRow(ctx, q, accountID, defaultSlug))
}

func (s *Store) createDefaultAutomation(ctx context.Context, accountID string) (Automation, error) {
	const q = `insert into automation.automations (account_id, type, name, slug, status)
		values ($1, $2, $3, $4, $5)
		returning id, account_id, type, name, slug, status, created_at, updated_at`
	return scanAutomation(s.pool.QueryRow(ctx, q, accountID, defaultType, defaultName, defaultSlug, statusDraft))
}

func (s *Store) getChannel(ctx context.Context, automationID string) (Channel, error) {
	const q = `select id, automation_id, account_id, provider, session_name, status, connected_phone, updated_at
		from automation.channels
		where automation_id = $1`
	return scanChannel(s.pool.QueryRow(ctx, q, automationID))
}

func (s *Store) createChannel(ctx context.Context, a Automation) (Channel, error) {
	const q = `insert into automation.channels (automation_id, account_id, provider, session_name, status)
		values ($1, $2, $3, $4, 'STOPPED')
		returning id, automation_id, account_id, provider, session_name, status, connected_phone, updated_at`
	// session_name = automation UUID garante unicidade global. O WAHA Core conecta
	// apenas a sessao "default"; outras contas ficam STOPPED no painel mas nao
	// causam conflito de UNIQUE constraint.
	return scanChannel(s.pool.QueryRow(ctx, q, a.ID, a.AccountID, providerWAHA, a.ID))
}

// UpdateChannelStatus persiste o estado lido da WAHA.
func (s *Store) UpdateChannelStatus(ctx context.Context, channelID, status, phone string) error {
	var phonePtr *string
	if phone != "" {
		phonePtr = &phone
	}
	const q = `update automation.channels
		set status = $2, connected_phone = $3, updated_at = now()
		where id = $1`
	_, err := s.pool.Exec(ctx, q, channelID, status, phonePtr)
	return err
}

// UpdateAutomationStatus liga/desliga o robo (active|paused) ou outro status.
func (s *Store) UpdateAutomationStatus(ctx context.Context, automationID, status string) error {
	const q = `update automation.automations set status = $2, updated_at = now() where id = $1`
	_, err := s.pool.Exec(ctx, q, automationID, status)
	return err
}

// GetChannelBySession resolve a sessao WAHA -> channel (automacao/account). Usado
// no runtime (n8n manda o nome da sessao).
func (s *Store) GetChannelBySession(ctx context.Context, session string) (Channel, error) {
	const q = `select id, automation_id, account_id, provider, session_name, status, connected_phone, updated_at
		from automation.channels
		where session_name = $1`
	return scanChannel(s.pool.QueryRow(ctx, q, session))
}

// GetAutomationByID busca a automacao (para ler o status/enabled no runtime).
func (s *Store) GetAutomationByID(ctx context.Context, id string) (Automation, error) {
	const q = `select id, account_id, type, name, slug, status, created_at, updated_at
		from automation.automations
		where id = $1`
	return scanAutomation(s.pool.QueryRow(ctx, q, id))
}

// GetActivePersona retorna a persona ativa da automacao (ou pgx.ErrNoRows).
func (s *Store) GetActivePersona(ctx context.Context, automationID string) (Persona, error) {
	const q = `select id, automation_id, account_id, name, system_prompt, is_active
		from automation.personas
		where automation_id = $1 and is_active`
	return scanPersona(s.pool.QueryRow(ctx, q, automationID))
}

// CreatePersona cria uma persona para a automacao.
func (s *Store) CreatePersona(ctx context.Context, automationID, accountID, name, systemPrompt string, active bool) (Persona, error) {
	const q = `insert into automation.personas (automation_id, account_id, name, system_prompt, is_active)
		values ($1, $2, $3, $4, $5)
		returning id, automation_id, account_id, name, system_prompt, is_active`
	return scanPersona(s.pool.QueryRow(ctx, q, automationID, accountID, name, systemPrompt, active))
}

// UpdatePersona edita nome + system_prompt da persona (editor do painel).
func (s *Store) UpdatePersona(ctx context.Context, id, name, systemPrompt string) (Persona, error) {
	const q = `update automation.personas
		set name = $2, system_prompt = $3, updated_at = now()
		where id = $1
		returning id, automation_id, account_id, name, system_prompt, is_active`
	return scanPersona(s.pool.QueryRow(ctx, q, id, name, systemPrompt))
}

// ListKnowledgeDocs retorna todos os documentos da automacao em sort_order.
func (s *Store) ListKnowledgeDocs(ctx context.Context, automationID string) ([]KnowledgeDoc, error) {
	const q = `select id, automation_id, account_id, title, body, sort_order, enabled
		from automation.knowledge_documents
		where automation_id = $1
		order by sort_order, created_at`
	rows, err := s.pool.Query(ctx, q, automationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []KnowledgeDoc
	for rows.Next() {
		var d KnowledgeDoc
		if err := rows.Scan(&d.ID, &d.AutomationID, &d.AccountID, &d.Title, &d.Body, &d.SortOrder, &d.Enabled); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// CreateKnowledgeDoc insere um novo documento de conhecimento.
func (s *Store) CreateKnowledgeDoc(ctx context.Context, automationID, accountID, title, body string, sortOrder int) (KnowledgeDoc, error) {
	const q = `insert into automation.knowledge_documents
		(automation_id, account_id, title, body, sort_order)
		values ($1, $2, $3, $4, $5)
		returning id, automation_id, account_id, title, body, sort_order, enabled`
	return scanKnowledgeDoc(s.pool.QueryRow(ctx, q, automationID, accountID, title, body, sortOrder))
}

// UpdateKnowledgeDoc edita titulo, corpo, ordem e estado do documento.
// Filtra por automation_id para garantir isolamento de tenant.
func (s *Store) UpdateKnowledgeDoc(ctx context.Context, id, automationID, title, body string, sortOrder int, enabled bool) (KnowledgeDoc, error) {
	const q = `update automation.knowledge_documents
		set title = $3, body = $4, sort_order = $5, enabled = $6, updated_at = now()
		where id = $1 and automation_id = $2
		returning id, automation_id, account_id, title, body, sort_order, enabled`
	return scanKnowledgeDoc(s.pool.QueryRow(ctx, q, id, automationID, title, body, sortOrder, enabled))
}

// DeleteKnowledgeDoc remove um documento verificando que pertence a esta automacao.
func (s *Store) DeleteKnowledgeDoc(ctx context.Context, id, automationID string) error {
	const q = `delete from automation.knowledge_documents where id = $1 and automation_id = $2`
	tag, err := s.pool.Exec(ctx, q, id, automationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetContact retorna a memoria de conversa de um contato (por chatId).
// Retorna pgx.ErrNoRows se ainda nao houve nenhuma interacao.
func (s *Store) GetContact(ctx context.Context, automationID, chatID string) (Contact, error) {
	const q = `select id, automation_id, account_id, chat_id, seg, last_msg, last_msg_ts, long_memory, paused_until
		from automation.contacts
		where automation_id = $1 and chat_id = $2`
	return scanContact(s.pool.QueryRow(ctx, q, automationID, chatID))
}

// SetContactPause grava (ou limpa) a janela de handover humano de um contato.
// pausedUntil nil = retomar o bot (paused_until = NULL). Faz upsert: se o contato
// ainda nao existe (handover antes da 1a interacao registrada), cria a linha minima.
func (s *Store) SetContactPause(ctx context.Context, automationID, chatID string, pausedUntil *time.Time) error {
	const q = `insert into automation.contacts
		(automation_id, account_id, chat_id, paused_until, updated_at)
		values ($1, (select account_id from automation.automations where id = $1), $2, $3, now())
		on conflict (automation_id, chat_id) do update
		set paused_until = excluded.paused_until,
		    updated_at = now()`
	_, err := s.pool.Exec(ctx, q, automationID, chatID, pausedUntil)
	return err
}

// UpsertContact salva o estado de conversa de um contato.
// Se long_memory vier vazio, preserva o valor existente (uso: salvar so seg/ts/lastMsg).
// Se vier preenchido, sobrescreve (uso: salvar resumo).
func (s *Store) UpsertContact(ctx context.Context, automationID, chatID string, seg int, lastMsg string, ts int64, longMem string) error {
	const q = `insert into automation.contacts
		(automation_id, account_id, chat_id, seg, last_msg, last_msg_ts, long_memory, updated_at)
		values ($1, (select account_id from automation.automations where id = $1), $2, $3, $4, $5, $6, now())
		on conflict (automation_id, chat_id) do update
		set seg = excluded.seg,
		    last_msg = excluded.last_msg,
		    last_msg_ts = excluded.last_msg_ts,
		    long_memory = case when excluded.long_memory = '' then automation.contacts.long_memory
		                       else excluded.long_memory end,
		    updated_at = now()`
	_, err := s.pool.Exec(ctx, q, automationID, chatID, seg, lastMsg, ts, longMem)
	return err
}

// ClearLongMemory zera a memoria longa de TODOS os contatos da automacao.
// Chamado quando um doc de conhecimento e deletado ou editado pelo painel.
func (s *Store) ClearLongMemory(ctx context.Context, automationID string) error {
	const q = `update automation.contacts set long_memory = '', updated_at = now() where automation_id = $1`
	_, err := s.pool.Exec(ctx, q, automationID)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPersona(row rowScanner) (Persona, error) {
	var p Persona
	err := row.Scan(&p.ID, &p.AutomationID, &p.AccountID, &p.Name, &p.SystemPrompt, &p.IsActive)
	return p, err
}

func scanKnowledgeDoc(row rowScanner) (KnowledgeDoc, error) {
	var d KnowledgeDoc
	err := row.Scan(&d.ID, &d.AutomationID, &d.AccountID, &d.Title, &d.Body, &d.SortOrder, &d.Enabled)
	return d, err
}

func scanAutomation(row rowScanner) (Automation, error) {
	var a Automation
	err := row.Scan(&a.ID, &a.AccountID, &a.Type, &a.Name, &a.Slug, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

func scanChannel(row rowScanner) (Channel, error) {
	var c Channel
	err := row.Scan(&c.ID, &c.AutomationID, &c.AccountID, &c.Provider, &c.SessionName, &c.Status, &c.ConnectedPhone, &c.UpdatedAt)
	return c, err
}

func scanContact(row rowScanner) (Contact, error) {
	var c Contact
	err := row.Scan(&c.ID, &c.AutomationID, &c.AccountID, &c.ChatID, &c.Seg, &c.LastMsg, &c.LastMsgTs, &c.LongMemory, &c.PausedUntil)
	return c, err
}
