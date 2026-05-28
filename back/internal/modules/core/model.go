package core

import (
	"context"
	"time"
)

// Organization representa a entidade "agencia" opcional. Agrupa accounts.
type Organization struct {
	ID        string
	Slug      string
	Name      string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Account substitui o conceito legado de "tenant". Pode ou nao pertencer a uma
// Organization (cliente direto vs cliente-de-agencia).
type Account struct {
	ID             string
	OrganizationID string
	Slug           string
	Name           string
	Active         bool
	PlanCode       string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// User e a identidade global. Sem account_id; relacionamento com accounts vive
// em core.account_users.
type User struct {
	ID                 string
	Email              string
	DisplayName        string
	AvatarPath         string
	MustChangePassword bool
	IsPlatformAdmin    bool
	Active             bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ============================================================================
// Views (DTOs HTTP)
// ============================================================================

type UserView struct {
	ID                 string `json:"id"`
	Email              string `json:"email"`
	DisplayName        string `json:"displayName"`
	AvatarPath         string `json:"avatarPath,omitempty"`
	MustChangePassword bool   `json:"mustChangePassword"`
	IsPlatformAdmin    bool   `json:"isPlatformAdmin"`
	Active             bool   `json:"active"`
}

type OrganizationView struct {
	ID     string `json:"id"`
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// AccountSummary e o shape lean retornado por GET /v2/me/accounts.
// Sem permissoes nem roles — apenas o suficiente para o AccountSwitcher.
type AccountSummary struct {
	ID             string   `json:"id"`
	Slug           string   `json:"slug"`
	Name           string   `json:"name"`
	OrganizationID string   `json:"organizationId,omitempty"`
	PlanCode       string   `json:"planCode"`
	Active         bool     `json:"active"`
	Modules        []string `json:"modules"`
}

// AccountContext e o shape full retornado por GET /v2/me/context?accountId=...
// Inclui roles e permissoes resolvidas para o usuario na account informada.
type AccountContext struct {
	Account     AccountSummary    `json:"account"`
	User        UserView          `json:"user"`
	Roles       []RoleSummary     `json:"roles"`
	Permissions []string          `json:"permissions"`
	Org         *OrganizationView `json:"organization,omitempty"`
}

// RoleSummary aparece em AccountContext.Roles. Na Fase 3 (RBAC dinamico)
// passa a vir de core.roles. Por enquanto, espelha o role legado vindo do
// PermissionResolver atual via campo Code.
type RoleSummary struct {
	ID          string `json:"id,omitempty"`
	Code        string `json:"code"`
	Label       string `json:"label"`
	IsLocked    bool   `json:"isLocked"`
	IsDefault   bool   `json:"isDefault"`
	Description string `json:"description,omitempty"`
}

// MeAccountsResponse e o body de GET /v2/me/accounts.
type MeAccountsResponse struct {
	Accounts         []AccountSummary  `json:"accounts"`
	Organization     *OrganizationView `json:"organization,omitempty"`
	DefaultAccountID string            `json:"defaultAccountId,omitempty"`
}

// MeContextResponse e o body de GET /v2/me/context?accountId=...
type MeContextResponse struct {
	Context AccountContext `json:"context"`
}

// ============================================================================
// Repository
// ============================================================================

// Repository abstrai o acesso ao schema core. Implementacao em store_postgres.go.
type Repository interface {
	// FindUserByID busca o user global. Retorna ErrUserNotFound.
	FindUserByID(ctx context.Context, userID string) (User, error)

	// ListAccountsForUser retorna todas as accounts onde o user tem membership
	// ativa, ordenadas por nome. Lista vazia se nao tem nenhuma.
	ListAccountsForUser(ctx context.Context, userID string) ([]Account, error)

	// FindAccountIfMember retorna a account so se o user e membership ativo.
	// Retorna ErrAccountNotMember caso contrario.
	FindAccountIfMember(ctx context.Context, userID string, accountID string) (Account, error)

	// ListEnabledModuleIDs retorna os ids dos modulos habilitados na account.
	// Lista vazia ate a Fase 2 popular core.account_modules.
	ListEnabledModuleIDs(ctx context.Context, accountID string) ([]string, error)

	// FindOrganization retorna a organization da account, ou ErrOrganizationNotFound
	// quando organization_id e null.
	FindOrganization(ctx context.Context, organizationID string) (Organization, error)
}

// ============================================================================
// View helpers
// ============================================================================

func (user User) View() UserView {
	return UserView{
		ID:                 user.ID,
		Email:              user.Email,
		DisplayName:        user.DisplayName,
		AvatarPath:         user.AvatarPath,
		MustChangePassword: user.MustChangePassword,
		IsPlatformAdmin:    user.IsPlatformAdmin,
		Active:             user.Active,
	}
}

func (org Organization) View() OrganizationView {
	return OrganizationView{
		ID:     org.ID,
		Slug:   org.Slug,
		Name:   org.Name,
		Active: org.Active,
	}
}

func (account Account) Summary(modules []string) AccountSummary {
	if modules == nil {
		modules = []string{}
	}
	return AccountSummary{
		ID:             account.ID,
		Slug:           account.Slug,
		Name:           account.Name,
		OrganizationID: account.OrganizationID,
		PlanCode:       account.PlanCode,
		Active:         account.Active,
		Modules:        modules,
	}
}
