package customerintelligence

import (
	"context"
	"errors"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type permissionCheckerFake struct {
	allowed bool
	calls   int
}

func (f *permissionCheckerFake) HasAccountPermission(
	context.Context,
	string,
	string,
	string,
) (bool, error) {
	f.calls++
	return f.allowed, nil
}

func TestPermissionGateDoesNotBypassAccountRBACForOwnerOrPlatformAdmin(t *testing.T) {
	t.Parallel()
	for _, role := range []auth.Role{auth.RoleOwner, auth.RolePlatformAdmin} {
		role := role
		t.Run(string(role), func(t *testing.T) {
			t.Parallel()
			checker := &permissionCheckerFake{allowed: false}
			gate := newPermissionGate(checker)
			err := gate.Authorize(context.Background(), auth.Principal{
				AccountID: testAccount,
				UserID:    testSubject,
				Role:      role,
			}, PermissionSourcesManage)
			if !errors.Is(err, ErrForbidden) {
				t.Fatalf("role %s ignorou RBAC da account: %v", role, err)
			}
			if checker.calls != 1 {
				t.Fatalf("checker calls=%d, want 1", checker.calls)
			}
		})
	}
}

func TestPermissionGateKeepsPlatformPermissionsExplicit(t *testing.T) {
	t.Parallel()
	checker := &permissionCheckerFake{allowed: true}
	gate := newPermissionGate(checker)
	if err := gate.Authorize(context.Background(), auth.Principal{
		AccountID: testAccount,
		UserID:    testSubject,
		Role:      auth.RoleOwner,
	}, PermissionPromptsPlatform); !errors.Is(err, ErrForbidden) {
		t.Fatalf("owner recebeu permissao platform-only: %v", err)
	}
	if err := gate.Authorize(context.Background(), auth.Principal{
		AccountID: testAccount,
		UserID:    testSubject,
		Role:      auth.RolePlatformAdmin,
	}, PermissionPromptsPlatform); err != nil {
		t.Fatalf("platform_admin explicito rejeitado: %v", err)
	}
	if checker.calls != 0 {
		t.Fatalf("permissao platform-only consultou grants de account: %d", checker.calls)
	}
}
