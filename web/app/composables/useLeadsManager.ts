import type { LeadClientOption, LeadItem, LeadsListResponse } from '~/types/leads'

const DEFAULT_FETCH_LIMIT = 120

export function useLeadsManager() {
  const sessionSimulation = useSessionSimulationStore()
  const { bffFetch } = useBffFetch()
  const realtime = useTenantRealtime()
  realtime.start()

  const leads = ref<LeadItem[]>([])
  const clientOptions = ref<LeadClientOption[]>([])

  const loading = ref(false)
  const deletingId = ref<number | null>(null)
  const errorMessage = ref('')
  const savingMap = ref<Record<string, boolean>>({})

  function keyFor(id: number, field: 'delete') {
    return `${id}:${field}`
  }

  function setSaving(key: string, value: boolean) {
    const next = { ...savingMap.value }
    if (value) {
      next[key] = true
    } else {
      delete next[key]
    }

    savingMap.value = next
  }

  async function fetchLeads() {
    loading.value = true
    errorMessage.value = ''

    try {
      const response = await bffFetch<LeadsListResponse>('/api/admin/leads', {
        query: {
          page: 1,
          limit: DEFAULT_FETCH_LIMIT,
        },
      })

      leads.value = Array.isArray(response.data) ? response.data : []
      clientOptions.value = Array.isArray(response.filters?.clients) ? response.filters.clients : []
    } catch {
      errorMessage.value = 'Falha ao carregar leads.'
    } finally {
      loading.value = false
    }
  }

  async function deleteLead(id: number) {
    deletingId.value = id
    setSaving(keyFor(id, 'delete'), true)
    errorMessage.value = ''

    try {
      await bffFetch<{ status: 'success' }>(`/api/admin/leads/${id}`, {
        method: 'DELETE',
      })

      leads.value = leads.value.filter((item) => item.id !== id)
    } catch {
      errorMessage.value = 'Falha ao excluir lead.'
    } finally {
      deletingId.value = null
      setSaving(keyFor(id, 'delete'), false)
    }
  }

  watch(
    () => sessionSimulation.requestContextHash,
    () => {
      void fetchLeads()
    },
  )

  const stopRealtimeSubscription = realtime.subscribeEntity('leads', () => {
    void fetchLeads()
  })

  onScopeDispose(() => {
    stopRealtimeSubscription()
  })

  return {
    leads,
    clientOptions,
    loading,
    deletingId,
    errorMessage,
    savingMap,
    fetchLeads,
    deleteLead,
  }
}
