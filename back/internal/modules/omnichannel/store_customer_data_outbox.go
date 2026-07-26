package omnichannel

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type customerDataInboundSnapshot struct {
	AccountID              string
	ClientAccountID        string
	ContactID              string
	ConversationID         string
	MessageID              string
	ChannelClientBindingID string
	Channel                string
	Provider               string
	OccurredAt             time.Time
	BindingState           string
	FromMe                 bool
}

func (snapshot customerDataInboundSnapshot) eligible() bool {
	channel := strings.ToUpper(strings.TrimSpace(snapshot.Channel))
	return !snapshot.FromMe &&
		strings.TrimSpace(snapshot.AccountID) != "" &&
		strings.TrimSpace(snapshot.ClientAccountID) != "" &&
		strings.TrimSpace(snapshot.ContactID) != "" &&
		strings.TrimSpace(snapshot.ConversationID) != "" &&
		strings.TrimSpace(snapshot.MessageID) != "" &&
		strings.TrimSpace(snapshot.ChannelClientBindingID) != "" &&
		(channel == "WHATSAPP" || channel == "INSTAGRAM") &&
		strings.TrimSpace(snapshot.Provider) != "" &&
		strings.EqualFold(strings.TrimSpace(snapshot.BindingState), "resolved") &&
		!snapshot.OccurredAt.IsZero()
}

// insertCustomerDataInboundEventTx inserts the ID-only integration event in
// the same transaction as the newly-created inbound message. It never calls
// Customer Data; unresolved/quarantined bindings intentionally produce no
// external projection while the human inbox remains functional.
func (s *Store) insertCustomerDataInboundEventTx(
	ctx context.Context,
	tx pgx.Tx,
	snapshot customerDataInboundSnapshot,
) error {
	if !snapshot.eligible() {
		return nil
	}
	_, err := tx.Exec(ctx, `
		with event as (
			select gen_random_uuid() as id
		)
		insert into messaging.customer_data_outbox (
			id,
			account_id,
			client_account_id,
			contact_id,
			conversation_id,
			message_id,
			channel_client_binding_id,
			channel,
			provider,
			ordering_key,
			idempotency_key,
			kind,
			topic,
			schema_version,
			payload,
			max_attempts,
			occurred_at
		)
		select
			event.id,
			$1::uuid,
			$2::uuid,
			$3::uuid,
			$4::uuid,
			$5::uuid,
			$6::uuid,
			$7,
			$8,
			$3,
			'customer-data-inbound:' || $5,
			$9,
			$9,
			$10,
			jsonb_build_object(
				'schemaVersion', $10::text,
				'eventId', event.id::text,
				'accountId', $1::text,
				'clientAccountId', $2::text,
				'contactId', $3::text,
				'conversationId', $4::text,
				'messageId', $5::text,
				'channelClientBindingId', $6::text,
				'channel', $7::text,
				'provider', $8::text,
				'occurredAt', $11::timestamptz
			),
			8,
			$11::timestamptz
		from event
		on conflict do nothing`,
		snapshot.AccountID,
		snapshot.ClientAccountID,
		snapshot.ContactID,
		snapshot.ConversationID,
		snapshot.MessageID,
		snapshot.ChannelClientBindingID,
		strings.ToUpper(strings.TrimSpace(snapshot.Channel)),
		strings.TrimSpace(snapshot.Provider),
		customerDataRelationshipJobKind,
		customerDataInboundSchemaVersion,
		snapshot.OccurredAt.UTC(),
	)
	return err
}
