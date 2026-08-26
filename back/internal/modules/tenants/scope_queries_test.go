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

func TestBuildListAccessibleQueryFiltersEnabledModuleForEveryScope(t *testing.T) {
	tests := []struct {
		name        string
		role        auth.Role
		placeholder string
		argIndex    int
	}{
		{name: "platform", role: auth.RolePlatformAdmin, placeholder: "$1", argIndex: 0},
		{name: "tenant", role: auth.RoleDirector, placeholder: "$3", argIndex: 2},
		{name: "store", role: auth.RoleManager, placeholder: "$3", argIndex: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query, args := buildListAccessibleQuery(auth.Principal{
				UserID: "user-1",
				Role:   test.role,
			}, ListInput{ModuleID: " omnichannel "})

			assertQueryContains(t, query, "core.account_modules")
			assertQueryContains(t, query, "am.module_id = "+test.placeholder)
			assertQueryContains(t, query, "am.enabled = true")
			if len(args) <= test.argIndex || args[test.argIndex] != "omnichannel" {
				t.Fatalf("expected module argument at %d, got %#v", test.argIndex, args)
			}
		})
	}
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

func TestBuildListAccessibleQueryBuildsOrganizationClientCatalogForCustomRole(t *testing.T) {
	query, args := buildListAccessibleQuery(auth.Principal{
		UserID: "user-agency-member",
		Role:   "",
	}, ListInput{ClientCatalog: true})

	assertQueryContains(t, query, "t.is_agency = false")
	assertQueryContains(t, query, "core.organization_users")
	assertQueryContains(t, query, "ou.organization_id = t.organization_id")
	assertQueryContains(t, query, "core.account_users")
	if len(args) != 1 || args[0] != "user-agency-member" {
		t.Fatalf("expected only user id arg, got %#v", args)
	}
}

func TestBuildListAccessibleQueryBuildsGlobalClientCatalogForPlatformAdmin(t *testing.T) {
	query, _ := buildListAccessibleQuery(auth.Principal{
		UserID: "platform-user",
		Role:   auth.RolePlatformAdmin,
	}, ListInput{ClientCatalog: true})

	assertQueryContains(t, query, "t.is_agency = false")
	assertQueryContains(t, query, "true or")
}

func TestBuildListAccessibleQueryCanIncludeInactiveClientsWithoutChangingOrganizationScope(t *testing.T) {
	query, args := buildListAccessibleQuery(auth.Principal{
		UserID: "user-agency-member",
	}, ListInput{ClientCatalog: true, IncludeInactive: true})

	assertQueryContains(t, query, "t.is_agency = false")
	assertQueryContains(t, query, "core.organization_users")
	if strings.Contains(query, "t.is_active = true") {
		t.Fatalf("catalogo com includeInactive nao deveria filtrar clientes inativos:\n%s", query)
	}
	if len(args) != 1 || args[0] != "user-agency-member" {
		t.Fatalf("expected organization-scoped user id arg, got %#v", args)
	}
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
