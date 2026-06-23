import type {
  AccountMembershipItem,
  AdminUserCreateInput,
  AdminUserFieldKey,
  AdminUserItem,
} from '~/types/admin-users'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

const PATCH_DELAY_MS = 380
// Espelha o minimo do backend (admin_users_service.go: "must be at least 8 chars").
const PASSWORD_MIN_LENGTH = 8

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
    clientAccountId: String(raw.clientAccountId ?? ''),
    isAgencyMember: Boolean(raw.isAgencyMember),
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
    role: String(raw.role ?? ''),
    isAgency: Boolean(raw.isAgency),
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
    // Filtro server-side por cliente (account). Vazio = todos. Quando preenchido,
    // a lista volta so com os membros ativos daquele cliente.
    accountId: '',
  })
  // Paginacao server-side: a tela busca UMA pagina por vez com os filtros
  // aplicados no backend (q/status/platformAdmin), em vez de baixar todos os
  // usuarios e filtrar no cliente. Espelha AGENT_RULES "lista grande -> paginacao".
  const page = ref(1)
  const perPage = ref(50)
  const total = ref(0)
  const loading = ref(false)
  const creating = ref(false)
  const deletingId = ref<string | null>(null)
  const errorMessage = ref('')
  const savingMap = ref<Record<string, boolean>>({})
  const canResetFilters = computed(() =>
    Boolean(filters.q || filters.status || filters.platformAdmin || filters.accountId),
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

  function buildListQuery(): string {
    const params = new URLSearchParams()
    params.set('page', String(page.value))
    params.set('perPage', String(perPage.value))
    if (filters.q.trim()) params.set('q', filters.q.trim())
    if (filters.status) params.set('status', filters.status)
    if (filters.platformAdmin) params.set('platformAdmin', filters.platformAdmin)
    if (filters.accountId.trim()) params.set('accountId', filters.accountId.trim())
    return params.toString()
  }

  async function fetchUsers(opts?: { page?: number }) {
    if (opts?.page) page.value = opts.page
    loading.value = true
    errorMessage.value = ''
    try {
      // UMA pagina por vez, filtrada no servidor. A tela renderiza exatamente o
      // que o backend devolve (sem filtro client-side sobre o conjunto inteiro).
      const resp = await apiRequest(`/v1/admin/users?${buildListQuery()}`)
      const batch = (resp.users as Record<string, unknown>[]) ?? []
      users.value = batch.map(normalizeUser)
      total.value = Number(resp.total ?? batch.length)
      page.value = Number(resp.page ?? page.value) || 1
      perPage.value = Number(resp.perPage ?? perPage.value) || perPage.value
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
      total.value += 1
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
      // Soft-delete (is_active=false). Recarrega a pagina atual para refletir o
      // estado real do servidor (filtros/paginacao server-side).
      await fetchUsers()
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao excluir usuario.')
    } finally {
      deletingId.value = null
      setSaving(`${id}:delete`, false)
    }
  }

  // Define/reseta a senha de um usuario (acao explicita do admin). Envia no mesmo
  // PATCH de admin users: o backend so toca no password_hash quando vem `password`
  // nao-vazio (semantica "ausente = nao mexe"). Gate de platform_admin e na UI.
  async function setPassword(id: string, password: string): Promise<boolean> {
    const pw = String(password ?? '').trim()
    if (pw.length < PASSWORD_MIN_LENGTH) {
      errorMessage.value = `A senha deve ter pelo menos ${PASSWORD_MIN_LENGTH} caracteres.`
      return false
    }
    setSaving(`${id}:password`, true)
    errorMessage.value = ''
    try {
      const resp = await apiRequest(`/v1/admin/users/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        body: { password: pw },
      })
      applyPatch(id, resp as Record<string, unknown>)
      return true
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao definir a senha.')
      return false
    } finally {
      setSaving(`${id}:password`, false)
    }
  }

  // Move o usuario para outro cliente (account), como owner. Backend:
  // PUT /v1/admin/users/{id}/account body { accountId, role }. Retorna o user
  // atualizado (mesmo shape do PATCH) — aplicamos na linha. Destino inexistente
  // -> 404; conta-agencia -> 400 (o errorMessage carrega a mensagem do backend).
  async function moveUserAccount(userId: string, accountId: string): Promise<boolean> {
    const target = String(accountId ?? '').trim()
    if (!userId || !target) return false
    setSaving(`${userId}:account`, true)
    errorMessage.value = ''
    try {
      const resp = await apiRequest(`/v1/admin/users/${encodeURIComponent(userId)}/account`, {
        method: 'PUT',
        body: { accountId: target, role: 'owner' },
      })
      applyPatch(userId, resp as Record<string, unknown>)
      return true
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao mover o usuario de cliente.')
      return false
    } finally {
      setSaving(`${userId}:account`, false)
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

  // Troca o nivel/papel do usuario numa conta (cliente ou conta-agencia). Devolve a
  // lista de memberships atualizada (o backend re-monta) ou null em caso de erro.
  async function updateMembershipRole(
    userId: string,
    accountId: string,
    role: string,
  ): Promise<AccountMembershipItem[] | null> {
    const key = `${userId}:membership:${accountId}`
    setSaving(key, true)
    errorMessage.value = ''
    try {
      const resp = await apiRequest(
        `/v1/admin/users/${encodeURIComponent(userId)}/memberships/${encodeURIComponent(accountId)}`,
        { method: 'PATCH', body: { role } },
      )
      const raw = (resp as { memberships?: Record<string, unknown>[] }).memberships ?? []
      return raw.map(normalizeMembership)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao atualizar o nivel do usuario.')
      return null
    } finally {
      setSaving(key, false)
    }
  }

  function resetFilters() {
    filters.q = ''
    filters.status = ''
    filters.platformAdmin = ''
    filters.accountId = ''
    page.value = 1
  }

  onBeforeUnmount(() => {
    for (const timer of pendingTimers.values()) clearTimeout(timer)
    pendingTimers.clear()
  })

  return {
    users,
    filters,
    page,
    perPage,
    total,
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
    setPassword,
    moveUserAccount,
    fetchMemberships,
    updateMembershipRole,
    resetFilters,
  }
}
