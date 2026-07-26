import { onBeforeUnmount, ref, toValue, watch, type MaybeRefOrGetter } from 'vue'
import {
  createOfflineInteraction,
  fetchOfflineInteractions,
} from '~/domain/customer-data/offline-interaction-api'
import type {
  CreateOfflineInteractionInput,
  OfflineInteractionCreateDescriptor,
  OfflineInteractionView,
} from '~/domain/customer-data/offline-interaction-types'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import { useAuthStore } from '~/stores/auth'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { createApiRequest } from '~/utils/api-client'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'

export function useOfflineInteractions(relationshipId: MaybeRefOrGetter<string>) {
  const auth = useAuthStore()
  const scope = useCustomerIntelligenceStore()
  const access = useCustomerIntelligenceAccess()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
  const items = ref<OfflineInteractionView[]>([])
  const nextCursor = ref('')
  const descriptor = ref<OfflineInteractionCreateDescriptor | null>(null)
  const loading = ref(false)
  const saving = ref(false)
  const error = ref<CustomerApiErrorState | null>(null)
  let controller: AbortController | null = null
  let generation = 0

  function clear(): void {
    controller?.abort()
    controller = null
    generation += 1
    items.value = []
    nextCursor.value = ''
    descriptor.value = null
    loading.value = false
    saving.value = false
    error.value = null
  }

  async function load(append = false): Promise<void> {
    const id = String(toValue(relationshipId) || '').trim()
    if (!id || !access.canViewOffline.value || !access.clientScopeReady.value) {
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
      const page = await fetchOfflineInteractions(
        api,
        id,
        scope.clientAccountId,
        append ? nextCursor.value : '',
        request.signal,
      )
      if (request.signal.aborted || current !== generation) return
      items.value = append ? [...items.value, ...page.items] : page.items
      nextCursor.value = page.nextCursor
      descriptor.value = page.createDescriptor ?? null
    } catch (cause) {
      if (request.signal.aborted || current !== generation) return
      error.value = classifyCustomerApiError(cause, 'Interacoes offline indisponiveis.')
    } finally {
      if (current === generation) loading.value = false
    }
  }

  async function create(
    input: Omit<CreateOfflineInteractionInput, 'clientAccountId' | 'idempotencyKey'>,
  ): Promise<boolean> {
    const id = String(toValue(relationshipId) || '').trim()
    if (!id || !access.canManageOffline.value || saving.value) return false
    saving.value = true
    error.value = null
    try {
      await createOfflineInteraction(api, id, {
        ...input,
        clientAccountId: scope.clientAccountId,
        idempotencyKey: globalThis.crypto?.randomUUID?.() ?? `panel-${Date.now()}`,
      })
      await load()
      return true
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel registrar a interacao.')
      return false
    } finally {
      saving.value = false
    }
  }

  watch(
    [
      () => String(toValue(relationshipId) || ''),
      () => scope.scopeKey,
      () => access.canViewOffline.value,
    ],
    () => void load(),
    { immediate: true },
  )
  onBeforeUnmount(clear)

  return {
    access,
    items,
    nextCursor,
    descriptor,
    loading,
    saving,
    error,
    load,
    create,
    clear,
  }
}
