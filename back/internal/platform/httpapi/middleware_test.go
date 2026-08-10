package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type deadlineResponseWriter struct {
	header        http.Header
	readDeadline  time.Time
	writeDeadline time.Time
	readSet       bool
	writeSet      bool
}

func (w *deadlineResponseWriter) Header() http.Header            { return w.header }
func (w *deadlineResponseWriter) Write(body []byte) (int, error) { return len(body), nil }
func (w *deadlineResponseWriter) WriteHeader(int)                {}
func (w *deadlineResponseWriter) SetReadDeadline(deadline time.Time) error {
	w.readDeadline = deadline
	w.readSet = true
	return nil
}
func (w *deadlineResponseWriter) SetWriteDeadline(deadline time.Time) error {
	w.writeDeadline = deadline
	w.writeSet = true
	return nil
}

func TestStatusRecorderUnwrapsForResponseController(t *testing.T) {
	inner := &deadlineResponseWriter{header: make(http.Header)}
	recorder := &statusRecorder{ResponseWriter: inner, status: http.StatusOK}
	controller := http.NewResponseController(recorder)
	readDeadline := time.Now().Add(time.Minute)
	writeDeadline := time.Now().Add(2 * time.Minute)

	if err := controller.SetReadDeadline(readDeadline); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	if err := controller.SetWriteDeadline(writeDeadline); err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}
	if !inner.readDeadline.Equal(readDeadline) {
		t.Fatalf("read deadline = %v, want %v", inner.readDeadline, readDeadline)
	}
	if !inner.writeDeadline.Equal(writeDeadline) {
		t.Fatalf("write deadline = %v, want %v", inner.writeDeadline, writeDeadline)
	}
}

func TestResponseControllerTraversesGzipAndLoggingWriters(t *testing.T) {
	inner := &deadlineResponseWriter{header: make(http.Header)}
	recorder := &statusRecorder{ResponseWriter: inner, status: http.StatusOK}
	gzipWriter := &gzipResponseWriter{ResponseWriter: recorder, compress: true}
	controller := http.NewResponseController(gzipWriter)

	if err := controller.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	if err := controller.SetWriteDeadline(time.Time{}); err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}
	if !inner.readSet || !inner.writeSet || !inner.readDeadline.IsZero() || !inner.writeDeadline.IsZero() {
		t.Fatalf("deadlines nao foram zerados: read_set=%v write_set=%v read=%v write=%v",
			inner.readSet, inner.writeSet, inner.readDeadline, inner.writeDeadline)
	}
}

// TestCORS_PublicRouteWildcard: qualquer origem em /v1/public/* ganha
// Access-Control-Allow-Origin: * e nunca Allow-Credentials.
func TestCORS_PublicRouteWildcard(t *testing.T) {
	handler := CORS([]string{"https://painel.omni.app"})(newOkHandler())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/public/restaurants/x", nil)
	req.Header.Set("Origin", "https://site-aleatorio-do-cliente.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Allow-Origin: esperava *, recebi %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Allow-Credentials nao deveria existir em rota publica, recebi %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Fatalf("Allow-Methods: esperava 'GET, POST, OPTIONS', recebi %q", got)
	}
}

// TestCORS_PublicPreflightNoContent: OPTIONS em /v1/public/* responde 204.
func TestCORS_PublicPreflightNoContent(t *testing.T) {
	handler := CORS(nil)(newOkHandler())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/v1/public/resolve", nil)
	req.Header.Set("Origin", "https://qualquer.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("preflight publico: esperava 204, recebi %d", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("preflight publico Allow-Origin: esperava *, recebi %q", got)
	}
}

// TestCORS_NormalRouteOutsideAllowlist: rota normal com origem fora da allowlist
// NAO recebe header de Allow-Origin (segue o fluxo allowlist intacto).
func TestCORS_NormalRouteOutsideAllowlist(t *testing.T) {
	handler := CORS([]string{"https://painel.omni.app"})(newOkHandler())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/cardapio/restaurants", nil)
	req.Header.Set("Origin", "https://site-aleatorio-do-cliente.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("rota normal fora da allowlist nao deveria ganhar Allow-Origin, recebi %q", got)
	}
}

// TestCORS_NormalRouteAllowedOrigin: rota normal com origem na allowlist recebe
// o proprio Origin (comportamento preexistente preservado).
func TestCORS_NormalRouteAllowedOrigin(t *testing.T) {
	handler := CORS([]string{"https://painel.omni.app"})(newOkHandler())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/cardapio/restaurants", nil)
	req.Header.Set("Origin", "https://painel.omni.app")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://painel.omni.app" {
		t.Fatalf("rota normal allowlist: esperava o proprio origin, recebi %q", got)
	}
}

func TestCORSAllowsIdempotencyKeyForControlledUploads(t *testing.T) {
	handler := CORS([]string{"http://localhost:3003"})(newOkHandler())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/v1/storage/test-upload", nil)
	req.Header.Set("Origin", "http://localhost:3003")
	req.Header.Set("Access-Control-Request-Headers", "authorization,x-account-id,idempotency-key")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("preflight: esperava 204, recebi %d", rr.Code)
	}
	if !strings.Contains(strings.ToLower(rr.Header().Get("Access-Control-Allow-Headers")), "idempotency-key") {
		t.Fatalf("Idempotency-Key ausente em Access-Control-Allow-Headers: %q", rr.Header().Get("Access-Control-Allow-Headers"))
	}
}
