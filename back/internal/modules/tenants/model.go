package tenants

import (
	"context"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Tenant struct {
	ID        string
	Slug      string
	Name      string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TenantView struct {
	ID     string `json:"id"`
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type Repository interface {
	ListAccessible(ctx context.Context, principal auth.Principal) ([]Tenant, error)
}

func (tenant Tenant) View() TenantView {
	return TenantView{
		ID:     tenant.ID,
		Slug:   tenant.Slug,
		Name:   tenant.Name,
		Active: tenant.Active,
	}
}
