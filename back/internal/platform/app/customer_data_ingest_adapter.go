package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerdata"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerintelligence"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel"
)

type customerDataRelationshipResolver interface {
	ResolveRelationship(
		context.Context,
		customerdata.ResolveRelationshipRequest,
	) (customerdata.ResolveRelationshipResult, error)
}

type customerIntelligenceSourceEventTrigger interface {
	TriggerSourceEvent(
		context.Context,
		customerintelligence.SourceEventRequest,
	) (customerintelligence.SourceEventResult, error)
}

type omnichannelCustomerDataSource struct {
	DisplayName      string
	Phone            string
	ExternalID       string
	InstanceScopeKey string
	SourceOccurredAt time.Time
}

type omnichannelCustomerDataSourceReader interface {
	ReadInboundCustomerDataSource(
		context.Context,
		omnichannel.CustomerDataInboundEvent,
	) (omnichannelCustomerDataSource, error)
}

type postgresOmnichannelCustomerDataSourceReader struct {
	pool *pgxpool.Pool
}

func (reader postgresOmnichannelCustomerDataSourceReader) ReadInboundCustomerDataSource(
	ctx context.Context,
	event omnichannel.CustomerDataInboundEvent,
) (omnichannelCustomerDataSource, error) {
	if reader.pool == nil {
		return omnichannelCustomerDataSource{}, customerdata.ErrNotFound
	}
	var source omnichannelCustomerDataSource
	err := reader.pool.QueryRow(ctx, `
		select
			coalesce(contact.name, ''),
			coalesce(contact.phone, ''),
			coalesce(identity.external_id, ''),
			conversation.instance_scope_key,
			touchpoint.occurred_at
		from messaging.messages message
		join messaging.conversations conversation
		  on conversation.account_id = message.account_id
		 and conversation.id = message.conversation_id
		join messaging.contacts contact
		  on contact.account_id = conversation.account_id
		 and contact.id = conversation.contact_id
		join messaging.contact_touchpoints touchpoint
		  on touchpoint.account_id = message.account_id
		 and touchpoint.message_id = message.id
		 and touchpoint.conversation_id = conversation.id
		 and touchpoint.contact_id = contact.id
		 and touchpoint.client_account_id = conversation.client_account_id
		 and touchpoint.channel_client_binding_id = conversation.channel_client_binding_id
		left join lateral (
			select candidate.external_id
			from messaging.contact_identities candidate
			where candidate.account_id = contact.account_id
			  and candidate.contact_id = contact.id
			  and candidate.channel = $7
			  and candidate.provider = $8
			  and candidate.instance_scope_key = conversation.instance_scope_key
			order by candidate.last_seen_at desc, candidate.id
			limit 1
		) identity on true
		where message.account_id = $1::uuid
		  and conversation.client_account_id = $2::uuid
		  and contact.id = $3::uuid
		  and conversation.id = $4::uuid
		  and message.id = $5::uuid
		  and conversation.channel_client_binding_id = $6::uuid
		  and conversation.client_binding_state = 'resolved'
		  and message.direction = 'INBOUND'
		  and message.origin = 'contact'
		  and touchpoint.channel = $7
		  and touchpoint.provider = $8
		  and touchpoint.occurred_at = $9::timestamptz
		limit 1`,
		event.AccountID,
		event.ClientAccountID,
		event.ContactID,
		event.ConversationID,
		event.MessageID,
		event.ChannelClientBindingID,
		event.Channel,
		event.Provider,
		event.OccurredAt.UTC(),
	).Scan(
		&source.DisplayName,
		&source.Phone,
		&source.ExternalID,
		&source.InstanceScopeKey,
		&source.SourceOccurredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return omnichannelCustomerDataSource{}, customerdata.ErrNotFound
	}
	return source, err
}

// omnichannelCustomerDataIngestAdapter is the only component that knows both
// module contracts. It rehydrates the minimum verified identity from the
// Omnichannel source of truth and delegates all matching/creation policy to
// Customer Data.
type omnichannelCustomerDataIngestAdapter struct {
	source               omnichannelCustomerDataSourceReader
	customerData         func() customerDataRelationshipResolver
	customerIntelligence func() customerIntelligenceSourceEventTrigger
}

func (adapter omnichannelCustomerDataIngestAdapter) ResolveInboundRelationship(
	ctx context.Context,
	event omnichannel.CustomerDataInboundEvent,
) error {
	if adapter.source == nil || adapter.customerData == nil {
		return customerdata.ErrNotFound
	}
	service := adapter.customerData()
	if service == nil {
		return customerdata.ErrNotFound
	}
	source, err := adapter.source.ReadInboundCustomerDataSource(ctx, event)
	if err != nil {
		return err
	}
	identities := customerDataIdentityInputs(
		event.Channel,
		event.ContactID,
		source.Phone,
		source.ExternalID,
		event.OccurredAt,
	)
	resolved, err := service.ResolveRelationship(ctx, customerdata.ResolveRelationshipRequest{
		AccountID:       event.AccountID,
		ClientAccountID: event.ClientAccountID,
		RequestID:       "omnichannel-contact:" + event.ClientAccountID + ":" + event.ContactID,
		Source: customerdata.SourceReference{
			SourceModule:     "omnichannel",
			SourceKey:        strings.ToLower(strings.TrimSpace(event.Channel)),
			SourceEntityType: "contact",
			SourceEntityID:   event.ContactID,
			SourceVersion:    event.SchemaVersion,
		},
		Identities:  identities,
		DisplayName: strings.TrimSpace(source.DisplayName),
		OccurredAt:  event.OccurredAt.UTC(),
		Purpose:     "customer_service",
		AllowCreate: true,
	})
	if err != nil {
		return err
	}
	switch resolved.Status {
	case "created", "resolved":
		if adapter.customerIntelligence == nil {
			return nil
		}
		trigger := adapter.customerIntelligence()
		if trigger == nil {
			return errors.New("customer intelligence: source event trigger unavailable")
		}
		_, err = trigger.TriggerSourceEvent(ctx, customerintelligence.SourceEventRequest{
			AccountID:       event.AccountID,
			ClientAccountID: event.ClientAccountID,
			SourceKey:       "omnichannel",
			RelationshipID:  resolved.RelationshipID,
			EventID:         event.EventID,
		})
		return err
	case "candidate", "quarantined", "not_found":
		return nil
	default:
		return fmt.Errorf("customer data: unsupported relationship resolution status")
	}
}

func customerDataIdentityInputs(
	channel, contactID, phone, externalID string,
	occurredAt time.Time,
) []customerdata.IdentityInput {
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	verified := func(kind, value string) customerdata.IdentityInput {
		return customerdata.IdentityInput{
			Kind:               kind,
			Issuer:             "omnichannel",
			Value:              value,
			VerificationStatus: "verified",
			VerificationMethod: "provider_inbound",
			SourceRefType:      "messaging.contact",
			SourceRefID:        contactID,
			OccurredAt:         &occurredAt,
		}
	}
	channel = strings.ToUpper(strings.TrimSpace(channel))
	phone = strings.TrimSpace(phone)
	externalID = strings.TrimSpace(externalID)
	var identities []customerdata.IdentityInput
	switch channel {
	case "WHATSAPP":
		if phone == "" {
			phone = whatsappIdentityValue(externalID)
		}
		if phone != "" {
			identities = append(identities, verified("whatsapp", phone))
		}
	case "INSTAGRAM":
		if externalID != "" {
			identities = append(identities, verified("instagram", externalID))
		}
	}
	return identities
}

func whatsappIdentityValue(value string) string {
	local := strings.TrimSpace(strings.SplitN(value, "@", 2)[0])
	var normalized strings.Builder
	for _, char := range local {
		if unicode.IsDigit(char) || (char == '+' && normalized.Len() == 0) {
			normalized.WriteRune(char)
		}
	}
	return normalized.String()
}

var _ omnichannel.CustomerDataInboundBridge = omnichannelCustomerDataIngestAdapter{}
