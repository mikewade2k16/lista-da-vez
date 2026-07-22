package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

// ---- stub provider (controla sucesso/falha do SendMessage) ----

type stubProvider struct {
	result  channel.SendResult
	sendErr error
	calls   int
}

func (p *stubProvider) ID() string { return "stub" }
func (p *stubProvider) VerifyWebhook(http.Header, []byte, channel.Credentials) error {
	return nil
}
func (p *stubProvider) ParseWebhook(context.Context, http.Header, []byte) ([]channel.Event, error) {
	return nil, nil
}
func (p *stubProvider) SendMessage(_ context.Context, _ channel.Credentials, out channel.OutboundMessage) (channel.SendResult, error) {
	p.calls++
	if p.sendErr != nil {
		return channel.SendResult{}, p.sendErr
	}
	res := p.result
	if res.ExternalMessageID == "" {
		res.ExternalMessageID = "ext-" + out.IdempotencyKey
	}
	return res, nil
}
func (p *stubProvider) DownloadMedia(context.Context, channel.Credentials, channel.MediaRef) (io.ReadCloser, channel.MediaMeta, error) {
	return nil, channel.MediaMeta{}, errors.New("stub: no media")
}
func (p *stubProvider) SendReaction(context.Context, channel.Credentials, channel.ReactionInput) error {
	return nil
}
func (p *stubProvider) DeleteForAll(context.Context, channel.Credentials, channel.DeleteInput) error {
	return nil
}
func (p *stubProvider) Capabilities() channel.Capabilities { return channel.Capabilities{} }

// ---- fake store ----

type fakeOutboundStore struct {
	data           outboundSendData
	getErr         error
	sentID         string
	sentExt        string
	failedID       string
	auditKind      []string
	dispatchStatus string
}

func (f *fakeOutboundStore) DispatchOutbound(_ context.Context, _, _ string,
	send func(outboundSendData) (string, error)) (outboundDispatchResult, error) {
	result := outboundDispatchResult{
		MessageID: f.data.MessageID, ConversationID: f.data.ConversationID,
		Status: f.data.Status,
	}
	if f.getErr != nil {
		return result, f.getErr
	}
	if f.data.Origin == "ai" && (f.data.ConversationState != StateAIActive ||
		f.data.MessageAIGeneration == nil || *f.data.MessageAIGeneration != f.data.ConversationAIGeneration) {
		return result, ErrAILeaseInvalid
	}
	if f.data.Status != "PENDING" {
		return result, nil
	}
	ext, err := send(f.data)
	if err != nil {
		return result, err
	}
	f.sentID, f.sentExt = f.data.MessageID, ext
	result.Status = firstNonEmpty(f.dispatchStatus, "SENT")
	result.ExternalMessageID = ext
	result.UpdatedAt = time.Now()
	result.Dispatched = true
	return result, nil
}
func (f *fakeOutboundStore) MarkMessageFailed(_ context.Context, _, messageID string) (time.Time, error) {
	f.failedID = messageID
	return time.Now(), nil
}
func (f *fakeOutboundStore) InsertAudit(_ context.Context, _, _, _, _, eventType string, _ json.RawMessage) error {
	f.auditKind = append(f.auditKind, eventType)
	return nil
}

// ---- capturing publisher ----

type capturePublisher struct{ events []RealtimeEvent }

func (c *capturePublisher) PublishOmnichannelEvent(_ context.Context, evt RealtimeEvent) {
	c.events = append(c.events, evt)
}

func baseSendData() outboundSendData {
	phone := "5511999999999"
	return outboundSendData{
		MessageID:        "msg-1",
		ConversationID:   "conv-1",
		Status:           "PENDING",
		MessageType:      "TEXT",
		Content:          "ola",
		ToPhone:          &phone,
		ConversationExt:  "ext-conv",
		InstanceScopeKey: "inst-a",
		Provider:         "stub",
		Origin:           "human",
	}
}

func newJob(t *testing.T, attempts int) jobs.Job {
	t.Helper()
	payload, _ := json.Marshal(outboundJobPayload{MessageID: "msg-1", ConversationID: "conv-1"})
	return jobs.Job{AccountID: "acc-1", OrderingKey: "conv-1", Kind: OutboundJobKind,
		Payload: payload, Attempts: attempts, MaxAttempts: 5}
}

func TestOutboundHandlerSendsAndMarksSent(t *testing.T) {
	store := &fakeOutboundStore{data: baseSendData()}
	pub := &capturePublisher{}
	h := NewOutboundHandler(store, channel.NewRegistry(&stubProvider{}), nil, pub, nil)

	if err := h.Handle(context.Background(), newJob(t, 1)); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if store.sentID != "msg-1" || store.sentExt != "ext-msg-1" {
		t.Errorf("MarkMessageSent(id=%q ext=%q), want msg-1/ext-msg-1", store.sentID, store.sentExt)
	}
	if len(pub.events) != 1 || pub.events[0].Type != RealtimeEventMessageUpdated ||
		pub.events[0].Payload["status"] != "SENT" {
		t.Fatalf("publish = %+v, want 1 message.updated SENT", pub.events)
	}
	if len(store.auditKind) != 1 || store.auditKind[0] != "MESSAGE_OUTBOUND_SENT" {
		t.Errorf("audit = %v, want [MESSAGE_OUTBOUND_SENT]", store.auditKind)
	}
}

func TestOutboundHandlerAlreadySentIsNoop(t *testing.T) {
	data := baseSendData()
	data.Status = "SENT"
	store := &fakeOutboundStore{data: data}
	h := NewOutboundHandler(store, channel.NewRegistry(&stubProvider{}), nil, &capturePublisher{}, nil)

	if err := h.Handle(context.Background(), newJob(t, 1)); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if store.sentID != "" {
		t.Errorf("MarkMessageSent nao deveria ter sido chamado (id=%q)", store.sentID)
	}
}

func TestOutboundHandlerPublishesPersistedDeliveryState(t *testing.T) {
	for _, tc := range []struct {
		status string
		audit  string
	}{
		{status: "READ", audit: "MESSAGE_OUTBOUND_SENT"},
		{status: "FAILED", audit: "MESSAGE_OUTBOUND_FAILED"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			store := &fakeOutboundStore{data: baseSendData(), dispatchStatus: tc.status}
			pub := &capturePublisher{}
			h := NewOutboundHandler(store, channel.NewRegistry(&stubProvider{}), nil, pub, nil)

			if err := h.Handle(context.Background(), newJob(t, 1)); err != nil {
				t.Fatalf("Handle: %v", err)
			}
			if len(pub.events) != 1 || pub.events[0].Payload["status"] != tc.status {
				t.Fatalf("publish=%+v, want persisted %s", pub.events, tc.status)
			}
			if len(store.auditKind) != 1 || store.auditKind[0] != tc.audit {
				t.Fatalf("audit=%v, want %s", store.auditKind, tc.audit)
			}
		})
	}
}

func TestOutboundHandlerInvalidAILeaseNeverCallsProviderOrDuplicatesAudit(t *testing.T) {
	generation := int64(3)
	data := baseSendData()
	data.Origin = "ai"
	data.ConversationState = StateHumanActive
	data.ConversationAIGeneration = generation + 1
	data.MessageAIGeneration = &generation
	store := &fakeOutboundStore{data: data}
	provider := &stubProvider{}
	h := NewOutboundHandler(store, channel.NewRegistry(provider), nil, &capturePublisher{}, nil)

	err := h.Handle(context.Background(), newJob(t, 1))
	if err == nil || !jobs.Classify(err).Unrecoverable {
		t.Fatalf("Handle err=%v, want unrecoverable", err)
	}
	if provider.calls != 0 || store.failedID != "" || len(store.auditKind) != 0 {
		t.Fatalf("calls=%d failed=%q audit=%v", provider.calls, store.failedID, store.auditKind)
	}
}

func TestOutboundHandlerTerminalMarksFailed(t *testing.T) {
	// provider sem adapter (Provider="") => unrecoverable => terminal na 1a tentativa.
	data := baseSendData()
	data.Provider = ""
	store := &fakeOutboundStore{data: data}
	pub := &capturePublisher{}
	h := NewOutboundHandler(store, channel.NewRegistry(&stubProvider{}), nil, pub, nil)

	err := h.Handle(context.Background(), newJob(t, 1))
	if err == nil {
		t.Fatal("Handle deveria devolver erro terminal para o engine dead-letter")
	}
	if store.failedID != "msg-1" {
		t.Errorf("MarkMessageFailed(id=%q), want msg-1", store.failedID)
	}
	if len(pub.events) != 1 || pub.events[0].Payload["status"] != "FAILED" {
		t.Fatalf("publish = %+v, want message.updated FAILED", pub.events)
	}
	if len(store.auditKind) != 1 || store.auditKind[0] != "MESSAGE_OUTBOUND_FAILED" {
		t.Errorf("audit = %v, want [MESSAGE_OUTBOUND_FAILED]", store.auditKind)
	}
}

func TestOutboundHandlerTransientKeepsPending(t *testing.T) {
	// erro transitorio (timeout) na 1a de 5 tentativas => NAO terminal: mensagem segue PENDING.
	store := &fakeOutboundStore{data: baseSendData()}
	provider := &stubProvider{sendErr: context.DeadlineExceeded}
	h := NewOutboundHandler(store, channel.NewRegistry(provider), nil, &capturePublisher{}, nil)

	if err := h.Handle(context.Background(), newJob(t, 1)); err == nil {
		t.Fatal("Handle deveria devolver erro para o engine reagendar")
	}
	if store.failedID != "" {
		t.Errorf("MarkMessageFailed nao deveria ter sido chamado (transitorio), got %q", store.failedID)
	}
}
