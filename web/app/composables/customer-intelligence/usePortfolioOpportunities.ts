import { onBeforeUnmount, ref, watch } from 'vue'
import { fetchPortfolioOpportunities } from '~/domain/customer-intelligence/portfolio-api'
import type {
  PortfolioFilters,
  PortfolioOpportunitiesPage,
  PortfolioOpportunityView,
} from '~/domain/customer-intelligence/portfolio-types'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import { useAuthStore } from '~/stores/auth'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { createApiRequest } from '~/utils/api-client'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'

const EMPTY_POLICY: PortfolioOpportunitiesPage['policySummary'] = {
  purposeKey: '',
  policyVersionRef: '',
  minimumCohortLabel: '',
  suppressionMode: '',
  freshnessPolicy: '',
}

export function usePortfolioOpportunities() {
  const auth = useAuthStore()
  const scope = useCustomerIntelligenceStore()
  const access = useCustomerIntelligenceAccess()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
  const items = ref<PortfolioOpportunityView[]>([])
  const nextCursor = ref('')
  const descriptors = ref<PortfolioOpportunitiesPage['filters']>([])
  const decisionReasons = ref<PortfolioOpportunitiesPage['decisionReasons']>({
    approve: [],
    reject: [],
  })
  const policySummary = ref({ ...EMPTY_POLICY })
  const loading = ref(false)
  const mutatingId = ref('')
  const error = ref<CustomerApiErrorState | null>(null)
  let controller: AbortController | null = null
  let generation = 0

  function clear(): void {
    controller?.abort()
    controller = null
    generation += 1
    items.value = []
    nextCursor.value = ''
    descriptors.value = []
    decisionReasons.value = { approve: [], reject: [] }
    policySummary.value = { ...EMPTY_POLICY }
    loading.value = false
    mutatingId.value = ''
    error.value = null
  }

  async function load(filters: PortfolioFilters = {}, append = false): Promise<void> {
    if (!access.canViewPortfolio.value || !access.clientScopeReady.value) {
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
      const page = await fetchPortfolioOpportunities(
        api,
        {
          ...filters,
          targetClientAccountId: scope.clientAccountId,
          cursor: append ? nextCursor.value : '',
        },
        request.signal,
      )
      if (request.signal.aborted || current !== generation) return
      items.value = append ? [...items.value, ...page.items] : page.items
      nextCursor.value = page.nextCursor
      descriptors.value = page.filters
      decisionReasons.value = page.decisionReasons
      policySummary.value = page.policySummary
    } catch (cause) {
      if (request.signal.aborted || current !== generation) return
      error.value = classifyCustomerApiError(cause, 'Portfolio indisponivel.')
    } finally {
      if (current === generation) loading.value = false
    }
  }

  function decide(
    _opportunity: PortfolioOpportunityView,
    _decision: 'approve' | 'reject',
    _reasonCode: string,
    _reason = '',
  ): Promise<boolean> {
    return Promise.resolve(false)
  }

  watch([() => scope.scopeKey, () => access.canViewPortfolio.value], () => void load(), {
    immediate: true,
  })
  onBeforeUnmount(clear)

  return {
    access,
    items,
    nextCursor,
    descriptors,
    decisionReasons,
    policySummary,
    loading,
    mutatingId,
    error,
    load,
    decide,
    clear,
  }
}
