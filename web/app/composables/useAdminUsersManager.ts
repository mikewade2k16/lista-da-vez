import type {
  AccountMembershipItem,
  AdminUserCreateInput,
  AdminUserFieldKey,
  AdminUserItem,
} from '~/types/admin-users'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

const PATCH_DELAY_MS = 380

// Campos editaveis inline → campo do PATCH backend (AdminUpdateUserInput).
const FIELD_TO_PATCH: Record<AdminUserFieldKey, string> = {
  email: 'email',
  displayName: 'displayName',
  nick: 'nick',
  isActive: 'isActive',
  isPlatformAdmin: 'isPlatformAdmin',
}

function normalizeUser(raw: Record<string, unknown>): AdminUserItem {
  return {
    id: String(raw.id ?? ''),
    email: String(raw.email ?? ''),
    displayName: String(raw.displayName ?? ''),
    nick: String(raw.nick ?? ''),
    avatarPath: String(raw.avatarPath ?? ''),
    isActive: Boolean(raw.isActive),
    isPlatformAdmin: Boolean(raw.isPlatformAdmin),
    mustChangePassword: Boolean(raw.mustChangePassword),
    accountCount: Number(raw.accountCount ?? 0) || 0,
    accountNames: String(raw.accountNames ?? ''),
    createdAt: String(raw.createdAt ?? ''),
    updatedAt: String(raw.updatedAt ?? ''),
  }
}

function normalizeMembership(raw: Record<string, unknown>): AccountMembershipItem {
  return {
    accountId: String(raw.accountId ?? ''),
    accountSlug: String(raw.accountSlug ?? ''),
    accountName: String(raw.accountName ?? ''),
    isActive: Boolean(raw.isActive),
    joinedAt: String(raw.joinedAt ?? ''),
  }
}

export function useAdminUsersManager() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const users = ref<AdminUserItem[]>([])
  const filters = reactive({
    q: '',
    status: '' as '' | 'active' | 'inactive',
    platformAdmin: '' as '' | 'true' | 'false',
  })
  const loading = ref(false)
  const creating = ref(false)
  const deletingId = ref<string | null>(null)
  const errorMessage = ref('')
  const savingMap = ref<Record<string, boolean>>({})
  const canResetFilters = computed(() =>
    Boolean(filters.q || filters.status || filters.platformAdmin),
  )

  const pendingTimers = new Map<string, ReturnType<typeof setTimeout>>()

  function setSaving(key: string, value: boolean) {
    const next = { ...savingMap.value }
    if (value) next[key] = true
    else delete next[key]
    savingMap.value = next
  }

  function rowIsSaving(id: string) {
    return Object.keys(savingMap.value).some((k) => k.startsWith(`${id}:`))
  }

  function applyPatch(id: string, raw: Record<string, unknown>) {
    const idx = users.value.findIndex((u) => u.id === id)
    if (idx >= 0) users.value[idx] = normalizeUser(raw)
  }

  function patchLocal(id: string, field: AdminUserFieldKey, value: unknown) {
    const idx = users.value.findIndex((u) => u.id === id)
    if (idx < 0) return
    ;(users.value[idx] as Record<string, unknown>)[field] = value
  }

  async function fetchUsers() {
    loading.value = true
    errorMessage.value = ''
    try {
      // O endpoint pagina (default 20, cap 100). A tela nao tem UI de paginacao,
      // entao buscamos TODAS as paginas (perPage=100) ate cobrir o total — senao
      // usuarios alem da pagina 1 "somem" da listagem.
      const perPage = 100
      const collected: Record<string, unknown>[] = []
      let page = 1
      let total = Infinity
      while (collected.length < total) {
        const resp = await apiRequest(`/v1/admin/users?page=${page}&perPage=${perPage}`)
        const batch = (resp.users as Record<string, unknown>[]) ?? []
        collected.push(...batch)
        total = Number(resp.total ?? collected.length)
        if (batch.length < perPage) break
        page += 1
      }
      users.value = collected.map(normalizeUser)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao carregar usuarios.')
    } finally {
      loading.value = false
    }
  }

  async function persistPatch(id: string, fieldKey: string, patch: Record<string, unknown>) {
    const key = `${id}:${fieldKey}`
    setSaving(key, true)
    try {
      const resp = await apiRequest(`/v1/admin/users/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        body: patch,
      })
      applyPatch(id, resp as Record<string, unknown>)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao salvar.')
    } finally {
      setSaving(key, false)
    }
  }

  function updateField(
    id: string,
    field: AdminUserFieldKey,
    value: unknown,
    opts?: { immediate?: boolean },
  ) {
    patchLocal(id, field, value)

    const backendField = FIELD_TO_PATCH[field]
    if (!backendField) return

    const patch = { [backendField]: value }
    const timerKey = `${id}:${field}`

    if (pendingTimers.has(timerKey)) clearTimeout(pendingTimers.get(timerKey)!)

    if (opts?.immediate) {
      void persistPatch(id, field, patch)
      return
    }

    pendingTimers.set(
      timerKey,
      setTimeout(() => {
        pendingTimers.delete(timerKey)
        void persistPatch(id, field, patch)
      }, PATCH_DELAY_MS),
    )
  }

  async function createUser(input: AdminUserCreateInput): Promise<string | null> {
    creating.value = true
    errorMessage.value = ''
    try {
      const resp = await apiRequest('/v1/admin/users', {
        method: 'POST',
        body: input,
      })
      const created = normalizeUser(resp as Record<string, unknown>)
      users.value.unshift(created)
      return created.id
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao criar usuario.')
      return null
    } finally {
      creating.value = false
    }
  }

  async function deleteUser(id: string) {
    deletingId.value = id
    setSaving(`${id}:delete`, true)
    errorMessage.value = ''
    try {
      await apiRequest(`/v1/admin/users/${encodeURIComponent(id)}`, { method: 'DELETE' })
      users.value = users.value.filter((u) => u.id !== id)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao excluir usuario.')
    } finally {
      deletingId.value = null
      setSaving(`${id}:delete`, false)
    }
  }

  async function fetchMemberships(id: string): Promise<AccountMembershipItem[]> {
    try {
      const resp = await apiRequest(`/v1/admin/users/${encodeURIComponent(id)}/memberships`)
      const raw = (resp as { memberships?: Record<string, unknown>[] }).memberships ?? []
      return raw.map(normalizeMembership)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao carregar memberships.')
      return []
    }
  }

  function resetFilters() {
    filters.q = ''
    filters.status = ''
    filters.platformAdmin = ''
  }

  onBeforeUnmount(() => {
    for (const timer of pendingTimers.values()) clearTimeout(timer)
    pendingTimers.clear()
  })

  return {
    users,
    filters,
    loading,
    creating,
    deletingId,
    errorMessage,
    savingMap,
    canResetFilters,
    rowIsSaving,
    fetchUsers,
    updateField,
    createUser,
    deleteUser,
    fetchMemberships,
    resetFilters,
  }
}
