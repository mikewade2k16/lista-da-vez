package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

type customerDataInboundBridgeStub struct {
	events []CustomerDataInboundEvent
	err    error
}

func (stub *customerDataInboundBridgeStub) ResolveInboundRelationship(
	_ context.Context,
	event CustomerDataInboundEvent,
) error {
	stub.events = append(stub.events, event)
	return stub.err
}

func validCustomerDataInboundEvent() CustomerDataInboundEvent {
	return CustomerDataInboundEvent{
		SchemaVersion:          customerDataInboundSchemaVersion,
		EventID:                "8f85d172-a483-411e-bbb9-668eb98a2ae2",
		AccountID:              "609a986a-e4b9-4dbe-a209-2d543a462419",
		ClientAccountID:        "ae078a93-c7dd-4fcb-b399-9e5a18526d1d",
		ContactID:              "a9009b91-4c30-415f-892f-e1653a14061e",
		ConversationID:         "4f24a24c-b6f0-49ff-9e52-80640b131c12",
		MessageID:              "053ace70-3c0c-4f6b-9761-a7947ff0e9ea",
		ChannelClientBindingID: "a9084194-1267-45bb-996b-842929b10311",
		Channel:                "WHATSAPP",
		Provider:               "evolution",
		OccurredAt:             time.Date(2026, 7, 23, 18, 0, 0, 0, time.UTC),
	}
}

func TestCustomerDataInboundHandlerRunsIndependentlyFromAI(t *testing.T) {
	t.Parallel()
	event := validCustomerDataInboundEvent()
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	bridge := &customerDataInboundBridgeStub{}
	handler := customerDataInboundHandler{bridge: bridge, historyLease: allowCustomerDataHistory}
	err = handler.Handle(context.Background(), jobs.Job{
		ID: event.EventID, AccountID: event.AccountID, OrderingKey: event.ContactID,
		Kind: customerDataRelationshipJobKind, Payload: payload,
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(bridge.events) != 1 || bridge.events[0].MessageID != event.MessageID {
		t.Fatalf("unexpected bridge events: %#v", bridge.events)
	}
}

func TestCustomerDataInboundHandlerRejectsCrossAccountPayload(t *testing.T) {
	t.Parallel()
	event := validCustomerDataInboundEvent()
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	bridge := &customerDataInboundBridgeStub{}
	handler := customerDataInboundHandler{bridge: bridge, historyLease: allowCustomerDataHistory}
	err = handler.Handle(context.Background(), jobs.Job{
		ID: event.EventID, AccountID: "cc09ab06-f2e7-4726-a84c-a3b3f054ddea",
		OrderingKey: event.ContactID, Kind: customerDataRelationshipJobKind, Payload: payload,
	})
	var statusErr *jobs.StatusError
	if !errors.As(err, &statusErr) || !statusErr.Unrecoverable || statusErr.StatusCode != 422 {
		t.Fatalf("expected terminal 422, got %v", err)
	}
	if len(bridge.events) != 0 {
		t.Fatal("cross-account event reached bridge")
	}
}

func TestCustomerDataInboundHandlerStopsAfterHistoryReset(t *testing.T) {
	t.Parallel()
	event := validCustomerDataInboundEvent()
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	bridge := &customerDataInboundBridgeStub{}
	handlerErr := (customerDataInboundHandler{
		bridge: bridge,
		historyLease: func(context.Context, string, string, string, func() error) (bool, error) {
			return false, nil
		},
	}).Handle(context.Background(), jobs.Job{
		ID: event.EventID, AccountID: event.AccountID, OrderingKey: event.ContactID,
		Kind: customerDataRelationshipJobKind, Payload: payload,
	})
	var statusErr *jobs.StatusError
	if !errors.As(handlerErr, &statusErr) || !statusErr.Unrecoverable ||
		!errors.Is(handlerErr, ErrHistoryResetInvalidated) || len(bridge.events) != 0 {
		t.Fatalf("history reset reached customer data bridge: err=%v events=%#v", handlerErr, bridge.events)
	}
}

func allowCustomerDataHistory(_ context.Context, _, _, _ string, effect func() error) (bool, error) {
	if effect != nil {
		return true, effect()
	}
	return true, nil
}

func TestCustomerDataInboundEventIsIDOnly(t *testing.T) {
	t.Parallel()
	payload, err := json.Marshal(validCustomerDataInboundEvent())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{
		"contactName", "contactPhone", "contactExternalId", "messageContent", "prompt",
	} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("payload contains forbidden field %q: %s", forbidden, payload)
		}
	}
}

func TestCustomerDataInboundSnapshotEligibility(t *testing.T) {
	t.Parallel()
	base := customerDataInboundSnapshot{
		AccountID:              "account",
		ClientAccountID:        "client",
		ContactID:              "contact",
		ConversationID:         "conversation",
		MessageID:              "message",
		ChannelClientBindingID: "binding",
		Channel:                "WHATSAPP",
		Provider:               "evolution",
		OccurredAt:             time.Now().UTC(),
		BindingState:           "resolved",
	}
	if !base.eligible() {
		t.Fatal("resolved inbound should be eligible")
	}
	unresolved := base
	unresolved.BindingState = "unresolved"
	if unresolved.eligible() {
		t.Fatal("unresolved inbound must not feed Customer Data")
	}
	fromMe := base
	fromMe.FromMe = true
	if fromMe.eligible() {
		t.Fatal("provider-device outbound must not feed Customer Data")
	}
	unsupported := base
	unsupported.Channel = "EMAIL"
	if unsupported.eligible() {
		t.Fatal("unsupported channel must not block inbound by reaching the outbox")
	}
}

var _ CustomerDataInboundBridge = (*customerDataInboundBridgeStub)(nil)
