package tenants

import (
	"context"
	"strings"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (service *Service) ListAccessible(ctx context.Context, principal auth.Principal, input ListInput) ([]TenantView, error) {
	tenants, err := service.repository.ListAccessible(ctx, principal, input)
	if err != nil {
		return nil, err
	}

	views := make([]TenantView, 0, len(tenants))
	for _, tenant := range tenants {
		views = append(views, tenant.View())
	}

	return views, nil
}

func (service *Service) Create(ctx context.Context, principal auth.Principal, input CreateInput) (TenantView, error) {
	if !canEditTenants(principal) {
		return TenantView{}, ErrForbidden
	}

	if principal.Role != auth.RolePlatformAdmin {
		return TenantView{}, ErrForbidden
	}

	name := strings.TrimSpace(input.Name)
	slug := normalizeSlug(input.Slug)
	active := true
	if input.IsActive != nil {
		active = *input.IsActive
	}

	if name == "" || slug == "" {
		return TenantView{}, ErrValidation
	}

	created, err := service.repository.Create(ctx, Tenant{
		Name:   name,
		Slug:   slug,
		Active: active,
	})
	if err != nil {
		return TenantView{}, err
	}

	return created.View(), nil
}

func (service *Service) Update(ctx context.Context, principal auth.Principal, input UpdateInput) (TenantView, error) {
	if !canEditTenants(principal) {
		return TenantView{}, ErrForbidden
	}

	tenantID := strings.TrimSpace(input.ID)
	if tenantID == "" {
		return TenantView{}, ErrValidation
	}

	existing, err := service.repository.FindAccessibleByID(ctx, principal, tenantID)
	if err != nil {
		return TenantView{}, err
	}

	if input.Name != nil {
		existing.Name = strings.TrimSpace(*input.Name)
	}

	if input.Slug != nil {
		existing.Slug = normalizeSlug(*input.Slug)
	}

	if input.IsActive != nil {
		existing.Active = *input.IsActive
	}

	if existing.Name == "" || existing.Slug == "" {
		return TenantView{}, ErrValidation
	}

	updated, err := service.repository.Update(ctx, existing)
	if err != nil {
		return TenantView{}, err
	}

	return updated.View(), nil
}

func (service *Service) Archive(ctx context.Context, principal auth.Principal, tenantID string) (TenantView, error) {
	active := false
	return service.Update(ctx, principal, UpdateInput{
		ID:       strings.TrimSpace(tenantID),
		IsActive: &active,
	})
}

func (service *Service) Restore(ctx context.Context, principal auth.Principal, tenantID string) (TenantView, error) {
	active := true
	return service.Update(ctx, principal, UpdateInput{
		ID:       strings.TrimSpace(tenantID),
		IsActive: &active,
	})
}

func ResolveDefaultActiveTenantID(principal auth.Principal, tenants []TenantView) string {
	if principal.TenantID != "" {
		for _, tenant := range tenants {
			if tenant.ID == principal.TenantID {
				return tenant.ID
			}
		}
	}

	if len(tenants) == 0 {
		return ""
	}

	return tenants[0].ID
}

func canEditTenants(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		if accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionClientsEdit) {
			return true
		}
	}

	return principal.Role == auth.RoleOwner || principal.Role == auth.RolePlatformAdmin
}

func normalizeSlug(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, " ", "-")
	normalized = strings.ReplaceAll(normalized, "_", "-")

	builder := strings.Builder{}
	lastWasDash := false
	for _, runeValue := range normalized {
		if (runeValue >= 'a' && runeValue <= 'z') || (runeValue >= '0' && runeValue <= '9') {
			builder.WriteRune(runeValue)
			lastWasDash = false
			continue
		}

		if runeValue == '-' && !lastWasDash {
			builder.WriteRune('-')
			lastWasDash = true
		}
	}

	return strings.Trim(builder.String(), "-")
}
