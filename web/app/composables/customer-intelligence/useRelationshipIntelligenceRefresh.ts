import { ref, watch } from 'vue'
import {
  enqueueRelationshipIntelligenceRefresh,
  type RelationshipRefreshJob,
} from '~/domain/customer-intelligence/refresh-api'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import { useAuthStore } from '~/stores/auth'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { createApiRequest } from '~/utils/api-client'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'

export function useRelationshipIntelligenceRefresh() {
  const auth = useAuthStore()
  const scope = useCustomerIntelligenceStore()
  const access = useCustomerIntelligenceAccess()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
  const enqueuing = ref(false)
  const lastJob = ref<RelationshipRefreshJob | null>(null)
  const error = ref<CustomerApiErrorState | null>(null)

  watch(
    () => scope.scopeKey,
    () => {
      enqueuing.value = false
      lastJob.value = null
      error.value = null
    },
    { flush: 'sync' },
  )

  async function enqueue(subjectId: string, relationshipId: string): Promise<boolean> {
    const normalizedSubject = String(subjectId || '').trim()
    const normalizedRelationship = String(relationshipId || '').trim()
    if (
      enqueuing.value ||
      !access.canManageIntelligenceProfile.value ||
      !access.clientScopeReady.value ||
      !normalizedSubject ||
      !normalizedRelationship
    ) {
      return false
    }
    const requestScopeKey = String(scope.scopeKey || '')
    enqueuing.value = true
    error.value = null
    try {
      const idempotencyKey =
        globalThis.crypto?.randomUUID?.() ??
        `panel.${Date.now()}.${Math.random().toString(36).slice(2)}`
      const response = await enqueueRelationshipIntelligenceRefresh(
        api,
        normalizedRelationship,
        scope.clientAccountId,
        normalizedSubject,
        `panel.${idempotencyKey}`,
      )
      if (requestScopeKey !== String(scope.scopeKey || '')) return false
      lastJob.value = response
      return true
    } catch (cause) {
      if (requestScopeKey !== String(scope.scopeKey || '')) return false
      error.value = classifyCustomerApiError(
        cause,
        'Nao foi possivel agendar a atualizacao da inteligencia.',
      )
      return false
    } finally {
      if (requestScopeKey === String(scope.scopeKey || '')) {
        enqueuing.value = false
      }
    }
  }

  return { access, enqueuing, lastJob, error, enqueue }
}
