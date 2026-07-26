import { onBeforeUnmount, ref, toValue, watch, type MaybeRefOrGetter } from 'vue'
import {
  decideRecommendation,
  fetchRecommendations,
} from '~/domain/customer-intelligence/recommendation-api'
import type {
  CustomerRecommendationView,
  RecommendationDecisionOptions,
  RecommendationDecisionInput,
} from '~/domain/customer-intelligence/recommendation-types'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import { useAuthStore } from '~/stores/auth'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { createApiRequest } from '~/utils/api-client'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'

export function useRecommendations(relationshipId: MaybeRefOrGetter<string>) {
  const auth = useAuthStore()
  const scope = useCustomerIntelligenceStore()
  const access = useCustomerIntelligenceAccess()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
  const items = ref<CustomerRecommendationView[]>([])
  const nextCursor = ref('')
  const decisionOptions = ref<RecommendationDecisionOptions>({
    approveReasons: [],
    rejectReasons: [],
    invalidateReasons: [],
    executeReasons: [],
  })
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
    decisionOptions.value = {
      approveReasons: [],
      rejectReasons: [],
      invalidateReasons: [],
      executeReasons: [],
    }
    loading.value = false
    mutatingId.value = ''
    error.value = null
  }

  async function load(append = false): Promise<void> {
    const id = String(toValue(relationshipId) || '').trim()
    if (!id || !access.canViewIntelligenceProfile.value || !access.clientScopeReady.value) {
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
      const page = await fetchRecommendations(
        api,
        id,
        scope.clientAccountId,
        append ? nextCursor.value : '',
        request.signal,
      )
      if (request.signal.aborted || current !== generation) return
      items.value = append ? [...items.value, ...page.items] : page.items
      nextCursor.value = page.nextCursor
      decisionOptions.value = page.decisionOptions
    } catch (cause) {
      if (request.signal.aborted || current !== generation) return
      error.value = classifyCustomerApiError(cause, 'Recomendacoes indisponiveis.')
    } finally {
      if (current === generation) loading.value = false
    }
  }

  async function act(
    recommendation: CustomerRecommendationView,
    action: 'approve' | 'reject',
    reason: string,
  ): Promise<boolean> {
    if (
      !access.canManageIntelligenceProfile.value ||
      !recommendation.allowedActions.includes(action)
    ) {
      return false
    }
    const input: RecommendationDecisionInput = { reason: reason.trim() }
    if (!input.reason || input.reason.length > 160) return false
    mutatingId.value = recommendation.id
    error.value = null
    try {
      await decideRecommendation(api, recommendation.id, scope.clientAccountId, action, input)
      await load()
      return true
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Acao de recomendacao falhou.')
      return false
    } finally {
      mutatingId.value = ''
    }
  }

  watch(
    [
      () => String(toValue(relationshipId) || ''),
      () => scope.scopeKey,
      () => access.canViewIntelligenceProfile.value,
    ],
    () => void load(),
    { immediate: true },
  )
  onBeforeUnmount(clear)

  return {
    items,
    nextCursor,
    decisionOptions,
    loading,
    mutatingId,
    error,
    load,
    act,
    clear,
  }
}
