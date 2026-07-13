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

// CanValidateAutoCloseRole decide quem pode VALIDAR/CANCELAR uma pendencia de
// auto-encerramento (2h). E' uma acao de GESTAO: gerente, dono e platform_admin.
// Consultor e terminal operam a fila mas NAO validam a metrica (least-privilege).
func CanValidateAutoCloseRole(role string) bool {
	switch role {
	case RoleManager, RoleOwner, RolePlatformAdmin:
		return true
	default:
		return false
	}
}

// canValidateAutoClose gateia validar/cancelar pendencia. Exige poder mutar a
// operacao E ser papel de gestao (gerente+). O gate por papel vale mesmo com
// permissoes resolvidas, ate existir uma permissao dinamica dedicada
// (queue.operations.validate) semeada nos role templates.
func canValidateAutoClose(access AccessContext) bool {
	return canMutateOperations(access) && CanValidateAutoCloseRole(access.Role)
}
