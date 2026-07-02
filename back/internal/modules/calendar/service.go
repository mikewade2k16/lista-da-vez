package calendar

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Erros de dominio. Fora-do-escopo e nao-encontrado colapsam em ErrNotFound
// (404) para nao vazar existencia de recurso de outra account.
var (
	ErrNotFound      = errors.New("calendar: not found")
	ErrForbidden     = errors.New("calendar: forbidden")
	ErrInvalidDate   = errors.New("calendar: invalid date")
	ErrInvalidTitle  = errors.New("calendar: invalid title")
	ErrInvalidMedia  = errors.New("calendar: invalid media")
	ErrMediaTooLarge = errors.New("calendar: media too large")
)

var (
	dateRe  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	monthRe = regexp.MustCompile(`^\d{4}-\d{2}$`)
	uuidRe  = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// calendarStore e a fatia da persistencia que o Service consome.
type calendarStore interface {
	ListEvents(ctx context.Context, accountID string, f EventFilter) ([]EventView, error)
	GetEvent(ctx context.Context, id, accountID string) (CalendarEvent, error)
	CreateEvent(ctx context.Context, accountID string, in EventInput) (CalendarEvent, error)
	UpdateEvent(ctx context.Context, id, accountID string, in EventInput) (CalendarEvent, error)
	DeleteEvent(ctx context.Context, id, accountID string) error
	GetNotes(ctx context.Context, accountID, month string) (NoteView, error)
	PutNotes(ctx context.Context, accountID, month, content, updatedBy string) (NoteView, error)
	GetConfig(ctx context.Context, accountID string) (CalendarConfig, error)
	PutConfig(ctx context.Context, accountID string, cfg CalendarConfig) (CalendarConfig, error)
	ListMembers(ctx context.Context, accountID string) ([]Member, error)
	ListDayMedia(ctx context.Context, accountID, from, to string) ([]DayMediaView, error)
	PutDayMedia(ctx context.Context, accountID, date string, media []MediaItem) (DayMediaView, error)
	GetMediaLimits(ctx context.Context) (MediaLimits, error)
	PutMediaLimits(ctx context.Context, limits MediaLimits, updatedBy string) error
}

// Service implementa as regras do modulo calendar. O calendario e SEMPRE escopado
// pela account do contexto (X-Account-Id / TenantID); nao ha visao cross-account.
type Service struct {
	store   calendarStore
	storage MediaStorage
}

// NewService cria o Service. storage pode ser nil quando o modulo roda sem upload.
func NewService(store calendarStore, storage MediaStorage) *Service {
	return &Service{store: store, storage: storage}
}

// ListEvents devolve os eventos da account na janela pedida.
func (s *Service) ListEvents(ctx context.Context, accountID string, f EventFilter) ([]EventView, error) {
	f.ClientID = normalizeUUID(f.ClientID)
	return s.store.ListEvents(ctx, strings.TrimSpace(accountID), f)
}

// GetEvent devolve um evento dentro do escopo da account.
func (s *Service) GetEvent(ctx context.Context, accountID, id string) (EventView, error) {
	e, err := s.store.GetEvent(ctx, id, strings.TrimSpace(accountID))
	if err != nil {
		return EventView{}, mapNotFound(err)
	}
	return e.view(), nil
}

// CreateEvent cria um evento na account.
func (s *Service) CreateEvent(ctx context.Context, accountID string, in EventInput) (EventView, error) {
	in, err := validateEvent(in)
	if err != nil {
		return EventView{}, err
	}
	e, err := s.store.CreateEvent(ctx, strings.TrimSpace(accountID), in)
	if err != nil {
		return EventView{}, err
	}
	return e.view(), nil
}

// UpdateEvent substitui os campos do evento no escopo da account.
func (s *Service) UpdateEvent(ctx context.Context, accountID, id string, in EventInput) (EventView, error) {
	in, err := validateEvent(in)
	if err != nil {
		return EventView{}, err
	}
	e, err := s.store.UpdateEvent(ctx, id, strings.TrimSpace(accountID), in)
	if err != nil {
		return EventView{}, mapNotFound(err)
	}
	return e.view(), nil
}

// DeleteEvent remove um evento no escopo da account.
func (s *Service) DeleteEvent(ctx context.Context, accountID, id string) error {
	if err := s.store.DeleteEvent(ctx, id, strings.TrimSpace(accountID)); err != nil {
		return mapNotFound(err)
	}
	return nil
}

// GetNotes devolve a nota do mes ('YYYY-MM').
func (s *Service) GetNotes(ctx context.Context, accountID, month string) (NoteView, error) {
	month = strings.TrimSpace(month)
	if !monthRe.MatchString(month) {
		return NoteView{}, ErrInvalidDate
	}
	return s.store.GetNotes(ctx, strings.TrimSpace(accountID), month)
}

// PutNotes faz upsert da nota do mes.
func (s *Service) PutNotes(ctx context.Context, accountID, month, content, updatedBy string) (NoteView, error) {
	month = strings.TrimSpace(month)
	if !monthRe.MatchString(month) {
		return NoteView{}, ErrInvalidDate
	}
	return s.store.PutNotes(ctx, strings.TrimSpace(accountID), month, content, strings.TrimSpace(updatedBy))
}

// ============================================================================
// Config + responsaveis (Fase 2)
// ============================================================================

// GetConfig devolve a config da account (defaults se nao existe).
func (s *Service) GetConfig(ctx context.Context, accountID string) (CalendarConfig, error) {
	return s.store.GetConfig(ctx, strings.TrimSpace(accountID))
}

// PutConfig salva a config da account (normaliza os ids de responsavel para UUID).
func (s *Service) PutConfig(ctx context.Context, accountID string, cfg CalendarConfig) (CalendarConfig, error) {
	ids := make([]string, 0, len(cfg.ResponsibleUserIDs))
	seen := map[string]bool{}
	for _, raw := range cfg.ResponsibleUserIDs {
		id := normalizeUUID(raw)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	cfg.ResponsibleUserIDs = ids
	return s.store.PutConfig(ctx, strings.TrimSpace(accountID), cfg)
}

// ListMembers lista os usuarios da account (candidatos a responsavel).
func (s *Service) ListMembers(ctx context.Context, accountID string) ([]Member, error) {
	return s.store.ListMembers(ctx, strings.TrimSpace(accountID))
}

// ListHolidays devolve os feriados/datas comemorativas na janela [from, to]
// (inclusive), filtrados pelos conjuntos ligados na config da conta. Read-only:
// as datas sao calculadas/seed em codigo (sem tabela). from/to malformados ->
// ErrInvalidDate.
func (s *Service) ListHolidays(ctx context.Context, accountID, from, to string) ([]Holiday, error) {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if !dateRe.MatchString(from) || !dateRe.MatchString(to) {
		return nil, ErrInvalidDate
	}
	cfg, err := s.store.GetConfig(ctx, strings.TrimSpace(accountID))
	if err != nil {
		return nil, err
	}
	return HolidaysInRange(from, to, cfg), nil
}

// ListResponsibles devolve os responsaveis ativos: subconjunto configurado ou,
// se vazio, todos os membros da account.
func (s *Service) ListResponsibles(ctx context.Context, accountID string) ([]Member, error) {
	account := strings.TrimSpace(accountID)
	cfg, err := s.store.GetConfig(ctx, account)
	if err != nil {
		return nil, err
	}
	members, err := s.store.ListMembers(ctx, account)
	if err != nil {
		return nil, err
	}
	if len(cfg.ResponsibleUserIDs) == 0 {
		return members, nil
	}
	allowed := make(map[string]bool, len(cfg.ResponsibleUserIDs))
	for _, id := range cfg.ResponsibleUserIDs {
		allowed[id] = true
	}
	out := make([]Member, 0, len(members))
	for _, m := range members {
		if allowed[m.ID] {
			out = append(out, m)
		}
	}
	return out, nil
}

// ============================================================================
// Anexos / midia (Fase 3)
// ============================================================================

// GetMediaLimits devolve os tetos de upload (globais da plataforma; default se
// nao configurado).
func (s *Service) GetMediaLimits(ctx context.Context) (MediaLimits, error) {
	return s.store.GetMediaLimits(ctx)
}

// SaveMediaLimits persiste os tetos de upload (so platform_admin, gate no HTTP).
// Valores <= 0 sao rejeitados (evita travar o upload por config invalida).
func (s *Service) SaveMediaLimits(ctx context.Context, limits MediaLimits, updatedBy string) (MediaLimits, error) {
	if limits.ImageMaxBytes <= 0 || limits.VideoMaxBytes <= 0 {
		return MediaLimits{}, ErrInvalidMedia
	}
	if err := s.store.PutMediaLimits(ctx, limits, strings.TrimSpace(updatedBy)); err != nil {
		return MediaLimits{}, err
	}
	return limits, nil
}

// SaveMedia valida (mime + tamanho contra os limites da plataforma) e grava o
// anexo, devolvendo o MediaItem para o front anexar a um evento ou dia.
func (s *Service) SaveMedia(ctx context.Context, accountID, fileName, contentType string, content []byte) (MediaItem, error) {
	if s.storage == nil {
		return MediaItem{}, ErrInvalidMedia
	}
	limits, err := s.store.GetMediaLimits(ctx)
	if err != nil {
		return MediaItem{}, err
	}
	return s.storage.Save(strings.TrimSpace(accountID), fileName, contentType, content, limits)
}

// ListDayMedia devolve os anexos avulsos por dia na janela [from, to] (inclusive).
func (s *Service) ListDayMedia(ctx context.Context, accountID, from, to string) ([]DayMediaView, error) {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if !dateRe.MatchString(from) || !dateRe.MatchString(to) {
		return nil, ErrInvalidDate
	}
	return s.store.ListDayMedia(ctx, strings.TrimSpace(accountID), from, to)
}

// PutDayMedia substitui (full replace) a lista de anexos avulsos de um dia.
func (s *Service) PutDayMedia(ctx context.Context, accountID, date string, media []MediaItem) (DayMediaView, error) {
	date = strings.TrimSpace(date)
	if !dateRe.MatchString(date) {
		return DayMediaView{}, ErrInvalidDate
	}
	return s.store.PutDayMedia(ctx, strings.TrimSpace(accountID), date, normalizeMedia(media))
}

// ============================================================================
// Helpers
// ============================================================================

// validateEvent valida os campos minimos e devolve o input normalizado (defaults
// + trims + client_id descartado se nao for UUID valido, evitando erro de cast).
func validateEvent(in EventInput) (EventInput, error) {
	in.Date = strings.TrimSpace(in.Date)
	if !dateRe.MatchString(in.Date) {
		return in, ErrInvalidDate
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return in, ErrInvalidTitle
	}
	in.Time = strings.TrimSpace(in.Time)
	in.ClientID = normalizeUUID(in.ClientID)
	in.Type = firstNonEmpty(strings.TrimSpace(in.Type), "post")
	in.Status = firstNonEmpty(strings.TrimSpace(in.Status), "planejado")
	in.Priority = firstNonEmpty(strings.TrimSpace(in.Priority), "media")
	in.ResponsibleID = strings.TrimSpace(in.ResponsibleID)
	if in.InvolvedIDs == nil {
		in.InvolvedIDs = []string{}
	}
	in.Media = normalizeMedia(in.Media)
	return in, nil
}

// normalizeMedia sanitiza a lista de anexos: descarta itens sem url interna
// (/uploads/calendar/), normaliza type (image/video) e nao deixa size negativo.
// Defesa contra injecao de URL arbitraria no jsonb da account.
func normalizeMedia(items []MediaItem) []MediaItem {
	out := make([]MediaItem, 0, len(items))
	for _, m := range items {
		m.URL = strings.TrimSpace(m.URL)
		if !strings.HasPrefix(m.URL, "/uploads/calendar/") {
			continue
		}
		m.Type = strings.ToLower(strings.TrimSpace(m.Type))
		if m.Type != "video" {
			m.Type = "image"
		}
		m.ID = strings.TrimSpace(m.ID)
		m.Name = strings.TrimSpace(m.Name)
		m.ContentType = strings.TrimSpace(m.ContentType)
		if m.SizeBytes < 0 {
			m.SizeBytes = 0
		}
		out = append(out, m)
	}
	return out
}

// normalizeUUID devolve o UUID em minusculas se valido; senao "" (sem cliente/
// sem filtro), evitando erro de cast ::uuid no banco com ids nao-UUID (ex.: os
// clientes de demonstracao do front mock).
func normalizeUUID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if uuidRe.MatchString(id) {
		return id
	}
	return ""
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// mapNotFound colapsa pgx.ErrNoRows em ErrNotFound (404).
func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
