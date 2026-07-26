import { ref } from 'vue'
import { defineStore } from 'pinia'

import type { QueueCommunication, QueueCommunicationInput } from '~/domain/operation/communications'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

interface CommunicationsResponse {
  items?: Omit<QueueCommunication, 'accountId'>[]
}

interface CommunicationResponse {
  communication?: Omit<QueueCommunication, 'accountId'>
}

export const useCommunicationsStore = defineStore('communications', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const items = ref<QueueCommunication[]>([])
  const publishedItems = ref<QueueCommunication[]>([])
  const loading = ref(false)
  const publishedLoading = ref(false)
  const saving = ref(false)
  const deletingId = ref('')
  const errorMessage = ref('')
  const editorErrorMessage = ref('')
  let publishedRequestVersion = 0

  function accountHeaders(accountId: string) {
    const normalized = String(accountId || '').trim()
    return normalized ? { 'X-Account-Id': normalized } : {}
  }

  function accountIds(): string[] {
    const ids = (auth.storeContext || [])
      .map((store) => String(store?.tenantId || '').trim())
      .filter(Boolean)
    const activeAccountId = String(auth.activeTenantId || '').trim()
    if (activeAccountId) ids.push(activeAccountId)
    return Array.from(new Set(ids))
  }

  function withAccount(
    item: Omit<QueueCommunication, 'accountId'>,
    accountId: string,
  ): QueueCommunication {
    return {
      ...item,
      accountId,
      storeIds: Array.isArray(item.storeIds) ? item.storeIds : [],
    }
  }

  function sortItems(values: QueueCommunication[]): QueueCommunication[] {
    return [...values].sort(
      (left, right) =>
        Number(left.displayOrder || 0) - Number(right.displayOrder || 0) ||
        new Date(right.updatedAt).getTime() - new Date(left.updatedAt).getTime(),
    )
  }

  async function load(): Promise<boolean> {
    if (loading.value) return false
    loading.value = true
    errorMessage.value = ''
    try {
      const scopes = accountIds()
      const responses = await Promise.all(
        (scopes.length ? scopes : ['']).map(async (accountId) => {
          const response = (await apiRequest('/v1/operations/communications', {
            headers: accountHeaders(accountId),
            dedupe: false,
          })) as CommunicationsResponse
          return (response.items || []).map((item) => withAccount(item, accountId))
        }),
      )
      items.value = sortItems(responses.flat())
      return true
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Não foi possível carregar os comunicados.')
      return false
    } finally {
      loading.value = false
    }
  }

  async function loadPublishedForStores(storeIds: string[]): Promise<boolean> {
    const normalizedStoreIds = Array.from(
      new Set((storeIds || []).map((storeId) => String(storeId || '').trim()).filter(Boolean)),
    )
    if (normalizedStoreIds.length === 0) {
      publishedItems.value = []
      return true
    }

    const requestVersion = ++publishedRequestVersion
    publishedLoading.value = true
    try {
      const responses = await Promise.all(
        normalizedStoreIds.map(async (storeId) => {
          const store = (auth.storeContext || []).find(
            (candidate) => String(candidate?.id || '').trim() === storeId,
          )
          const accountId = String(store?.tenantId || auth.activeTenantId || '').trim()
          const query = new URLSearchParams({ storeId, publishedOnly: 'true' })
          const response = (await apiRequest(`/v1/operations/communications?${query.toString()}`, {
            headers: accountHeaders(accountId),
            dedupe: false,
            skipLoadingIndicator: true,
          })) as CommunicationsResponse
          return (response.items || []).map((item) => withAccount(item, accountId))
        }),
      )
      if (requestVersion !== publishedRequestVersion) return false
      const unique = new Map<string, QueueCommunication>()
      responses.flat().forEach((item) => unique.set(`${item.accountId}:${item.id}`, item))
      publishedItems.value = sortItems(Array.from(unique.values()))
      return true
    } catch {
      if (requestVersion === publishedRequestVersion) publishedItems.value = []
      return false
    } finally {
      if (requestVersion === publishedRequestVersion) publishedLoading.value = false
    }
  }

  async function create(
    accountId: string,
    input: QueueCommunicationInput,
  ): Promise<QueueCommunication | null> {
    if (saving.value) return null
    saving.value = true
    editorErrorMessage.value = ''
    try {
      const response = (await apiRequest('/v1/operations/communications', {
        method: 'POST',
        headers: accountHeaders(accountId),
        body: input,
      })) as CommunicationResponse
      if (!response.communication) return null
      const created = withAccount(response.communication, accountId)
      items.value = sortItems([...items.value, created])
      return created
    } catch (error) {
      editorErrorMessage.value = getApiErrorMessage(error, 'Não foi possível criar o comunicado.')
      return null
    } finally {
      saving.value = false
    }
  }

  async function update(
    item: QueueCommunication,
    input: QueueCommunicationInput,
  ): Promise<QueueCommunication | null> {
    if (saving.value) return null
    saving.value = true
    editorErrorMessage.value = ''
    try {
      const response = (await apiRequest(
        `/v1/operations/communications/${encodeURIComponent(item.id)}`,
        {
          method: 'PUT',
          headers: accountHeaders(item.accountId),
          body: input,
        },
      )) as CommunicationResponse
      if (!response.communication) return null
      const updated = withAccount(response.communication, item.accountId)
      items.value = sortItems(
        items.value.map((candidate) =>
          candidate.id === item.id && candidate.accountId === item.accountId ? updated : candidate,
        ),
      )
      return updated
    } catch (error) {
      editorErrorMessage.value = getApiErrorMessage(
        error,
        'Não foi possível atualizar o comunicado.',
      )
      return null
    } finally {
      saving.value = false
    }
  }

  async function remove(item: QueueCommunication): Promise<boolean> {
    if (deletingId.value) return false
    deletingId.value = item.id
    editorErrorMessage.value = ''
    try {
      await apiRequest(`/v1/operations/communications/${encodeURIComponent(item.id)}`, {
        method: 'DELETE',
        headers: accountHeaders(item.accountId),
      })
      items.value = items.value.filter(
        (candidate) => candidate.id !== item.id || candidate.accountId !== item.accountId,
      )
      publishedItems.value = publishedItems.value.filter(
        (candidate) => candidate.id !== item.id || candidate.accountId !== item.accountId,
      )
      return true
    } catch (error) {
      editorErrorMessage.value = getApiErrorMessage(error, 'Não foi possível excluir o comunicado.')
      return false
    } finally {
      deletingId.value = ''
    }
  }

  function clearEditorError(): void {
    editorErrorMessage.value = ''
  }

  return {
    items,
    publishedItems,
    loading,
    publishedLoading,
    saving,
    deletingId,
    errorMessage,
    editorErrorMessage,
    load,
    loadPublishedForStores,
    create,
    update,
    remove,
    clearEditorError,
  }
})
