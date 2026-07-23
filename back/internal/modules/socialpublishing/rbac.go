package socialpublishing

import (
	"context"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type permissionChecker interface {
	HasAccountPermission(ctx context.Context, accountID, userID, permission string) (bool, error)
}

type permissionGate struct {
	checker permissionChecker
}

func newPermissionGate(checker permissionChecker) *permissionGate {
	return &permissionGate{checker: checker}
}

func (g *permissionGate) Authorize(
	ctx context.Context,
	principal auth.Principal,
	permission string,
) error {
	if principal.AccountID == "" || principal.UserID == "" {
		return ErrForbidden
	}
	if principal.Role == auth.RolePlatformAdmin || principal.Role == auth.RoleOwner {
		return nil
	}
	if g == nil || g.checker == nil {
		return ErrForbidden
	}
	allowed, err := g.checker.HasAccountPermission(
		ctx,
		principal.AccountID,
		principal.UserID,
		permission,
	)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}
