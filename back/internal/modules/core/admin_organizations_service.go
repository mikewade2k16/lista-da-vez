package core

import (
	"context"
	"errors"
	"strings"
)

// AdminOrganizationService orquestra regras de negocio dos endpoints
// /v1/admin/organizations. Valida slug e nome.
type AdminOrganizationService struct {
	repo AdminOrganizationRepository
}

// NewAdminOrganizationService cria o service com suas dependencias.
func NewAdminOrganizationService(repo AdminOrganizationRepository) *AdminOrganizationService {
	return &AdminOrganizationService{repo: repo}
}

// ListOrganizations passa filtros para o repositorio e devolve paginado.
func (s *AdminOrganizationService) ListOrganizations(ctx context.Context, filter AdminOrganizationListFilter) (AdminOrganizationListResponse, error) {
	orgs, total, err := s.repo.ListOrganizations(ctx, filter)
	if err != nil {
		return AdminOrganizationListResponse{}, err
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	return AdminOrganizationListResponse{
		Organizations: orgs,
		Total:         total,
		Page:          page,
		PerPage:       perPage,
	}, nil
}

// GetOrganization devolve uma organization pelo id.
func (s *AdminOrganizationService) GetOrganization(ctx context.Context, orgID string) (OrganizationAdminView, error) {
	return s.repo.FindAdminOrganization(ctx, orgID)
}

// CreateOrganization valida slug + nome e cria.
func (s *AdminOrganizationService) CreateOrganization(ctx context.Context, input AdminCreateOrganizationInput) (OrganizationAdminView, error) {
	input.Slug = strings.ToLower(strings.TrimSpace(input.Slug))
	input.Name = strings.TrimSpace(input.Name)
	if input.Slug == "" || input.Name == "" {
		return OrganizationAdminView{}, errors.New("slug and name are required")
	}
	if len(input.Slug) < 2 {
		return OrganizationAdminView{}, errors.New("slug must have at least 2 chars")
	}
	return s.repo.CreateOrganization(ctx, input)
}

// UpdateOrganization valida o patch e aplica.
func (s *AdminOrganizationService) UpdateOrganization(ctx context.Context, orgID string, input AdminUpdateOrganizationInput) (OrganizationAdminView, error) {
	if input.Slug != nil {
		normalized := strings.ToLower(strings.TrimSpace(*input.Slug))
		if len(normalized) < 2 {
			return OrganizationAdminView{}, errors.New("slug must have at least 2 chars")
		}
		input.Slug = &normalized
	}
	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if trimmed == "" {
			return OrganizationAdminView{}, errors.New("name cannot be blank")
		}
		input.Name = &trimmed
	}
	return s.repo.UpdateOrganization(ctx, orgID, input)
}

// DeleteOrganization aplica soft delete (is_active=false).
func (s *AdminOrganizationService) DeleteOrganization(ctx context.Context, orgID string) error {
	return s.repo.SoftDeleteOrganization(ctx, orgID)
}
