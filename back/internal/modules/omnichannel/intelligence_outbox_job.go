package omnichannel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

type intelligenceAcceptedHandler struct {
	bridge CustomerIntelligenceBridge
}

func (h intelligenceAcceptedHandler) Handle(ctx context.Context, job jobs.Job) error {
	if h.bridge == nil {
		return &jobs.StatusError{
			StatusCode:    503,
			Unrecoverable: false,
			Err:           errors.New("omnichannel: customer intelligence bridge unavailable"),
		}
	}
	var event CustomerIntelligenceAcceptedOutcome
	if json.Unmarshal(job.Payload, &event) != nil ||
		event.AccountID != job.AccountID ||
		event.EventID == "" ||
		event.DecisionID == "" {
		return &jobs.StatusError{
			StatusCode:    422,
			Unrecoverable: true,
			Err:           errors.New("omnichannel: invalid intelligence accepted event"),
		}
	}
	return h.bridge.RecordAcceptedOutcome(ctx, event)
}

var _ jobs.Handler = intelligenceAcceptedHandler{}
