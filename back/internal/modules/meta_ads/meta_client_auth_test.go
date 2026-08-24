package metaads

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetaClientUsesBearerWithoutPuttingTokenInURL(t *testing.T) {
	const token = "sensitive-access-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.String(), token) || r.URL.Query().Get("access_token") != "" {
			t.Errorf("token leaked in URL: %s", r.URL.String())
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	client := NewMetaClient(server.URL)
	if _, err := client.GetAdAccounts(context.Background(), token); err != nil {
		t.Fatalf("GetAdAccounts: %v", err)
	}
}
