package metaads

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListPagesWithInstagramRejectsRepeatedCursor(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"paging":{"next":"present","cursors":{"after":"same"}}}`))
	}))
	defer server.Close()

	_, err := NewMetaClient(server.URL).ListPagesWithInstagram(context.Background(), "token")
	if err == nil || !strings.Contains(err.Error(), "cursor de paginacao repetido") {
		t.Fatalf("error = %v", err)
	}
}

func TestListPagesWithInstagramFailsAtSafePageCap(t *testing.T) {
	t.Parallel()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{},
			"paging": map[string]any{
				"next": "present",
				"cursors": map[string]string{
					"after": fmt.Sprintf("cursor-%d-%s", requests, r.URL.Query().Get("after")),
				},
			},
		})
	}))
	defer server.Close()

	_, err := NewMetaClient(server.URL).ListPagesWithInstagram(context.Background(), "token")
	if err == nil || !strings.Contains(err.Error(), "limite seguro de paginas excedido") {
		t.Fatalf("error = %v", err)
	}
	if requests != instagramAccountMaxPages {
		t.Fatalf("requests=%d want=%d", requests, instagramAccountMaxPages)
	}
}
