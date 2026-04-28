package access

import (
	"context"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Service struct {
	repository      Repository
	subjectResolver SubjectResolver
	notifier        ContextPublisher
}

type ContextPublisher interface {
	PublishContextEvent(ctx context.Context, tenantID string, resource string, action string, resourceID string, savedAt time.Time)
}

func NewService(repository Repository, subjectResolver SubjectResolver) *Service {
	return &Service{
		repository:      repository,
		subjectResolver: subjectResolver,
	}
}

func (service *Service) SetContextPublisher(notifier ContextPublisher) {
	service.notifier = notifier
}

func (service *Service) ResolveUserPermissions(ctx context.Context, userID string, role auth.Role) ([]string, error) {
	basePermissionKeys, err := service.repository.ListRolePermissions(ctx, role)
	if err != nil {
		return nil, err
	}
	if len(basePermissionKeys) == 0 {
		basePermissionKeys = DefaultRolePermissions(role)
	}

	overrides, err := service.repository.ListUserOverrides(ctx, userID)
	if err != nil {
		return nil, err
	}

	return EffectivePermissionKeys(basePermissionKeys, overrides), nil
}

func (service *Service) ListRoleMatrix(ctx context.Context, principal auth.Principal) (RoleMatrixView, error) {
	if !canViewUserAccess(principal) {
		return RoleMatrixView{}, ErrForbidden
	}

	storedGrants, err := service.repository.ListAllRolePermissions(ctx)
	if err != nil {
		return RoleMatrixView{}, err
	}

	storedByRole := make(map[auth.Role][]string)
	for _, grant := range storedGrants {
		storedByRole[grant.Role] = append(storedByRole[grant.Role], grant.PermissionKey)
	}

	entries := make([]RoleMatrixEntry, 0, len(auth.RoleCatalog()))
	for _, definition := range auth.RoleCatalog() {
		permissionKeys := RecognizedPermissionKeys(storedByRole[definition.ID])
		if len(permissionKeys) == 0 {
			permissionKeys = DefaultRolePermissions(definition.ID)
		}

		entries = append(entries, RoleMatrixEntry{
			Role:           definition.ID,
			Label:          definition.Label,
			Scope:          definition.Scope,
			PermissionKeys: permissionKeys,
		})
	}

	return RoleMatrixView{
		Permissions: PermissionCatalog(),
		Roles:       entries,
	}, nil
}

func (service *Service) UpdateRolePermissions(ctx context.Context, principal auth.Principal, role auth.Role, permissionKeys []string) (RoleMatrixEntry, error) {
	if !canEditRoleMatrix(principal) {
		return RoleMatrixEntry{}, ErrForbidden
	}
	if !auth.IsValidRole(role) {
		return RoleMatrixEntry{}, ErrValidation
	}

	normalizedKeys := RecognizedPermissionKeys(permissionKeys)
	if err := service.repository.ReplaceRolePermissions(ctx, role, normalizedKeys); err != nil {
		return RoleMatrixEntry{}, err
	}

	service.publishRoleMatrixUpdate(ctx, role)

	for _, definition := range auth.RoleCatalog() {
		if definition.ID != role {
			continue
		}

		return RoleMatrixEntry{
			Role:           definition.ID,
			Label:          definition.Label,
			Scope:          definition.Scope,
			PermissionKeys: normalizedKeys,
		}, nil
	}

	return RoleMatrixEntry{}, ErrValidation
}

func (service *Service) GetUserAccess(ctx context.Context, principal auth.Principal, userID string) (UserAccessView, error) {
	if !canViewUserAccess(principal) {
		return UserAccessView{}, ErrForbidden
	}
	if service.subjectResolver == nil {
		return UserAccessView{}, ErrValidation
	}

	subject, err := service.subjectResolver.FindAccessibleSubject(ctx, principal, userID)
	if err != nil {
		return UserAccessView{}, err
	}

	return service.buildUserAccessView(ctx, subject)
}

func (service *Service) UpdateUserOverrides(ctx context.Context, principal auth.Principal, userID string, overrides []UserOverride) (UserAccessView, error) {
	if !canEditUserAccess(principal) {
		return UserAccessView{}, ErrForbidden
	}
	if service.subjectResolver == nil {
		return UserAccessView{}, ErrValidation
	}

	subject, err := service.subjectResolver.FindAccessibleSubject(ctx, principal, userID)
	if err != nil {
		return UserAccessView{}, err
	}

	normalizedOverrides, err := normalizeOverridesForSubject(subject, overrides)
	if err != nil {
		return UserAccessView{}, err
	}

	if _, err := service.repository.ReplaceUserOverrides(ctx, subject.UserID, normalizedOverrides, principal.UserID); err != nil {
		return UserAccessView{}, err
	}

	service.publishContextEvent(ctx, subject.TenantID, "user-overrides-updated", subject.UserID)

	return service.buildUserAccessView(ctx, subject)
}

func (service *Service) buildUserAccessView(ctx context.Context, subject UserSubject) (UserAccessView, error) {
	basePermissionKeys, err := service.repository.ListRolePermissions(ctx, subject.Role)
	if err != nil {
		return UserAccessView{}, err
	}
	if len(basePermissionKeys) == 0 {
		basePermissionKeys = DefaultRolePermissions(subject.Role)
	}

	overrides, err := service.repository.ListUserOverrides(ctx, subject.UserID)
	if err != nil {
		return UserAccessView{}, err
	}

	effectivePermissionKeys := EffectivePermissionKeys(basePermissionKeys, overrides)

	return UserAccessView{
		UserID:                  subject.UserID,
		Role:                    subject.Role,
		TenantID:                subject.TenantID,
		StoreIDs:                append([]string{}, subject.StoreIDs...),
		Permissions:             PermissionCatalog(),
		BasePermissionKeys:      basePermissionKeys,
		EffectivePermissionKeys: effectivePermissionKeys,
		Overrides:               overrides,
	}, nil
}

func normalizeOverridesForSubject(subject UserSubject, overrides []UserOverride) ([]UserOverride, error) {
	normalizedOverrides := make([]UserOverride, 0, len(overrides))
	for _, override := range overrides {
		permissionKey := strings.TrimSpace(override.PermissionKey)
		definition, ok := PermissionDefinitionForKey(permissionKey)
		if !ok {
			return nil, ErrValidation
		}

		effect := strings.TrimSpace(override.Effect)
		if effect != EffectAllow && effect != EffectDeny {
			return nil, ErrValidation
		}

		normalized := UserOverride{
			UserID:        subject.UserID,
			PermissionKey: permissionKey,
			Effect:        effect,
			Note:          strings.TrimSpace(override.Note),
			IsActive:      true,
		}

		switch definition.Scope {
		case ScopeTenant:
			normalized.TenantID = strings.TrimSpace(subject.TenantID)
		case ScopeStore:
			if len(subject.StoreIDs) > 0 {
				normalized.StoreID = strings.TrimSpace(subject.StoreIDs[0])
			}
			normalized.TenantID = strings.TrimSpace(subject.TenantID)
		}

		normalizedOverrides = append(normalizedOverrides, normalized)
	}

	return normalizedOverrides, nil
}

func canViewUserAccess(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return HasPermission(principal.Permissions, PermissionUsersView)
	}

	return principal.Role == auth.RoleOwner || principal.Role == auth.RolePlatformAdmin
}

func canEditUserAccess(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return HasPermission(principal.Permissions, PermissionUsersEdit)
	}

	return principal.Role == auth.RoleOwner || principal.Role == auth.RolePlatformAdmin
}

func canEditRoleMatrix(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return HasPermission(principal.Permissions, PermissionRoleMatrixEdit)
	}

	return principal.Role == auth.RolePlatformAdmin
}

func (service *Service) publishRoleMatrixUpdate(ctx context.Context, role auth.Role) {
	if service.notifier == nil {
		return
	}

	tenantIDs, err := service.repository.ListActiveTenantIDs(ctx)
	if err != nil {
		return
	}

	service.publishContextEvents(ctx, tenantIDs, "role-defaults-updated", string(role))
}

func (service *Service) publishContextEvents(ctx context.Context, tenantIDs []string, action string, resourceID string) {
	seenTenantIDs := make(map[string]struct{}, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		normalizedTenantID := strings.TrimSpace(tenantID)
		if normalizedTenantID == "" {
			continue
		}
		if _, seen := seenTenantIDs[normalizedTenantID]; seen {
			continue
		}

		seenTenantIDs[normalizedTenantID] = struct{}{}
		service.publishContextEvent(ctx, normalizedTenantID, action, resourceID)
	}
}

func (service *Service) publishContextEvent(ctx context.Context, tenantID string, action string, resourceID string) {
	if service.notifier == nil {
		return
	}

	normalizedTenantID := strings.TrimSpace(tenantID)
	if normalizedTenantID == "" {
		return
	}

	service.notifier.PublishContextEvent(ctx, normalizedTenantID, "access", strings.TrimSpace(action), strings.TrimSpace(resourceID), time.Now().UTC())
}
