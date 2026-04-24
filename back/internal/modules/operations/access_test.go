package operations

import "testing"

func TestCanReadOperationsAllowsTenantReadOnlyRoles(t *testing.T) {
	roles := []string{RoleMarketing, RoleDirector}

	for _, role := range roles {
		if !CanAccessOperationsRole(role) {
			t.Fatalf("expected role %s to have read access", role)
		}
	}
}

func TestCanMutateOperationsKeepsTenantReadOnlyRolesBlocked(t *testing.T) {
	roles := []string{RoleMarketing, RoleDirector}

	for _, role := range roles {
		if CanMutateOperationsRole(role) {
			t.Fatalf("expected role %s to stay read-only", role)
		}
	}
}

func TestCanMutateOperationsAllowsStoreTerminal(t *testing.T) {
	if !CanMutateOperationsRole(RoleStoreTerminal) {
		t.Fatalf("expected role %s to mutate operations", RoleStoreTerminal)
	}
}
