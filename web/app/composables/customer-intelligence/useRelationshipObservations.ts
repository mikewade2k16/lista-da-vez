import { onBeforeUnmount, ref, toValue, watch, type MaybeRefOrGetter } from 'vue'
import { fetchRelationshipObservations } from '~/domain/customer-intelligence/observation-api'
import type { IntelligenceObservationView } from '~/domain/customer-intelligence/audit-types'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import { useAuthStore } from '~/stores/auth'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { createApiRequest } from '~/utils/api-client'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'

export function useRelationshipObservations(
  relationshipId: MaybeRefOrGetter<string>,
  sourceKeys: MaybeRefOrGetter<string[]> = [],
) {
  const auth = useAuthStore()
  const scope = useCustomerIntelligenceStore()
  const access = useCustomerIntelligenceAccess()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
  const items = ref<IntelligenceObservationView[]>([])
  const loading = ref(false)
  const error = ref<CustomerApiErrorState | null>(null)
  let controller: AbortController | null = null
  let generation = 0

  function currentScopeKey(): string {
    return String(scope.scopeKey || '').trim()
  }

  function clear(): void {
    controller?.abort()
    controller = null
    generation += 1
    items.value = []
    loading.value = false
    error.value = null
  }

  async function load(): Promise<void> {
    const id = String(toValue(relationshipId) || '').trim()
    if (!id || !access.canViewIntelligenceProfile.value || !access.clientScopeReady.value) {
      clear()
      return
    }
    controller?.abort()
    const request = new AbortController()
    controller = request
    const current = ++generation
    const requestScopeKey = currentScopeKey()
    const requestClientAccountId = String(scope.clientAccountId || '').trim()
    loading.value = true
    error.value = null
    try {
      const response = await fetchRelationshipObservations(
        api,
        id,
        requestClientAccountId,
        toValue(sourceKeys),
        request.signal,
      )
      if (
        request.signal.aborted ||
        current !== generation ||
        requestScopeKey !== currentScopeKey()
      ) {
        return
      }
      items.value = response
    } catch (cause) {
      if (
        request.signal.aborted ||
        current !== generation ||
        requestScopeKey !== currentScopeKey()
      ) {
        return
      }
      error.value = classifyCustomerApiError(cause, 'As evidencias de origem estao indisponiveis.')
    } finally {
      if (current === generation) loading.value = false
    }
  }

  watch(
    [
      () => String(toValue(relationshipId) || ''),
      () => JSON.stringify(toValue(sourceKeys)),
      () => scope.scopeKey,
      () => access.clientScopeReady.value,
      () => access.canViewIntelligenceProfile.value,
    ],
    () => {
      // A troca de conta/cliente retira o snapshot anterior no mesmo tick.
      // Abort + generation + scopeKey impedem que uma resposta tardia o recoloque.
      clear()
      void load()
    },
    { immediate: true, flush: 'sync' },
  )
  onBeforeUnmount(clear)

  return { items, loading, error, load, clear }
}
