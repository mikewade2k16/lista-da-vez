package settings

import (
	"context"
	"strings"
	"time"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// RealtimePublisher e o contrato leve com o modulo realtime.
// Settings agora publica apenas no canal de contexto (tenant-wide), porque a
// configuracao deixou de ser por loja: qualquer loja do tenant deve revalidar.
type RealtimePublisher interface {
	PublishContextEvent(ctx context.Context, tenantID string, resource string, action string, resourceID string, savedAt time.Time)
}

type Service struct {
	repository Repository
	notifier   RealtimePublisher
}

func NewService(repository Repository, notifier RealtimePublisher) *Service {
	return &Service{repository: repository, notifier: notifier}
}

// resolveTenantID usa um tenant explicito quando a UI envia activeTenantId.
// Sem tenant explicito, principals globais ainda usam apenas o fallback seguro
// de tenant unico; zero ou multiplos tenants acessiveis seguem ambiguos.
func (service *Service) resolveTenantID(ctx context.Context, principal auth.Principal, requestedTenantID string) (string, error) {
	requestedTenantID = strings.TrimSpace(requestedTenantID)
	if requestedTenantID != "" {
		if tenantID := strings.TrimSpace(principal.TenantID); tenantID != "" && tenantID != requestedTenantID {
			return "", ErrForbidden
		}

		allowed, err := service.repository.CanAccessTenant(ctx, principal, requestedTenantID)
		if err != nil {
			return "", err
		}

		if !allowed {
			return "", ErrForbidden
		}

		return requestedTenantID, nil
	}

	tenantID := strings.TrimSpace(principal.TenantID)
	if tenantID != "" {
		return tenantID, nil
	}

	return service.repository.ResolveDefaultTenantID(ctx, principal)
}

func (service *Service) resolveWritableTenantID(ctx context.Context, principal auth.Principal, requestedTenantID string) (string, error) {
	if !canEditSettings(principal) {
		return "", ErrForbidden
	}

	return service.resolveTenantID(ctx, principal, requestedTenantID)
}

func (service *Service) finalizeMutation(ctx context.Context, ack MutationAck, err error) (MutationAck, error) {
	if err != nil {
		return MutationAck{}, err
	}

	service.publishSettingsEvent(ctx, ack.TenantID, "updated", ack.SavedAt)
	return ack, nil
}

func (service *Service) publishSettingsEvent(ctx context.Context, tenantID string, action string, savedAt time.Time) {
	if service.notifier == nil {
		return
	}

	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return
	}

	service.notifier.PublishContextEvent(ctx, tenantID, "settings", strings.TrimSpace(action), tenantID, savedAt)
}

func canViewSettings(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionSettingsView) ||
			accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionOperationsView)
	}

	defaultPermissions := accesscontrol.DefaultRolePermissions(principal.Role)
	return accesscontrol.HasPermission(defaultPermissions, accesscontrol.PermissionSettingsView) ||
		accesscontrol.HasPermission(defaultPermissions, accesscontrol.PermissionOperationsView)
}

func canEditSettings(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionSettingsEdit)
	}

	return principal.Role == auth.RoleOwner || principal.Role == auth.RolePlatformAdmin
}

func newMutationAck(tenantID string, savedAt time.Time) MutationAck {
	return MutationAck{
		OK:       true,
		TenantID: tenantID,
		SavedAt:  savedAt,
	}
}

func isValidOptionGroup(optionGroup string) bool {
	switch optionGroup {
	case optionKindVisitReason, optionKindCustomerSource, optionKindPauseReason,
		optionKindCancelReason, optionKindStopReason, optionKindQueueJump, optionKindLossReason, optionKindProfession:
		return true
	default:
		return false
	}
}
