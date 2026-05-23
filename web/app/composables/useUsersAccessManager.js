import { computed, inject, provide, reactive, ref, watch } from 'vue'

import {
  ADVANCED_ACCESS_DEFINITIONS,
  WORKSPACE_ACCESS_DEFINITIONS,
  canManageUserPasswords,
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

export async function useUsersAccessManager() {
  const auth = useAuthStore()
  const ui = useUiStore()
  const usersStore = useUsersStore()
  const accessStore = useAccessControlStore()

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

  const gridColumns = computed(() => [
    { id: 'name', label: 'Nome', width: '1.55fr', locked: true },
    { id: 'nick', label: 'Nick', width: '0.78fr' },
    { id: 'email', label: 'Email', width: '1.35fr' },
    { id: 'status', label: 'Status', width: '0.68fr', align: 'center' },
    { id: 'profile', label: 'Perfil', width: '0.92fr' },
    { id: 'store', label: 'Loja', width: '0.96fr' },
    { id: 'employeeCode', label: 'Matricula', width: '0.72fr', align: 'center' },
    { id: 'onboarding', label: 'Acesso', width: '0.9fr' },
    { id: 'actions', label: 'Opcoes', width: '0.76fr', locked: true, align: 'end' },
  ])

  const filteredUsers = computed(() => {
    return [...usersStore.users]
      .filter((user) => {
        const role = normalizeText(user.role)
        const tenantId = normalizeText(user.tenantId)
        const searchHaystack = normalizeSearch(
          [
            user.displayName,
            user.email,
            user.employeeCode,
            user.jobTitle,
            buildNickname(user.displayName),
            getStoreLabel(user),
            getRoleLabel(role),
          ].join(' '),
        )

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

  function getStoreLabel(user) {
    if (!isStoreScopedRole(user.role)) return 'ALL'

    const names = (Array.isArray(user.storeIds) ? user.storeIds : [])
      .map((storeId) => getStoreName(storeId))
      .filter(Boolean)
    return names.join(', ') || 'Loja nao vinculada'
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
    return getScopedStoreOptions(user?.tenantId || auth.activeTenantId)
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
    return {
      active: Boolean(draft.active),
      displayName: normalizeText(draft.displayName),
      email: normalizeText(draft.email),
      employeeCode: normalizeText(draft.employeeCode),
      role: normalizeText(draft.role),
      storeIds: isStoreScopedRole(draft.role) ? [normalizeText(draft.storeId)].filter(Boolean) : [],
      tenantId: normalizeText(user.tenantId || auth.activeTenantId),
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
    if (!isStoreScopedRole(draft.role)) {
      draft.storeId = ALL_STORES_VALUE
    } else if (!draft.storeId || draft.storeId === ALL_STORES_VALUE) {
      draft.storeId = getStoreSelectOptions(user, draft)[0]?.value || ''
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

    const payload = buildDetailUpdatePayload()
    if (!payload.displayName || !payload.email) {
      ui.error('Nome e email sao obrigatorios.')
      return
    }

    if (isStoreScopedRole(payload.role) && payload.storeIds.length === 0) {
      ui.error('Selecione uma loja valida para esse perfil.')
      return
    }

    detailSaving.value = true

    const updateResult = await usersStore.updateUser(selectedDetailUser.value.id, payload)
    if (updateResult?.ok === false) {
      detailSaving.value = false
      ui.error(updateResult.message || 'Nao foi possivel salvar o usuario.')
      return
    }

    if (!detailAccessReady.value) {
      detailSaving.value = false
      await refreshDetail(selectedDetailUser.value.id)
      ui.success('Dados do usuario atualizados.')
      if (detailAccessError.value) {
        ui.info(
          'A area de permissoes continua indisponivel enquanto a API de access nao estiver ativa.',
        )
      }
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

  await usersStore.ensureLoaded()
  resetCreateDraft()
  resetDetailOverrides()

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
    getWorkspaceAccessOptions,
    gridColumns,
    handleArchiveAction,
    handleInlineBlur,
    handleInviteAction,
    handleResetPassword,
    handleRoleChange,
    handleStatusChange,
    handleStoreChange,
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
    storeFilterOptions,
    statusFilterOptions,
    submitCreate,
    switchToInviteMode,
    tenantLookup,
    usersStore,
  })
}
