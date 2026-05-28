import { reactive, ref, type ComputedRef } from 'vue'

import { ALL_STORES_VALUE, isStoreScopedRole, normalizeText } from '~/domain/utils/user-access'

type RoleOption = {
  label: string
  value: string
}

type AuthContext = {
  activeTenantId?: unknown
  storeContext?: Array<Record<string, unknown>>
  tenantContext?: Array<Record<string, unknown>>
}

type UserAccessDraftUser = {
  active?: boolean
  displayName?: unknown
  email?: unknown
  employeeCode?: unknown
  id?: unknown
  role?: unknown
  storeIds?: unknown[]
  tenantId?: unknown
}

type UserAccessDraftOptions = {
  auth: AuthContext
  createRoleOptions: ComputedRef<RoleOption[]>
  syncCreateScope: () => void
  syncDetailScope: () => void
}

function firstStoreId(user: UserAccessDraftUser | null | undefined) {
  return Array.isArray(user?.storeIds) ? user.storeIds[0] : ''
}

export function useUserAccessDrafts(options: UserAccessDraftOptions) {
  const rowDrafts = ref<Record<string, ReturnType<typeof createRowDraft>>>({})

  const createDraft = reactive({
    active: true,
    displayName: '',
    email: '',
    employeeCode: '',
    password: '',
    role: 'manager',
    storeId: '',
    tenantId: '',
  })

  const detailDraft = reactive({
    active: true,
    displayName: '',
    email: '',
    employeeCode: '',
    role: 'manager',
    storeId: ALL_STORES_VALUE,
    tenantId: '',
  })

  function createRowDraft(user: UserAccessDraftUser) {
    return {
      active: Boolean(user.active),
      displayName: normalizeText(user.displayName),
      email: normalizeText(user.email),
      employeeCode: normalizeText(user.employeeCode),
      role: normalizeText(user.role),
      tenantId: normalizeText(
        user.tenantId || options.auth.activeTenantId || options.auth.tenantContext?.[0]?.id,
      ),
      storeId: isStoreScopedRole(user.role) ? normalizeText(firstStoreId(user)) : ALL_STORES_VALUE,
    }
  }

  function createDetailDraft(user: UserAccessDraftUser | null = null) {
    return {
      active: Boolean(user?.active ?? true),
      displayName: normalizeText(user?.displayName),
      email: normalizeText(user?.email),
      employeeCode: normalizeText(user?.employeeCode),
      role: normalizeText(user?.role) || options.createRoleOptions.value[0]?.value || 'manager',
      storeId: isStoreScopedRole(user?.role) ? normalizeText(firstStoreId(user)) : ALL_STORES_VALUE,
      tenantId: normalizeText(
        user?.tenantId || options.auth.activeTenantId || options.auth.tenantContext?.[0]?.id,
      ),
    }
  }

  function assignDetailDraft(user: UserAccessDraftUser | null) {
    const draft = createDetailDraft(user)
    detailDraft.active = draft.active
    detailDraft.displayName = draft.displayName
    detailDraft.email = draft.email
    detailDraft.employeeCode = draft.employeeCode
    detailDraft.role = draft.role
    detailDraft.storeId = draft.storeId
    detailDraft.tenantId = draft.tenantId
    options.syncDetailScope()
  }

  function getRowDraft(user: UserAccessDraftUser) {
    const userId = normalizeText(user.id)
    if (!rowDrafts.value[userId]) {
      rowDrafts.value[userId] = createRowDraft(user)
    }

    return rowDrafts.value[userId]
  }

  function resetRowDraft(user: UserAccessDraftUser) {
    rowDrafts.value[normalizeText(user.id)] = createRowDraft(user)
  }

  function resetCreateDraft() {
    createDraft.active = true
    createDraft.displayName = ''
    createDraft.email = ''
    createDraft.employeeCode = ''
    createDraft.password = ''
    createDraft.role = options.createRoleOptions.value[0]?.value || 'manager'
    createDraft.storeId = normalizeText(options.auth.storeContext?.[0]?.id)
    createDraft.tenantId = normalizeText(
      options.auth.activeTenantId || options.auth.tenantContext?.[0]?.id,
    )
    options.syncCreateScope()
  }

  return {
    assignDetailDraft,
    createDraft,
    createRowDraft,
    detailDraft,
    getRowDraft,
    resetCreateDraft,
    resetRowDraft,
    rowDrafts,
  }
}
