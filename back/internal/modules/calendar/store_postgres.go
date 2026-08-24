package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store e a persistencia do modulo (schema calendar.*).
type Store struct {
	pool *pgxpool.Pool
}

// NewStore cria o Store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

type rowScanner interface {
	Scan(dest ...any) error
}

// Colunas do evento na ordem esperada por scanEvent. client_id e event_date saem
// como text (” ->; 'YYYY-MM-DD'); os jsonb saem com coalesce para '[]'. version e o
// contador de optimistic locking (C12), sempre a ultima coluna base.
const eventCols = `id, account_id, client_id::text, event_date::text, event_time, type, title,
	status, priority, responsible_id, coalesce(involved_ids, '[]'::jsonb),
	coalesce(media, '[]'::jsonb), description, created_at, updated_at, version,
	coalesce(source, 'manual'), coalesce(linked_media, '[]'::jsonb)`

// eventTaskIDCol resolve a task vinculada ao evento (contrato C10) como subquery
// ESCALAR (uma task por evento, sem multiplicar linhas no LEFT JOIN) sobre
// tasks.task_relations, filtrada por module/resource_type e amarrada a MESMA account
// (defesa em profundidade: nunca mostra task de outra conta). NULL = sem vinculo. Usa o
// indice tasks_task_relations_module_idx (module, resource_type, resource_id) — sem N+1.
// Exige o alias `e` na tabela calendar.events para a correlacao (e.id / e.account_id).
const eventTaskIDCol = `,
	(select tr.task_id::text
	 from tasks.task_relations tr
	 join tasks.tasks t on t.id = tr.task_id
	 where tr.module = 'calendar' and tr.resource_type = 'event'
	   and tr.resource_id = e.id::text and t.account_id = e.account_id
	 order by tr.refreshed_at desc
	 limit 1)`

func scanEvent(row rowScanner) (CalendarEvent, error) {
	var e CalendarEvent
	err := row.Scan(&e.ID, &e.AccountID, &e.ClientID, &e.Date, &e.Time, &e.Type, &e.Title,
		&e.Status, &e.Priority, &e.ResponsibleID, &e.InvolvedIDs, &e.Media, &e.Description,
		&e.CreatedAt, &e.UpdatedAt, &e.Version, &e.Source, &e.LinkedMedia)
	return e, err
}

// scanEventWithTask e o scanEvent + a coluna taskId (eventTaskIDCol). Usado so nas
// leituras que fazem o join de vinculo (ListEvents/GetEvent). A ordem das colunas e
// eventCols (com version por ultimo) seguido de eventTaskIDCol.
func scanEventWithTask(row rowScanner) (CalendarEvent, error) {
	var e CalendarEvent
	err := row.Scan(&e.ID, &e.AccountID, &e.ClientID, &e.Date, &e.Time, &e.Type, &e.Title,
		&e.Status, &e.Priority, &e.ResponsibleID, &e.InvolvedIDs, &e.Media, &e.Description,
		&e.CreatedAt, &e.UpdatedAt, &e.Version, &e.Source, &e.LinkedMedia, &e.TaskID)
	return e, err
}

// ResolveCalendarScope resolve a conta que armazena a agenda e os clientes que a
// account ativa pode enxergar. Para conta-cliente, a agenda pertence a conta-agencia
// ativa da mesma organization e o client_id fica travado na propria account. A query
// nunca recebe organization/account do browser alem da account ativa ja validada.
func (s *Store) ResolveCalendarScope(ctx context.Context, activeAccountID string) (CalendarScope, error) {
	const scopeQuery = `
		select
			a.id::text,
			coalesce(a.name, ''),
			a.is_agency,
			coalesce(a.organization_id::text, ''),
			case
				when a.is_agency then a.id::text
				else coalesce((
					select owner.id::text
					from core.accounts owner
					where owner.organization_id = a.organization_id
					  and owner.is_agency = true
					  and owner.is_active = true
					order by owner.created_at asc, owner.id asc
					limit 1
				), a.id::text)
			end
		from core.accounts a
		where a.id = $1::uuid and a.is_active = true`

	var activeID, activeName, organizationID, storageAccountID string
	var isAgency bool
	if err := s.pool.QueryRow(ctx, scopeQuery, activeAccountID).Scan(
		&activeID, &activeName, &isAgency, &organizationID, &storageAccountID,
	); err != nil {
		return CalendarScope{}, err
	}

	scope := CalendarScope{
		StorageAccountID: storageAccountID,
		CanSelect:        isAgency,
		Clients:          make([]CalendarScopeClient, 0),
	}
	if !isAgency {
		scope.LockedClientID = activeID
		scope.Clients = append(scope.Clients, CalendarScopeClient{ID: activeID, Name: activeName})
		return scope, nil
	}
	if organizationID == "" {
		return scope, nil
	}

	const clientsQuery = `
		select c.id::text, coalesce(c.name, '')
		from core.accounts c
		where c.organization_id = $1::uuid
		  and c.is_agency = false
		  and c.is_active = true
		order by lower(c.name), c.id`
	rows, err := s.pool.Query(ctx, clientsQuery, organizationID)
	if err != nil {
		return CalendarScope{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var client CalendarScopeClient
		if err := rows.Scan(&client.ID, &client.Name); err != nil {
			return CalendarScope{}, err
		}
		scope.Clients = append(scope.Clients, client)
	}
	return scope, rows.Err()
}

// ListEvents retorna os eventos da account na janela [from, to] (inclusive),
// opcionalmente filtrados por cliente. Datas/cliente vazios = sem aquele filtro.
func (s *Store) ListEvents(ctx context.Context, accountID string, f EventFilter) ([]EventView, error) {
	query := `select ` + eventCols + eventTaskIDCol + ` from calendar.events e where account_id = $1::uuid`
	args := []any{accountID}
	if strings.TrimSpace(f.From) != "" {
		args = append(args, strings.TrimSpace(f.From))
		query += " and event_date >= $" + strconv.Itoa(len(args)) + "::date"
	}
	if strings.TrimSpace(f.To) != "" {
		args = append(args, strings.TrimSpace(f.To))
		query += " and event_date <= $" + strconv.Itoa(len(args)) + "::date"
	}
	if strings.TrimSpace(f.ClientID) != "" {
		args = append(args, strings.TrimSpace(f.ClientID))
		query += " and client_id = $" + strconv.Itoa(len(args)) + "::uuid"
	}
	query += " order by event_date, event_time, created_at"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]EventView, 0)
	for rows.Next() {
		e, err := scanEventWithTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e.view())
	}
	return out, rows.Err()
}

// GetEvent retorna um evento. accountID vazio = sem filtro de escopo (admin);
// preenchido = defesa em profundidade (evento de outra account => ErrNoRows).
func (s *Store) GetEvent(ctx context.Context, id, accountID, clientScopeID string) (CalendarEvent, error) {
	query := `select ` + eventCols + eventTaskIDCol + ` from calendar.events e where id = $1::uuid`
	args := []any{id}
	if strings.TrimSpace(accountID) != "" {
		args = append(args, accountID)
		query += " and account_id = $" + strconv.Itoa(len(args)) + "::uuid"
	}
	if strings.TrimSpace(clientScopeID) != "" {
		args = append(args, clientScopeID)
		query += " and client_id = $" + strconv.Itoa(len(args)) + "::uuid"
	}
	return scanEventWithTask(s.pool.QueryRow(ctx, query, args...))
}

// CreateEvent insere um novo evento na account.
func (s *Store) CreateEvent(ctx context.Context, accountID string, in EventInput) (CalendarEvent, error) {
	const q = `
		insert into calendar.events
			(account_id, client_id, event_date, event_time, type, title, status, priority,
			 responsible_id, involved_ids, media, description, source)
		values ($1::uuid, $2::uuid, $3::date, $4, $5, $6, $7, $8, $9, $10::jsonb, $11::jsonb, $12, $13)
		returning ` + eventCols
	source := strings.TrimSpace(in.Source)
	if source == "" {
		source = "manual"
	}
	return scanEvent(s.pool.QueryRow(ctx, q,
		accountID, nullUUID(in.ClientID), in.Date, in.Time, in.Type, in.Title, in.Status,
		in.Priority, in.ResponsibleID, jsonArray(in.InvolvedIDs), jsonMedia(in.Media), in.Description, source))
}

// UpdateEvent substitui os campos mutaveis do evento (full replace) e incrementa
// version (C12). accountID vazio = sem filtro (admin); preenchido = defesa em
// profundidade. expectedVersion nao-nil adiciona o guard `and version = $n` (optimistic
// locking): se ninguem casar, retorna pgx.ErrNoRows e o service desambigua 404 x 409.
func (s *Store) UpdateEvent(ctx context.Context, id, accountID, clientScopeID string, in EventInput, expectedVersion *int) (CalendarEvent, error) {
	query := `
		update calendar.events set
			client_id = $2::uuid, event_date = $3::date, event_time = $4, type = $5, title = $6,
			status = $7, priority = $8, responsible_id = $9, involved_ids = $10::jsonb,
			media = $11::jsonb, description = $12, updated_at = now(), version = version + 1
		where id = $1::uuid`
	args := []any{id, nullUUID(in.ClientID), in.Date, in.Time, in.Type, in.Title, in.Status,
		in.Priority, in.ResponsibleID, jsonArray(in.InvolvedIDs), jsonMedia(in.Media), in.Description}
	if strings.TrimSpace(accountID) != "" {
		args = append(args, accountID)
		query += " and account_id = $" + strconv.Itoa(len(args)) + "::uuid"
	}
	if strings.TrimSpace(clientScopeID) != "" {
		args = append(args, clientScopeID)
		query += " and client_id = $" + strconv.Itoa(len(args)) + "::uuid"
	}
	if expectedVersion != nil {
		args = append(args, *expectedVersion)
		query += " and version = $" + strconv.Itoa(len(args))
	}
	query += " returning " + eventCols
	return scanEvent(s.pool.QueryRow(ctx, query, args...))
}

// SetEventLinkedMedia atualiza SO a coluna linked_media do evento (WAVE 6 cruzamento B): a midia
// espelhada da task vinculada, read-only. NAO bumpa version (nao e conteudo editavel do usuario;
// nao pode invalidar o optimistic locking dele) nem updated_at. accountID = defesa em profundidade.
func (s *Store) SetEventLinkedMedia(ctx context.Context, id, accountID string, media []MediaItem) error {
	query := `update calendar.events set linked_media = $2::jsonb where id = $1::uuid`
	args := []any{id, jsonMedia(media)}
	if strings.TrimSpace(accountID) != "" {
		args = append(args, accountID)
		query += " and account_id = $3::uuid"
	}
	_, err := s.pool.Exec(ctx, query, args...)
	return err
}

// DeleteEvent remove um evento. Retorna pgx.ErrNoRows quando nada foi apagado.
func (s *Store) DeleteEvent(ctx context.Context, id, accountID, clientScopeID string) error {
	query := `delete from calendar.events where id = $1::uuid`
	args := []any{id}
	if strings.TrimSpace(accountID) != "" {
		args = append(args, accountID)
		query += " and account_id = $2::uuid"
	}
	if strings.TrimSpace(clientScopeID) != "" {
		args = append(args, clientScopeID)
		query += " and client_id = $" + strconv.Itoa(len(args)) + "::uuid"
	}
	tag, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetNotes retorna a nota do mes (vazia se ainda nao existe).
func (s *Store) GetNotes(ctx context.Context, accountID, month string) (NoteView, error) {
	const q = `select month_key, content, updated_by, updated_at
		from calendar.notes where account_id = $1::uuid and month_key = $2`
	var n NoteView
	err := s.pool.QueryRow(ctx, q, accountID, month).Scan(&n.Month, &n.Content, &n.UpdatedBy, &n.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return NoteView{Month: month, UpdatedAt: time.Now()}, nil
	}
	return n, err
}

// PutNotes faz upsert da nota do mes.
func (s *Store) PutNotes(ctx context.Context, accountID, month, content, updatedBy string) (NoteView, error) {
	const q = `
		insert into calendar.notes (account_id, month_key, content, updated_by, updated_at)
		values ($1::uuid, $2, $3, $4, now())
		on conflict (account_id, month_key) do update
		set content = excluded.content, updated_by = excluded.updated_by, updated_at = now()
		returning month_key, content, updated_by, updated_at`
	var n NoteView
	err := s.pool.QueryRow(ctx, q, accountID, month, content, updatedBy).
		Scan(&n.Month, &n.Content, &n.UpdatedBy, &n.UpdatedAt)
	return n, err
}

// ============================================================================
// Config por conta + membros (Fase 2)
// ============================================================================

// GetConfig le a config da account. Sem linha -> defaults. jsonb parcial: os
// campos ausentes ficam com o default (ex.: conjunto de feriados omitido = true).
func (s *Store) GetConfig(ctx context.Context, accountID string) (CalendarConfig, error) {
	const q = `select config from calendar.config where account_id = $1::uuid`
	cfg := defaultConfig()
	var raw json.RawMessage
	err := s.pool.QueryRow(ctx, q, accountID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &cfg)
	}
	// Garante shape estavel do C2 mesmo com jsonb parcial ou com null explicito
	// (unmarshal de "clientColors": null zera o mapa; defaults acima nao bastam).
	if cfg.ResponsibleUserIDs == nil {
		cfg.ResponsibleUserIDs = []string{}
	}
	if cfg.ClientColors == nil {
		cfg.ClientColors = map[string]string{}
	}
	if cfg.TypeColors == nil {
		cfg.TypeColors = map[string]string{}
	}
	if strings.TrimSpace(cfg.WeekStartsOn) == "" {
		cfg.WeekStartsOn = "sunday"
	}
	if strings.TrimSpace(cfg.AI.Provider) == "" {
		cfg.AI.Provider = "claude"
	}
	// Shape v4 estavel mesmo com jsonb parcial ou null explicito nas secoes novas
	// (unmarshal de "ai":{...} sem transcribeProvider, ou "chat":null, deixa vazio).
	if strings.TrimSpace(cfg.AI.TranscribeProvider) == "" {
		cfg.AI.TranscribeProvider = "gemini"
	}
	if strings.TrimSpace(cfg.Chat.Position) == "" {
		cfg.Chat.Position = "center"
	}
	// Shape v4.1 estavel (WAVE 3.1) para conta antiga: scopeMode ausente => general;
	// disabledClientIds null/ausente => lista vazia (nunca nil no round-trip do jsonb).
	if strings.TrimSpace(cfg.AI.ScopeMode) == "" {
		cfg.AI.ScopeMode = scopeModeGeneral
	}
	if cfg.AI.DisabledClientIDs == nil {
		cfg.AI.DisabledClientIDs = []string{}
	}
	return cfg, nil
}

// PutConfig faz upsert da config da account.
func (s *Store) PutConfig(ctx context.Context, accountID string, cfg CalendarConfig) (CalendarConfig, error) {
	body, err := json.Marshal(cfg)
	if err != nil {
		return cfg, err
	}
	const q = `
		insert into calendar.config (account_id, config, updated_at)
		values ($1::uuid, $2::jsonb, now())
		on conflict (account_id) do update
		set config = excluded.config, updated_at = now()`
	if _, err := s.pool.Exec(ctx, q, accountID, body); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// ListMembers lista os usuarios ativos membros da account (candidatos a
// responsavel). Nome = display_name (fallback email).
func (s *Store) ListMembers(ctx context.Context, accountID string) ([]Member, error) {
	const q = `
		select u.id::text, coalesce(nullif(trim(u.display_name), ''), u.email, '')
		from core.account_users au
		join core.users u on u.id = au.user_id
		where au.account_id = $1::uuid and u.is_active = true
		order by 2`
	rows, err := s.pool.Query(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Member, 0)
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Name); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ResolveUserLabel resolve o nome exibivel de UM usuario pelo id (WAVE 6): nick > display_name
// > email. Usado no sync evento->task para gravar o nome fresco em ui_metadata.responsible (a
// task cacheia o nome e ficava velho quando so o id era sincronizado). "" se nao achar.
func (s *Store) ResolveUserLabel(ctx context.Context, userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ""
	}
	var label string
	err := s.pool.QueryRow(ctx, `
		select coalesce(nullif(trim(nick), ''), nullif(trim(display_name), ''), email, '')
		from core.users where id = $1::uuid
	`, userID).Scan(&label)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(label)
}

// ============================================================================
// Anexos / midia (Fase 3)
// ============================================================================

// GetMediaLimits le os tetos de upload da config GLOBAL (core.platform_settings,
// chave 'media_limits'). Sem linha -> defaults; valores <= 0 caem no default.
func (s *Store) GetMediaLimits(ctx context.Context) (MediaLimits, error) {
	const q = `select config from core.platform_settings where key = 'media_limits'`
	limits := defaultMediaLimits()
	var raw json.RawMessage
	err := s.pool.QueryRow(ctx, q).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return limits, nil
	}
	if err != nil {
		return limits, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &limits)
	}
	def := defaultMediaLimits()
	if limits.ImageMaxBytes <= 0 {
		limits.ImageMaxBytes = def.ImageMaxBytes
	}
	if limits.VideoMaxBytes <= 0 {
		limits.VideoMaxBytes = def.VideoMaxBytes
	}
	return limits, nil
}

// PutMediaLimits faz upsert dos tetos globais. updatedBy = userID (uuid) ou vazio.
func (s *Store) PutMediaLimits(ctx context.Context, limits MediaLimits, updatedBy string) error {
	body, err := json.Marshal(limits)
	if err != nil {
		return err
	}
	const q = `
		insert into core.platform_settings (key, config, updated_at, updated_by)
		values ('media_limits', $1::jsonb, now(), $2::uuid)
		on conflict (key) do update
		set config = excluded.config, updated_at = now(), updated_by = excluded.updated_by`
	_, err = s.pool.Exec(ctx, q, body, nullUUID(updatedBy))
	return err
}

// ============================================================================
// Helpers
// ============================================================================

// jsonMedia serializa uma lista de MediaItem em jsonb (sempre um array).
func jsonMedia(items []MediaItem) []byte {
	if items == nil {
		items = []MediaItem{}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return []byte("[]")
	}
	return b
}

// nullUUID devolve *string (nil se vazio) para colunas uuid nullable.
func nullUUID(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// jsonArray serializa uma lista de strings em jsonb (sempre um array).
func jsonArray(items []string) []byte {
	if items == nil {
		items = []string{}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return []byte("[]")
	}
	return b
}
