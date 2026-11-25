import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { useAuthStore } from '~/stores/auth'

// Ultimos atendimentos encerrados (auditoria de platform_admin). Fonte unica:
// GET /v1/operations/recent-services (queue.operation_service_history). storeId
// vazio = todas as lojas da conta ativa; o backend valida o escopo.

export interface RecentServiceItem {
  serviceId: string
  storeId: string
  storeName: string
  personId: string
  personName: string
  startedAt: number
  finishedAt: number
  durationMs: number
  finishOutcome: string
  startMode: string
  queueWaitMs: number
  skippedCount: number
  saleAmount: number
  customerName: string
  isExistingCustomer: boolean
  closeReason: string
  validationStatus: string
}

function normalizeRecentService(raw: Record<string, unknown>): RecentServiceItem {
  return {
    serviceId: String(raw.serviceId ?? ''),
    storeId: String(raw.storeId ?? ''),
    storeName: String(raw.storeName ?? ''),
    personId: String(raw.personId ?? ''),
    personName: String(raw.personName ?? ''),
    startedAt: Number(raw.startedAt ?? 0) || 0,
    finishedAt: Number(raw.finishedAt ?? 0) || 0,
    durationMs: Number(raw.durationMs ?? 0) || 0,
    finishOutcome: String(raw.finishOutcome ?? ''),
    startMode: String(raw.startMode ?? ''),
    queueWaitMs: Number(raw.queueWaitMs ?? 0) || 0,
    skippedCount: Number(raw.skippedCount ?? 0) || 0,
    saleAmount: Number(raw.saleAmount ?? 0) || 0,
    customerName: String(raw.customerName ?? ''),
    isExistingCustomer: Boolean(raw.isExistingCustomer),
    closeReason: String(raw.closeReason ?? ''),
    validationStatus: String(raw.validationStatus ?? ''),
  }
}

export function useRecentServices(perPageSize = 10) {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const items = ref<RecentServiceItem[]>([])
  const total = ref(0)
  const page = ref(1)
  const perPage = ref(perPageSize)
  const loading = ref(false)
  const errorMessage = ref('')
  let requestSeq = 0

  async function fetchPage(storeId: string, nextPage = 1) {
    const seq = ++requestSeq
    loading.value = true
    errorMessage.value = ''
    const params = new URLSearchParams({ page: String(nextPage), perPage: String(perPage.value) })
    if (storeId) params.set('storeId', storeId)
    try {
      const response = (await apiRequest(`/v1/operations/recent-services?${params}`)) as Record<
        string,
        unknown
      >
      if (seq !== requestSeq) return
      const rows = Array.isArray(response?.items) ? response.items : []
      items.value = rows.map((row) => normalizeRecentService(row as Record<string, unknown>))
      total.value = Number(response?.total ?? 0) || 0
      page.value = Number(response?.page ?? nextPage) || nextPage
    } catch (error) {
      if (seq !== requestSeq) return
      errorMessage.value = getApiErrorMessage(error, 'Nao foi possivel carregar os atendimentos.')
    } finally {
      if (seq === requestSeq) loading.value = false
    }
  }

  return { items, total, page, perPage, loading, errorMessage, fetchPage }
}
