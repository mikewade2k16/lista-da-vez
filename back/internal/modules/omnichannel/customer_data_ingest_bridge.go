package omnichannel

import (
	"context"
	"time"
)

const (
	customerDataInboundSchemaVersion = "omnichannel.customer_data.inbound.v1"
	customerDataRelationshipJobKind  = "omnichannel.customer_data.relationship.resolve"
)

// CustomerDataInboundEvent is owned by the Omnichannel consumer boundary. It
// intentionally carries identifiers only: Customer Data never receives a
// copied message body, name or phone from the integration outbox.
type CustomerDataInboundEvent struct {
	SchemaVersion          string    `json:"schemaVersion"`
	EventID                string    `json:"eventId"`
	AccountID              string    `json:"accountId"`
	ClientAccountID        string    `json:"clientAccountId"`
	ContactID              string    `json:"contactId"`
	ConversationID         string    `json:"conversationId"`
	MessageID              string    `json:"messageId"`
	ChannelClientBindingID string    `json:"channelClientBindingId"`
	Channel                string    `json:"channel"`
	Provider               string    `json:"provider"`
	OccurredAt             time.Time `json:"occurredAt"`
}

// CustomerDataInboundBridge resolves the channel-local contact into the
// deterministic Customer Data relationship. The concrete adapter belongs to
// the composition root; Omnichannel does not import Customer Data.
type CustomerDataInboundBridge interface {
	ResolveInboundRelationship(context.Context, CustomerDataInboundEvent) error
}
