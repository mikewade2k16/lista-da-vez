import { computed, onBeforeUnmount, ref, toValue, watch, type MaybeRefOrGetter } from 'vue'
import { fetchCustomerClaims, reviewCustomerClaim } from '~/domain/customer-intelligence/claim-api'
import {
  type CustomerClaimReviewStatus,
  type CustomerClaimStatus,
  type CustomerClaimView,
  validCustomerClaimReasonCode,
} from '~/domain/customer-intelligence/claim-types'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import { useAuthStore } from '~/stores/auth'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { createApiRequest } from '~/utils/api-client'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'

function validationError(message: string): CustomerApiErrorState {
  return {
    kind: 'error',
    message,
    reasonCode: 'claim_review_reason_invalid',
    statusCode: 422,
  }
}

export function useCandidateClaims(relationshipId: MaybeRefOrGetter<string>) {
  const auth = useAuthStore()
  const scope = useCustomerIntelligenceStore()
  const access = useCustomerIntelligenceAccess()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)

  const activeStatus = ref<CustomerClaimStatus>('candidate')
  const items = ref<CustomerClaimView[]>([])
  const loading = ref(false)
  const reviewingId = ref('')
  const error = ref<CustomerApiErrorState | null>(null)
  const normalizedRelationshipId = computed(() => String(toValue(relationshipId) ?? '').trim())
  const canLoad = computed(
    () => access.canViewIntelligenceProfile.value && access.clientScopeReady.value,
  )

  let controller: AbortController | null = null
  let generation = 0
  let contentScopeKey = ''

  function clear(): void {
    controller?.abort()
    controller = null
    generation += 1
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
    const clientAccountId = scope.clientAccountId
    const requestedStatus = activeStatus.value
    const requestedScopeKey = `${scope.scopeKey}:${id}:${requestedStatus}`
    if (contentScopeKey !== requestedScopeKey) items.value = []
    loading.value = true
    error.value = null
    try {
      const response = await fetchCustomerClaims(
        api,
        id,
        clientAccountId,
        requestedStatus,
        request.signal,
      )
      if (
        request.signal.aborted ||
        current !== generation ||
        requestedStatus !== activeStatus.value
      ) {
        return
      }
      items.value = response
      contentScopeKey = requestedScopeKey
    } catch (cause) {
      if (request.signal.aborted || current !== generation) return
      const classified = classifyCustomerApiError(cause, 'Claims indisponiveis.')
      if (classified.kind !== 'aborted') error.value = classified
    } finally {
      if (current === generation) loading.value = false
    }
  }

  function selectStatus(value: string): void {
    if (value !== 'candidate' && value !== 'accepted' && value !== 'rejected') return
    if (activeStatus.value === value) return
    items.value = []
    error.value = null
    activeStatus.value = value
  }

  async function review(
    claim: CustomerClaimView,
    status: CustomerClaimReviewStatus,
    reasonCode: string,
  ): Promise<boolean> {
    const normalizedReason = reasonCode.trim()
    if (
      !access.canManageIntelligenceProfile.value ||
      claim.status !== 'candidate' ||
      reviewingId.value
    ) {
      return false
    }
    if (!validCustomerClaimReasonCode(normalizedReason)) {
      error.value = validationError(
        'Use um reason code de ate 160 caracteres, iniciado por letra minuscula.',
      )
      return false
    }
    const relationship = normalizedRelationshipId.value
    const clientAccountId = scope.clientAccountId
    const scopeKey = scope.scopeKey
    reviewingId.value = claim.id
    error.value = null
    try {
      await reviewCustomerClaim(api, claim.id, clientAccountId, {
        status,
        reasonCode: normalizedReason,
        expectedRevision: claim.revision,
      })
      if (relationship === normalizedRelationshipId.value && scopeKey === scope.scopeKey) {
        await load()
      }
      return true
    } catch (cause) {
      if (relationship !== normalizedRelationshipId.value || scopeKey !== scope.scopeKey) {
        return false
      }
      const classified = classifyCustomerApiError(cause, 'Nao foi possivel revisar a claim.')
      error.value =
        classified.statusCode === 409
          ? {
              ...classified,
              message:
                'A claim foi alterada por outra revisao. Recarregue antes de tentar novamente.',
              reasonCode: classified.reasonCode || 'claim_revision_conflict',
            }
          : classified
      return false
    } finally {
      reviewingId.value = ''
    }
  }

  watch(
    [normalizedRelationshipId, () => scope.scopeKey, canLoad, activeStatus],
    () => void load(),
    { immediate: true },
  )
  onBeforeUnmount(clear)

  return {
    access,
    activeStatus,
    items,
    loading,
    reviewingId,
    error,
    selectStatus,
    load,
    review,
    clear,
  }
}
