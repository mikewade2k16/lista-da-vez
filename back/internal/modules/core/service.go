package core

import (
	"context"
	"errors"
	"strings"
)

// Service expoe o contexto multi-account (v2) para o frontend.
//
// Fase 1 entrega leituras (GET /v2/me/accounts, GET /v2/me/context). RBAC
// dinamico (roles e permissions resolvidas a partir de core.role_permissions)
// chega na Fase 3. Ate la, AccountContext.Roles e Permissions ficam vazios.
type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
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

	summaries := make([]AccountSummary, 0, len(accounts))
	var orgView *OrganizationView
	for _, account := range accounts {
		moduleIDs, err := s.repository.ListEnabledModuleIDs(ctx, account.ID)
		if err != nil {
			return MeAccountsResponse{}, err
		}

		summaries = append(summaries, account.Summary(moduleIDs))

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

	return MeContextResponse{
		Context: AccountContext{
			Account:     account.Summary(moduleIDs),
			User:        user.View(),
			Roles:       []RoleSummary{},
			Permissions: []string{},
			Org:         orgView,
		},
	}, nil
}
