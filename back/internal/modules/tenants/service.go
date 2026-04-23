package tenants

import (
	"context"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (service *Service) ListAccessible(ctx context.Context, principal auth.Principal) ([]TenantView, error) {
	tenants, err := service.repository.ListAccessible(ctx, principal)
	if err != nil {
		return nil, err
	}

	views := make([]TenantView, 0, len(tenants))
	for _, tenant := range tenants {
		views = append(views, tenant.View())
	}

	return views, nil
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
