package operations

import (
	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
)

// canReadOperations decide se o principal pode LER a operacao. Quando as permissoes
// ja vierem resolvidas, o gate e a permissao dinamica (workspace.operacao.view);
// senao, cai no papel (compat enquanto nem toda sessao traz permissoes resolvidas).
func canReadOperations(access AccessContext) bool {
	if access.PermissionsResolved {
		return accesscontrol.HasPermission(access.Permissions, accesscontrol.PermissionOperationsView)
	}

	return CanAccessOperationsRole(access.Role)
}

func CanAccessOperationsRole(role string) bool {
	switch role {
	case RoleConsultant, RoleStoreTerminal, RoleManager, RoleMarketing, RoleDirector, RoleOwner, RolePlatformAdmin:
		return true
	default:
		return false
	}
}

func CanMutateOperationsRole(role string) bool {
	switch role {
	case RoleConsultant, RoleStoreTerminal, RoleManager, RoleOwner, RolePlatformAdmin:
		return true
	default:
		return false
	}
}

// canMutateOperations decide se o principal pode COMANDAR a operacao. View sozinho
// NAO muta (least-privilege): exige workspace.operacao.edit quando ha permissao
// resolvida.
func canMutateOperations(access AccessContext) bool {
	if access.PermissionsResolved {
		return accesscontrol.HasPermission(access.Permissions, accesscontrol.PermissionOperationsEdit)
	}

	return CanMutateOperationsRole(access.Role)
}
