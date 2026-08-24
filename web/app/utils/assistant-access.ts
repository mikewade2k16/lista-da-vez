type AssistantAccessContext = {
  role?: string
  effectivePermissionKeys?: readonly string[]
  effectivePermissionsResolved?: boolean
}

const ASSISTANT_CONFIG_VIEW_PERMISSIONS = new Set([
  'automation.manage',
  'calendar.manage',
  'meta_ads.manage',
  'omnichannel.agents.manage',
  'core.account.manage',
  'workspace.configuracoes.edit',
])

const ASSISTANT_CONFIG_MANAGE_PERMISSIONS = new Set([
  'automation.manage',
  'core.account.manage',
  'workspace.configuracoes.edit',
])

function hasAssistantPermission(
  context: AssistantAccessContext,
  accepted: ReadonlySet<string>,
): boolean {
  if (context.role === 'platform_admin' || context.role === 'owner') return true
  if (!context.effectivePermissionsResolved) return false
  return (context.effectivePermissionKeys || []).some((permission) => accepted.has(permission))
}

export function canViewAssistantConfiguration(context: AssistantAccessContext): boolean {
  return hasAssistantPermission(context, ASSISTANT_CONFIG_VIEW_PERMISSIONS)
}

export function canManageAssistantConfiguration(context: AssistantAccessContext): boolean {
  return hasAssistantPermission(context, ASSISTANT_CONFIG_MANAGE_PERMISSIONS)
}
