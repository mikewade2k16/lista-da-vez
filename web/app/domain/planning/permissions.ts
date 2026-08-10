import { hasPermission, normalizeAppRole } from '~/domain/utils/permissions'

const planningViewRoles = new Set(['platform_admin', 'owner', 'director', 'marketing', 'manager'])
const planningEditRoles = new Set(['platform_admin', 'owner', 'director', 'manager'])

export function canViewPlanning(
  role: unknown,
  permissionKeys: string[] = [],
  permissionsResolved = false,
): boolean {
  const normalizedRole = normalizeAppRole(role)
  if (permissionsResolved) {
    return (
      hasPermission(permissionKeys, 'workspace.planejamento.view') ||
      hasPermission(permissionKeys, 'workspace.planejamento.edit')
    )
  }
  return planningViewRoles.has(normalizedRole)
}

export function canEditPlanning(
  role: unknown,
  permissionKeys: string[] = [],
  permissionsResolved = false,
): boolean {
  const normalizedRole = normalizeAppRole(role)
  if (permissionsResolved) {
    return hasPermission(permissionKeys, 'workspace.planejamento.edit')
  }
  return planningEditRoles.has(normalizedRole)
}
