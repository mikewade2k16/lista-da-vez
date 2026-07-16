package consultants

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func TestRegisterRoutesServesStoreStaffRoute(t *testing.T) {
	mux := http.NewServeMux()
	authService := auth.NewService(nil, nil, nil, nil, nil, nil, nil)
	middleware := auth.NewMiddleware(authService)
	RegisterRoutes(mux, NewService(nil, nil, "", ""), middleware)

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/store-staff?storeId=store-1",
		nil,
	)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code == http.StatusMethodNotAllowed {
		t.Fatalf("expected GET /v1/store-staff to match its route, got 405 with Allow=%q", recorder.Header().Get("Allow"))
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected auth middleware to handle the matched route with 401, got %d", recorder.Code)
	}
}

func TestStoreStaffServiceReturnsEmptyShapeWithoutAccessibleStores(t *testing.T) {
	service := NewService(&staffStubRepository{}, nil, "", "")

	principal := auth.Principal{
		UserID:              "user-1",
		Role:                auth.RoleOwner,
		TenantID:            "tenant-1",
		StoreIDs:            []string{},
		PermissionsResolved: false,
	}

	views, err := service.ListStoreStaff(context.Background(), principal, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if views == nil {
		t.Fatalf("expected non-nil slice for JSON array shape")
	}
	if len(views) != 0 {
		t.Fatalf("expected empty staff list, got %d", len(views))
	}

	payload, err := json.Marshal(storeStaffResponse{Items: views})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(payload) != `{"items":[]}` {
		t.Fatalf("unexpected JSON shape: %s", payload)
	}
}

func TestNormalizeStaffRole(t *testing.T) {
	cases := []struct {
		code     string
		template string
		want     string
	}{
		{"queue.manager", "queue.supervisor", "manager"},
		{"manager", "", "manager"},
		{"queue.store_terminal", "queue.supervisor", "cashier"},
		{"queue.caixa", "queue.supervisor", "caixa"},
		{"", "queue.supervisor", "manager"},
	}

	for _, tc := range cases {
		if got := normalizeStaffRole(tc.code, tc.template); got != tc.want {
			t.Fatalf("normalizeStaffRole(%q,%q) = %q, want %q", tc.code, tc.template, got, tc.want)
		}
	}
}

// staffStubRepository implementa Repository com apenas os metodos exercitados
// pelo teste de contrato; os demais retornam zero values.
type staffStubRepository struct{}

func (staffStubRepository) StoreExists(context.Context, string) (bool, error) { return false, nil }
func (staffStubRepository) CanAccessTenant(context.Context, auth.Principal, string) (bool, error) {
	return true, nil
}
func (staffStubRepository) ResolveStoreAccessContext(context.Context, string) (StoreAccessContext, error) {
	return StoreAccessContext{}, nil
}
func (staffStubRepository) ListByStore(context.Context, string) ([]Consultant, error) {
	return nil, nil
}
func (staffStubRepository) ListOrphansByTenant(context.Context, string) ([]Consultant, error) {
	return nil, nil
}
func (staffStubRepository) ListStoreStaff(context.Context, []string) ([]StoreStaffMember, error) {
	return []StoreStaffMember{}, nil
}
func (staffStubRepository) ListAccessibleStoreIDsForTenant(context.Context, string) ([]string, error) {
	return []string{}, nil
}
func (staffStubRepository) FindByID(context.Context, string) (Consultant, error) {
	return Consultant{}, nil
}
func (staffStubRepository) SyncLinkedIdentity(context.Context, string, string, string) error {
	return nil
}
func (staffStubRepository) SyncLinkedAccess(context.Context, LinkedAccessSyncInput) ([]string, error) {
	return nil, nil
}
func (staffStubRepository) Create(context.Context, Consultant, ConsultantAccessSeed) (Consultant, error) {
	return Consultant{}, nil
}
func (staffStubRepository) AttachAccess(context.Context, Consultant, ConsultantAccessSeed) (Consultant, error) {
	return Consultant{}, nil
}
func (staffStubRepository) Update(context.Context, Consultant) (Consultant, error) {
	return Consultant{}, nil
}
func (staffStubRepository) Archive(context.Context, string) error { return nil }
