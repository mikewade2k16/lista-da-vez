package calendar

import "context"

// Realtime do calendario (contrato C11). O Service publica eventos LEAN de invalidacao
// no canal calendar:account:{id}; o front so recebe a dica e refaz o fetch (nunca patch
// local). Espelho do tasks/publisher.go: o modulo calendar define a interface e o tipo do
// evento; o modulo realtime a implementa (realtime -> calendar, sem ciclo). Default no-op
// quando o realtime nao e injetado (ex.: testes de service).

// Tipos de evento publicados no canal do calendario (C11). Os MESMOS literais estao
// espelhados como constantes exportadas em realtime/model.go (canal de documentacao) — os
// dois lados precisam concordar.
const (
	realtimeEventCreated  = "calendar.event_created"
	realtimeEventUpdated  = "calendar.event_updated"
	realtimeEventDeleted  = "calendar.event_deleted"
	realtimeNoteUpdated   = "calendar.note_updated"
	realtimeConfigUpdated = "calendar.config_updated"
	realtimePlanUpdated   = "calendar.plan_updated"
	// WAVE 10: perfil estrategico do cliente mudou (PutClientProfile). ResourceID=clientId.
	realtimeClientProfileUpdated = "calendar.client_profile_updated"
)

// RealtimeEvent e o payload lean de um evento do canal do calendario (C11). O realtime
// mapeia ResourceID/Version para o Event e joga Date/MonthKey/Status no Payload map:
//   - event_created|updated|deleted: ResourceID=eventId, Date=YYYY-MM-DD, Version (updated)
//   - note_updated:                  MonthKey=YYYY-MM
//   - config_updated:                (sem campos extras)
//   - plan_updated:                  ResourceID=planId, Status
type RealtimeEvent struct {
	Type       string
	AccountID  string
	ResourceID string
	Date       string
	MonthKey   string
	Status     string
	Version    int
}

// Publisher entrega os eventos do calendario ao transporte realtime.
type Publisher interface {
	PublishCalendarEvent(ctx context.Context, evt RealtimeEvent)
}

// noopPublisher e o default quando o realtime nao e injetado (nunca publica).
type noopPublisher struct{}

func (noopPublisher) PublishCalendarEvent(context.Context, RealtimeEvent) {}

// WithPublisher injeta o Publisher do realtime no Module (contrato C11). Uso no app.go:
// calendar.New(storage, calendar.WithPublisher(realtimeService)). nil = canal desligado.
func WithPublisher(p Publisher) Option {
	return func(m *Module) { m.publisher = p }
}
