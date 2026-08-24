package metaads

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const runnerTestAccountID = "11111111-1111-4111-8111-111111111111"

func TestRunnerClientScopesHealthAndOAuthByAccount(t *testing.T) {
	t.Parallel()

	var seenHealth, seenStart, seenComplete bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer service-token" {
			t.Errorf("Authorization = %q", got)
		}
		switch r.URL.Path {
		case "/healthz":
			seenHealth = true
			if got := r.URL.Query().Get("accountId"); got != runnerTestAccountID {
				t.Errorf("health accountId = %q", got)
			}
			writeRunnerTestJSON(t, w, map[string]any{"ok": true, "claudeAuth": true, "metaAuth": "oauth"})
		case "/auth/start":
			seenStart = true
			assertRunnerBodyAccount(t, r, runnerTestAccountID)
			writeRunnerTestJSON(t, w, map[string]any{"url": "https://example.test/login"})
		case "/auth/complete":
			seenComplete = true
			assertRunnerBodyAccount(t, r, runnerTestAccountID)
			writeRunnerTestJSON(t, w, map[string]any{"ok": true, "detail": "connected"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client := NewRunnerClient(server.URL, "service-token")
	health, err := client.Health(context.Background(), runnerTestAccountID)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if health.MetaAuth != "oauth" {
		t.Fatalf("Health metaAuth = %q, want oauth", health.MetaAuth)
	}
	if _, err := client.AuthStart(context.Background(), runnerTestAccountID, RunnerOpts{}); err != nil {
		t.Fatalf("AuthStart: %v", err)
	}
	if _, err := client.AuthComplete(context.Background(), runnerTestAccountID, "", RunnerOpts{}); err != nil {
		t.Fatalf("AuthComplete: %v", err)
	}
	if !seenHealth || !seenStart || !seenComplete {
		t.Fatalf("routes called: health=%v start=%v complete=%v", seenHealth, seenStart, seenComplete)
	}
}

func TestRunnerClientMapsOAuthCallbackConflict(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		writeRunnerTestJSON(t, w, map[string]string{"error": "oauth_callback_conflict"})
	}))
	t.Cleanup(server.Close)

	client := NewRunnerClient(server.URL, "service-token")
	_, err := client.AuthStart(context.Background(), runnerTestAccountID, RunnerOpts{})
	if !errors.Is(err, errOAuthCallbackConflict) {
		t.Fatalf("AuthStart error = %v, want errOAuthCallbackConflict", err)
	}
}

func assertRunnerBodyAccount(t *testing.T, r *http.Request, want string) {
	t.Helper()
	var body struct {
		AccountID string `json:"accountId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.AccountID != want {
		t.Errorf("body accountId = %q, want %q", body.AccountID, want)
	}
}

func writeRunnerTestJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
