package calendar

import (
	"encoding/json"
	"time"
)

// CalendarEvent e um evento de conteudo do calendario (schema calendar.events).
type CalendarEvent struct {
	ID            string
	AccountID     string
	ClientID      *string // null = evento sem cliente
	Date          string  // 'YYYY-MM-DD'
	Time          string
	Type          string
	Title         string
	Status        string
	Priority      string
	ResponsibleID string
	InvolvedIDs   json.RawMessage
	Media         json.RawMessage
	Description   string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// EventView e a projecao JSON do evento. As chaves batem 1:1 com o tipo
// CalendarEvent do front (web/app/utils/calendar.ts).
type EventView struct {
	ID            string          `json:"id"`
	Date          string          `json:"date"`
	Time          string          `json:"time"`
	ClientID      string          `json:"clientId"`
	Type          string          `json:"type"`
	Title         string          `json:"title"`
	Status        string          `json:"status"`
	Priority      string          `json:"priority"`
	ResponsibleID string          `json:"responsibleId"`
	InvolvedIDs   json.RawMessage `json:"involvedIds"`
	Media         json.RawMessage `json:"media"`
	Description   string          `json:"description"`
}

func (e CalendarEvent) view() EventView {
	client := ""
	if e.ClientID != nil {
		client = *e.ClientID
	}
	return EventView{
		ID:            e.ID,
		Date:          e.Date,
		Time:          e.Time,
		ClientID:      client,
		Type:          e.Type,
		Title:         e.Title,
		Status:        e.Status,
		Priority:      e.Priority,
		ResponsibleID: e.ResponsibleID,
		InvolvedIDs:   normalizeArray(e.InvolvedIDs),
		Media:         normalizeArray(e.Media),
		Description:   e.Description,
	}
}

// normalizeArray garante "[]" no lugar de nil (sempre um array no JSON).
func normalizeArray(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("[]")
	}
	return raw
}

// EventInput e o body de POST/PUT de evento (full replace dos campos mutaveis).
type EventInput struct {
	Date          string      `json:"date"`
	Time          string      `json:"time"`
	ClientID      string      `json:"clientId"`
	Type          string      `json:"type"`
	Title         string      `json:"title"`
	Status        string      `json:"status"`
	Priority      string      `json:"priority"`
	ResponsibleID string      `json:"responsibleId"`
	InvolvedIDs   []string    `json:"involvedIds"`
	Media         []MediaItem `json:"media"`
	Description   string      `json:"description"`
}

// MediaItem e um anexo (imagem ou video) de um evento ou dia. url sempre aponta
// para /uploads/calendar/{accountId}/{arquivo} (nunca URL externa). Persistido
// como jsonb em calendar.events.media / calendar.day_media.media.
type MediaItem struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Name        string `json:"name"`
	Type        string `json:"type"` // "image" | "video"
	ContentType string `json:"contentType"`
	SizeBytes   int    `json:"sizeBytes"`
}

// MediaLimits sao os tetos de upload definidos NA PLATAFORMA (global, editavel por
// platform_admin) — core.platform_settings, chave 'media_limits'.
type MediaLimits struct {
	ImageMaxBytes int64 `json:"imageMaxBytes"`
	VideoMaxBytes int64 `json:"videoMaxBytes"`
}

// defaultMediaLimits: imagem 10MB, video 300MB (o "atualmente 300mb").
func defaultMediaLimits() MediaLimits {
	return MediaLimits{ImageMaxBytes: 10 * 1024 * 1024, VideoMaxBytes: 300 * 1024 * 1024}
}

// DayMediaView sao os anexos avulsos de um dia (calendar.day_media).
type DayMediaView struct {
	Date  string      `json:"date"`
	Media []MediaItem `json:"media"`
}

// DayMediaInput e o body do PUT de anexos do dia (full replace da lista).
type DayMediaInput struct {
	Media []MediaItem `json:"media"`
}

// EventFilter sao os filtros da listagem: janela de datas (inclusive) + cliente.
type EventFilter struct {
	From     string // 'YYYY-MM-DD'
	To       string // 'YYYY-MM-DD'
	ClientID string
}

// NoteView e a nota de um mes (calendar.notes).
type NoteView struct {
	Month     string    `json:"month"`
	Content   string    `json:"content"`
	UpdatedBy string    `json:"updatedBy,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// NoteInput e o body do PUT de notas.
type NoteInput struct {
	Content string `json:"content"`
}

// ============================================================================
// Config por conta (Fase 2)
// ============================================================================

// HolidayConfig liga/desliga cada conjunto de feriados/datas comemorativas.
type HolidayConfig struct {
	BrNational bool `json:"brNational"`
	Sergipe    bool `json:"sergipe"`
	Aracaju    bool `json:"aracaju"`
	LuxuryIntl bool `json:"luxuryIntl"`
}

// CalendarConfig e a config do calendario por account (jsonb em calendar.config).
// ResponsibleUserIDs vazio = todos os membros da conta aparecem como responsaveis.
type CalendarConfig struct {
	ResponsibleUserIDs []string      `json:"responsibleUserIds"`
	Holidays           HolidayConfig `json:"holidays"`
}

// defaultConfig e o estado inicial: todos os conjuntos de feriados ligados e
// nenhum responsavel filtrado (= todos os membros).
func defaultConfig() CalendarConfig {
	return CalendarConfig{
		ResponsibleUserIDs: []string{},
		Holidays:           HolidayConfig{BrNational: true, Sergipe: true, Aracaju: true, LuxuryIntl: true},
	}
}

// Member e um usuario da conta (candidato/atual a responsavel).
type Member struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
