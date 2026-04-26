package access

import (
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func TestDefaultRolePermissionsReflectsCurrentPanelBaseline(t *testing.T) {
	permissions := DefaultRolePermissions(auth.RolePlatformAdmin)

	expected := []string{
		PermissionOperationsView,
		PermissionOperationsEdit,
		PermissionClientsView,
		PermissionClientsEdit,
		PermissionUsersView,
		PermissionUsersEdit,
		PermissionSettingsView,
		PermissionSettingsEdit,
		PermissionRoleMatrixEdit,
	}

	for _, key := range expected {
		if !HasPermission(permissions, key) {
			t.Fatalf("expected default platform admin permissions to include %s", key)
		}
	}
}

func TestEffectivePermissionKeysAppliesUserOverridesOnTopOfRoleDefaults(t *testing.T) {
	base := DefaultRolePermissions(auth.RoleOwner)
	overrides := []UserOverride{
		{PermissionKey: PermissionReportsView, Effect: EffectDeny, IsActive: true},
		{PermissionKey: PermissionCampaignsEdit, Effect: EffectDeny, IsActive: true},
		{PermissionKey: PermissionRoleMatrixEdit, Effect: EffectAllow, IsActive: true},
		{PermissionKey: PermissionUsersPasswordEdit, Effect: EffectAllow, IsActive: false},
	}

	effective := EffectivePermissionKeys(base, overrides)

	if HasPermission(effective, PermissionReportsView) {
		t.Fatalf("expected reports permission to be removed by deny override")
	}

	if HasPermission(effective, PermissionCampaignsEdit) {
		t.Fatalf("expected campaigns edit permission to be removed by deny override")
	}

	if !HasPermission(effective, PermissionRoleMatrixEdit) {
		t.Fatalf("expected allow override to add role matrix permission")
	}

	if HasPermission(effective, PermissionUsersPasswordEdit) {
		t.Fatalf("inactive overrides must not change the effective permission set")
	}
}
