package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

type fakeMediaFetchStore struct {
	data       mediaFetchData
	getErr     error
	hidden     bool
	ready      *StoredMedia
	failedCode string
	audits     []string
}

func (f *fakeMediaFetchStore) GetMediaFetchData(context.Context, string, string) (mediaFetchData, error) {
	return f.data, f.getErr
}
func (f *fakeMediaFetchStore) IsMessageHistoryVisible(context.Context, string, string, string) (bool, error) {
	return !f.hidden, nil
}
func (f *fakeMediaFetchStore) WithMessageExternalEffectLease(_ context.Context, _, _, _ string, effect func() error) (bool, error) {
	if f.hidden {
		return false, nil
	}
	if effect != nil {
		return true, effect()
	}
	return true, nil
}
func (f *fakeMediaFetchStore) UpdateFetchedMedia(_ context.Context, _, _, _ string, media StoredMedia) (MessageView, error) {
	f.ready = &media
	return MessageView{ID: f.data.MessageID, MediaState: "ready"}, nil
}
func (f *fakeMediaFetchStore) MarkMediaFetchFailed(_ context.Context, _, _, _, code string) (MessageView, error) {
	f.failedCode = code
	return MessageView{ID: f.data.MessageID, MediaState: "failed", CanRetryMedia: true}, nil
}
func (f *fakeMediaFetchStore) InsertAudit(_ context.Context, _, _, _, _, event string, _ json.RawMessage) error {
	f.audits = append(f.audits, event)
	return nil
}

type fakeMediaProvider struct {
	reader      string
	meta        channel.MediaMeta
	err         error
	limit       int64
	lastRef     channel.MediaRef
	credentials channel.Credentials
	downloads   int
}

func (p *fakeMediaProvider) ID() string { return "media-test" }
func (p *fakeMediaProvider) VerifyWebhook(http.Header, []byte, channel.Credentials) error {
	return nil
}
func (p *fakeMediaProvider) ParseWebhook(context.Context, http.Header, []byte) ([]channel.Event, error) {
	return nil, nil
}
func (p *fakeMediaProvider) SendMessage(context.Context, channel.Credentials, channel.OutboundMessage) (channel.SendResult, error) {
	return channel.SendResult{}, nil
}
func (p *fakeMediaProvider) DownloadMedia(_ context.Context, credentials channel.Credentials, ref channel.MediaRef) (io.ReadCloser, channel.MediaMeta, error) {
	p.downloads++
	p.credentials, p.lastRef = credentials, ref
	if p.err != nil {
		return nil, channel.MediaMeta{}, p.err
	}
	return io.NopCloser(strings.NewReader(p.reader)), p.meta, nil
}

func TestMediaFetchHandlerHistoryResetStopsBeforeProvider(t *testing.T) {
	store := &fakeMediaFetchStore{data: baseMediaFetchData(), hidden: true}
	provider := &fakeMediaProvider{reader: "must-not-download"}
	handler := NewMediaFetchHandler(store, NewDiskMediaStorage(t.TempDir()), channel.NewRegistry(provider), nil, nil, nil)

	err := handler.Handle(context.Background(), mediaJob(t, 1))
	if err == nil || !jobs.Classify(err).Unrecoverable || !errors.Is(err, ErrHistoryResetInvalidated) {
		t.Fatalf("Handle err=%v, want history reset unrecoverable", err)
	}
	if provider.downloads != 0 || store.ready != nil || store.failedCode != "" {
		t.Fatalf("downloads=%d ready=%v failed=%q", provider.downloads, store.ready, store.failedCode)
	}
}
func (p *fakeMediaProvider) SendReaction(context.Context, channel.Credentials, channel.ReactionInput) error {
	return nil
}
func (p *fakeMediaProvider) DeleteForAll(context.Context, channel.Credentials, channel.DeleteInput) error {
	return nil
}
func (p *fakeMediaProvider) Capabilities() channel.Capabilities {
	return channel.Capabilities{MaxMediaBytes: p.limit}
}

type fakeProviderHTTPError struct{ status int }

func (e *fakeProviderHTTPError) Error() string       { return "provider failure" }
func (e *fakeProviderHTTPError) HTTPStatusCode() int { return e.status }

func baseMediaFetchData() mediaFetchData {
	return mediaFetchData{
		MessageID: "msg-1", ConversationID: "conv-1", InstanceScopeKey: "instance-1",
		ExternalMessageID: "external-1", MessageType: "IMAGE", MediaURL: "provider-ref",
		MimeType: "image/png", FileName: "photo.png", Provider: "media-test",
		ProviderConfig: map[string]string{"baseURL": "https://provider.invalid"}, MaxBytes: 1 << 20,
	}
}

func mediaJob(t *testing.T, attempts int) jobs.Job {
	t.Helper()
	payload, err := json.Marshal(mediaFetchJobPayload{MessageID: "msg-1"})
	if err != nil {
		t.Fatal(err)
	}
	return jobs.Job{AccountID: "acc-1", OrderingKey: "conv-1", Kind: MediaFetchJobKind,
		Payload: payload, Attempts: attempts, MaxAttempts: 5}
}

func TestMediaFetchHandlerStoresAndPublishesReady(t *testing.T) {
	store := &fakeMediaFetchStore{data: baseMediaFetchData()}
	provider := &fakeMediaProvider{reader: "image-bytes", meta: channel.MediaMeta{MimeType: "image/png", FileName: "provider.png"}}
	publisher := &capturePublisher{}
	handler := NewMediaFetchHandler(store, NewDiskMediaStorage(t.TempDir()), channel.NewRegistry(provider), nil, publisher, nil)

	if err := handler.Handle(context.Background(), mediaJob(t, 1)); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if store.ready == nil || store.ready.SHA256 == "" || store.ready.SizeBytes != int64(len("image-bytes")) {
		t.Fatalf("midia persistida = %#v", store.ready)
	}
	if provider.lastRef.InstanceName != "instance-1" || provider.lastRef.ExternalMessageID != "external-1" {
		t.Fatalf("ref = %#v", provider.lastRef)
	}
	if provider.credentials.Config["baseURL"] == "" {
		t.Fatal("configuracao da instancia nao chegou ao provider")
	}
	if len(publisher.events) != 1 || publisher.events[0].Payload["mediaState"] != "ready" {
		t.Fatalf("eventos = %#v", publisher.events)
	}
}

func TestMediaFetchHandlerRetriesProvider404BeforeFailing(t *testing.T) {
	for _, tc := range []struct {
		name       string
		attempts   int
		wantFailed bool
	}{
		{name: "temporary", attempts: 1, wantFailed: false},
		{name: "exhausted", attempts: 4, wantFailed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeMediaFetchStore{data: baseMediaFetchData()}
			provider := &fakeMediaProvider{err: &fakeProviderHTTPError{status: http.StatusNotFound}}
			handler := NewMediaFetchHandler(store, NewDiskMediaStorage(t.TempDir()), channel.NewRegistry(provider), nil, nil, nil)

			err := handler.Handle(context.Background(), mediaJob(t, tc.attempts))
			if err == nil {
				t.Fatal("Handle deveria devolver erro para o engine")
			}
			if got := store.failedCode != ""; got != tc.wantFailed {
				t.Fatalf("failed=%v code=%q, want %v", got, store.failedCode, tc.wantFailed)
			}
			if tc.wantFailed && store.failedCode != "provider_not_ready" {
				t.Fatalf("code = %q", store.failedCode)
			}
		})
	}
}

func TestMediaFetchHandlerMarksTerminalProviderAndStorageFailures(t *testing.T) {
	tests := []struct {
		name     string
		provider *fakeMediaProvider
		wantCode string
	}{
		{name: "unauthorized", provider: &fakeMediaProvider{err: &fakeProviderHTTPError{status: http.StatusUnauthorized}}, wantCode: "unauthorized"},
		{name: "unsupported mime", provider: &fakeMediaProvider{reader: "bad", meta: channel.MediaMeta{MimeType: "application/x-msdownload"}}, wantCode: "unsupported_media"},
		{name: "provider limit", provider: &fakeMediaProvider{reader: "too-big", meta: channel.MediaMeta{MimeType: "image/png"}, limit: 3}, wantCode: "media_too_large"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeMediaFetchStore{data: baseMediaFetchData()}
			handler := NewMediaFetchHandler(store, NewDiskMediaStorage(t.TempDir()), channel.NewRegistry(tc.provider), nil, nil, nil)
			err := handler.Handle(context.Background(), mediaJob(t, 1))
			if err == nil || !errors.As(err, new(*jobs.StatusError)) {
				t.Fatalf("erro = %v", err)
			}
			if store.failedCode != tc.wantCode {
				t.Fatalf("failed code = %q, want %q", store.failedCode, tc.wantCode)
			}
		})
	}
}
