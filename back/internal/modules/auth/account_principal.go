package auth

import (
	"context"
	"errors"
)

var ErrAccountPrincipalDenied = errors.New("auth: account principal denied")

// ResolveAccountPrincipal revalidates the same account access and effective RBAC
// used by RequireAuthWithAccount, without reading passwords or session secrets.
func (c *PostgresAccountMemberChecker) ResolveAccountPrincipal(ctx context.Context, account, user string) (Principal, error) {
	member, err := c.IsMember(ctx, account, user)
	if err != nil {
		return Principal{}, err
	}
	if !member {
		return Principal{}, ErrAccountPrincipalDenied
	}
	rows, err := c.pool.Query(ctx, `select u.is_platform_admin,coalesce(r.code,''),coalesce(r.cloned_from_template_id,'') from core.users u left join core.user_role_assignments ura on ura.user_id=u.id and ura.account_id=$1::uuid left join core.roles r on r.id=ura.role_id where u.id=$2::uuid and u.is_active=true`, account, user)
	if err != nil {
		return Principal{}, err
	}
	defer rows.Close()
	p := Principal{AccountID: account, UserID: user, PermissionsResolved: true}
	found := false
	for rows.Next() {
		var admin bool
		var code, template string
		if err = rows.Scan(&admin, &code, &template); err != nil {
			return Principal{}, err
		}
		found = true
		if admin {
			p.Role = RolePlatformAdmin
		} else if p.Role != RolePlatformAdmin && CoarseRoleFromCoreRole(code, template) == RoleOwner {
			p.Role = RoleOwner
		}
	}
	if err = rows.Err(); err != nil {
		return Principal{}, err
	}
	rows.Close()
	if !found {
		return Principal{}, ErrAccountPrincipalDenied
	}
	p.Permissions, err = c.ResolveAccountPermissions(ctx, account, user)
	return p, err
}
