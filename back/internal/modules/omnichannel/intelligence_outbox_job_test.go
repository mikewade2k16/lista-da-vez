package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

type intelligenceBridgeRecorder struct {
	outcomes []CustomerIntelligenceAcceptedOutcome
}

func (f *intelligenceBridgeRecorder) ExecuteInteraction(
	context.Context,
	CustomerIntelligenceInteractionRequest,
) (CustomerIntelligenceDecision, error) {
	return CustomerIntelligenceDecision{}, nil
}

func (f *intelligenceBridgeRecorder) RecordAcceptedOutcome(
	_ context.Context,
	outcome CustomerIntelligenceAcceptedOutcome,
) error {
	f.outcomes = append(f.outcomes, outcome)
	return nil
}

func TestIntelligenceAcceptedHandlerDeliversScopedEvent(t *testing.T) {
	bridge := &intelligenceBridgeRecorder{}
	event := CustomerIntelligenceAcceptedOutcome{
		EventID:         "243a6aa9-e19d-5106-97ea-e3cb16c9a412",
		AccountID:       "10000000-0000-4000-8000-000000000001",
		ClientAccountID: "10000000-0000-4000-8000-000000000002",
		ConversationID:  "20000000-0000-4000-8000-000000000001",
		DispatchID:      "30000000-0000-4000-8000-000000000001",
		DecisionID:      "decision-1",
		SubjectID:       "40000000-0000-4000-8000-000000000001",
		RelationshipID:  "50000000-0000-4000-8000-000000000001",
		Generation:      7,
		Outcome:         "reply",
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	handler := intelligenceAcceptedHandler{bridge: bridge, acceptanceLease: allowIntelligenceEffect}
	if err := handler.Handle(context.Background(), jobs.Job{
		AccountID: event.AccountID,
		Payload:   payload,
	}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(bridge.outcomes) != 1 || bridge.outcomes[0].EventID != event.EventID {
		t.Fatalf("evento nao entregue: %#v", bridge.outcomes)
	}
}

func TestIntelligenceAcceptedHandlerRejectsCrossAccountPayload(t *testing.T) {
	bridge := &intelligenceBridgeRecorder{}
	payload := json.RawMessage(`{
		"eventId":"243a6aa9-e19d-5106-97ea-e3cb16c9a412",
		"accountId":"10000000-0000-4000-8000-000000000001",
		"decisionId":"decision-1"
	}`)
	err := (intelligenceAcceptedHandler{bridge: bridge, acceptanceLease: allowIntelligenceEffect}).Handle(
		context.Background(),
		jobs.Job{
			AccountID: "10000000-0000-4000-8000-000000000099",
			Payload:   payload,
		},
	)
	if err == nil || len(bridge.outcomes) != 0 {
		t.Fatalf("payload cross-account deveria ser rejeitado: err=%v", err)
	}
}

func TestIntelligenceAcceptedHandlerStopsAfterHistoryReset(t *testing.T) {
	bridge := &intelligenceBridgeRecorder{}
	event := CustomerIntelligenceAcceptedOutcome{
		EventID:         "243a6aa9-e19d-5106-97ea-e3cb16c9a412",
		AccountID:       "10000000-0000-4000-8000-000000000001",
		ClientAccountID: "10000000-0000-4000-8000-000000000002",
		ConversationID:  "20000000-0000-4000-8000-000000000001",
		DispatchID:      "30000000-0000-4000-8000-000000000001",
		DecisionID:      "decision-1",
		Generation:      7,
		Outcome:         "reply",
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	handlerErr := (intelligenceAcceptedHandler{
		bridge: bridge,
		acceptanceLease: func(context.Context, string, string, int64, func() error) (bool, error) {
			return false, nil
		},
	}).Handle(context.Background(), jobs.Job{AccountID: event.AccountID, Payload: payload})
	var statusErr *jobs.StatusError
	if !errors.As(handlerErr, &statusErr) || !statusErr.Unrecoverable ||
		!errors.Is(handlerErr, ErrHistoryResetInvalidated) || len(bridge.outcomes) != 0 {
		t.Fatalf("history reset reached intelligence bridge: err=%v outcomes=%#v", handlerErr, bridge.outcomes)
	}
}

func allowIntelligenceEffect(_ context.Context, _, _ string, _ int64, effect func() error) (bool, error) {
	if effect != nil {
		return true, effect()
	}
	return true, nil
}
