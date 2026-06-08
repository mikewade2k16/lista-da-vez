package consultants

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func TestRegisterRoutesServesOrphansRouteBeforeDynamicConsultantID(t *testing.T) {
	mux := http.NewServeMux()
	authService := auth.NewService(nil, nil, nil, nil, nil, nil, nil)
	middleware := auth.NewMiddleware(authService)
	RegisterRoutes(mux, NewService(nil, nil, "", ""), middleware)

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/consultants/orphans?tenantId=tenant-demo",
		nil,
	)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code == http.StatusMethodNotAllowed {
		t.Fatalf("expected GET /v1/consultants/orphans to match its route, got 405 with Allow=%q", recorder.Header().Get("Allow"))
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected auth middleware to handle the matched route with 401, got %d", recorder.Code)
	}
}
