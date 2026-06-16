package auth

import (
	"strings"
	"testing"
)

// TestAccountAccessibleQueryCoversThreePaths garante que o portao de membership
// do middleware (IsMember) permanece org-aware: platform_admin, agency_owner e
// membership explicita. Se alguem reduzir a query para so core.account_users, o
// login-agencia volta a levar 403 ao usar a conta-agencia (board Tasks). Espelha
// a visibilidade de core.ListAccountsForUser (Etapa 3 do AGENCY_TENANT plan).
func TestAccountAccessibleQueryCoversThreePaths(t *testing.T) {
	q := accountAccessibleQuery

	checks := []struct {
		name string
		want string
	}{
		{"account ativa", "a.is_active = true"},
		{"platform_admin", "u.is_platform_admin = true"},
		{"agency_owner", "ou.org_role = 'agency_owner'"},
		{"agency_owner casa a org da account", "ou.organization_id = a.organization_id"},
		{"membership ativa", "au.account_id = a.id and au.is_active = true"},
	}
	for _, c := range checks {
		if !strings.Contains(q, c.want) {
			t.Errorf("accountAccessibleQuery sem o caminho %q (esperava conter %q)", c.name, c.want)
		}
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
