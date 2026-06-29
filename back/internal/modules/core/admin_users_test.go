package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// Sem infra de teste de integracao (PostgresAdminUserRepository carrega um
// *pgxpool.Pool concreto), estes testes seguem o padrao de store_postgres_test.go:
// (a) validam o CONTRATO das queries (fragmentos SQL) e do scanner via scannable
// fake; (b) exercitam a logica do service contra um fake do AdminUserRepository.
// O comportamento transacional de MoveUserAccount no banco (move real de
// vinculos) so e coberto em integracao manual — ver "lembrete" no resumo.

// ============================================================================
// Contrato SQL: clientAccountId e filtro accountId
// ============================================================================

// TestClientAccountIDSelectResolvesSingleNonAgencyClient garante que a subquery de
// clientAccountId so considera cliente ATIVO NAO-agencia e devolve o id apenas
// quando ha exatamente 1 (having count = 1), caindo para ” (coalesce) caso 0/>1.
func TestClientAccountIDSelectResolvesSingleNonAgencyClient(t *testing.T) {
	fragments := []string{
		"a3.is_agency = false",      // exclui conta-agencia
		"au3.is_active = true",      // membership ativa
		"a3.is_active = true",       // conta ativa
		"having count(*) = 1",       // exatamente 1 cliente
		"max(au3.account_id::text)", // id como texto
		"au3.user_id = u.id",        // escopo do usuario corrente
	}
	for _, fragment := range fragments {
		if !strings.Contains(clientAccountIDSelect, fragment) {
			t.Fatalf("clientAccountIDSelect missing %q:\n%s", fragment, clientAccountIDSelect)
		}
	}
	// coalesce para '' quando 0 ou >1 (subquery NULL).
	if !strings.HasPrefix(strings.TrimSpace(clientAccountIDSelect), "coalesce((") {
		t.Fatalf("clientAccountIDSelect must coalesce to '' when 0/>1 clients:\n%s", clientAccountIDSelect)
	}
	if !strings.Contains(clientAccountIDSelect, "), '')") {
		t.Fatalf("clientAccountIDSelect must default to empty string:\n%s", clientAccountIDSelect)
	}
}

// ============================================================================
// Scanner: ordem das colunas + clientAccountId
// ============================================================================

// fakeUserRow implementa scannable para exercitar scanAdminUser sem Postgres.
type fakeUserRow struct {
	values []any
	err    error
}

func (row fakeUserRow) Scan(dest ...any) error {
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
		case *int:
			*target = value.(int)
		case *time.Time:
			*target = value.(time.Time)
		default:
			return errors.New("fakeUserRow: unexpected dest type")
		}
	}
	return nil
}

// TestScanAdminUserMapsClientAccountID confirma que scanAdminUser le a coluna nova
// na ORDEM correta (logo apos accountNames) e a mapeia para ClientAccountID.
func TestScanAdminUserMapsClientAccountID(t *testing.T) {
	now := time.Now().UTC()
	view, err := scanAdminUser(fakeUserRow{values: []any{
		"user-1",       // id
		"a@b.com",      // email
		"Alice",        // display_name
		"alice",        // nick (nullable)
		nil,            // avatar_path (nullable)
		true,           // is_active
		false,          // is_platform_admin
		false,          // must_change_password
		now,            // created_at
		now,            // updated_at
		2,              // account_count
		"Acme, Beta",   // account_names
		"acc-client-1", // clientAccountId
		false,          // isAgencyMember (ultima coluna)
	}})
	if err != nil {
		t.Fatalf("scanAdminUser returned error: %v", err)
	}
	if view.ClientAccountID != "acc-client-1" {
		t.Fatalf("expected ClientAccountID acc-client-1, got %q", view.ClientAccountID)
	}
	if view.AccountCount != 2 || view.AccountNames != "Acme, Beta" {
		t.Fatalf("column order broke aggregates: count=%d names=%q", view.AccountCount, view.AccountNames)
	}
	if view.IsAgencyMember {
		t.Fatalf("expected IsAgencyMember false for this row, got true")
	}
	if view.Nick != "alice" || view.AvatarPath != "" {
		t.Fatalf("nullable mapping broke: nick=%q avatar=%q", view.Nick, view.AvatarPath)
	}
}

// TestScanAdminUserMapsAgencyMember confirma que scanAdminUser le a coluna nova
// isAgencyMember como ULTIMO campo (bool nao-nullable) e a mapeia para IsAgencyMember.
func TestScanAdminUserMapsAgencyMember(t *testing.T) {
	now := time.Now().UTC()
	view, err := scanAdminUser(fakeUserRow{values: []any{
		"user-3",    // id
		"e@f.com",   // email
		"Carol",     // display_name
		nil,         // nick (nullable)
		nil,         // avatar_path (nullable)
		true,        // is_active
		false,       // is_platform_admin
		false,       // must_change_password
		now,         // created_at
		now,         // updated_at
		1,           // account_count
		"Agencia X", // account_names
		"",          // clientAccountId (membro de agencia -> sem cliente unico)
		true,        // isAgencyMember (ultima coluna)
	}})
	if err != nil {
		t.Fatalf("scanAdminUser returned error: %v", err)
	}
	if !view.IsAgencyMember {
		t.Fatalf("expected IsAgencyMember true, got false")
	}
}

// TestScanAdminUserEmptyClientAccountID cobre o caso 0/>1 clientes -> ”.
func TestScanAdminUserEmptyClientAccountID(t *testing.T) {
	now := time.Now().UTC()
	view, err := scanAdminUser(fakeUserRow{values: []any{
		"user-2", "c@d.com", "Bob", nil, nil,
		true, false, false, now, now,
		0, "", "", // sem cliente -> clientAccountId ''
		false, // isAgencyMember (ultima coluna)
	}})
	if err != nil {
		t.Fatalf("scanAdminUser returned error: %v", err)
	}
	if view.ClientAccountID != "" {
		t.Fatalf("expected empty ClientAccountID for 0/>1 clients, got %q", view.ClientAccountID)
	}
}

// ============================================================================
// Service: MoveUserAccount (validacao de input + passagem ao repo)
// ============================================================================

// fakeAdminUserRepo registra a chamada de MoveUserAccount e devolve um resultado
// configuravel. So os metodos usados pelos testes tem corpo real.
type fakeAdminUserRepo struct {
	AdminUserRepository // embed para satisfazer a interface sem stubar tudo

	movedUserID    string
	movedAccountID string
	movedRole      string
	moveResult     AdminUserView
	moveErr        error
}

func (f *fakeAdminUserRepo) MoveUserAccount(_ context.Context, userID, targetAccountID, role string) (AdminUserView, error) {
	f.movedUserID = userID
	f.movedAccountID = targetAccountID
	f.movedRole = role
	if f.moveErr != nil {
		return AdminUserView{}, f.moveErr
	}
	return f.moveResult, nil
}

// fakeScopeRepo implementa AdminScopeRepository com respostas configuraveis. Por
// default tudo true (ator = platform_admin) para os testes de MoveUserAccount,
// que so passam por IsPlatformAdmin antes de chegar no repo.
type fakeScopeRepo struct {
	canManageAccount bool
	canManageUser    bool
	canManageOrg     bool
	isPlatformAdmin  bool
	isAdminOfAny     bool
}

func (f fakeScopeRepo) CanManageAccount(context.Context, string, string) (bool, error) {
	return f.canManageAccount, nil
}
func (f fakeScopeRepo) CanManageUser(context.Context, string, string) (bool, error) {
	return f.canManageUser, nil
}
func (f fakeScopeRepo) CanManageOrganization(context.Context, string, string) (bool, error) {
	return f.canManageOrg, nil
}
func (f fakeScopeRepo) IsPlatformAdmin(context.Context, string) (bool, error) {
	return f.isPlatformAdmin, nil
}
func (f fakeScopeRepo) IsAdminOfAnything(context.Context, string) (bool, error) {
	return f.isAdminOfAny, nil
}

// platformAdminScope devolve um resolver cujo ator e platform_admin (libera os
// gates identity-global de MoveUserAccount/Create/Delete/Update).
func platformAdminScope() *AdminScopeResolver {
	return NewAdminScopeResolver(fakeScopeRepo{
		canManageAccount: true,
		canManageUser:    true,
		canManageOrg:     true,
		isPlatformAdmin:  true,
		isAdminOfAny:     true,
	})
}

// newTestAdminUserService monta o service com escopo de platform_admin e sem
// links repo (os testes atuais nao exercitam vinculos).
func newTestAdminUserService(repo AdminUserRepository) *AdminUserService {
	return NewAdminUserService(repo, nil, platformAdminScope(), nil)
}

func TestMoveUserAccountRequiresAccountID(t *testing.T) {
	repo := &fakeAdminUserRepo{}
	svc := newTestAdminUserService(repo)
	_, err := svc.MoveUserAccount(context.Background(), "actor-1", "user-1", MoveUserAccountInput{AccountID: "  "})
	if err == nil {
		t.Fatal("expected error when accountId is blank")
	}
	if repo.movedAccountID != "" {
		t.Fatalf("repo should not be called when accountId is blank, got %q", repo.movedAccountID)
	}
}

func TestMoveUserAccountDefaultsRoleToOwner(t *testing.T) {
	repo := &fakeAdminUserRepo{moveResult: AdminUserView{ID: "user-1", ClientAccountID: "acc-2"}}
	svc := newTestAdminUserService(repo)
	view, err := svc.MoveUserAccount(context.Background(), "actor-1", "user-1", MoveUserAccountInput{AccountID: "acc-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.movedRole != "owner" {
		t.Fatalf("expected default role owner, got %q", repo.movedRole)
	}
	if repo.movedUserID != "user-1" || repo.movedAccountID != "acc-2" {
		t.Fatalf("repo got wrong args: user=%q account=%q", repo.movedUserID, repo.movedAccountID)
	}
	if view.ClientAccountID != "acc-2" {
		t.Fatalf("expected updated view returned, got %#v", view)
	}
}

func TestMoveUserAccountRejectsInvalidRole(t *testing.T) {
	repo := &fakeAdminUserRepo{}
	svc := newTestAdminUserService(repo)
	_, err := svc.MoveUserAccount(context.Background(), "actor-1", "user-1", MoveUserAccountInput{AccountID: "acc-2", Role: "superuser"})
	if !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("expected ErrInvalidRole, got %v", err)
	}
	if repo.movedAccountID != "" {
		t.Fatal("repo should not be called with invalid role")
	}
}

func TestMoveUserAccountAcceptsKnownRoles(t *testing.T) {
	for _, role := range []string{"owner", "director", "marketing", "OWNER", " Director "} {
		repo := &fakeAdminUserRepo{moveResult: AdminUserView{ID: "user-1"}}
		svc := newTestAdminUserService(repo)
		if _, err := svc.MoveUserAccount(context.Background(), "actor-1", "user-1", MoveUserAccountInput{AccountID: "acc-2", Role: role}); err != nil {
			t.Fatalf("role %q should be accepted, got %v", role, err)
		}
		if repo.movedRole != strings.ToLower(strings.TrimSpace(role)) {
			t.Fatalf("role %q not normalized, repo got %q", role, repo.movedRole)
		}
	}
}

// TestMoveUserAccountPropagatesRepoErrors garante que erros de dominio do repo
// (conta inexistente -> 404, conta agencia -> 400) chegam intactos ao handler.
func TestMoveUserAccountPropagatesRepoErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"account_not_found", ErrAccountNotFound},
		{"account_is_agency", ErrAccountIsAgency},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeAdminUserRepo{moveErr: tc.err}
			svc := newTestAdminUserService(repo)
			_, err := svc.MoveUserAccount(context.Background(), "actor-1", "user-1", MoveUserAccountInput{AccountID: "acc-2"})
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected %v to propagate, got %v", tc.err, err)
			}
		})
	}
}
