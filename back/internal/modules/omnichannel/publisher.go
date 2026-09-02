package omnichannel

import "context"

// Seam de realtime (F5). O modulo define o evento e o modulo `realtime` implementa o
// transporte sem ciclo de imports. A invalidacao e opaca e account-scoped: o consumidor
// refaz as leituras autorizadas em vez de receber IDs de recursos.

// Literais do canal. O boundary de invalidacao e o contrato seguro para mudancas de escopo.
const (
	RealtimeEventMessageCreated      = "message.created"
	RealtimeEventMessageUpdated      = "message.updated"
	RealtimeEventConversationUpdated = "conversation.updated"
	RealtimeEventInvalidate          = "omnichannel.invalidate"

	RealtimeInvalidationReasonMessageChanged     = "message_changed"
	RealtimeInvalidationReasonHistoryReset       = "history_reset"
	RealtimeInvalidationReasonAccessScopeChanged = "access_scope_changed"
)

// RealtimeEvent e um evento pronto para o transporte. AccountID vem SEMPRE do Principal.
// Invalidacoes nao usam ResourceID e carregam apenas reason/occurredAt; o boundary cria eventId.
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
