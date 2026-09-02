package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

type aiInboundStoreFake struct {
	conv          convTriage
	convErr       error
	agent         agentRow
	agentEnabled  bool
	agentErr      error
	config        aiDispatchConfig
	configErr     error
	schemaEnabled bool
	upsertErr     error
	upsertCalls   int
	upsertAccount string
	upsertConv    string
	upsertVersion string
	upsertMessage string
	upsertAfter   time.Time
}

func (f *aiInboundStoreFake) ConvTriageContext(context.Context, string, string) (convTriage, error) {
	return f.conv, f.convErr
}

func (f *aiInboundStoreFake) ActiveAgentForInstance(context.Context, string, string) (agentRow, bool, error) {
	return f.agent, f.agentEnabled, f.agentErr
}

func (f *aiInboundStoreFake) AIDispatchConfig(context.Context, string, string) (aiDispatchConfig, error) {
	return f.config, f.configErr
}

func (f *aiInboundStoreFake) UpsertAIDispatch(
	_ context.Context,
	accountID string,
	conversationID string,
	versionID string,
	messageID string,
	runAfter time.Time,
) (AIDispatchRecord, error) {
	f.upsertCalls++
	f.upsertAccount = accountID
	f.upsertConv = conversationID
	f.upsertVersion = versionID
	f.upsertMessage = messageID
	f.upsertAfter = runAfter
	return AIDispatchRecord{}, f.upsertErr
}

func (f *aiInboundStoreFake) AIDispatchV2Enabled() bool {
	return f.schemaEnabled
}

type aiInboundDomainFake struct {
	transition func(Event) (State, error)
	events     []Event
	routeCalls int
	routeErr   error
}

func (f *aiInboundDomainFake) SystemTransition(
	_ context.Context,
	_, _ string,
	event Event,
	_ TransitionPayload,
) (State, error) {
	f.events = append(f.events, event)
	if f.transition == nil {
		return StateAIActive, nil
	}
	return f.transition(event)
}

func (f *aiInboundDomainFake) SystemRoute(context.Context, string, string) (State, error) {
	f.routeCalls++
	return StateQueued, f.routeErr
}

func TestAIInboundHandlerCreatesDurableDispatch(t *testing.T) {
	instanceID := "instance-1"
	versionID := "version-1"
	store := &aiInboundStoreFake{
		conv: convTriage{
			Found: true, State: string(StateAIActive), InstanceID: &instanceID,
		},
		agent:         agentRow{ActiveVersionID: &versionID},
		agentEnabled:  true,
		config:        aiDispatchConfig{DebounceMS: 125},
		schemaEnabled: true,
	}
	domain := &aiInboundDomainFake{}
	handler := newAIInboundHandler(store, domain, nil)
	job := validAIInboundJob(t)

	started := time.Now().UTC()
	if err := handler.Handle(context.Background(), job); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if store.upsertCalls != 1 {
		t.Fatalf("upsert calls = %d, want 1", store.upsertCalls)
	}
	if store.upsertAccount != job.AccountID ||
		store.upsertConv != "conversation-1" ||
		store.upsertVersion != versionID ||
		store.upsertMessage != "message-1" {
		t.Fatalf(
			"unexpected upsert scope: account=%q conv=%q version=%q message=%q",
			store.upsertAccount,
			store.upsertConv,
			store.upsertVersion,
			store.upsertMessage,
		)
	}
	if store.upsertAfter.Before(started.Add(100*time.Millisecond)) ||
		store.upsertAfter.After(time.Now().UTC().Add(250*time.Millisecond)) {
		t.Fatalf("run_after does not honor debounce: %s", store.upsertAfter)
	}
	if domain.routeCalls != 0 {
		t.Fatalf("route calls = %d, want 0", domain.routeCalls)
	}
}

func TestAIInboundHandlerRoutesHumanWhenDispatchSchemaIsUnavailable(t *testing.T) {
	instanceID := "instance-1"
	store := &aiInboundStoreFake{
		conv:          convTriage{Found: true, State: string(StateAIActive), InstanceID: &instanceID},
		schemaEnabled: false,
	}
	domain := &aiInboundDomainFake{
		transition: func(event Event) (State, error) {
			if event == EventAITriageFailed {
				return StateRouting, nil
			}
			return StateAIActive, nil
		},
	}

	if err := newAIInboundHandler(store, domain, nil).Handle(
		context.Background(), validAIInboundJob(t),
	); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if store.upsertCalls != 0 {
		t.Fatalf("upsert calls = %d, want 0", store.upsertCalls)
	}
	if domain.routeCalls != 1 {
		t.Fatalf("route calls = %d, want 1", domain.routeCalls)
	}
	wantEvents := []Event{EventAITriageFailed}
	if len(domain.events) != len(wantEvents) {
		t.Fatalf("events = %v, want %v", domain.events, wantEvents)
	}
	for i := range wantEvents {
		if domain.events[i] != wantEvents[i] {
			t.Fatalf("events = %v, want %v", domain.events, wantEvents)
		}
	}
}

func TestAIInboundHandlerDoesNotUndoLaterHumanState(t *testing.T) {
	for _, state := range []State{StateClosed, StateHumanActive, StatePending, StateQueued} {
		t.Run(string(state), func(t *testing.T) {
			store := &aiInboundStoreFake{
				conv:          convTriage{Found: true, State: string(state)},
				schemaEnabled: true,
			}
			domain := &aiInboundDomainFake{}
			if err := newAIInboundHandler(store, domain, nil).Handle(
				context.Background(), validAIInboundJob(t),
			); err != nil {
				t.Fatalf("Handle: %v", err)
			}
			if len(domain.events) != 0 || domain.routeCalls != 0 || store.upsertCalls != 0 {
				t.Fatalf(
					"delayed intent changed state: events=%v routes=%d upserts=%d",
					domain.events, domain.routeCalls, store.upsertCalls,
				)
			}
		})
	}
}

func TestAIInboundHandlerHistoryResetNeverRoutesOrRetries(t *testing.T) {
	instanceID := "instance-1"
	versionID := "version-1"
	store := &aiInboundStoreFake{
		conv:          convTriage{Found: true, State: string(StateAIActive), InstanceID: &instanceID},
		agent:         agentRow{ActiveVersionID: &versionID},
		agentEnabled:  true,
		config:        aiDispatchConfig{DebounceMS: 1},
		schemaEnabled: true,
		upsertErr:     ErrHistoryResetInvalidated,
	}
	domain := &aiInboundDomainFake{}
	err := newAIInboundHandler(store, domain, nil).Handle(context.Background(), validAIInboundJob(t))
	var statusErr *jobs.StatusError
	if !errors.As(err, &statusErr) || !statusErr.Unrecoverable || statusErr.StatusCode != 409 ||
		!errors.Is(statusErr, ErrHistoryResetInvalidated) {
		t.Fatalf("error=%v, want unrecoverable history reset", err)
	}
	if len(domain.events) != 0 || domain.routeCalls != 0 {
		t.Fatalf("history reset mutated FSM: events=%v routes=%d", domain.events, domain.routeCalls)
	}
}

func TestAIInboundHandlerRetriesThenFailsOpenOnTerminalError(t *testing.T) {
	instanceID := "instance-1"
	versionID := "version-1"
	infraErr := errors.New("temporary database error")
	newStore := func() *aiInboundStoreFake {
		return &aiInboundStoreFake{
			conv:          convTriage{Found: true, State: string(StateAIActive), InstanceID: &instanceID},
			agent:         agentRow{ActiveVersionID: &versionID},
			agentEnabled:  true,
			configErr:     infraErr,
			schemaEnabled: true,
		}
	}
	newDomain := func() *aiInboundDomainFake {
		return &aiInboundDomainFake{
			transition: func(event Event) (State, error) {
				if event == EventAITriageFailed {
					return StateRouting, nil
				}
				return StateAIActive, nil
			},
		}
	}

	nonTerminalStore := newStore()
	nonTerminalDomain := newDomain()
	nonTerminal := validAIInboundJob(t)
	nonTerminal.Attempts = 1
	if err := newAIInboundHandler(nonTerminalStore, nonTerminalDomain, nil).Handle(
		context.Background(), nonTerminal,
	); !errors.Is(err, infraErr) {
		t.Fatalf("non-terminal error = %v, want %v", err, infraErr)
	}
	if nonTerminalDomain.routeCalls != 0 {
		t.Fatalf("non-terminal route calls = %d, want 0", nonTerminalDomain.routeCalls)
	}

	terminalStore := newStore()
	terminalDomain := newDomain()
	terminal := validAIInboundJob(t)
	terminal.Attempts = 4 // no-status errors become terminal on the fourth attempt.
	if err := newAIInboundHandler(terminalStore, terminalDomain, nil).Handle(
		context.Background(), terminal,
	); err != nil {
		t.Fatalf("terminal Handle: %v", err)
	}
	if terminalDomain.routeCalls != 1 {
		t.Fatalf("terminal route calls = %d, want 1", terminalDomain.routeCalls)
	}
}

func TestAIInboundHandlerRejectsInvalidPayload(t *testing.T) {
	err := newAIInboundHandler(&aiInboundStoreFake{}, &aiInboundDomainFake{}, nil).Handle(
		context.Background(),
		jobs.Job{AccountID: "account-1", Payload: json.RawMessage(`{"messageId":"message-1"}`)},
	)
	var statusErr *jobs.StatusError
	if !errors.As(err, &statusErr) || !statusErr.Unrecoverable || statusErr.StatusCode != 422 {
		t.Fatalf("error = %v, want unrecoverable 422", err)
	}
}

func validAIInboundJob(t *testing.T) jobs.Job {
	t.Helper()
	payload, err := json.Marshal(aiInboundJobPayload{
		ConversationID: "conversation-1",
		MessageID:      "message-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	return jobs.Job{
		ID:          "job-1",
		AccountID:   "account-1",
		OrderingKey: "conversation-1",
		Kind:        AIInboundJobKind,
		Payload:     payload,
		Attempts:    1,
		MaxAttempts: 5,
	}
}
