package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

type customerDataInboundHandler struct {
	bridge CustomerDataInboundBridge
}

func (h customerDataInboundHandler) Handle(ctx context.Context, job jobs.Job) error {
	if h.bridge == nil {
		return &jobs.StatusError{
			StatusCode:    503,
			Unrecoverable: false,
			Err:           errors.New("omnichannel: customer data bridge unavailable"),
		}
	}
	var event CustomerDataInboundEvent
	if err := json.Unmarshal(job.Payload, &event); err != nil ||
		job.Kind != customerDataRelationshipJobKind ||
		job.OrderingKey != event.ContactID ||
		event.SchemaVersion != customerDataInboundSchemaVersion ||
		event.EventID == "" ||
		event.EventID != job.ID ||
		event.AccountID != job.AccountID ||
		event.ClientAccountID == "" ||
		event.ContactID == "" ||
		event.ConversationID == "" ||
		event.MessageID == "" ||
		event.ChannelClientBindingID == "" ||
		(event.Channel != "WHATSAPP" && event.Channel != "INSTAGRAM") ||
		strings.TrimSpace(event.Provider) == "" ||
		event.OccurredAt.IsZero() {
		return &jobs.StatusError{
			StatusCode:    422,
			Unrecoverable: true,
			Err:           errors.New("omnichannel: invalid customer data inbound event"),
		}
	}
	return h.bridge.ResolveInboundRelationship(ctx, event)
}

var _ jobs.Handler = customerDataInboundHandler{}
