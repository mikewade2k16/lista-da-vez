package omnichannel

import "context"

// Seam de realtime (F5). O modulo define a interface e o tipo do evento; o modulo `realtime`
// a implementa (direcao realtime -> omnichannel, sem ciclo), no mesmo padrao de
// calendar/publisher.go. O app injeta o realtimeService via WithPublisher (module.go). Default
// no-op quando nada e injetado, para o webhook nunca inlinar transporte (armadilha 4 da spec).
//
// DIVERGENCIA CONSCIENTE do canal calendar (registrada no realtime/AGENT.md, principio 4): o
// calendar manda payload LEAN de invalidacao (o front refaz fetch); o omnichannel carrega o
// payload COMPLETO porque o front e verbatim (D-B) e faz patch local (mergeMessages/
// upsertConversation). O shape de cada evento vem do CALL-SITE (spec F5 "shapes por call-site"),
// nunca unificado. Alvo de reavaliacao na F14.

// Literais dos 3 eventos do canal (spec F5). Espelhados como EventTypeOmnichannel* em
// realtime/model.go (canal de documentacao) — os dois lados precisam concordar.
const (
	RealtimeEventMessageCreated      = "message.created"
	RealtimeEventMessageUpdated      = "message.updated"
	RealtimeEventConversationUpdated = "conversation.updated"
)

// RealtimeEvent e um evento ja pronto para o transporte. AccountID vem SEMPRE do Principal
// (nunca do body). Payload e o shape do call-site, em camelCase e JA SANITIZADO (mediaUrl com
// data: -> ausente/null; nunca base64 no WS). ResourceID = messageId | conversationId.
type RealtimeEvent struct {
	Type       string
	AccountID  string
	ResourceID string
	Payload    map[string]any
}

// Publisher entrega os eventos do omnichannel ao transporte realtime (F5).
type Publisher interface {
	PublishOmnichannelEvent(ctx context.Context, evt RealtimeEvent)
}

// noopPublisher e o default: nunca publica (canal desligado; testes de service dispensam o
// realtime). NewInboundService substitui nil por este.
type noopPublisher struct{}

func (noopPublisher) PublishOmnichannelEvent(context.Context, RealtimeEvent) {}
