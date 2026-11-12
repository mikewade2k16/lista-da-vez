package cardapio

import (
	"context"
	"errors"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// memberStore simula vinculos de usuario com varios clientes.
type memberStore struct {
	unimplementedStore
	members    map[string]bool // accountID -> membro com cardapio ligado
	listedWith []string
}

func (m *memberStore) IsCardapioMember(_ context.Context, accountID, _ string) (bool, error) {
	return m.members[accountID], nil
}

func (m *memberStore) CardapioMemberAccounts(context.Context, string) ([]string, error) {
	out := make([]string, 0)
	for id, ok := range m.members {
		if ok {
			out = append(out, id)
		}
	}
	return out, nil
}

func (m *memberStore) ListRestaurantsLeanForAccounts(_ context.Context, accountIDs []string, _ string) ([]RestaurantLean, error) {
	m.listedWith = accountIDs
	out := make([]RestaurantLean, 0, len(accountIDs))
	for _, id := range accountIDs {
		out = append(out, RestaurantLean{ID: "r-" + id, AccountID: id})
	}
	return out, nil
}

// permsByAccount simula a RBAC por account (cardapio.view em cada cliente).
type permsByAccount map[string]bool

func (p permsByAccount) HasAccountPermission(_ context.Context, accountID, _, _ string) (bool, error) {
	return p[accountID], nil
}

func TestOwnerShortcutOnlyOnOwnAccount(t *testing.T) {
	gate := newCardapioGate(permsByAccount{}, nil)
	owner := auth.Principal{UserID: "u1", Role: auth.RoleOwner, TenantID: "acc-a"}
	if err := gate.Authorize(context.Background(), owner, "acc-a", permManage); err != nil {
		t.Fatalf("owner deve gerenciar a propria account: %v", err)
	}
	if err := gate.Authorize(context.Background(), owner, "acc-b", permManage); !errors.Is(err, ErrForbidden) {
		t.Fatalf("owner de A nao pode ter atalho em B, veio %v", err)
	}
	withPerm := newCardapioGate(permsByAccount{"acc-b": true}, nil)
	if err := withPerm.Authorize(context.Background(), owner, "acc-b", permManage); err != nil {
		t.Fatalf("em B vale a permissao fina concedida: %v", err)
	}
}

func TestCrossAccountRequiresMembership(t *testing.T) {
	store := &memberStore{members: map[string]bool{"acc-mostarda": true}}
	svc := newServiceWithStore(store, ServiceConfig{})
	user := auth.Principal{UserID: "junior"}
	ctx := context.Background()

	if got := svc.crossAccountFor(ctx, user, "acc-bari", "acc-mostarda"); got != "acc-mostarda" {
		t.Fatalf("membro do outro cliente deve abrir, veio %q", got)
	}
	if got := svc.crossAccountFor(ctx, user, "acc-bari", "acc-estranho"); got != "" {
		t.Fatalf("nao-membro nunca recebe escopo, veio %q", got)
	}
	if got := svc.crossAccountFor(ctx, user, "acc-bari", "acc-bari"); got != "" {
		t.Fatalf("propria account nao e cross-account, veio %q", got)
	}
	admin := auth.Principal{UserID: "adm", Role: auth.RolePlatformAdmin}
	if got := svc.crossAccountFor(ctx, admin, "", "acc-mostarda"); got != "" {
		t.Fatalf("platform_admin segue o fluxo proprio, veio %q", got)
	}
}

func TestListMemberRestaurantsFiltersByPermission(t *testing.T) {
	store := &memberStore{members: map[string]bool{"acc-bari": true, "acc-mostarda": true, "acc-sem-perm": true}}
	svc := newServiceWithStore(store, ServiceConfig{}).
		WithGate(newCardapioGate(permsByAccount{"acc-bari": true, "acc-mostarda": true}, nil))

	items, err := svc.ListMemberRestaurants(context.Background(), auth.Principal{UserID: "junior"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("esperava os 2 clientes com cardapio.view, veio %d (%v)", len(items), store.listedWith)
	}
	for _, id := range store.listedWith {
		if id == "acc-sem-perm" {
			t.Fatal("account sem cardapio.view nao pode ser listada")
		}
	}
}
