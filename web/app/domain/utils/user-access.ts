export const ALL_STORES_VALUE = 'all'

export const ROLE_LABELS: Record<string, string> = {
  consultant: 'Consultor',
  manager: 'Gerente',
  marketing: 'Marketing',
  director: 'Diretor',
  owner: 'Gestao geral',
  platform_admin: 'Admin sistema',
  store_terminal: 'Usuario de loja',
}

export const ACCESS_STATE_LABELS: Record<string, string> = {
  inherit: 'Herdar padrao',
  none: 'Sem acesso',
  view: 'Somente ver',
  edit: 'Ver e editar',
  allow: 'Permitir',
  deny: 'Negar',
}

export const PERMISSION_OVERRIDE_OPTIONS = [
  { value: 'inherit', label: 'Herdar padrao' },
  { value: 'allow', label: 'Permitir' },
  { value: 'deny', label: 'Negar' },
]

type MaybeUser = {
  active?: boolean
  managedBy?: unknown
  onboarding?: {
    mustChangePassword?: boolean
    status?: unknown
  }
  role?: unknown
}

type MaybeOverride = {
  effect?: unknown
  isActive?: boolean
  permissionKey?: unknown
}

export function normalizeText(value: unknown) {
  return String(value || '').trim()
}

export function normalizeSearch(value: unknown) {
  return normalizeText(value)
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
}

export function getRoleLabel(role: unknown) {
  const roleId = normalizeText(role)
  return ROLE_LABELS[roleId] || roleId || 'Sem papel'
}

export function isStoreScopedRole(role: unknown) {
  const normalizedRole = normalizeText(role)
  return (
    normalizedRole === 'consultant' ||
    normalizedRole === 'manager' ||
    normalizedRole === 'store_terminal'
  )
}

export function isConsultantManaged(user: MaybeUser | null | undefined) {
  return (
    normalizeText(user?.managedBy) === 'consultants' || normalizeText(user?.role) === 'consultant'
  )
}

export function buildNickname(displayName: unknown) {
  const parts = normalizeText(displayName).split(/\s+/).filter(Boolean)
  if (!parts.length) return '-'

  const first = parts[0]
  const second = parts.length > 1 ? parts[1] : ''
  const nickname = second ? `${first} ${second.charAt(0).toUpperCase()}.` : first
  return nickname.length > 18 ? `${first.slice(0, 16)}...` : nickname
}

export function getOnboardingLabel(user: MaybeUser) {
  if (!user.active) return 'Conta inativa'
  if (user.onboarding?.mustChangePassword) return 'Troca pendente'

  switch (normalizeText(user.onboarding?.status)) {
    case 'ready':
      return 'Pronto'
    case 'pending':
      return 'Convite pendente'
    case 'expired':
      return 'Convite expirado'
    case 'inactive':
      return 'Conta inativa'
    default:
      return 'Sem convite'
  }
}

export function getOnboardingTone(user: MaybeUser) {
  if (user.onboarding?.mustChangePassword) {
    return 'users-access-manager__pill users-access-manager__pill--warning'
  }

  if (normalizeText(user.onboarding?.status) === 'ready') {
    return 'users-access-manager__pill users-access-manager__pill--success'
  }

  if (normalizeText(user.onboarding?.status) === 'pending') {
    return 'users-access-manager__pill users-access-manager__pill--info'
  }

  return 'users-access-manager__pill'
}

export function getAccessStateLabel(state: unknown) {
  return ACCESS_STATE_LABELS[normalizeText(state)] || 'Sem acesso'
}

export function getAccessStateTone(state: unknown) {
  switch (normalizeText(state)) {
    case 'edit':
    case 'allow':
      return 'users-access-manager__permission-pill users-access-manager__permission-pill--success'
    case 'view':
      return 'users-access-manager__permission-pill users-access-manager__permission-pill--info'
    case 'deny':
    case 'none':
      return 'users-access-manager__permission-pill users-access-manager__permission-pill--danger'
    default:
      return 'users-access-manager__permission-pill'
  }
}

export function getOverrideEffect(overrides: MaybeOverride[] | undefined, permissionKey: unknown) {
  const normalizedPermissionKey = normalizeText(permissionKey)
  const match = [...(Array.isArray(overrides) ? overrides : [])]
    .filter(
      (override) =>
        override?.isActive !== false &&
        normalizeText(override?.permissionKey) === normalizedPermissionKey,
    )
    .pop()

  return normalizeText(match?.effect)
}

export function applyPermissionOverrides(
  basePermissionKeys: unknown[],
  overrides: MaybeOverride[],
) {
  const effectivePermissions = new Set(basePermissionKeys.map((key) => normalizeText(key)))

  for (const override of overrides) {
    const permissionKey = normalizeText(override?.permissionKey)
    if (!permissionKey) continue

    if (normalizeText(override?.effect) === 'allow') {
      effectivePermissions.add(permissionKey)
      continue
    }

    if (normalizeText(override?.effect) === 'deny') {
      effectivePermissions.delete(permissionKey)
    }
  }

  return [...effectivePermissions]
}
