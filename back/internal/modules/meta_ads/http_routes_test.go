package metaads

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func TestRegisterRoutesDoesNotMountLegacyRunnerAssistant(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, nil, auth.NewMiddleware(nil))

	legacyPaths := []string{
		"/v1/meta-ads/assistant/messages",
		"/v1/meta-ads/assistant/health",
		"/v1/meta-ads/assistant/auth/start",
		"/v1/meta-ads/assistant/auth/complete",
	}
	for _, path := range legacyPaths {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, nil)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != http.StatusNotFound {
				t.Fatalf("legacy runner route %s must stay unmounted; got status %d", path, response.Code)
			}
		})
	}
}

func TestWriteServiceErrorMapsChangedConnectionToConflict(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/meta-ads/sync", nil)
	response := httptest.NewRecorder()
	writeServiceError(response, request, ErrConnectionChanged, "internal")

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d", response.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "connection_changed" {
		t.Fatalf("code = %q", body.Error.Code)
	}
}
