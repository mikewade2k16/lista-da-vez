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
// como text ('' ->; 'YYYY-MM-DD'); os jsonb saem com coalesce para '[]'.
const eventCols = `id, account_id, client_id::text, event_date::text, event_time, type, title,
	status, priority, responsible_id, coalesce(involved_ids, '[]'::jsonb),
	coalesce(media, '[]'::jsonb), description, created_at, updated_at`

func scanEvent(row rowScanner) (CalendarEvent, error) {
	var e CalendarEvent
	err := row.Scan(&e.ID, &e.AccountID, &e.ClientID, &e.Date, &e.Time, &e.Type, &e.Title,
		&e.Status, &e.Priority, &e.ResponsibleID, &e.InvolvedIDs, &e.Media, &e.Description,
		&e.CreatedAt, &e.UpdatedAt)
	return e, err
}

// ListEvents retorna os eventos da account na janela [from, to] (inclusive),
// opcionalmente filtrados por cliente. Datas/cliente vazios = sem aquele filtro.
func (s *Store) ListEvents(ctx context.Context, accountID string, f EventFilter) ([]EventView, error) {
	query := `select ` + eventCols + ` from calendar.events where account_id = $1::uuid`
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
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e.view())
	}
	return out, rows.Err()
}

// GetEvent retorna um evento. accountID vazio = sem filtro de escopo (admin);
// preenchido = defesa em profundidade (evento de outra account => ErrNoRows).
func (s *Store) GetEvent(ctx context.Context, id, accountID string) (CalendarEvent, error) {
	query := `select ` + eventCols + ` from calendar.events where id = $1::uuid`
	args := []any{id}
	if strings.TrimSpace(accountID) != "" {
		args = append(args, accountID)
		query += " and account_id = $2::uuid"
	}
	return scanEvent(s.pool.QueryRow(ctx, query, args...))
}

// CreateEvent insere um novo evento na account.
func (s *Store) CreateEvent(ctx context.Context, accountID string, in EventInput) (CalendarEvent, error) {
	const q = `
		insert into calendar.events
			(account_id, client_id, event_date, event_time, type, title, status, priority,
			 responsible_id, involved_ids, media, description)
		values ($1::uuid, $2::uuid, $3::date, $4, $5, $6, $7, $8, $9, $10::jsonb, $11::jsonb, $12)
		returning ` + eventCols
	return scanEvent(s.pool.QueryRow(ctx, q,
		accountID, nullUUID(in.ClientID), in.Date, in.Time, in.Type, in.Title, in.Status,
		in.Priority, in.ResponsibleID, jsonArray(in.InvolvedIDs), jsonMedia(in.Media), in.Description))
}

// UpdateEvent substitui os campos mutaveis do evento (full replace). accountID
// vazio = sem filtro (admin); preenchido = defesa em profundidade.
func (s *Store) UpdateEvent(ctx context.Context, id, accountID string, in EventInput) (CalendarEvent, error) {
	query := `
		update calendar.events set
			client_id = $2::uuid, event_date = $3::date, event_time = $4, type = $5, title = $6,
			status = $7, priority = $8, responsible_id = $9, involved_ids = $10::jsonb,
			media = $11::jsonb, description = $12, updated_at = now()
		where id = $1::uuid`
	args := []any{id, nullUUID(in.ClientID), in.Date, in.Time, in.Type, in.Title, in.Status,
		in.Priority, in.ResponsibleID, jsonArray(in.InvolvedIDs), jsonMedia(in.Media), in.Description}
	if strings.TrimSpace(accountID) != "" {
		args = append(args, accountID)
		query += " and account_id = $13::uuid"
	}
	query += " returning " + eventCols
	return scanEvent(s.pool.QueryRow(ctx, query, args...))
}

// DeleteEvent remove um evento. Retorna pgx.ErrNoRows quando nada foi apagado.
func (s *Store) DeleteEvent(ctx context.Context, id, accountID string) error {
	query := `delete from calendar.events where id = $1::uuid`
	args := []any{id}
	if strings.TrimSpace(accountID) != "" {
		args = append(args, accountID)
		query += " and account_id = $2::uuid"
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
	if cfg.ResponsibleUserIDs == nil {
		cfg.ResponsibleUserIDs = []string{}
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

// ============================================================================
// Anexos / midia (Fase 3)
// ============================================================================

// ListDayMedia devolve os anexos avulsos por dia na janela [from, to] (inclusive).
func (s *Store) ListDayMedia(ctx context.Context, accountID, from, to string) ([]DayMediaView, error) {
	const q = `
		select event_date::text, coalesce(media, '[]'::jsonb)
		from calendar.day_media
		where account_id = $1::uuid and event_date >= $2::date and event_date <= $3::date
		order by event_date`
	rows, err := s.pool.Query(ctx, q, accountID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DayMediaView, 0)
	for rows.Next() {
		var date string
		var raw json.RawMessage
		if err := rows.Scan(&date, &raw); err != nil {
			return nil, err
		}
		out = append(out, DayMediaView{Date: date, Media: decodeMedia(raw)})
	}
	return out, rows.Err()
}

// PutDayMedia faz upsert (full replace) dos anexos de um dia.
func (s *Store) PutDayMedia(ctx context.Context, accountID, date string, media []MediaItem) (DayMediaView, error) {
	const q = `
		insert into calendar.day_media (account_id, event_date, media, updated_at)
		values ($1::uuid, $2::date, $3::jsonb, now())
		on conflict (account_id, event_date) do update
		set media = excluded.media, updated_at = now()`
	if _, err := s.pool.Exec(ctx, q, accountID, date, jsonMedia(media)); err != nil {
		return DayMediaView{}, err
	}
	if media == nil {
		media = []MediaItem{}
	}
	return DayMediaView{Date: date, Media: media}, nil
}

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

// decodeMedia desserializa o jsonb de anexos; falha/nulo -> lista vazia.
func decodeMedia(raw json.RawMessage) []MediaItem {
	out := []MediaItem{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	if out == nil {
		out = []MediaItem{}
	}
	return out
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
