import type {
  ShortLinkItem,
  ShortLinksListResponse,
  ShortLinkMutationResponse,
  CreateShortLinkPayload,
} from '~/types/tools'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { useInlineEditManager } from '~/composables/useInlineEditManager'

const DEFAULT_FETCH_LIMIT = 200

// Campos editaveis inline na tabela (o resto e read-only).
type ShortLinkFieldKey = 'slug' | 'targetUrl'

// useShortLinksManager: estado + CRUD dos links curtos (/v1/tools/short-links).
// X-Account-Id e injetado pelo api-client (conta ativa); platform_admin em
// platformView recebe todas as contas. Edicao inline reusa a mecanica
// compartilhada (debounce + savingMap) do useInlineEditManager.
export function useShortLinksManager() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const shortLinks = ref<ShortLinkItem[]>([])
  const loading = ref(false)
  const creating = ref(false)
  const deletingId = ref<string | null>(null)
  const errorMessage = ref('')

  const { savingMap, setSaving, rowIsSaving, schedulePatch } = useInlineEditManager()

  async function fetchShortLinks() {
    loading.value = true
    errorMessage.value = ''
    try {
      const response = (await apiRequest(
        `/v1/tools/short-links?limit=${DEFAULT_FETCH_LIMIT}`,
      )) as ShortLinksListResponse
      shortLinks.value = Array.isArray(response?.data) ? response.data : []
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Falha ao carregar links curtos.')
    } finally {
      loading.value = false
    }
  }

  async function createShortLink(payload: CreateShortLinkPayload) {
    creating.value = true
    errorMessage.value = ''
    try {
      const body: CreateShortLinkPayload = { targetUrl: payload.targetUrl }
      if (payload.slug) {
        body.slug = payload.slug
      }
      if (payload.accountId) {
        body.accountId = payload.accountId
      }
      const response = (await apiRequest('/v1/tools/short-links', {
        method: 'POST',
        body,
      })) as ShortLinkMutationResponse
      if (response?.data) {
        shortLinks.value = [response.data, ...shortLinks.value]
      }
      return response?.data ?? null
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Falha ao criar link curto.')
      return null
    } finally {
      creating.value = false
    }
  }

  // patchLocal aplica o valor no objeto local (feedback imediato); persistPatch
  // grava e re-hidrata com o objeto autoritativo do banco (o slug pode ganhar
  // sufixo e o shortUrl muda junto).
  function patchLocal(id: string, field: ShortLinkFieldKey, value: unknown) {
    shortLinks.value = shortLinks.value.map((item) =>
      item.id === id ? { ...item, [field]: value } : item,
    )
  }

  async function persistPatch(id: string, field: ShortLinkFieldKey, value: unknown) {
    const key = `${id}:${field}`
    setSaving(key, true)
    errorMessage.value = ''
    try {
      const response = (await apiRequest(`/v1/tools/short-links/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        body: { [field]: value },
      })) as ShortLinkMutationResponse
      const updated = response?.data
      if (updated) {
        // Re-hidrata SO os campos afetados por este patch (o back normaliza o valor;
        // trocar o slug muda o shortUrl junto). Nao substitui a linha inteira para
        // nao pisar em outra edicao inline em voo na mesma linha.
        shortLinks.value = shortLinks.value.map((item) => {
          if (item.id !== id) return item
          const merged = { ...item, [field]: updated[field] }
          if (field === 'slug') merged.shortUrl = updated.shortUrl
          return merged
        })
      }
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Falha ao atualizar link curto.')
      // Reverte o patch otimista relendo o estado real do banco.
      await fetchShortLinks()
    } finally {
      setSaving(key, false)
    }
  }

  function updateShortLink(
    id: string,
    field: ShortLinkFieldKey,
    value: unknown,
    opts?: { immediate?: boolean },
  ) {
    if (!id) return
    patchLocal(id, field, value)
    schedulePatch(`${id}:${field}`, () => void persistPatch(id, field, value), {
      immediate: opts?.immediate,
    })
  }

  async function deleteShortLink(id: string) {
    deletingId.value = id
    setSaving(`${id}:delete`, true)
    errorMessage.value = ''
    try {
      await apiRequest(`/v1/tools/short-links/${encodeURIComponent(id)}`, { method: 'DELETE' })
      shortLinks.value = shortLinks.value.filter((item) => item.id !== id)
      return true
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Falha ao excluir link curto.')
      return false
    } finally {
      deletingId.value = null
      setSaving(`${id}:delete`, false)
    }
  }

  return {
    shortLinks,
    loading,
    creating,
    deletingId,
    errorMessage,
    savingMap,
    rowIsSaving,
    fetchShortLinks,
    createShortLink,
    updateShortLink,
    deleteShortLink,
  }
}
