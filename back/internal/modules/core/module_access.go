package core

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ModuleAccess exposes the host catalog without consumers reading core tables.
type ModuleAccess struct{ pool *pgxpool.Pool }

func NewModuleAccess(pool *pgxpool.Pool) *ModuleAccess { return &ModuleAccess{pool: pool} }
func (a *ModuleAccess) Enabled(ctx context.Context, account, module string) (bool, error) {
	var enabled bool
	e := a.pool.QueryRow(ctx, `select exists(select 1 from core.account_modules where account_id=$1::uuid and module_id=$2 and enabled=true)`, account, module).Scan(&enabled)
	return enabled, e
}

// ActiveMember is deliberately stricter than the host's platform-admin visibility.
func (a *ModuleAccess) ActiveMember(ctx context.Context, account, user string) (bool, error) {
	var active bool
	e := a.pool.QueryRow(ctx, `select exists(select 1 from core.account_users au join core.users u on u.id=au.user_id join core.accounts a on a.id=au.account_id where au.account_id=$1::uuid and au.user_id=$2::uuid and au.is_active=true and u.is_active=true and a.is_active=true)`, account, user).Scan(&active)
	return active, e
}
