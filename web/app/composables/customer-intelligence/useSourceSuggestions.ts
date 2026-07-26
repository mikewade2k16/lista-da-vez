import { computed, onBeforeUnmount, ref, toValue, watch, type MaybeRefOrGetter } from 'vue'
import {
  fetchSourceSuggestions,
  reviewSourceSuggestion,
} from '~/domain/customer-intelligence/source-suggestion-api'
import {
  SOURCE_SUGGESTION_REVIEW_REASONS,
  type SourceSuggestionReviewStatus,
  type SourceSuggestionView,
  validSourceSuggestionReviewReason,
} from '~/domain/customer-intelligence/source-suggestion-types'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import { useAuthStore } from '~/stores/auth'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { createApiRequest } from '~/utils/api-client'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'

function validationError(): CustomerApiErrorState {
  return {
    kind: 'error',
    message: 'Selecione um motivo registrado para esta decisao.',
    reasonCode: 'source_suggestion_review_reason_invalid',
    statusCode: 422,
  }
}

export function useSourceSuggestions(relationshipId: MaybeRefOrGetter<string>) {
  const auth = useAuthStore()
  const scope = useCustomerIntelligenceStore()
  const access = useCustomerIntelligenceAccess()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)

  const items = ref<SourceSuggestionView[]>([])
  const loading = ref(false)
  const reviewingId = ref('')
  const error = ref<CustomerApiErrorState | null>(null)
  const normalizedRelationshipId = computed(() => String(toValue(relationshipId) ?? '').trim())
  const canLoad = computed(
    () => access.canViewIntelligenceProfile.value && access.clientScopeReady.value,
  )

  let controller: AbortController | null = null
  let reviewController: AbortController | null = null
  let generation = 0
  let reviewGeneration = 0
  let contentScopeKey = ''

  function currentContentScopeKey(): string {
    return `${String(scope.scopeKey || '').trim()}:${normalizedRelationshipId.value}`
  }

  function clear(): void {
    controller?.abort()
    reviewController?.abort()
    controller = null
    reviewController = null
    generation += 1
    reviewGeneration += 1
    items.value = []
    loading.value = false
    reviewingId.value = ''
    error.value = null
    contentScopeKey = ''
  }

  async function load(): Promise<void> {
    const id = normalizedRelationshipId.value
    if (!id || !canLoad.value) {
      clear()
      return
    }
    controller?.abort()
    const request = new AbortController()
    controller = request
    const current = ++generation
    const requestedScopeKey = currentContentScopeKey()
    const clientAccountId = String(scope.clientAccountId || '').trim()
    if (contentScopeKey !== requestedScopeKey) items.value = []
    loading.value = true
    error.value = null
    try {
      const response = await fetchSourceSuggestions(api, id, clientAccountId, request.signal)
      if (
        request.signal.aborted ||
        current !== generation ||
        requestedScopeKey !== currentContentScopeKey()
      ) {
        return
      }
      items.value = response
      contentScopeKey = requestedScopeKey
    } catch (cause) {
      if (
        request.signal.aborted ||
        current !== generation ||
        requestedScopeKey !== currentContentScopeKey()
      ) {
        return
      }
      const classified = classifyCustomerApiError(cause, 'Sugestoes de fontes indisponiveis.')
      if (classified.kind !== 'aborted') error.value = classified
    } finally {
      if (current === generation) loading.value = false
    }
  }

  async function review(
    suggestion: SourceSuggestionView,
    status: SourceSuggestionReviewStatus,
    reason: string,
  ): Promise<boolean> {
    const normalizedReason = String(reason || '').trim()
    if (
      !access.canManageSources.value ||
      reviewingId.value ||
      !suggestion.allowedActions.includes(status)
    ) {
      return false
    }
    if (!validSourceSuggestionReviewReason(status, normalizedReason)) {
      error.value = validationError()
      return false
    }

    reviewController?.abort()
    const request = new AbortController()
    reviewController = request
    const current = ++reviewGeneration
    const relationship = normalizedRelationshipId.value
    const clientAccountId = String(scope.clientAccountId || '').trim()
    const requestedScopeKey = currentContentScopeKey()
    reviewingId.value = suggestion.id
    error.value = null
    try {
      await reviewSourceSuggestion(
        api,
        suggestion.id,
        clientAccountId,
        { status, reason: normalizedReason },
        request.signal,
      )
      if (
        request.signal.aborted ||
        current !== reviewGeneration ||
        relationship !== normalizedRelationshipId.value ||
        requestedScopeKey !== currentContentScopeKey()
      ) {
        return false
      }
      await load()
      return (
        current === reviewGeneration &&
        relationship === normalizedRelationshipId.value &&
        requestedScopeKey === currentContentScopeKey()
      )
    } catch (cause) {
      if (
        request.signal.aborted ||
        current !== reviewGeneration ||
        requestedScopeKey !== currentContentScopeKey()
      ) {
        return false
      }
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel revisar a sugestao de fonte.')
      return false
    } finally {
      if (current === reviewGeneration) {
        reviewingId.value = ''
        reviewController = null
      }
    }
  }

  watch(
    [normalizedRelationshipId, () => scope.scopeKey, canLoad],
    () => {
      // Remove o snapshot anterior no mesmo tick e invalida respostas do client scope antigo.
      clear()
      void load()
    },
    { immediate: true, flush: 'sync' },
  )
  onBeforeUnmount(clear)

  return {
    access,
    reviewReasons: SOURCE_SUGGESTION_REVIEW_REASONS,
    items,
    loading,
    reviewingId,
    error,
    load,
    review,
    clear,
  }
}
