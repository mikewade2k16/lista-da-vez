package alerts

import (
	"context"
	"errors"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// --- resolveTenantScope ---

func TestResolveTenantScope_BlocksCrossAccountAccess(t *testing.T) {
	principal := auth.Principal{UserID: "u1", Role: auth.RoleOwner, TenantID: "tenant-a"}
	_, err := resolveTenantScope(principal, "tenant-b")
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("cross-tenant should return ErrForbidden; got %v", err)
	}
}

func TestResolveTenantScope_AllowsSameTenant(t *testing.T) {
	principal := auth.Principal{UserID: "u1", Role: auth.RoleOwner, TenantID: "tenant-a"}
	id, err := resolveTenantScope(principal, "tenant-a")
	if err != nil || id != "tenant-a" {
		t.Errorf("same tenant should pass; got id=%q err=%v", id, err)
	}
}

func TestResolveTenantScope_EmptyRequestedIDFallsBackToPrincipal(t *testing.T) {
	principal := auth.Principal{UserID: "u1", Role: auth.RoleOwner, TenantID: "tenant-a"}
	id, err := resolveTenantScope(principal, "")
	if err != nil || id != "tenant-a" {
		t.Errorf("empty requestedID should use principal.TenantID; got id=%q err=%v", id, err)
	}
}

func TestResolveTenantScope_PlatformAdminPassesAnyTenant(t *testing.T) {
	principal := auth.Principal{UserID: "admin", Role: auth.RolePlatformAdmin}
	id, err := resolveTenantScope(principal, "tenant-x")
	if err != nil || id != "tenant-x" {
		t.Errorf("platform_admin should access any tenant; got id=%q err=%v", id, err)
	}
}

func TestResolveTenantScope_PlatformAdminRequiresTenantID(t *testing.T) {
	principal := auth.Principal{UserID: "admin", Role: auth.RolePlatformAdmin}
	_, err := resolveTenantScope(principal, "")
	if !errors.Is(err, ErrTenantRequired) {
		t.Errorf("platform_admin without tenantID should return ErrTenantRequired; got %v", err)
	}
}

// --- resolveStoreScope ---

func TestResolveStoreScope_BlocksCrossStoreAccess(t *testing.T) {
	principal := auth.Principal{
		UserID:   "u1",
		Role:     auth.RoleConsultant,
		TenantID: "tenant-a",
		StoreIDs: []string{"store-1"},
	}
	_, _, err := resolveStoreScope(principal, "store-2")
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("store not in principal.StoreIDs should return ErrForbidden; got %v", err)
	}
}

func TestResolveStoreScope_AllowsOwnStore(t *testing.T) {
	principal := auth.Principal{
		UserID:   "u1",
		Role:     auth.RoleConsultant,
		TenantID: "tenant-a",
		StoreIDs: []string{"store-1", "store-2"},
	}
	storeID, storeIDs, err := resolveStoreScope(principal, "store-1")
	if err != nil || storeID != "store-1" || len(storeIDs) != 0 {
		t.Errorf("own store should pass; got storeID=%q storeIDs=%v err=%v", storeID, storeIDs, err)
	}
}

func TestResolveStoreScope_PlatformAdminPassesAnyStore(t *testing.T) {
	principal := auth.Principal{UserID: "admin", Role: auth.RolePlatformAdmin}
	storeID, _, err := resolveStoreScope(principal, "any-store")
	if err != nil || storeID != "any-store" {
		t.Errorf("platform_admin should pass any store; got storeID=%q err=%v", storeID, err)
	}
}

// --- canAccessAlert ---

func TestCanAccessAlert_BlocksAlertFromOtherTenant(t *testing.T) {
	principal := auth.Principal{UserID: "u1", Role: auth.RoleOwner, TenantID: "tenant-a"}
	alert := &Alert{ID: "alert-1", TenantID: "tenant-b"}
	if canAccessAlert(principal, alert) {
		t.Error("alert from different tenant should be blocked")
	}
}

func TestCanAccessAlert_AllowsAlertFromSameTenant(t *testing.T) {
	principal := auth.Principal{UserID: "u1", Role: auth.RoleOwner, TenantID: "tenant-a"}
	alert := &Alert{ID: "alert-1", TenantID: "tenant-a"}
	if !canAccessAlert(principal, alert) {
		t.Error("alert from same tenant should be allowed")
	}
}

// --- service-level integration ---

func TestServiceList_BlocksCrossAccountAccess(t *testing.T) {
	svc := NewService(&fakeRepository{})
	principal := auth.Principal{UserID: "u1", Role: auth.RoleOwner, TenantID: "tenant-a"}

	_, err := svc.List(context.Background(), principal, ListInput{TenantID: "tenant-b"})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("List with cross-tenant input should return ErrForbidden; got %v", err)
	}
}

func TestServiceOverview_BlocksCrossAccountAccess(t *testing.T) {
	svc := NewService(&fakeRepository{})
	principal := auth.Principal{UserID: "u1", Role: auth.RoleOwner, TenantID: "tenant-a"}

	_, err := svc.Overview(context.Background(), principal, OverviewInput{TenantID: "tenant-b"})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("Overview with cross-tenant input should return ErrForbidden; got %v", err)
	}
}
