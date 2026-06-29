import type { LeadCreateInput, LeadFieldKey, LeadItem, LeadStatus } from '~/types/leads'
import { useInlineEditManager } from '~/composables/useInlineEditManager'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

const FIELD_TO_PATCH: Record<LeadFieldKey, string> = {
  nome: 'nome',
  email: 'email',
  telefone: 'telefone',
  status: 'status',
  notes: 'notes',
}

function normalizeLead(raw: Record<string, unknown>): LeadItem {
  const status = String(raw.status ?? 'new')
  const valid: LeadStatus = ['new', 'contacted', 'qualified', 'lost'].includes(status)
    ? (status as LeadStatus)
    : 'new'
  return {
    id: String(raw.id ?? ''),
    accountId: String(raw.accountId ?? ''),
    sourceId: String(raw.sourceId ?? ''),
    sourceLabel: String(raw.sourceLabel ?? ''),
    nome: String(raw.nome ?? ''),
    email: String(raw.email ?? ''),
    telefone: String(raw.telefone ?? ''),
    page: String(raw.page ?? ''),
    cupom: String(raw.cupom ?? ''),
    consent: Boolean(raw.consent),
    consentLabel: String(raw.consentLabel ?? ''),
    trackingData: String(raw.trackingData ?? ''),
    payloadRaw: String(raw.payloadRaw ?? ''),
    status: valid,
    notes: String(raw.notes ?? ''),
    createdAt: String(raw.createdAt ?? ''),
    updatedAt: String(raw.updatedAt ?? ''),
  }
}

export function useLeadsManager() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  function accountHeaders() {
    return auth.activeTenantId ? { 'X-Account-Id': auth.activeTenantId } : {}
  }

  const leads = ref<LeadItem[]>([])
  const filters = reactive({
    q: '',
    status: '' as '' | LeadStatus,
    sourceId: '',
  })
  const loading = ref(false)
  const creating = ref(false)
  const deletingId = ref<string | null>(null)
  const errorMessage = ref('')
  const canResetFilters = computed(() => Boolean(filters.q || filters.status || filters.sourceId))

  // Mecanica de edicao inline (savingMap/setSaving/rowIsSaving + debounce + cleanup)
  // compartilhada com os demais managers de grade. Comportamento identico ao anterior.
  const { savingMap, setSaving, rowIsSaving, schedulePatch } = useInlineEditManager()

  function applyPatch(id: string, raw: Record<string, unknown>) {
    const idx = leads.value.findIndex((l) => l.id === id)
    if (idx >= 0) leads.value[idx] = normalizeLead(raw)
  }

  function patchLocal(id: string, field: LeadFieldKey, value: unknown) {
    const idx = leads.value.findIndex((l) => l.id === id)
    if (idx < 0) return
    ;(leads.value[idx] as Record<string, unknown>)[field] = value
  }

  async function fetchLeads() {
    loading.value = true
    errorMessage.value = ''
    try {
      const query = new URLSearchParams()
      if (filters.q) query.set('q', filters.q)
      if (filters.status) query.set('status', filters.status)
      if (filters.sourceId) query.set('sourceId', filters.sourceId)
      const path = `/v1/admin/leads${query.toString() ? `?${query.toString()}` : ''}`
      const resp = await apiRequest(path, { headers: accountHeaders() })
      leads.value = (resp.leads as Record<string, unknown>[]).map(normalizeLead)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao carregar leads.')
    } finally {
      loading.value = false
    }
  }

  async function persistPatch(id: string, fieldKey: string, patch: Record<string, unknown>) {
    const key = `${id}:${fieldKey}`
    setSaving(key, true)
    try {
      const resp = await apiRequest(`/v1/admin/leads/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        body: patch,
        headers: accountHeaders(),
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
    field: LeadFieldKey,
    value: unknown,
    opts?: { immediate?: boolean },
  ) {
    patchLocal(id, field, value)
    const backendField = FIELD_TO_PATCH[field]
    if (!backendField) return

    const patch = { [backendField]: value }
    schedulePatch(`${id}:${field}`, () => void persistPatch(id, field, patch), {
      immediate: opts?.immediate,
    })
  }

  async function createLead(input: LeadCreateInput): Promise<string | null> {
    creating.value = true
    errorMessage.value = ''
    try {
      const resp = await apiRequest('/v1/admin/leads', {
        method: 'POST',
        body: input,
        headers: accountHeaders(),
      })
      const created = normalizeLead(resp as Record<string, unknown>)
      leads.value.unshift(created)
      return created.id
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao criar lead.')
      return null
    } finally {
      creating.value = false
    }
  }

  async function deleteLead(id: string) {
    deletingId.value = id
    setSaving(`${id}:delete`, true)
    errorMessage.value = ''
    try {
      await apiRequest(`/v1/admin/leads/${encodeURIComponent(id)}`, {
        method: 'DELETE',
        headers: accountHeaders(),
      })
      leads.value = leads.value.filter((l) => l.id !== id)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao excluir lead.')
    } finally {
      deletingId.value = null
      setSaving(`${id}:delete`, false)
    }
  }

  function resetFilters() {
    filters.q = ''
    filters.status = ''
    filters.sourceId = ''
  }

  return {
    leads,
    filters,
    loading,
    creating,
    deletingId,
    errorMessage,
    savingMap,
    canResetFilters,
    rowIsSaving,
    fetchLeads,
    updateField,
    createLead,
    deleteLead,
    resetFilters,
  }
}
