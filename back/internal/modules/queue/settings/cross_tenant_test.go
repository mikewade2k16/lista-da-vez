package settings

import (
	"context"
	"errors"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// TestGetBundle_BlocksCrossAccountAccess prova o isolamento cross-tenant na camada de service:
// um owner de "tenant-a" nao pode ler configuracoes de "tenant-b" mesmo passando o tenantID
// explicitamente. Regressao para IDOR via parametro de query.
func TestGetBundle_BlocksCrossAccountAccess(t *testing.T) {
	repo := &fakeRepository{
		accessible: map[string]bool{"tenant-a": true, "tenant-b": true},
	}
	svc := NewService(repo, nil)

	principal := auth.Principal{UserID: "u1", Role: auth.RoleOwner, TenantID: "tenant-a"}
	_, err := svc.GetBundle(context.Background(), principal, "tenant-b")
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("GetBundle cross-tenant should return ErrForbidden; got %v", err)
	}
}

func TestGetBundle_AllowsSameTenant(t *testing.T) {
	repo := &fakeRepository{
		accessible: map[string]bool{"tenant-a": true},
		records:    map[string]Record{"tenant-a": {TenantID: "tenant-a"}},
	}
	svc := NewService(repo, nil)

	principal := auth.Principal{UserID: "u1", Role: auth.RoleOwner, TenantID: "tenant-a"}
	bundle, err := svc.GetBundle(context.Background(), principal, "tenant-a")
	if err != nil {
		t.Fatalf("same-tenant GetBundle should pass; got %v", err)
	}
	if bundle.TenantID != "tenant-a" {
		t.Errorf("expected TenantID=tenant-a; got %q", bundle.TenantID)
	}
}

// TestResolveTenantID_BlocksCrossAccountAccess testa o metodo privado diretamente.
// Garante que o bloqueio ocorre antes de qualquer consulta ao banco (sem DB mock).
func TestResolveTenantID_BlocksCrossAccountAccess(t *testing.T) {
	svc := NewService(&fakeRepository{}, nil)
	principal := auth.Principal{UserID: "u1", Role: auth.RoleOwner, TenantID: "tenant-a"}

	_, err := svc.resolveTenantID(context.Background(), principal, "tenant-b")
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("resolveTenantID cross-tenant should return ErrForbidden; got %v", err)
	}
}

func TestResolveTenantID_PlatformAdminPassesAnyTenant(t *testing.T) {
	repo := &fakeRepository{accessible: map[string]bool{"tenant-x": true}}
	svc := NewService(repo, nil)

	principal := auth.Principal{UserID: "admin", Role: auth.RolePlatformAdmin}
	id, err := svc.resolveTenantID(context.Background(), principal, "tenant-x")
	if err != nil || id != "tenant-x" {
		t.Errorf("platform_admin should access any tenant; got id=%q err=%v", id, err)
	}
}

func TestResolveTenantID_NonMemberBlockedByCanAccessTenant(t *testing.T) {
	// Sem TenantID no principal (sem account ativa), mas tenta acessar um tenant
	// ao qual nao pertence — CanAccessTenant bloqueia.
	repo := &fakeRepository{accessible: map[string]bool{"tenant-a": false}}
	svc := NewService(repo, nil)

	principal := auth.Principal{UserID: "u-outsider", Role: auth.RoleOwner}
	_, err := svc.resolveTenantID(context.Background(), principal, "tenant-a")
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("non-member should be blocked by CanAccessTenant; got %v", err)
	}
}
