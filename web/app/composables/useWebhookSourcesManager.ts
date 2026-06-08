import type {
  WebhookEntityType,
  WebhookSourceCreateInput,
  WebhookSourceCreatedResponse,
  WebhookSourceItem,
  WebhookSourceRotateResponse,
} from '~/types/webhook-sources'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

function normalize(raw: Record<string, unknown>): WebhookSourceItem {
  const entityType = String(raw.entityType ?? 'leads') as WebhookEntityType
  return {
    id: String(raw.id ?? ''),
    accountId: String(raw.accountId ?? ''),
    slug: String(raw.slug ?? ''),
    name: String(raw.name ?? ''),
    entityType,
    isActive: Boolean(raw.isActive),
    createdAt: String(raw.createdAt ?? ''),
    updatedAt: String(raw.updatedAt ?? ''),
  }
}

export function useWebhookSourcesManager() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const sources = ref<WebhookSourceItem[]>([])
  const loading = ref(false)
  const creating = ref(false)
  const errorMessage = ref('')
  // secret revelado apenas no create/rotate; UI usa para mostrar e copiar.
  const lastSecret = ref<string>('')
  const lastSecretFor = ref<string>('')

  async function fetchSources() {
    loading.value = true
    errorMessage.value = ''
    try {
      const resp = await apiRequest('/v1/admin/webhook-sources')
      sources.value = ((resp.sources ?? []) as Record<string, unknown>[]).map(normalize)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao carregar fontes.')
    } finally {
      loading.value = false
    }
  }

  async function createSource(
    input: WebhookSourceCreateInput,
  ): Promise<WebhookSourceCreatedResponse | null> {
    creating.value = true
    errorMessage.value = ''
    try {
      const resp = (await apiRequest('/v1/admin/webhook-sources', {
        method: 'POST',
        body: input,
      })) as { source?: Record<string, unknown>; secret?: string }
      if (!resp.source) throw new Error('Resposta sem source')
      const created = normalize(resp.source)
      sources.value.unshift(created)
      lastSecret.value = String(resp.secret ?? '')
      lastSecretFor.value = created.id
      return { source: created, secret: String(resp.secret ?? '') }
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao criar fonte.')
      return null
    } finally {
      creating.value = false
    }
  }

  async function rotateSecret(id: string): Promise<WebhookSourceRotateResponse | null> {
    errorMessage.value = ''
    try {
      const resp = (await apiRequest(`/v1/admin/webhook-sources/${encodeURIComponent(id)}/rotate`, {
        method: 'POST',
      })) as { secret?: string }
      lastSecret.value = String(resp.secret ?? '')
      lastSecretFor.value = id
      return { secret: String(resp.secret ?? '') }
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao rotacionar secret.')
      return null
    }
  }

  async function deleteSource(id: string) {
    errorMessage.value = ''
    try {
      await apiRequest(`/v1/admin/webhook-sources/${encodeURIComponent(id)}`, { method: 'DELETE' })
      sources.value = sources.value.filter((s) => s.id !== id)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao excluir fonte.')
    }
  }

  function clearSecret() {
    lastSecret.value = ''
    lastSecretFor.value = ''
  }

  return {
    sources,
    loading,
    creating,
    errorMessage,
    lastSecret,
    lastSecretFor,
    fetchSources,
    createSource,
    rotateSecret,
    deleteSource,
    clearSecret,
  }
}
