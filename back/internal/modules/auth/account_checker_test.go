package auth

import (
	"strings"
	"testing"
)

// TestAccountAccessibleQueryMatchesCoreVisibility garante que o portao do middleware
// nao aceite account que /v2/me/accounts nao lista: platform_admin ou membership explicita.
func TestAccountAccessibleQueryMatchesCoreVisibility(t *testing.T) {
	q := accountAccessibleQuery

	checks := []struct {
		name string
		want string
	}{
		{"account ativa", "a.is_active = true"},
		{"platform_admin", "u.is_platform_admin = true"},
		{"membership ativa", "au.account_id = a.id and au.is_active = true"},
	}
	for _, c := range checks {
		if !strings.Contains(q, c.want) {
			t.Errorf("accountAccessibleQuery sem o caminho %q (esperava conter %q)", c.name, c.want)
		}
	}
	if strings.Contains(q, "core.organization_users") || strings.Contains(q, "agency_owner") {
		t.Fatalf("membership de organizacao nao pode conceder contexto de cliente:\n%s", q)
	}
}

// TestAccountAccessibleQueryIsParameterized garante uso de $1/$2 e nenhum
// id concatenado (sem superficie de SQLi).
func TestAccountAccessibleQueryIsParameterized(t *testing.T) {
	q := accountAccessibleQuery
	if !strings.Contains(q, "a.id = $1::uuid") {
		t.Error("accountAccessibleQuery deve filtrar a account por $1::uuid")
	}
	if !strings.Contains(q, "$2::uuid") {
		t.Error("accountAccessibleQuery deve usar $2::uuid para o user")
	}
	if strings.Contains(q, "fmt.Sprintf") || strings.Contains(q, "+ accountID") {
		t.Error("accountAccessibleQuery nao deve concatenar ids na query")
	}
}

func TestAccountPermissionsQueryUsesEffectiveAccountRBAC(t *testing.T) {
	q := accountPermissionsQuery
	checks := []string{
		"core.user_role_assignments",
		"core.role_permissions",
		"core.user_permission_overrides",
		"upo.effect = 'allow'",
		"upo.effect = 'deny'",
		"ura.account_id = $1::uuid",
		"ura.user_id = $2::uuid",
	}
	for _, want := range checks {
		if !strings.Contains(q, want) {
			t.Errorf("accountPermissionsQuery deveria conter %q", want)
		}
	}
}
