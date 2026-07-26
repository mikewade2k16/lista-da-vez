import { onBeforeUnmount, ref, watch } from 'vue'
import {
  fetchIntelligenceAuditEvents,
  fetchIntelligenceObservation,
  revealIntelligenceObservation,
} from '~/domain/customer-intelligence/audit-api'
import type {
  IntelligenceAuditEventView,
  IntelligenceAuditFilters,
  IntelligenceAuditPage,
  IntelligenceObservationView,
} from '~/domain/customer-intelligence/audit-types'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import { useAuthStore } from '~/stores/auth'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { createApiRequest } from '~/utils/api-client'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'

const EMPTY_OPTIONS: IntelligenceAuditPage['filterOptions'] = {
  actions: [],
  entityTypes: [],
  statuses: [],
  provenances: [],
}

export function useIntelligenceAudit() {
  const auth = useAuthStore()
  const scope = useCustomerIntelligenceStore()
  const access = useCustomerIntelligenceAccess()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
  const events = ref<IntelligenceAuditEventView[]>([])
  const nextCursor = ref('')
  const options = ref({ ...EMPTY_OPTIONS })
  const observation = ref<IntelligenceObservationView | null>(null)
  const observationOpen = ref(false)
  const loading = ref(false)
  const loadingObservation = ref(false)
  const revealingObservation = ref(false)
  const error = ref<CustomerApiErrorState | null>(null)
  const observationError = ref<CustomerApiErrorState | null>(null)
  let listController: AbortController | null = null
  let observationController: AbortController | null = null
  let listGeneration = 0
  let observationGeneration = 0

  function currentScopeKey(): string {
    return String(scope.scopeKey || '').trim()
  }

  function clearObservation(): void {
    observationController?.abort()
    observationController = null
    observationGeneration += 1
    observation.value = null
    observationOpen.value = false
    loadingObservation.value = false
    revealingObservation.value = false
    observationError.value = null
  }

  function clear(): void {
    listController?.abort()
    listController = null
    listGeneration += 1
    events.value = []
    nextCursor.value = ''
    options.value = { ...EMPTY_OPTIONS }
    loading.value = false
    error.value = null
    clearObservation()
  }

  async function load(
    filters: Omit<IntelligenceAuditFilters, 'clientAccountId' | 'cursor'> = {},
    append = false,
  ): Promise<void> {
    if (!access.canViewAudit.value || !access.clientScopeReady.value) {
      clear()
      return
    }
    listController?.abort()
    const request = new AbortController()
    listController = request
    const current = ++listGeneration
    const requestScopeKey = currentScopeKey()
    const requestClientAccountId = String(scope.clientAccountId || '').trim()
    loading.value = true
    error.value = null
    try {
      const page = await fetchIntelligenceAuditEvents(
        api,
        {
          ...filters,
          clientAccountId: requestClientAccountId,
          cursor: append ? nextCursor.value : '',
        },
        request.signal,
      )
      if (
        request.signal.aborted ||
        current !== listGeneration ||
        requestScopeKey !== currentScopeKey()
      ) {
        return
      }
      events.value = append ? [...events.value, ...page.items] : page.items
      nextCursor.value = page.nextCursor
      options.value = {
        actions: [
          ...new Map(
            [...options.value.actions, ...page.filterOptions.actions].map((option) => [
              option.value,
              option,
            ]),
          ).values(),
        ],
        entityTypes: [
          ...new Map(
            [...options.value.entityTypes, ...page.filterOptions.entityTypes].map((option) => [
              option.value,
              option,
            ]),
          ).values(),
        ],
        statuses: [],
        provenances: [],
      }
    } catch (cause) {
      if (
        request.signal.aborted ||
        current !== listGeneration ||
        requestScopeKey !== currentScopeKey()
      ) {
        return
      }
      error.value = classifyCustomerApiError(cause, 'Auditoria indisponivel.')
    } finally {
      if (current === listGeneration && requestScopeKey === currentScopeKey()) {
        loading.value = false
      }
    }
  }

  async function openObservation(event: IntelligenceAuditEventView): Promise<boolean> {
    if (!event.canOpenObservation || !event.observationRef || !access.canViewAudit.value) {
      return false
    }
    observationController?.abort()
    const request = new AbortController()
    observationController = request
    const current = ++observationGeneration
    const requestScopeKey = currentScopeKey()
    const requestClientAccountId = String(scope.clientAccountId || '').trim()
    observationOpen.value = true
    loadingObservation.value = true
    observation.value = null
    observationError.value = null
    try {
      const response = await fetchIntelligenceObservation(
        api,
        event.observationRef,
        requestClientAccountId,
        request.signal,
      )
      if (
        request.signal.aborted ||
        current !== observationGeneration ||
        requestScopeKey !== currentScopeKey()
      ) {
        return false
      }
      observation.value = response
      return true
    } catch (cause) {
      if (
        request.signal.aborted ||
        current !== observationGeneration ||
        requestScopeKey !== currentScopeKey()
      ) {
        return false
      }
      observation.value = null
      observationError.value = classifyCustomerApiError(cause, 'Observacao indisponivel.')
      return false
    } finally {
      if (current === observationGeneration && requestScopeKey === currentScopeKey()) {
        loadingObservation.value = false
      }
    }
  }

  async function revealObservation(reasonCode: string): Promise<boolean> {
    const currentObservation = observation.value
    const normalizedReason = String(reasonCode || '').trim()
    if (
      !currentObservation ||
      currentObservation.revealed ||
      revealingObservation.value ||
      !access.canViewAudit.value ||
      !normalizedReason
    ) {
      return false
    }
    observationController?.abort()
    const request = new AbortController()
    observationController = request
    const current = ++observationGeneration
    const requestScopeKey = currentScopeKey()
    const requestClientAccountId = String(scope.clientAccountId || '').trim()
    const observationId = currentObservation.id
    revealingObservation.value = true
    observationError.value = null
    try {
      const response = await revealIntelligenceObservation(
        api,
        observationId,
        requestClientAccountId,
        normalizedReason,
        request.signal,
      )
      if (
        request.signal.aborted ||
        current !== observationGeneration ||
        requestScopeKey !== currentScopeKey() ||
        observation.value?.id !== observationId
      ) {
        return false
      }
      observation.value = response
      return true
    } catch (cause) {
      if (
        request.signal.aborted ||
        current !== observationGeneration ||
        requestScopeKey !== currentScopeKey()
      ) {
        return false
      }
      observationError.value = classifyCustomerApiError(
        cause,
        'Nao foi possivel revelar a observacao.',
      )
      return false
    } finally {
      if (current === observationGeneration && requestScopeKey === currentScopeKey()) {
        revealingObservation.value = false
      }
    }
  }

  function setObservationOpen(value: boolean): void {
    if (!value) clearObservation()
  }

  watch(
    [() => scope.scopeKey, () => access.clientScopeReady.value, () => access.canViewAudit.value],
    () => {
      // Limpa lista, detalhe e drawer no mesmo tick da troca de escopo.
      clear()
      void load()
    },
    { immediate: true, flush: 'sync' },
  )
  onBeforeUnmount(clear)

  return {
    events,
    nextCursor,
    options,
    observation,
    observationOpen,
    loading,
    loadingObservation,
    revealingObservation,
    error,
    observationError,
    load,
    openObservation,
    revealObservation,
    setObservationOpen,
    clear,
  }
}
