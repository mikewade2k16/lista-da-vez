package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
