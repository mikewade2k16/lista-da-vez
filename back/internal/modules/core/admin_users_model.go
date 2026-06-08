package core

import (
	"context"
	"time"
)

// AccountMembershipView é um item de "accounts em que o user participa".
// Usado por GET /v1/admin/users/{id}/memberships.
type AccountMembershipView struct {
	AccountID   string    `json:"accountId"`
	AccountSlug string    `json:"accountSlug"`
	AccountName string    `json:"accountName"`
	IsActive    bool      `json:"isActive"`
	JoinedAt    time.Time `json:"joinedAt"`
}

// AdminUserView é o DTO de um user para o painel /manage/users.
// Inclui agregados (accountCount, accountSlugs) computados no backend.
type AdminUserView struct {
	ID                 string    `json:"id"`
	Email              string    `json:"email"`
	DisplayName        string    `json:"displayName"`
	Nick               string    `json:"nick"`
	AvatarPath         string    `json:"avatarPath,omitempty"`
	IsActive           bool      `json:"isActive"`
	IsPlatformAdmin    bool      `json:"isPlatformAdmin"`
	MustChangePassword bool      `json:"mustChangePassword"`
	AccountCount       int       `json:"accountCount"`
	AccountNames       string    `json:"accountNames"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// AdminUserListFilter parametriza GET /v1/admin/users.
type AdminUserListFilter struct {
	Q             string
	Status        string // "active" | "inactive" | "" (todos)
	PlatformAdmin string // "true" | "false" | "" (todos)
	Page          int
	PerPage       int
}

// AdminUserListResponse e o body de GET /v1/admin/users.
type AdminUserListResponse struct {
	Users   []AdminUserView `json:"users"`
	Total   int             `json:"total"`
	Page    int             `json:"page"`
	PerPage int             `json:"perPage"`
}

// AdminCreateUserInput e o body de POST /v1/admin/users.
// TemporaryPassword opcional: se vazio, user fica com password_hash null e
// must_change_password true (precisa fluxo de aceite de convite).
type AdminCreateUserInput struct {
	Email             string `json:"email"`
	DisplayName       string `json:"displayName"`
	Nick              string `json:"nick"`
	IsPlatformAdmin   bool   `json:"isPlatformAdmin"`
	TemporaryPassword string `json:"temporaryPassword"`
	// AccountID opcional: vincula o user a uma account (cliente) via membership
	// em core.account_users. Sem ele, o user fica sem cliente (ex.: platform_admin).
	AccountID string `json:"accountId,omitempty"`
	// OrganizationID opcional: vincula o user a uma organization (agencia) via
	// core.organization_users (org_role 'agency_member').
	OrganizationID string `json:"organizationId,omitempty"`
	// Role: papel no tenant do AccountID (owner/director/marketing). Cria
	// user_tenant_roles legado — NECESSARIO para login (auth resolve papel pelo
	// legado) e para aparecer em /operacao/usuarios. Default 'owner' quando
	// AccountID setado. accountId == tenantId (core.accounts.id == public.tenants.id).
	Role string `json:"role,omitempty"`
}

// AdminUpdateUserInput e o body de PATCH /v1/admin/users/:id.
// Semantica de patch: campos nil sao ignorados.
type AdminUpdateUserInput struct {
	Email           *string `json:"email"`
	DisplayName     *string `json:"displayName"`
	Nick            *string `json:"nick"`
	IsActive        *bool   `json:"isActive"`
	IsPlatformAdmin *bool   `json:"isPlatformAdmin"`
}

// AdminMembershipsResponse e o body de GET /v1/admin/users/:id/memberships.
type AdminMembershipsResponse struct {
	Memberships []AccountMembershipView `json:"memberships"`
}

// AdminUserRepository abstrai persistencia para os endpoints admin de users.
type AdminUserRepository interface {
	ListUsers(ctx context.Context, filter AdminUserListFilter) ([]AdminUserView, int, error)
	FindAdminUser(ctx context.Context, userID string) (AdminUserView, error)
	CreateUser(ctx context.Context, input AdminCreateUserInput, passwordHash string) (AdminUserView, error)
	UpdateUser(ctx context.Context, userID string, input AdminUpdateUserInput) (AdminUserView, error)
	SoftDeleteUser(ctx context.Context, userID string) error
	GetMemberships(ctx context.Context, userID string) ([]AccountMembershipView, error)
	CountActivePlatformAdmins(ctx context.Context) (int, error)
}
