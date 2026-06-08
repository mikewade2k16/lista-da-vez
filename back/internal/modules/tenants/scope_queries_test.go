package tenants

import (
	"strings"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func TestBuildListAccessibleQueryReadsTenantScopedMembershipsFromCore(t *testing.T) {
	query, args := buildListAccessibleQuery(auth.Principal{
		UserID: "user-1",
		Role:   auth.RoleDirector,
	}, ListInput{})

	assertQueryUsesCoreRoles(t, query)
	assertQuerySkipsLegacyScopeTables(t, query)
	assertStringSliceArgContains(t, args, 1, "queue.owner")
	assertStringSliceArgContains(t, args, 1, "queue.director")
	assertStringSliceArgContains(t, args, 1, "queue.marketing")
}

func TestBuildListAccessibleQueryReadsStoreScopedTenantsFromCoreSettings(t *testing.T) {
	query, args := buildListAccessibleQuery(auth.Principal{
		UserID: "user-1",
		Role:   auth.RoleManager,
	}, ListInput{})

	assertQueryUsesCoreRoles(t, query)
	assertQuerySkipsLegacyScopeTables(t, query)
	assertQueryContains(t, query, "core.user_module_settings")
	assertQueryContains(t, query, "storeIdsByAccount")
	assertQueryContains(t, query, "queue.stores")
	assertStringSliceArgContains(t, args, 1, "queue.manager")
}

func TestBuildFindAccessibleQueryReadsStoreScopedTenantFromCoreSettings(t *testing.T) {
	query, args := buildFindAccessibleQuery(auth.Principal{
		UserID: "user-1",
		Role:   auth.RoleConsultant,
	}, "account-1")

	assertQueryUsesCoreRoles(t, query)
	assertQuerySkipsLegacyScopeTables(t, query)
	assertQueryContains(t, query, "core.user_module_settings")
	assertQueryContains(t, query, "storeIdsByAccount")
	assertQueryContains(t, query, "queue.stores")
	assertStringSliceArgContains(t, args, 2, "queue.consultant")
}

func assertQueryUsesCoreRoles(t *testing.T, query string) {
	t.Helper()
	assertQueryContains(t, query, "core.account_users")
	assertQueryContains(t, query, "core.user_role_assignments")
	assertQueryContains(t, query, "core.roles")
}

func assertQuerySkipsLegacyScopeTables(t *testing.T, query string) {
	t.Helper()
	for _, legacyTable := range []string{"user_tenant_roles", "user_store_roles"} {
		if strings.Contains(query, legacyTable) {
			t.Fatalf("query still reads legacy table %s:\n%s", legacyTable, query)
		}
	}
}

func assertQueryContains(t *testing.T, query string, fragment string) {
	t.Helper()
	if !strings.Contains(query, fragment) {
		t.Fatalf("expected query to contain %q, got:\n%s", fragment, query)
	}
}

func assertStringSliceArgContains(t *testing.T, args []any, index int, want string) {
	t.Helper()
	if len(args) <= index {
		t.Fatalf("expected arg %d in %#v", index, args)
	}
	values, ok := args[index].([]string)
	if !ok {
		t.Fatalf("expected arg %d to be []string, got %T", index, args[index])
	}
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("expected arg %d to contain %q, got %#v", index, want, values)
}
