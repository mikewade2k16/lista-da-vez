package metaads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetaClientPaginatesWithCursorWithoutFollowingNextURL(t *testing.T) {
	const token = "pagination-secret"
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/me/adaccounts" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if strings.Contains(r.URL.String(), token) || r.URL.Query().Get("access_token") != "" {
			t.Errorf("token leaked in URL: %s", r.URL.String())
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("after") == "" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "act_1", "account_id": "1", "name": "A"}},
				"paging": map[string]any{
					"next":    "https://attacker.invalid/steal?access_token=" + token,
					"cursors": map[string]string{"after": "opaque-cursor-2"},
				},
			})
			return
		}
		if r.URL.Query().Get("after") != "opaque-cursor-2" {
			t.Errorf("after = %q", r.URL.Query().Get("after"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"id": "act_2", "account_id": "2", "name": "B"}},
		})
	}))
	defer server.Close()

	client := NewMetaClient(server.URL)
	accounts, err := client.GetAdAccounts(context.Background(), token)
	if err != nil {
		t.Fatalf("GetAdAccounts: %v", err)
	}
	if requests != 2 || len(accounts) != 2 || accounts[0].ID != "act_1" || accounts[1].ID != "act_2" {
		t.Fatalf("requests=%d accounts=%#v", requests, accounts)
	}
}

func TestMetaClientRejectsRepeatedPagingCursor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"paging":{"next":"present","cursors":{"after":"same"}}}`))
	}))
	defer server.Close()

	client := NewMetaClient(server.URL)
	_, err := client.GetAdAccounts(context.Background(), "token")
	if err == nil || !strings.Contains(err.Error(), "cursor de paginacao repetido") {
		t.Fatalf("error = %v", err)
	}
}

func TestSyncDatePresetCoversLargestDashboardRange(t *testing.T) {
	if syncDatePreset != "last_90d" {
		t.Fatalf("syncDatePreset = %q", syncDatePreset)
	}
}
