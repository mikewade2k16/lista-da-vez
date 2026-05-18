package tasks

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// principalForRole monta um Principal minimo para os testes de escopo.
func principalForRole(role auth.Role, userID, tenantID string) auth.Principal {
	return auth.Principal{
		UserID:   userID,
		Role:     role,
		TenantID: tenantID,
	}
}

func TestResolveAccessContext_RejectsEmptyAccountID(t *testing.T) {
	service := NewService(&repositoryMock{}, nil, nil, nil)
	_, err := service.ResolveAccessContext(context.Background(), principalForRole(auth.RoleOwner, "u", "acc-1"), "")
	if !errors.Is(err, ErrAccountRequired) {
		t.Fatalf("accountID vazio deve retornar ErrAccountRequired (400); got %v", err)
	}
}

func TestResolveAccessContext_AccountNotFound(t *testing.T) {
	repository := &repositoryMock{
		onAccountExists: func(_ context.Context, _ string) (bool, error) { return false, nil },
	}
	service := NewService(repository, nil, nil, nil)
	_, err := service.ResolveAccessContext(context.Background(), principalForRole(auth.RoleOwner, "u", "acc-1"), "acc-inexistente")
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("account inexistente deve retornar ErrAccountNotFound (404); got %v", err)
	}
}

func TestResolveAccessContext_CrossAccountReturns404NotForbidden(t *testing.T) {
	// User da acc-1 tentando acessar acc-2. Repository diz que a conta existe (acc-2 e' real),
	// mas IsAccountMember retorna false. Resultado esperado: ErrAccountNotFound (404), NUNCA
	// ErrForbidden (403). E' a regra "cross-account → 404" da T8.
	repository := &repositoryMock{
		onAccountExists: func(_ context.Context, _ string) (bool, error) { return true, nil },
		onIsAccountMember: func(_ context.Context, accountID, _ string) (bool, error) {
			if accountID == "acc-other" {
				return false, nil
			}
			return true, nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	_, err := service.ResolveAccessContext(context.Background(), principalForRole(auth.RoleOwner, "u-1", "acc-1"), "acc-other")
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("cross-account DEVE retornar 404 (ErrAccountNotFound), nunca 403; got %v", err)
	}
	if errors.Is(err, ErrForbidden) {
		t.Errorf("cross-account NUNCA pode retornar ErrForbidden (vaza existencia do recurso); got %v", err)
	}
}

func TestResolveAccessContext_PlatformAdminBypassesMembership(t *testing.T) {
	repository := &repositoryMock{
		onAccountExists: func(_ context.Context, _ string) (bool, error) { return true, nil },
		onIsAccountMember: func(_ context.Context, _, _ string) (bool, error) {
			t.Error("platform_admin nao deve chamar IsAccountMember")
			return false, nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	access, err := service.ResolveAccessContext(
		context.Background(),
		principalForRole(auth.RolePlatformAdmin, "u-admin", ""),
		"acc-qualquer",
	)
	if err != nil {
		t.Fatalf("platform_admin deve passar; got %v", err)
	}
	if !access.IsPlatformAdmin {
		t.Errorf("access.IsPlatformAdmin deve estar true")
	}
	if access.Perspective != PerspectiveAgency {
		t.Errorf("platform_admin nunca vira client_viewer; got %q", access.Perspective)
	}
}

func TestResolveAccessContext_ClientViewerPerspective(t *testing.T) {
	// User membro da account com tasks.client_view mas SEM tasks.boards.manage.
	repository := &repositoryMock{
		onAccountExists:   func(_ context.Context, _ string) (bool, error) { return true, nil },
		onIsAccountMember: func(_ context.Context, _, _ string) (bool, error) { return true, nil },
		onListPermissionsForUser: func(_ context.Context, _, _ string) ([]string, error) {
			return []string{PermClientView, PermTasksComment}, nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	access, err := service.ResolveAccessContext(context.Background(), principalForRole(auth.RoleConsultant, "u-1", "acc-1"), "acc-1")
	if err != nil {
		t.Fatalf("ResolveAccessContext: %v", err)
	}
	if access.Perspective != PerspectiveClientViewer {
		t.Errorf("user com client_view sem manage deve virar client_viewer; got %q", access.Perspective)
	}
}

func TestResolveAccessContext_BoardsManageOverridesClientViewer(t *testing.T) {
	// User tem AMBOS: tasks.client_view e tasks.boards.manage -> perspective continua agency.
	repository := &repositoryMock{
		onAccountExists:   func(_ context.Context, _ string) (bool, error) { return true, nil },
		onIsAccountMember: func(_ context.Context, _, _ string) (bool, error) { return true, nil },
		onListPermissionsForUser: func(_ context.Context, _, _ string) ([]string, error) {
			return []string{PermClientView, PermBoardsManage, PermTasksView}, nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	access, err := service.ResolveAccessContext(context.Background(), principalForRole(auth.RoleOwner, "u", "acc"), "acc")
	if err != nil {
		t.Fatalf("ResolveAccessContext: %v", err)
	}
	if access.Perspective != PerspectiveAgency {
		t.Errorf("user com manage + client_view continua agency; got %q", access.Perspective)
	}
}

// TestScope_FuzzCrossAccountReturns404 simula o "fuzz 100 IDs de outros accounts" da T9. Como
// nao temos DB real, simulamos no mock: qualquer accountID != accountValido retorna
// `IsAccountMember=false` e o resultado deve SEMPRE virar 404, nunca 403, nunca vazamento.
func TestScope_FuzzCrossAccountReturns404(t *testing.T) {
	ownAccount := "acc-mine"
	repository := &repositoryMock{
		onAccountExists: func(_ context.Context, accountID string) (bool, error) {
			// Outras accounts existem no banco, mas o user nao e' membro.
			return true, nil
		},
		onIsAccountMember: func(_ context.Context, accountID, _ string) (bool, error) {
			return accountID == ownAccount, nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	principal := principalForRole(auth.RoleOwner, "u", ownAccount)
	for index := 0; index < 100; index++ {
		foreignID := fmt.Sprintf("acc-other-%03d", index)
		_, err := service.ResolveAccessContext(context.Background(), principal, foreignID)
		if !errors.Is(err, ErrAccountNotFound) {
			t.Errorf("[%d] cross-account %q deveria 404; got %v", index, foreignID, err)
		}
	}
}

// TestScopedQuery_PanicsWithoutAccountID e' a defesa de ultimo recurso do `scopedQuery` no
// repository: programador esquece de passar accountID -> panic na inicializacao da query, antes
// de qualquer SQL ser enviado. Garante que nunca rodamos query "global" sem escopo de tenant.
func TestScopedQuery_PanicsWithoutAccountID(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("scopedQuery sem accountID deveria panicar (defesa em camada do repository)")
		}
		message, ok := recovered.(string)
		if !ok || message == "" {
			t.Errorf("panic deveria ter mensagem com 'accountID'; got %v", recovered)
		}
	}()
	// Importante: passamos nil pool — o panic do scopedQuery acontece ANTES de qualquer chamada
	// ao pool, validando que a guarda nao depende do estado do banco.
	repository := &PostgresRepository{pool: nil}
	_, _ = repository.scopedQuery("", "select 1")
}
