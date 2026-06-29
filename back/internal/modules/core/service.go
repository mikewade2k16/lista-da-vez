package core

import (
	"context"
	"errors"
	"strings"
)

// batchModuleLoader e uma interface opcional implementada por PostgresRepository.
// Permite ao Service usar uma unica query batch em vez de N queries por account
// (OPT-2/F-22). Definida aqui para nao alterar a interface publica Repository.
type batchModuleLoader interface {
	ListEnabledModuleIDsForAccounts(ctx context.Context, accountIDs []string) (map[string][]string, error)
}

// Service expoe o contexto multi-account (v2) para o frontend.
type Service struct {
	repository  Repository
	rbacService *RBACService
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

// WithRBAC conecta o RBACService para resolucao de roles/permissions em MeContext.
func (s *Service) WithRBAC(rbac *RBACService) {
	s.rbacService = rbac
}

// MeAccounts lista todas as accounts onde o user tem membership ativa.
// Lean: retorna apenas id, name, organizationId, planCode, modules[]. Sem
// permissoes (essas vem por GET /v2/me/context?accountId=...).
func (s *Service) MeAccounts(ctx context.Context, userID string) (MeAccountsResponse, error) {
	if strings.TrimSpace(userID) == "" {
		return MeAccountsResponse{}, ErrUserNotFound
	}

	accounts, err := s.repository.ListAccountsForUser(ctx, userID)
	if err != nil {
		return MeAccountsResponse{}, err
	}

	// Carrega os modulos de todas as accounts em uma unica query batch (OPT-2/F-22),
	// evitando N queries quando o usuario (especialmente platform_admin) enxerga
	// muitas accounts. Fallback para o loop N+1 se o repositorio nao implementar
	// batchModuleLoader (ex.: fake de teste que so satisfaz Repository).
	modulesMap, err := s.loadModulesBatch(ctx, accounts)
	if err != nil {
		return MeAccountsResponse{}, err
	}

	summaries := make([]AccountSummary, 0, len(accounts))
	var orgView *OrganizationView
	for _, account := range accounts {
		summaries = append(summaries, account.Summary(modulesMap[account.ID]))

		// Se mais de uma account compartilha a mesma organization, essa e a
		// organization "principal" do user (cenario tipico de agencia).
		// Carregamos uma vez no primeiro hit que tiver organization vinculada.
		if orgView == nil && account.OrganizationID != "" {
			org, err := s.repository.FindOrganization(ctx, account.OrganizationID)
			if err != nil && !errors.Is(err, ErrOrganizationNotFound) {
				return MeAccountsResponse{}, err
			}
			if err == nil {
				view := org.View()
				orgView = &view
			}
		}
	}

	defaultAccountID := ""
	if len(summaries) > 0 {
		defaultAccountID = summaries[0].ID
	}

	return MeAccountsResponse{
		Accounts:         summaries,
		Organization:     orgView,
		DefaultAccountID: defaultAccountID,
	}, nil
}

// loadModulesBatch carrega os modulos habilitados para uma lista de accounts
// em uma unica query, usando batchModuleLoader quando o repositorio o suporta.
// Fallback: loop N+1 via ListEnabledModuleIDs (compatibilidade com fakes de teste).
// Garante que toda account do slice tenha entrada no map retornado (slice vazio
// em vez de nil para as que nao tiverem modulos).
func (s *Service) loadModulesBatch(ctx context.Context, accounts []Account) (map[string][]string, error) {
	out := make(map[string][]string, len(accounts))
	// Pre-popula com slice vazio para garantir entradas para todas as accounts.
	for _, a := range accounts {
		out[a.ID] = []string{}
	}

	if len(accounts) == 0 {
		return out, nil
	}

	// Caminho otimizado: uma unica query batch.
	if bl, ok := s.repository.(batchModuleLoader); ok {
		ids := make([]string, len(accounts))
		for i, a := range accounts {
			ids[i] = a.ID
		}
		batch, err := bl.ListEnabledModuleIDsForAccounts(ctx, ids)
		if err != nil {
			return nil, err
		}
		for accountID, moduleIDs := range batch {
			out[accountID] = moduleIDs
		}
		return out, nil
	}

	// Fallback N+1 (repositorios que nao implementam batchModuleLoader).
	for _, a := range accounts {
		moduleIDs, err := s.repository.ListEnabledModuleIDs(ctx, a.ID)
		if err != nil {
			return nil, err
		}
		out[a.ID] = moduleIDs
	}
	return out, nil
}

// MeContext retorna o contexto completo de uma account especifica para o
// usuario autenticado. Valida que ele e membership antes de retornar.
//
// Roles e permissions ficam vazios ate a Fase 3 (RBAC dinamico) — ler diretamente
// de core.role_permissions. Por enquanto, modules[] reflete o que esta habilitado
// em core.account_modules.
func (s *Service) MeContext(ctx context.Context, userID string, accountID string) (MeContextResponse, error) {
	if strings.TrimSpace(userID) == "" {
		return MeContextResponse{}, ErrUserNotFound
	}
	if strings.TrimSpace(accountID) == "" {
		return MeContextResponse{}, ErrAccountNotFound
	}

	user, err := s.repository.FindUserByID(ctx, userID)
	if err != nil {
		return MeContextResponse{}, err
	}

	account, err := s.repository.FindAccountIfMember(ctx, userID, accountID)
	if err != nil {
		return MeContextResponse{}, err
	}

	moduleIDs, err := s.repository.ListEnabledModuleIDs(ctx, accountID)
	if err != nil {
		return MeContextResponse{}, err
	}

	var orgView *OrganizationView
	if account.OrganizationID != "" {
		org, err := s.repository.FindOrganization(ctx, account.OrganizationID)
		if err != nil && !errors.Is(err, ErrOrganizationNotFound) {
			return MeContextResponse{}, err
		}
		if err == nil {
			view := org.View()
			orgView = &view
		}
	}

	roles := []RoleSummary{}
	permissions := []string{}

	if s.rbacService != nil {
		roles, permissions, err = s.rbacService.ResolveUserContext(ctx, accountID, userID)
		if err != nil {
			return MeContextResponse{}, err
		}
	}

	return MeContextResponse{
		Context: AccountContext{
			Account:     account.Summary(moduleIDs),
			User:        user.View(),
			Roles:       roles,
			Permissions: permissions,
			Org:         orgView,
		},
	}, nil
}
