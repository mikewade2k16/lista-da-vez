import type {
  AdminOrganizationCreateInput,
  AdminOrganizationFieldKey,
  AdminOrganizationItem,
} from '~/types/admin-organizations'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

const PATCH_DELAY_MS = 380

// Mapeia campo frontend → campo do PATCH backend (AdminUpdateOrganizationInput).
const FIELD_TO_PATCH: Record<AdminOrganizationFieldKey, string> = {
  name: 'name',
  slug: 'slug',
  isActive: 'isActive',
}

function normalizeOrganization(raw: Record<string, unknown>): AdminOrganizationItem {
  return {
    id: String(raw.id ?? ''),
    slug: String(raw.slug ?? ''),
    name: String(raw.name ?? ''),
    isActive: Boolean(raw.isActive),
    accountCount: Number(raw.accountCount ?? 0) || 0,
    accountNames: String(raw.accountNames ?? ''),
    createdAt: String(raw.createdAt ?? ''),
    updatedAt: String(raw.updatedAt ?? ''),
  }
}

export function useAdminOrganizationsManager() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const organizations = ref<AdminOrganizationItem[]>([])
  const filters = reactive({
    q: '',
    status: '' as '' | 'active' | 'inactive',
  })
  const loading = ref(false)
  const creating = ref(false)
  const deletingId = ref<string | null>(null)
  const errorMessage = ref('')
  const savingMap = ref<Record<string, boolean>>({})
  const canResetFilters = computed(() => Boolean(filters.q || filters.status))

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
    const idx = organizations.value.findIndex((o) => o.id === id)
    if (idx >= 0) organizations.value[idx] = normalizeOrganization(raw)
  }

  function patchLocal(id: string, field: AdminOrganizationFieldKey, value: unknown) {
    const idx = organizations.value.findIndex((o) => o.id === id)
    if (idx < 0) return
    ;(organizations.value[idx] as Record<string, unknown>)[field] = value
  }

  async function fetchOrganizations() {
    loading.value = true
    errorMessage.value = ''
    try {
      const resp = await apiRequest('/v1/admin/organizations')
      organizations.value = (resp.organizations as Record<string, unknown>[]).map(
        normalizeOrganization,
      )
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao carregar organizations.')
    } finally {
      loading.value = false
    }
  }

  async function persistPatch(id: string, fieldKey: string, patch: Record<string, unknown>) {
    const key = `${id}:${fieldKey}`
    setSaving(key, true)
    try {
      const resp = await apiRequest(`/v1/admin/organizations/${encodeURIComponent(id)}`, {
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
    field: AdminOrganizationFieldKey,
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

  async function createOrganization(input: AdminOrganizationCreateInput): Promise<string | null> {
    creating.value = true
    errorMessage.value = ''
    try {
      const resp = await apiRequest('/v1/admin/organizations', {
        method: 'POST',
        body: input,
      })
      const created = normalizeOrganization(resp as Record<string, unknown>)
      organizations.value.unshift(created)
      return created.id
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao criar organization.')
      return null
    } finally {
      creating.value = false
    }
  }

  async function deleteOrganization(id: string) {
    deletingId.value = id
    setSaving(`${id}:delete`, true)
    errorMessage.value = ''
    try {
      await apiRequest(`/v1/admin/organizations/${encodeURIComponent(id)}`, { method: 'DELETE' })
      organizations.value = organizations.value.filter((o) => o.id !== id)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao excluir organization.')
    } finally {
      deletingId.value = null
      setSaving(`${id}:delete`, false)
    }
  }

  function resetFilters() {
    filters.q = ''
    filters.status = ''
  }

  onBeforeUnmount(() => {
    for (const timer of pendingTimers.values()) clearTimeout(timer)
    pendingTimers.clear()
  })

  return {
    organizations,
    filters,
    loading,
    creating,
    deletingId,
    errorMessage,
    savingMap,
    canResetFilters,
    rowIsSaving,
    fetchOrganizations,
    updateField,
    createOrganization,
    deleteOrganization,
    resetFilters,
  }
}
