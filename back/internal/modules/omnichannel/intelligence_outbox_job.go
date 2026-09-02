package omnichannel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

type intelligenceAcceptedHandler struct {
	bridge          CustomerIntelligenceBridge
	acceptanceLease func(context.Context, string, string, int64, func() error) (bool, error)
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
		event.DecisionID == "" || event.ConversationID == "" ||
		event.DispatchID == "" || event.Generation < 0 {
		return &jobs.StatusError{
			StatusCode:    422,
			Unrecoverable: true,
			Err:           errors.New("omnichannel: invalid intelligence accepted event"),
		}
	}
	if h.acceptanceLease == nil {
		return &jobs.StatusError{StatusCode: 503, Err: errors.New("omnichannel: intelligence lease unavailable")}
	}
	var bridgeErr error
	allowed, err := h.acceptanceLease(ctx, event.AccountID, event.DispatchID, event.Generation, func() error {
		bridgeErr = h.bridge.RecordAcceptedOutcome(ctx, event)
		return nil
	})
	if err != nil {
		return err
	}
	if !allowed {
		return &jobs.StatusError{Unrecoverable: true, Err: ErrHistoryResetInvalidated}
	}
	return bridgeErr
}

var _ jobs.Handler = intelligenceAcceptedHandler{}
