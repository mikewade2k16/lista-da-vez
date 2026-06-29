import { computed, inject, provide, reactive, ref, watch } from 'vue'

import {
  ADVANCED_ACCESS_DEFINITIONS,
  WORKSPACE_ACCESS_DEFINITIONS,
  canManageUserPasswords,
  getAllowedWorkspaces,
  getWorkspaceAccessOptions,
  hasPermission,
  normalizePermissionKeys,
  readWorkspaceAccessState,
} from '~/domain/utils/permissions'
import {
  ALL_STORES_VALUE,
  PERMISSION_OVERRIDE_OPTIONS,
  applyPermissionOverrides,
  buildNickname,
  getAccessStateLabel,
  getAccessStateTone,
  getOnboardingLabel,
  getOnboardingTone,
  getOverrideEffect,
  getRoleLabel,
  isConsultantManaged,
  isStoreScopedRole,
  normalizeSearch,
  normalizeText,
} from '~/domain/utils/user-access'
import { useAccessControlStore } from '~/stores/access-control'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { useUsersStore } from '~/stores/users'
import { useUserAccessDrafts } from '~/composables/useUserAccessDrafts'

export const usersAccessContextKey = Symbol('usersAccessManager')

export function provideUsersAccessContext(ctx) {
  provide(usersAccessContextKey, ctx)
}

export function useUsersAccessContext() {
  const ctx = inject(usersAccessContextKey)
  if (!ctx) throw new Error('Users access context not provided')
  return ctx
}

const WORKSPACE_PREVIEW_ORDER = [
  'operacao',
  'tasks',
  'erp',
  'crm',
  'consultor',
  'ranking',
  'dados',
  'relatorios',
  'campanhas',
  'clientes',
  'multiloja',
  'configuracoes',
  'alertas',
  'feedback',
  'bi',
  'roadmap',
  'banco',
]

export function useUsersAccessManager(options = {}) {
  const auth = useAuthStore()
  const ui = useUiStore()
  const usersStore = useUsersStore()
  const accessStore = useAccessControlStore()
  const workspaceMode = normalizeText(options?.mode) === 'queue' ? 'queue' : 'admin'

  const createComposerOpen = ref(false)
  const createMode = ref('invite')
  const selectedDetailUser = ref(null)
  const rowBusy = reactive({})
  const detailSaving = ref(false)
  const detailAccessError = ref('')
  const detailWorkspaceStates = ref({})
  const detailAdvancedStates = ref({})

  const filters = reactive({
    search: '',
    status: 'active',
    role: '',
    store: '',
    tenant: '',
  })

  const canManagePasswords = computed(() =>
    canManageUserPasswords(auth.role, auth.permissionKeys, auth.permissionsResolved),
  )
  const canOverrideConsultantManaged = computed(() => normalizeText(auth.role) === 'platform_admin')
  const showTenantControls = computed(
    () => workspaceMode !== 'queue' && normalizeText(auth.role) === 'platform_admin',
  )
  const gridStorageKey = computed(() => `users-access-grid-columns-${workspaceMode}-v2`)
  const storeLookup = computed(
    () => new Map((auth.storeContext || []).map((store) => [String(store.id || '').trim(), store])),
  )
  const tenantLookup = computed(
    () =>
      new Map((auth.tenantContext || []).map((tenant) => [String(tenant.id || '').trim(), tenant])),
  )
  const clientFilterOptions = computed(() => [
    { value: '', label: 'Cliente' },
    ...(auth.tenantContext || []).map((tenant) => ({
      value: String(tenant.id || '').trim(),
      label: String(tenant.name || '').trim(),
    })),
  ])
  const statusFilterOptions = [
    { value: 'active', label: 'Status: ativos' },
    { value: 'inactive', label: 'Status: inativos' },
    { value: '', label: 'Status: todos' },
  ]

  const createRoleOptions = computed(() =>
    usersStore.assignableRoles
      .filter((role) => role.id !== 'consultant')
      .map((role) => ({
        value: role.id,
        label: getRoleLabel(role.id),
      })),
  )
  const editableRoleOptions = computed(() =>
    (canOverrideConsultantManaged.value
      ? usersStore.assignableRoles
      : usersStore.assignableRoles.filter((role) => role.id !== 'consultant')
    ).map((role) => ({
      value: role.id,
      label: getRoleLabel(role.id),
    })),
  )

  const {
    assignDetailDraft,
    createDraft,
    createRowDraft,
    detailDraft,
    getRowDraft,
    resetCreateDraft,
    resetRowDraft,
    rowDrafts,
  } = useUserAccessDrafts({
    auth,
    createRoleOptions,
    syncCreateScope,
    syncDetailScope,
  })

  const filterRoleOptions = computed(() => {
    const seen = new Set()
    const options = [{ value: '', label: 'Perfil' }]

    for (const user of usersStore.users) {
      const roleId = normalizeText(user.role)
      if (!roleId || seen.has(roleId)) continue

      seen.add(roleId)
      options.push({ value: roleId, label: getRoleLabel(roleId) })
    }

    return options
  })

  const storeFilterOptions = computed(() => [
    { value: '', label: 'Loja' },
    { value: ALL_STORES_VALUE, label: 'ALL' },
    ...(auth.storeContext || []).map((store) => ({
      value: String(store.id || '').trim(),
      label: String(store.name || '').trim(),
    })),
  ])

  const gridColumns = computed(() => {
    const columns = [
      { id: 'name', label: 'Nome', width: '1.55fr', locked: true },
      { id: 'nick', label: 'Nick', width: '0.78fr' },
      { id: 'email', label: 'Email', width: '1.35fr' },
      { id: 'status', label: 'Status', width: '0.68fr', align: 'center' },
      { id: 'profile', label: 'Perfil', width: '0.92fr' },
    ]

    if (showTenantControls.value) {
      columns.push({ id: 'tenant', label: 'Cliente', width: '1.02fr' })
    }

    columns.push(
      { id: 'store', label: 'Loja', width: '0.96fr' },
      { id: 'modules', label: 'Modulos', width: '1.22fr' },
      { id: 'employeeCode', label: 'Matricula', width: '0.72fr', align: 'center' },
      { id: 'onboarding', label: 'Acesso', width: '0.9fr' },
      { id: 'actions', label: 'Opcoes', width: '0.76fr', locked: true, align: 'end' },
    )

    return columns
  })

  const filteredUsers = computed(() => {
    return [...usersStore.users]
      .filter((user) => {
        const role = normalizeText(user.role)
        const tenantId = normalizeText(user.tenantId)
        const workspaceDefinitions = getUserVisibleWorkspaceDefinitions(user)
        const searchHaystack = normalizeSearch(
          [
            user.displayName,
            user.email,
            user.employeeCode,
            user.jobTitle,
            buildNickname(user.displayName),
            getTenantLabel(user),
            getStoreLabel(user),
            workspaceDefinitions.map((workspaceDefinition) => workspaceDefinition.label).join(' '),
            getRoleLabel(role),
          ].join(' '),
        )

        if (
          workspaceMode === 'queue' &&
          !workspaceDefinitions.some((workspaceDefinition) => workspaceDefinition.id === 'operacao')
        ) {
          return false
        }
        if (filters.search && !searchHaystack.includes(normalizeSearch(filters.search))) {
          return false
        }
        if (filters.status === 'active' && !user.active) return false
        if (filters.status === 'inactive' && user.active) return false
        if (filters.role && role !== filters.role) return false
        if (filters.tenant && tenantId !== filters.tenant) return false
        if (filters.store === ALL_STORES_VALUE) return !isStoreScopedRole(role)
        if (filters.store) {
          return Array.isArray(user.storeIds) && user.storeIds.includes(filters.store)
        }

        return true
      })
      .sort((left, right) => left.displayName.localeCompare(right.displayName, 'pt-BR'))
  })

  const selectedDetailUserId = computed(() => normalizeText(selectedDetailUser.value?.id))
  const selectedUserAccess = computed(() => accessStore.getUserAccess(selectedDetailUserId.value))
  const detailLoading = computed(
    () =>
      Boolean(selectedDetailUser.value) && accessStore.isUserPending(selectedDetailUserId.value),
  )
  const detailAccessReady = computed(
    () =>
      !detailLoading.value &&
      !detailAccessError.value &&
      accessStore.roleMatrix.length > 0 &&
      Boolean(selectedUserAccess.value),
  )
  const detailRoleLocked = computed(
    () => Boolean(selectedDetailUser.value) && isDetailLocked(selectedDetailUser.value),
  )
  const detailRoleOptions = computed(() => {
    if (!selectedDetailUser.value) return createRoleOptions.value
    return getDetailRoleOptions(selectedDetailUser.value)
  })
  const detailStoreOptions = computed(() => {
    if (!isStoreScopedRole(detailDraft.role)) return [{ value: ALL_STORES_VALUE, label: 'ALL' }]
    return getScopedStoreOptions(detailDraft.tenantId)
  })
  const detailBasePermissionKeys = computed(() =>
    normalizePermissionKeys(
      accessStore.roleLookup.get(normalizeText(detailDraft.role))?.permissionKeys || [],
    ),
  )
  const detailOverridePayload = computed(() =>
    buildDetailOverridePayload(detailBasePermissionKeys.value),
  )
  const detailEffectivePermissionKeys = computed(() =>
    applyPermissionOverrides(detailBasePermissionKeys.value, detailOverridePayload.value),
  )
  const detailWorkspaceRows = computed(() =>
    WORKSPACE_ACCESS_DEFINITIONS.map((workspaceDefinition) => ({
      ...workspaceDefinition,
      baseState: readWorkspaceAccessState(
        workspaceDefinition,
        detailBasePermissionKeys.value,
        'none',
      ),
      effectiveState: readWorkspaceAccessState(
        workspaceDefinition,
        detailEffectivePermissionKeys.value,
        'none',
      ),
      overrideState: detailWorkspaceStates.value[workspaceDefinition.id] || 'inherit',
    })),
  )
  const detailAdvancedRows = computed(() =>
    ADVANCED_ACCESS_DEFINITIONS.map((permissionDefinition) => ({
      ...permissionDefinition,
      baseEnabled: hasPermission(detailBasePermissionKeys.value, permissionDefinition.key),
      effectiveEnabled: hasPermission(
        detailEffectivePermissionKeys.value,
        permissionDefinition.key,
      ),
      overrideState: detailAdvancedStates.value[permissionDefinition.key] || 'inherit',
    })),
  )

  function switchToInviteMode() {
    createMode.value = 'invite'
    createDraft.password = ''
  }

  function isInlineLocked(user) {
    return isConsultantManaged(user) && !canOverrideConsultantManaged.value
  }

  function isDetailLocked(user) {
    return isInlineLocked(user)
  }

  function getStoreName(storeId) {
    return storeLookup.value.get(normalizeText(storeId))?.name || normalizeText(storeId) || '-'
  }

  function getTenantLabel(user) {
    const tenantId = normalizeText(user?.tenantId)
    if (!tenantId) return 'Plataforma'
    return tenantLookup.value.get(tenantId)?.name || tenantId
  }

  function getStoreLabel(user) {
    if (!isStoreScopedRole(user.role)) return 'ALL'

    const names = (Array.isArray(user.storeIds) ? user.storeIds : [])
      .map((storeId) => getStoreName(storeId))
      .filter(Boolean)
    return names.join(', ') || 'Loja nao vinculada'
  }

  function getUserPermissionKeys(user) {
    const userId = normalizeText(user?.id)
    const accessView = userId ? accessStore.getUserAccess(userId) : null
    if (
      Array.isArray(accessView?.effectivePermissionKeys) &&
      accessView.effectivePermissionKeys.length
    ) {
      return normalizePermissionKeys(accessView.effectivePermissionKeys)
    }

    return normalizePermissionKeys(
      accessStore.roleLookup.get(normalizeText(user?.role))?.permissionKeys || [],
    )
  }

  function sortWorkspaceDefinitions(workspaceDefinitions) {
    return [...workspaceDefinitions].sort((left, right) => {
      const leftIndex = WORKSPACE_PREVIEW_ORDER.indexOf(left.id)
      const rightIndex = WORKSPACE_PREVIEW_ORDER.indexOf(right.id)
      const normalizedLeftIndex = leftIndex === -1 ? Number.MAX_SAFE_INTEGER : leftIndex
      const normalizedRightIndex = rightIndex === -1 ? Number.MAX_SAFE_INTEGER : rightIndex

      if (normalizedLeftIndex !== normalizedRightIndex) {
        return normalizedLeftIndex - normalizedRightIndex
      }

      return left.label.localeCompare(right.label, 'pt-BR')
    })
  }

  function getUserVisibleWorkspaceDefinitions(user) {
    const permissionKeys = getUserPermissionKeys(user)
    const permissionsResolved = permissionKeys.length > 0 || Boolean(accessStore.roleMatrix.length)
    const allowedWorkspaceIds = new Set(
      getAllowedWorkspaces(normalizeText(user?.role), permissionKeys, permissionsResolved),
    )

    return sortWorkspaceDefinitions(
      WORKSPACE_ACCESS_DEFINITIONS.filter(
        (workspaceDefinition) =>
          allowedWorkspaceIds.has(workspaceDefinition.id) && workspaceDefinition.id !== 'usuarios',
      ),
    )
  }

  function getUserEditableWorkspaceCount(user) {
    const permissionKeys = getUserPermissionKeys(user)

    return getUserVisibleWorkspaceDefinitions(user).filter((workspaceDefinition) => {
      if (!normalizeText(workspaceDefinition.editPermission)) {
        return false
      }

      return readWorkspaceAccessState(workspaceDefinition, permissionKeys, 'none') === 'edit'
    }).length
  }

  function getUserWorkspaceSummary(user) {
    const workspaceDefinitions = getUserVisibleWorkspaceDefinitions(user)
    const previewItems = workspaceDefinitions.slice(0, 3).map((workspaceDefinition) => ({
      id: workspaceDefinition.id,
      label: workspaceDefinition.label,
    }))

    return {
      editableCount: getUserEditableWorkspaceCount(user),
      hiddenCount: Math.max(0, workspaceDefinitions.length - previewItems.length),
      previewItems,
      visibleCount: workspaceDefinitions.length,
    }
  }

  function getUserWorkspaceSummaryText(user) {
    const summary = getUserWorkspaceSummary(user)
    if (!summary.visibleCount) {
      return 'Sem modulos liberados'
    }

    return `${summary.visibleCount} modulos · ${summary.editableCount} com edicao`
  }

  function resetDetailOverrides() {
    detailWorkspaceStates.value = Object.fromEntries(
      WORKSPACE_ACCESS_DEFINITIONS.map((workspaceDefinition) => [
        workspaceDefinition.id,
        'inherit',
      ]),
    )
    detailAdvancedStates.value = Object.fromEntries(
      ADVANCED_ACCESS_DEFINITIONS.map((permissionDefinition) => [
        permissionDefinition.key,
        'inherit',
      ]),
    )
  }

  function syncCreateScope() {
    if (isStoreScopedRole(createDraft.role)) {
      const scopedStores = getScopedStoreOptions(createDraft.tenantId)
      if (!scopedStores.some((option) => option.value === createDraft.storeId)) {
        createDraft.storeId = scopedStores[0]?.value || ''
      }
      return
    }

    createDraft.storeId = ALL_STORES_VALUE
  }

  function syncDetailScope() {
    if (isStoreScopedRole(detailDraft.role)) {
      const scopedStores = getScopedStoreOptions(detailDraft.tenantId)
      if (!scopedStores.some((option) => option.value === detailDraft.storeId)) {
        detailDraft.storeId = scopedStores[0]?.value || ''
      }
      return
    }

    detailDraft.storeId = ALL_STORES_VALUE
  }

  function getScopedStoreOptions(tenantId) {
    const normalizedTenantId = normalizeText(tenantId)
    return (auth.storeContext || [])
      .filter(
        (store) => !normalizedTenantId || normalizeText(store.tenantId) === normalizedTenantId,
      )
      .map((store) => ({
        value: normalizeText(store.id),
        label: normalizeText(store.name),
      }))
  }

  function getRoleSelectOptions(user) {
    if (isInlineLocked(user))
      return [{ value: normalizeText(user.role), label: getRoleLabel(user.role) }]
    return editableRoleOptions.value
  }

  function getDetailRoleOptions(user) {
    if (isDetailLocked(user))
      return [{ value: normalizeText(user.role), label: getRoleLabel(user.role) }]
    return editableRoleOptions.value
  }

  function getStoreSelectOptions(user, draft) {
    const role = normalizeText(draft?.role || user?.role)
    if (!isStoreScopedRole(role)) return [{ value: ALL_STORES_VALUE, label: 'ALL' }]
    return getScopedStoreOptions(draft?.tenantId || user?.tenantId || auth.activeTenantId)
  }

  function getTenantSelectOptions(user) {
    const currentTenantId = normalizeText(user?.tenantId)
    const options = (auth.tenantContext || []).map((tenant) => ({
      value: normalizeText(tenant.id),
      label: normalizeText(tenant.name),
    }))

    if (normalizeText(user?.role) === 'platform_admin') {
      return [{ value: '', label: 'Plataforma' }]
    }

    if (!options.length && currentTenantId) {
      return [{ value: currentTenantId, label: getTenantLabel(user) }]
    }

    return options
  }

  function findUserById(userId) {
    return usersStore.users.find((user) => normalizeText(user.id) === normalizeText(userId)) || null
  }

  function clearFilters() {
    filters.search = ''
    filters.status = 'active'
    filters.role = ''
    filters.store = ''
    filters.tenant = ''
  }

  async function presentInvitation(invitationPayload, successMessage) {
    const inviteUrl = normalizeText(invitationPayload?.inviteUrl)
    if (!inviteUrl) {
      ui.success(successMessage)
      return
    }

    if (import.meta.client && navigator?.clipboard?.writeText) {
      try {
        await navigator.clipboard.writeText(inviteUrl)
        ui.success(`${successMessage} Link copiado.`)
        return
      } catch {
        // Clipboard can be blocked by browser policy; fall back to the prompt below.
      }
    }

    await ui.prompt({
      title: 'Link de convite',
      message: 'Copie o link abaixo para enviar ao usuario.',
      inputLabel: 'Convite',
      initialValue: inviteUrl,
      confirmLabel: 'Fechar',
    })
  }

  function withRowBusy(userId, callback) {
    if (rowBusy[userId]) return Promise.resolve()

    rowBusy[userId] = true
    return Promise.resolve(callback()).finally(() => {
      rowBusy[userId] = false
    })
  }

  function buildUpdatePayload(user) {
    const draft = getRowDraft(user)
    const tenantId =
      normalizeText(draft.role) === 'platform_admin'
        ? ''
        : normalizeText(draft.tenantId || user.tenantId || auth.activeTenantId)
    return {
      active: Boolean(draft.active),
      displayName: normalizeText(draft.displayName),
      email: normalizeText(draft.email),
      employeeCode: normalizeText(draft.employeeCode),
      role: normalizeText(draft.role),
      storeIds: isStoreScopedRole(draft.role) ? [normalizeText(draft.storeId)].filter(Boolean) : [],
      tenantId,
    }
  }

  function buildDetailUpdatePayload() {
    return {
      active: Boolean(detailDraft.active),
      displayName: normalizeText(detailDraft.displayName),
      email: normalizeText(detailDraft.email),
      employeeCode: normalizeText(detailDraft.employeeCode),
      role: normalizeText(detailDraft.role),
      storeIds: isStoreScopedRole(detailDraft.role)
        ? [normalizeText(detailDraft.storeId)].filter(Boolean)
        : [],
      tenantId:
        detailDraft.role === 'platform_admin'
          ? ''
          : normalizeText(
              detailDraft.tenantId || selectedDetailUser.value?.tenantId || auth.activeTenantId,
            ),
    }
  }

  function syncDetailOverridesFromAccess(accessView) {
    const nextWorkspaceStates = {}
    const nextAdvancedStates = {}

    for (const workspaceDefinition of WORKSPACE_ACCESS_DEFINITIONS) {
      const viewEffect = getOverrideEffect(
        accessView?.overrides,
        workspaceDefinition.viewPermission,
      )
      const editEffect = getOverrideEffect(
        accessView?.overrides,
        workspaceDefinition.editPermission,
      )

      if (!viewEffect && !editEffect) {
        nextWorkspaceStates[workspaceDefinition.id] = 'inherit'
      } else if (viewEffect === 'deny') {
        nextWorkspaceStates[workspaceDefinition.id] = 'none'
      } else if (editEffect === 'allow') {
        nextWorkspaceStates[workspaceDefinition.id] = 'edit'
      } else if (viewEffect === 'allow' || editEffect === 'deny') {
        nextWorkspaceStates[workspaceDefinition.id] = 'view'
      } else {
        nextWorkspaceStates[workspaceDefinition.id] = 'inherit'
      }
    }

    for (const permissionDefinition of ADVANCED_ACCESS_DEFINITIONS) {
      const effect = getOverrideEffect(accessView?.overrides, permissionDefinition.key)
      nextAdvancedStates[permissionDefinition.key] =
        effect === 'allow' || effect === 'deny' ? effect : 'inherit'
    }

    detailWorkspaceStates.value = nextWorkspaceStates
    detailAdvancedStates.value = nextAdvancedStates
  }

  function buildDetailOverridePayload(basePermissionKeys) {
    const overrideMap = new Map()

    for (const workspaceDefinition of WORKSPACE_ACCESS_DEFINITIONS) {
      const selectedState = detailWorkspaceStates.value[workspaceDefinition.id] || 'inherit'
      const baseState = readWorkspaceAccessState(workspaceDefinition, basePermissionKeys, 'none')

      if (selectedState === 'inherit' || selectedState === baseState) continue

      if (selectedState === 'none') {
        if (workspaceDefinition.viewPermission) {
          overrideMap.set(workspaceDefinition.viewPermission, {
            permissionKey: workspaceDefinition.viewPermission,
            effect: 'deny',
          })
        }
        if (workspaceDefinition.editPermission) {
          overrideMap.set(workspaceDefinition.editPermission, {
            permissionKey: workspaceDefinition.editPermission,
            effect: 'deny',
          })
        }
        continue
      }

      if (selectedState === 'view') {
        if (baseState === 'none' && workspaceDefinition.viewPermission) {
          overrideMap.set(workspaceDefinition.viewPermission, {
            permissionKey: workspaceDefinition.viewPermission,
            effect: 'allow',
          })
        }
        if (baseState === 'edit' && workspaceDefinition.editPermission) {
          overrideMap.set(workspaceDefinition.editPermission, {
            permissionKey: workspaceDefinition.editPermission,
            effect: 'deny',
          })
        }
        continue
      }

      if (selectedState === 'edit') {
        if (baseState === 'none' && workspaceDefinition.viewPermission) {
          overrideMap.set(workspaceDefinition.viewPermission, {
            permissionKey: workspaceDefinition.viewPermission,
            effect: 'allow',
          })
        }
        if (workspaceDefinition.editPermission && baseState !== 'edit') {
          overrideMap.set(workspaceDefinition.editPermission, {
            permissionKey: workspaceDefinition.editPermission,
            effect: 'allow',
          })
        }
      }
    }

    for (const permissionDefinition of ADVANCED_ACCESS_DEFINITIONS) {
      const selectedState = detailAdvancedStates.value[permissionDefinition.key] || 'inherit'
      const baseEnabled = hasPermission(basePermissionKeys, permissionDefinition.key)

      if (selectedState === 'inherit') continue
      if (selectedState === 'allow' && !baseEnabled) {
        overrideMap.set(permissionDefinition.key, {
          permissionKey: permissionDefinition.key,
          effect: 'allow',
        })
      }
      if (selectedState === 'deny' && baseEnabled) {
        overrideMap.set(permissionDefinition.key, {
          permissionKey: permissionDefinition.key,
          effect: 'deny',
        })
      }
    }

    return [...overrideMap.values()]
  }

  async function saveRow(user, { silent = true } = {}) {
    if (isInlineLocked(user)) {
      ui.info('Esse consultor continua gerenciado pelo roster por enquanto.')
      resetRowDraft(user)
      return
    }

    const payload = buildUpdatePayload(user)
    if (!payload.displayName || !payload.email) {
      ui.error('Nome e email sao obrigatorios.')
      resetRowDraft(user)
      return
    }

    if (isStoreScopedRole(payload.role) && payload.storeIds.length === 0) {
      ui.error('Selecione uma loja valida para este perfil.')
      resetRowDraft(user)
      return
    }

    await withRowBusy(user.id, async () => {
      const result = await usersStore.updateUser(user.id, payload)
      if (result?.ok === false) {
        ui.error(result.message || 'Nao foi possivel atualizar o acesso.')
        resetRowDraft(user)
        return
      }

      if (!silent && !result?.noChange) ui.success('Acesso atualizado.')
    })
  }

  async function handleInlineBlur(user) {
    await saveRow(user)
  }

  async function handleStatusChange(user, nextValue) {
    const draft = getRowDraft(user)
    draft.active = nextValue
    await saveRow(user)
  }

  async function handleRoleChange(user, nextRole) {
    const draft = getRowDraft(user)
    draft.role = normalizeText(nextRole)
    if (draft.role === 'platform_admin') {
      draft.tenantId = ''
      draft.storeId = ALL_STORES_VALUE
    } else if (!draft.tenantId) {
      draft.tenantId = normalizeText(user.tenantId || auth.activeTenantId)
    }

    if (!isStoreScopedRole(draft.role)) {
      draft.storeId = ALL_STORES_VALUE
    } else if (!draft.storeId || draft.storeId === ALL_STORES_VALUE) {
      draft.storeId = getStoreSelectOptions(user, draft)[0]?.value || ''
    }

    await saveRow(user)
  }

  async function handleTenantChange(user, nextTenantId) {
    const draft = getRowDraft(user)
    draft.tenantId = normalizeText(nextTenantId)

    if (isStoreScopedRole(draft.role)) {
      const nextStoreOptions = getStoreSelectOptions(user, draft)
      if (!nextStoreOptions.some((option) => option.value === draft.storeId)) {
        draft.storeId = nextStoreOptions[0]?.value || ''
      }
    }

    await saveRow(user)
  }

  async function handleStoreChange(user, nextStoreId) {
    const draft = getRowDraft(user)
    draft.storeId = normalizeText(nextStoreId)
    await saveRow(user)
  }

  async function refreshDetail(userId) {
    const nextUser = findUserById(userId)
    if (nextUser) selectedDetailUser.value = nextUser

    assignDetailDraft(nextUser || selectedDetailUser.value)
    detailAccessError.value = ''

    await accessStore.ensureRoleMatrix()
    if (!accessStore.roleMatrix.length && accessStore.errorMessage) {
      detailAccessError.value = accessStore.errorMessage
      resetDetailOverrides()
      return
    }

    try {
      const accessView = await accessStore.loadUserAccess(userId)
      syncDetailOverridesFromAccess(accessView)
    } catch {
      detailAccessError.value =
        accessStore.errorMessage ||
        'Nao foi possivel carregar a configuracao de acesso deste usuario.'
      resetDetailOverrides()
    }
  }

  async function handleArchiveAction(user) {
    if (isInlineLocked(user)) {
      ui.info('Arquive consultores pelo fluxo de roster enquanto o atalho unificado nao entra.')
      return
    }

    if (user.active) {
      const { confirmed } = await ui.confirm({
        title: 'Inativar acesso',
        message: `Deseja inativar ${user.displayName}?`,
        confirmLabel: 'Inativar',
      })

      if (!confirmed) return
    }

    const draft = getRowDraft(user)
    draft.active = !user.active
    await saveRow(user, { silent: false })

    if (selectedDetailUserId.value === normalizeText(user.id)) {
      await refreshDetail(user.id)
    }
  }

  async function handleInviteAction(user) {
    const result = await usersStore.inviteUser(user.id)
    if (result?.ok === false) {
      ui.error(result.message || 'Nao foi possivel gerar o convite.')
      return
    }

    await presentInvitation(result?.invitation, 'Convite gerado.')

    if (selectedDetailUserId.value === normalizeText(user.id)) {
      await refreshDetail(user.id)
    }
  }

  async function handleResetPassword(user) {
    const { confirmed, value } = await ui.prompt({
      title: 'Redefinir senha',
      message: `Defina uma senha temporaria para ${user.displayName}.`,
      inputLabel: 'Nova senha temporaria',
      inputPlaceholder: 'Minimo de 8 caracteres',
      confirmLabel: 'Salvar senha',
      required: true,
    })

    if (!confirmed) return

    const nextPassword = normalizeText(value)
    if (nextPassword.length < 8) {
      ui.error('Defina uma senha com pelo menos 8 caracteres.')
      return
    }

    const result = await usersStore.resetPassword(user.id, nextPassword)
    if (result?.ok === false) {
      ui.error(result.message || 'Nao foi possivel redefinir a senha.')
      return
    }

    ui.success('Senha temporaria redefinida.')

    if (selectedDetailUserId.value === normalizeText(user.id)) {
      await refreshDetail(user.id)
    }
  }

  async function submitCreate() {
    if (!normalizeText(createDraft.displayName) || !normalizeText(createDraft.email)) {
      ui.error('Nome e email sao obrigatorios.')
      return
    }

    if (createMode.value === 'password' && normalizeText(createDraft.password).length < 8) {
      ui.error('Defina uma senha inicial com pelo menos 8 caracteres.')
      return
    }

    if (isStoreScopedRole(createDraft.role) && !normalizeText(createDraft.storeId)) {
      ui.error('Selecione uma loja para este novo acesso.')
      return
    }

    const result = await usersStore.createUser({
      active: createDraft.active,
      displayName: createDraft.displayName,
      email: createDraft.email,
      employeeCode: createDraft.employeeCode,
      password: createMode.value === 'password' ? createDraft.password : '',
      role: createDraft.role,
      storeIds: isStoreScopedRole(createDraft.role) ? [createDraft.storeId].filter(Boolean) : [],
      tenantId: createDraft.role === 'platform_admin' ? '' : createDraft.tenantId,
    })

    if (result?.ok === false) {
      ui.error(result.message || 'Nao foi possivel criar o acesso.')
      return
    }

    const createdMode = createMode.value
    resetCreateDraft()
    createComposerOpen.value = false

    if (createdMode === 'password') {
      ui.success('Usuario criado com senha temporaria.')
      return
    }

    await presentInvitation(result?.invitation, 'Usuario criado e convidado.')
  }

  async function openDetails(user) {
    selectedDetailUser.value = user
    assignDetailDraft(user)
    resetDetailOverrides()
    detailAccessError.value = ''

    await accessStore.ensureRoleMatrix()
    if (!accessStore.roleMatrix.length && accessStore.errorMessage) {
      detailAccessError.value = accessStore.errorMessage
      return
    }

    try {
      const accessView = await accessStore.loadUserAccess(user.id)
      syncDetailOverridesFromAccess(accessView)
    } catch {
      detailAccessError.value =
        accessStore.errorMessage || 'Nao foi possivel carregar os overrides do usuario.'
    }
  }

  function closeDetails() {
    selectedDetailUser.value = null
    detailSaving.value = false
    detailAccessError.value = ''
    resetDetailOverrides()
  }

  async function saveDetails() {
    if (!selectedDetailUser.value || detailSaving.value) return

    if (detailRoleLocked.value) {
      ui.info('Esse consultor segue bloqueado pelo fluxo de roster.')
      return
    }

    const current = selectedDetailUser.value
    const payload = buildDetailUpdatePayload()

    // So consideramos "dados basicos mudaram" quando algum campo de identidade/escopo
    // difere do usuario atual. Mexer SO nos modulos (overrides) nao deve disparar a
    // validacao de loja/nome — antes isso bloqueava o save inteiro de um store_terminal
    // sem loja vinculada e dava a impressao de "nao salva os modulos".
    const sameStores =
      JSON.stringify((payload.storeIds || []).map(normalizeText)) ===
      JSON.stringify((Array.isArray(current.storeIds) ? current.storeIds : []).map(normalizeText))
    const basicChanged =
      normalizeText(payload.displayName) !== normalizeText(current.displayName) ||
      normalizeText(payload.email).toLowerCase() !== normalizeText(current.email).toLowerCase() ||
      normalizeText(payload.employeeCode) !== normalizeText(current.employeeCode) ||
      normalizeText(payload.role) !== normalizeText(current.role) ||
      Boolean(payload.active) !== Boolean(current.active) ||
      normalizeText(payload.tenantId) !== normalizeText(current.tenantId) ||
      !sameStores

    if (basicChanged) {
      if (!payload.displayName || !payload.email) {
        ui.error('Nome e email sao obrigatorios.')
        return
      }

      if (isStoreScopedRole(payload.role) && payload.storeIds.length === 0) {
        ui.error('Selecione uma loja valida para esse perfil.')
        return
      }
    }

    detailSaving.value = true

    if (basicChanged) {
      const updateResult = await usersStore.updateUser(current.id, payload)
      if (updateResult?.ok === false) {
        detailSaving.value = false
        ui.error(updateResult.message || 'Nao foi possivel salvar o usuario.')
        return
      }
    }

    if (!detailAccessReady.value) {
      detailSaving.value = false
      await refreshDetail(current.id)
      if (basicChanged) {
        ui.success('Dados do usuario atualizados.')
      }
      // Honestidade: nunca dizer que os modulos foram salvos quando nao foram.
      // Mostra o motivo REAL (erro da API de access ou matriz nao carregada).
      const reason = detailAccessError.value || 'a matriz de acesso nao carregou neste ambiente'
      ui.error(`Modulos NAO foram salvos: ${reason}.`)
      return
    }

    const accessResult = await accessStore.saveUserOverrides(
      selectedDetailUser.value.id,
      detailOverridePayload.value,
    )
    detailSaving.value = false

    if (accessResult?.ok === false) {
      detailAccessError.value =
        accessResult.message || 'Nao foi possivel salvar os overrides do usuario.'
      ui.error(accessResult.message || 'Nao foi possivel salvar os overrides do usuario.')
      await refreshDetail(selectedDetailUser.value.id)
      return
    }

    await refreshDetail(selectedDetailUser.value.id)
    ui.success('Acesso do usuario atualizado.')
  }

  function canShowInviteAction(user) {
    if (isInlineLocked(user)) return false
    return user.active && normalizeText(user.onboarding?.status) !== 'ready'
  }

  watch(
    () => usersStore.users,
    (users) => {
      const nextDrafts = {}
      for (const user of users) {
        nextDrafts[user.id] = createRowDraft(user)
      }
      rowDrafts.value = nextDrafts
    },
    { immediate: true, deep: true },
  )

  watch(
    () => createDraft.role,
    () => {
      syncCreateScope()
    },
  )

  watch(
    () => createDraft.tenantId,
    () => {
      syncCreateScope()
    },
  )

  watch(
    () => detailDraft.role,
    () => {
      if (selectedDetailUser.value) syncDetailScope()
    },
  )

  watch(
    () => detailDraft.tenantId,
    () => {
      if (selectedDetailUser.value) syncDetailScope()
    },
  )

  // Action-first: dispara o load de usuarios + roles em BACKGROUND. Antes havia um
  // `await usersStore.ensureLoaded()` aqui que, via o `const ctx = await
  // useUsersAccessManager(...)` no UsersAccessManager.vue (await de topo no
  // <script setup>), suspendia a troca de rota ate /v1/users + /v1/auth/roles
  // responderem — a pagina /usuarios ficava parada na rota anterior, sem skeleton.
  // Agora o setup retorna na hora (o provide acontece sincrono, o AppEntityGrid
  // pinta o skeleton via usersStore.pending) e os drafts re-sincronizam quando os
  // dados chegam, preservando a ordem do reset anterior (reset pos-load).
  void usersStore
    .ensureLoaded()
    .then(() => {
      resetCreateDraft()
      resetDetailOverrides()
    })
    .catch(() => {
      // ensureLoaded ja trata internamente a falha de /v1/users (return false); este
      // catch cobre uma falha de /v1/auth/roles (ensureRoleCatalog) para nao deixar
      // rejection nao-tratada no fire-and-forget. O grid mostra o estado vazio/erro
      // do store; os drafts seguem nos defaults declarados.
    })

  return reactive({
    allStoresValue: ALL_STORES_VALUE,
    auth,
    buildNickname,
    canManagePasswords,
    canOverrideConsultantManaged,
    canShowInviteAction,
    clearFilters,
    clientFilterOptions,
    closeDetails,
    createComposerOpen,
    createDraft,
    createMode,
    createRoleOptions,
    detailAccessError,
    detailAccessReady,
    detailAdvancedRows,
    detailAdvancedStates,
    detailDraft,
    detailLoading,
    detailOverridePayload,
    detailRoleLocked,
    detailRoleOptions,
    detailSaving,
    detailStoreOptions,
    detailWorkspaceRows,
    detailWorkspaceStates,
    editableRoleOptions,
    filterRoleOptions,
    filteredUsers,
    filters,
    getAccessStateLabel,
    getAccessStateTone,
    getOnboardingLabel,
    getOnboardingTone,
    getRoleLabel,
    getRoleSelectOptions,
    getRowDraft,
    getScopedStoreOptions,
    getStoreLabel,
    getStoreName,
    getStoreSelectOptions,
    getTenantLabel,
    getTenantSelectOptions,
    getUserWorkspaceSummary,
    getUserWorkspaceSummaryText,
    getWorkspaceAccessOptions,
    gridStorageKey,
    gridColumns,
    handleArchiveAction,
    handleInlineBlur,
    handleInviteAction,
    handleResetPassword,
    handleRoleChange,
    handleStatusChange,
    handleStoreChange,
    handleTenantChange,
    isConsultantManaged,
    isInlineLocked,
    isStoreScopedRole,
    normalizeText,
    openDetails,
    permissionOverrideOptions: PERMISSION_OVERRIDE_OPTIONS,
    refreshDetail,
    rowBusy,
    saveDetails,
    selectedDetailUser,
    showTenantControls,
    storeFilterOptions,
    statusFilterOptions,
    submitCreate,
    switchToInviteMode,
    tenantLookup,
    usersStore,
    workspaceMode,
  })
}
