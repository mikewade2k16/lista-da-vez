package core

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// O PostgresRepository carrega um *pgxpool.Pool concreto, dificil de fingir sem
// um Postgres real (nao ha infra de teste de integracao no modulo core). Por
// isso estes testes seguem o padrao de TestBuildScopedQueryReadsUsersFromCore
// (users/core_projection_test.go): validam o CONTRATO da query org-aware mais a
// traducao de erro de FindAccountIfMember via scannable fake. Os 3 caminhos de
// visibilidade (platform_admin / agency_owner / membership) sao verificados no
// nivel da clausula SQL que decide o escopo no banco.

// TestAccountVisibilityCoversThreeAccessPaths garante que a clausula de escopo
// contempla os tres caminhos org-aware e que cada um le da tabela correta.
func TestAccountVisibilityCoversThreeAccessPaths(t *testing.T) {
	cases := []struct {
		name     string
		fragment string
	}{
		// (a) platform_admin ve todas as accounts ativas.
		{"platform_admin", "u.is_platform_admin = true"},
		{"platform_admin_table", "from core.users u"},
		// (b) agency_owner ve as accounts da sua organization.
		{"agency_owner_role", "ou.org_role = 'agency_owner'"},
		{"agency_owner_org_match", "ou.organization_id = a.organization_id"},
		{"agency_owner_table", "from core.organization_users ou"},
		// (c) membership explicita ativa (comportamento legado, inalterado).
		{"membership_table", "from core.account_users au"},
		{"membership_active", "au.is_active = true"},
		{"membership_account_match", "au.account_id = a.id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(accountVisibilityWhere, tc.fragment) {
				t.Fatalf("accountVisibilityWhere missing %q:\n%s", tc.fragment, accountVisibilityWhere)
			}
		})
	}
}

// TestAccountVisibilityIsParameterizedByUser confirma que o escopo depende do
// user via $1 (parametrizado, sem concatenacao de valores) e exige account ativa.
func TestAccountVisibilityIsParameterizedByUser(t *testing.T) {
	if !strings.Contains(accountVisibilityWhere, "$1::uuid") {
		t.Fatalf("expected user scope to be parameterized via $1::uuid:\n%s", accountVisibilityWhere)
	}
	if !strings.Contains(accountVisibilityWhere, "a.is_active = true") {
		t.Fatalf("expected visibility to require active account:\n%s", accountVisibilityWhere)
	}
	// As tres ramificacoes sao unidas por OR num unico predicado.
	if strings.Count(accountVisibilityWhere, " or exists (") != 2 {
		t.Fatalf("expected 3 OR-ed exists() branches:\n%s", accountVisibilityWhere)
	}
}

// TestListAccountsForUserQueryContract valida ordenacao, parametrizacao e que a
// query nao multiplica linhas (sem JOIN -> sem duplicatas, DISTINCT dispensavel).
func TestListAccountsForUserQueryContract(t *testing.T) {
	// Membership-first: as accounts do user vem antes (case ... then 0 else 1),
	// depois desempate por nome. defaultAccountID (summaries[0]) cai na "casa".
	if !strings.Contains(listAccountsForUserQuery, "then 0 else 1 end") {
		t.Fatalf("expected membership-first ordering (case ... then 0 else 1 end):\n%s", listAccountsForUserQuery)
	}
	if !strings.Contains(listAccountsForUserQuery, "lower(a.name) asc") {
		t.Fatalf("expected secondary ordering by lower(a.name):\n%s", listAccountsForUserQuery)
	}
	if !strings.Contains(listAccountsForUserQuery, "$1::uuid") {
		t.Fatalf("expected query parameterized by $1::uuid:\n%s", listAccountsForUserQuery)
	}
	// A regra de escopo precisa estar embutida (reutiliza accountVisibilityWhere).
	if !strings.Contains(listAccountsForUserQuery, "u.is_platform_admin = true") ||
		!strings.Contains(listAccountsForUserQuery, "ou.org_role = 'agency_owner'") ||
		!strings.Contains(listAccountsForUserQuery, "from core.account_users au") {
		t.Fatalf("list query must embed the 3-path visibility rule:\n%s", listAccountsForUserQuery)
	}
	// Sem JOIN multiplicador -> nenhuma chance de linha duplicada por account.
	if strings.Contains(listAccountsForUserQuery, "join core.account_users") {
		t.Fatalf("list query should not JOIN account_users (would duplicate rows):\n%s", listAccountsForUserQuery)
	}
	// Switcher precisa de is_agency (gate do Manage admin-global) e do nome da
	// organization (agrupamento), via left join que nao multiplica linhas (1:1).
	if !strings.Contains(listAccountsForUserQuery, "a.is_agency") {
		t.Fatalf("list query must select a.is_agency for the switcher:\n%s", listAccountsForUserQuery)
	}
	if !strings.Contains(listAccountsForUserQuery, "coalesce(o.name, '')") {
		t.Fatalf("list query must select coalesce(o.name, '') for organizationName:\n%s", listAccountsForUserQuery)
	}
	if !strings.Contains(listAccountsForUserQuery, "left join core.organizations o on o.id = a.organization_id") {
		t.Fatalf("list query must left join core.organizations for organizationName:\n%s", listAccountsForUserQuery)
	}
}

// TestFindAccountIfAccessibleQueryContract valida que a busca single-account usa
// o mesmo escopo, filtra por $2 e nao usa string building.
func TestFindAccountIfAccessibleQueryContract(t *testing.T) {
	if !strings.Contains(findAccountIfAccessibleQuery, "a.id = $2::uuid") {
		t.Fatalf("expected single-account filter via $2::uuid:\n%s", findAccountIfAccessibleQuery)
	}
	if !strings.Contains(findAccountIfAccessibleQuery, "$1::uuid") {
		t.Fatalf("expected user scope via $1::uuid:\n%s", findAccountIfAccessibleQuery)
	}
	if !strings.Contains(findAccountIfAccessibleQuery, "u.is_platform_admin = true") ||
		!strings.Contains(findAccountIfAccessibleQuery, "ou.org_role = 'agency_owner'") ||
		!strings.Contains(findAccountIfAccessibleQuery, "from core.account_users au") {
		t.Fatalf("find query must embed the same 3-path visibility rule:\n%s", findAccountIfAccessibleQuery)
	}
	// Mesmas colunas extras da list (scanAccount e compartilhado): is_agency +
	// organizationName via left join 1:1 com core.organizations.
	if !strings.Contains(findAccountIfAccessibleQuery, "a.is_agency") {
		t.Fatalf("find query must select a.is_agency for the switcher:\n%s", findAccountIfAccessibleQuery)
	}
	if !strings.Contains(findAccountIfAccessibleQuery, "coalesce(o.name, '')") {
		t.Fatalf("find query must select coalesce(o.name, '') for organizationName:\n%s", findAccountIfAccessibleQuery)
	}
	if !strings.Contains(findAccountIfAccessibleQuery, "left join core.organizations o on o.id = a.organization_id") {
		t.Fatalf("find query must left join core.organizations for organizationName:\n%s", findAccountIfAccessibleQuery)
	}
}

// fakeAccountRow implementa scannable para exercitar accountFromAccessibleRow
// sem Postgres real.
type fakeAccountRow struct {
	values []any
	err    error
}

func (row fakeAccountRow) Scan(dest ...any) error {
	if row.err != nil {
		return row.err
	}
	for index, value := range row.values {
		switch target := dest[index].(type) {
		case *string:
			*target = value.(string)
		case **string:
			if value == nil {
				*target = nil
			} else {
				str := value.(string)
				*target = &str
			}
		case *bool:
			*target = value.(bool)
		case *time.Time:
			*target = value.(time.Time)
		default:
			return errors.New("fakeAccountRow: unexpected dest type")
		}
	}
	return nil
}

// TestFindAccountOutsideScopeReturnsNotMember cobre o caso de uma account FORA do
// escopo (nenhum dos 3 caminhos): a query devolve zero linhas (pgx.ErrNoRows) e o
// repository traduz para ErrAccountNotMember sem vazar existencia.
func TestFindAccountOutsideScopeReturnsNotMember(t *testing.T) {
	_, err := accountFromAccessibleRow(fakeAccountRow{err: pgx.ErrNoRows})
	if !errors.Is(err, ErrAccountNotMember) {
		t.Fatalf("expected ErrAccountNotMember for out-of-scope account, got %v", err)
	}
}

// TestFindAccountAccessibleReturnsAccount cobre o caminho feliz: quando a query
// resolve a account (qualquer um dos 3 caminhos de visibilidade), ela retorna sem
// erro e nullable organization_id e tratado via **string.
func TestFindAccountAccessibleReturnsAccount(t *testing.T) {
	now := time.Now().UTC()
	account, err := accountFromAccessibleRow(fakeAccountRow{values: []any{
		"acc-1",      // id
		"org-1",      // organization_id (nullable)
		"acme",       // slug
		"Acme",       // name
		true,         // is_active
		"standard",   // plan_code
		now,          // created_at
		now,          // updated_at
		false,        // is_agency
		"Crow Group", // coalesce(o.name, '')
	}})
	if err != nil {
		t.Fatalf("expected accessible account, got error %v", err)
	}
	if account.ID != "acc-1" || account.OrganizationID != "org-1" {
		t.Fatalf("unexpected account mapping: %#v", account)
	}
	if account.IsAgency != false || account.OrganizationName != "Crow Group" {
		t.Fatalf("unexpected isAgency/organizationName mapping: %#v", account)
	}
}

// TestFindAccountHandlesNullOrganization garante que organization_id NULL (cliente
// direto, sem agencia) nao quebra o scan e vira string vazia.
func TestFindAccountHandlesNullOrganization(t *testing.T) {
	now := time.Now().UTC()
	account, err := accountFromAccessibleRow(fakeAccountRow{values: []any{
		"acc-2",
		nil, // organization_id NULL
		"direct",
		"Direct Client",
		true,
		"standard",
		now,
		now,
		false, // is_agency
		"",    // coalesce(o.name, '') -> '' quando sem organization
	}})
	if err != nil {
		t.Fatalf("expected scan to handle NULL organization_id, got %v", err)
	}
	if account.OrganizationID != "" {
		t.Fatalf("expected empty OrganizationID for NULL org, got %q", account.OrganizationID)
	}
	if account.OrganizationName != "" {
		t.Fatalf("expected empty OrganizationName for NULL org, got %q", account.OrganizationName)
	}
}
