import { onBeforeUnmount, ref, watch } from 'vue'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import { fetchIntelligenceRuns } from '~/domain/customer-intelligence/runs-api'
import type {
  IntelligenceRunsFilters,
  IntelligenceRunsFilterOptions,
  RuntimeRunListItem,
} from '~/domain/customer-intelligence/runs-types'
import { useAuthStore } from '~/stores/auth'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { createApiRequest } from '~/utils/api-client'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'

export function useIntelligenceRuns() {
  const auth = useAuthStore()
  const scope = useCustomerIntelligenceStore()
  const access = useCustomerIntelligenceAccess()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
  const items = ref<RuntimeRunListItem[]>([])
  const nextCursor = ref('')
  const options = ref<IntelligenceRunsFilterOptions>({
    statuses: [],
    processes: [],
    pipelines: [],
    executors: [],
  })
  const loading = ref(false)
  const error = ref<CustomerApiErrorState | null>(null)
  let controller: AbortController | null = null
  let generation = 0

  function clear(): void {
    controller?.abort()
    controller = null
    generation += 1
    items.value = []
    nextCursor.value = ''
    options.value = { statuses: [], processes: [], pipelines: [], executors: [] }
    loading.value = false
    error.value = null
  }

  async function load(
    filters: Omit<IntelligenceRunsFilters, 'clientAccountId' | 'cursor'> = {},
    append = false,
  ): Promise<void> {
    if (!access.canViewRuns.value || !access.clientScopeReady.value) {
      clear()
      return
    }
    controller?.abort()
    const request = new AbortController()
    controller = request
    const current = ++generation
    loading.value = true
    error.value = null
    try {
      const page = await fetchIntelligenceRuns(
        api,
        {
          ...filters,
          clientAccountId: scope.clientAccountId,
          cursor: append ? nextCursor.value : '',
        },
        request.signal,
      )
      if (request.signal.aborted || current !== generation) return
      items.value = append ? [...items.value, ...page.items] : page.items
      nextCursor.value = page.nextCursor
      options.value = page.filterOptions
    } catch (cause) {
      if (request.signal.aborted || current !== generation) return
      error.value = classifyCustomerApiError(cause, 'Runs indisponiveis.')
    } finally {
      if (current === generation) loading.value = false
    }
  }

  watch([() => scope.scopeKey, () => access.canViewRuns.value], () => void load(), {
    immediate: true,
  })
  onBeforeUnmount(clear)

  return { items, nextCursor, options, loading, error, load, clear }
}
