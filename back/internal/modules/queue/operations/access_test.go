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

// A faixa de consultores precisa do roster para qualquer papel operador; o
// roster vai DENTRO do snapshot (canReadOperations), entao a mutacao continua
// exigindo a permissao de edicao — view resolvido NAO muta.
func TestCanMutateOperationsRequiresResolvedOperationEdit(t *testing.T) {
	viewOnly := AccessContext{
		Permissions:         []string{"workspace.operacao.view"},
		PermissionsResolved: true,
	}
	if canMutateOperations(viewOnly) {
		t.Fatal("expected resolved operation view-only permission to stay read-only")
	}

	canEdit := AccessContext{
		Permissions:         []string{"workspace.operacao.edit"},
		PermissionsResolved: true,
	}
	if !canMutateOperations(canEdit) {
		t.Fatal("expected resolved operation edit permission to mutate operations")
	}
}
