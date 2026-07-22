package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type mediaDeadlineWriter struct {
	header       http.Header
	readCleared  bool
	writeCleared bool
}

func (w *mediaDeadlineWriter) Header() http.Header            { return w.header }
func (w *mediaDeadlineWriter) Write(body []byte) (int, error) { return len(body), nil }
func (w *mediaDeadlineWriter) WriteHeader(int)                {}
func (w *mediaDeadlineWriter) SetReadDeadline(deadline time.Time) error {
	w.readCleared = deadline.IsZero()
	return nil
}
func (w *mediaDeadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.writeCleared = deadline.IsZero()
	return nil
}

type mediaTimeoutError struct{}

func (mediaTimeoutError) Error() string { return "read timeout" }
func (mediaTimeoutError) Timeout() bool { return true }

type slowMediaReader struct {
	remaining int
	delay     time.Duration
}

func (r *slowMediaReader) Read(buffer []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	time.Sleep(r.delay)
	buffer[0] = 'x'
	r.remaining--
	return 1, nil
}

func TestClearMediaUploadDeadlines(t *testing.T) {
	w := &mediaDeadlineWriter{header: make(http.Header)}
	if err := clearMediaUploadDeadlines(w); err != nil {
		t.Fatalf("clearMediaUploadDeadlines: %v", err)
	}
	if !w.readCleared || !w.writeCleared {
		t.Fatalf("deadlines nao foram removidos: read=%v write=%v", w.readCleared, w.writeCleared)
	}
}

func TestClearMediaUploadDeadlinesAcrossRealServerAndMiddlewares(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := httpapi.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := clearMediaUploadDeadlines(w); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			http.Error(w, err.Error(), http.StatusRequestTimeout)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}), httpapi.Logging(logger), httpapi.Gzip())

	server := httptest.NewUnstartedServer(handler)
	server.Config.ReadTimeout = 50 * time.Millisecond
	server.Config.WriteTimeout = 50 * time.Millisecond
	server.Start()
	t.Cleanup(server.Close)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, server.URL,
		&slowMediaReader{remaining: 8, delay: 30 * time.Millisecond})
	req.RequestURI = ""
	req.Header.Set("Accept-Encoding", "gzip")
	client := server.Client()
	client.Timeout = 2 * time.Second
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("upload lento deveria atravessar os deadlines globais: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status = %d, body = %q", res.StatusCode, string(body))
	}
}

func TestWriteMediaUploadReadError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "body too large", err: &http.MaxBytesError{Limit: 10}, wantStatus: http.StatusRequestEntityTooLarge, wantCode: "media_too_large"},
		{name: "timeout", err: mediaTimeoutError{}, wantStatus: http.StatusRequestTimeout, wantCode: "upload_timeout"},
		{name: "invalid multipart", err: errors.New("multipart invalido"), wantStatus: http.StatusBadRequest, wantCode: "invalid_media"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/calendar/media", nil)
			res := httptest.NewRecorder()
			writeMediaUploadReadError(res, req, tc.err)

			if res.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", res.Code, tc.wantStatus)
			}
			var payload httpapi.ErrorPayload
			if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
				t.Fatalf("decode error payload: %v", err)
			}
			if payload.Error.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q", payload.Error.Code, tc.wantCode)
			}
		})
	}
}
