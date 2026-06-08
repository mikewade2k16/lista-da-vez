package core

import (
	"context"
	"time"
)

// OrganizationAdminView e o DTO de uma organization para o painel
// /manage/organizations. Inclui agregados de accounts (count + slugs).
type OrganizationAdminView struct {
	ID           string    `json:"id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	IsActive     bool      `json:"isActive"`
	AccountCount int       `json:"accountCount"`
	AccountNames string    `json:"accountNames"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// AdminOrganizationListFilter parametriza GET /v1/admin/organizations.
type AdminOrganizationListFilter struct {
	Q       string
	Status  string // "active" | "inactive" | "" (todos)
	Page    int
	PerPage int
}

// AdminOrganizationListResponse e o body de GET /v1/admin/organizations.
type AdminOrganizationListResponse struct {
	Organizations []OrganizationAdminView `json:"organizations"`
	Total         int                     `json:"total"`
	Page          int                     `json:"page"`
	PerPage       int                     `json:"perPage"`
}

// AdminCreateOrganizationInput e o body de POST /v1/admin/organizations.
type AdminCreateOrganizationInput struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// AdminUpdateOrganizationInput e o body de PATCH /v1/admin/organizations/:id.
// Semantica de patch: campos nil sao ignorados.
type AdminUpdateOrganizationInput struct {
	Slug     *string `json:"slug"`
	Name     *string `json:"name"`
	IsActive *bool   `json:"isActive"`
}

// AdminOrganizationRepository abstrai persistencia para os endpoints admin
// de organizations.
type AdminOrganizationRepository interface {
	ListOrganizations(ctx context.Context, filter AdminOrganizationListFilter) ([]OrganizationAdminView, int, error)
	FindAdminOrganization(ctx context.Context, orgID string) (OrganizationAdminView, error)
	CreateOrganization(ctx context.Context, input AdminCreateOrganizationInput) (OrganizationAdminView, error)
	UpdateOrganization(ctx context.Context, orgID string, input AdminUpdateOrganizationInput) (OrganizationAdminView, error)
	SoftDeleteOrganization(ctx context.Context, orgID string) error
}
