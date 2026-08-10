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
		PermissionTranscriptionsView,
		PermissionTranscriptionsEdit,
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

func TestSensitiveQueueWorkspaceDefaultsAreOwnerOnly(t *testing.T) {
	owner := DefaultRolePermissions(auth.RoleOwner)
	marketing := DefaultRolePermissions(auth.RoleMarketing)

	for _, key := range []string{
		PermissionTranscriptionsView,
		PermissionTranscriptionsEdit,
		PermissionCampaignsView,
		PermissionCampaignsEdit,
	} {
		if !HasPermission(owner, key) {
			t.Fatalf("expected owner permissions to include %s", key)
		}
		if HasPermission(marketing, key) {
			t.Fatalf("marketing permissions must not include %s", key)
		}
	}
}

func TestPlanningDefaultsUseDedicatedPermissions(t *testing.T) {
	t.Parallel()

	for _, role := range []auth.Role{auth.RoleManager, auth.RoleDirector, auth.RoleOwner, auth.RolePlatformAdmin} {
		permissions := DefaultRolePermissions(role)
		for _, key := range []string{PermissionPlanningView, PermissionPlanningEdit} {
			if !HasPermission(permissions, key) {
				t.Fatalf("expected %s permissions to include %s", role, key)
			}
		}
	}

	marketing := DefaultRolePermissions(auth.RoleMarketing)
	if !HasPermission(marketing, PermissionPlanningView) {
		t.Fatal("marketing must keep read-only planning access")
	}
	if HasPermission(marketing, PermissionPlanningEdit) {
		t.Fatal("marketing must not receive planning edit permission")
	}
}

func TestFeedbackDefaultsOpenConsultantWorkspaceForParticipants(t *testing.T) {
	t.Parallel()

	for _, role := range []auth.Role{auth.RoleConsultant, auth.RoleManager} {
		permissions := DefaultRolePermissions(role)
		for _, key := range []string{PermissionConsultantView, PermissionPerformanceFeedbackView} {
			if !HasPermission(permissions, key) {
				t.Fatalf("expected %s permissions to include %s", role, key)
			}
		}
	}

	if HasPermission(DefaultRolePermissions(auth.RoleConsultant), PermissionPerformanceFeedbackEdit) {
		t.Fatal("consultant must not receive performance feedback edit permission")
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

func TestPlatformAdminCanManageAccessEvenWithStaleResolvedPermissions(t *testing.T) {
	principal := auth.Principal{
		Role:                auth.RolePlatformAdmin,
		PermissionsResolved: true,
		Permissions:         []string{PermissionOperationsView},
	}

	if !canViewUserAccess(principal) {
		t.Fatal("platform_admin must view user access even when resolved permissions are stale")
	}
	if !canEditUserAccess(principal) {
		t.Fatal("platform_admin must edit user access even when resolved permissions are stale")
	}
	if !canEditRoleMatrix(principal) {
		t.Fatal("platform_admin must edit role matrix even when resolved permissions are stale")
	}
}
