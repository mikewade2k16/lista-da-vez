import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

// CatalogModelView espelha o catalogo global do back (automation.model_catalog).
// As flags vem do MODELOS.md: requiresResponsesApi (gpt-5*/o-series), acceptsTemperature
// (raciocinio nao aceita), visionOk (so esses funcionam no no de imagem).
export interface CatalogModelView {
  id: string
  provider: string
  kind: string
  label: string
  requiresResponsesApi: boolean
  acceptsTemperature: boolean
  visionOk: boolean
  sortOrder: number
}

export interface ModelParams {
  temperature?: number
  [key: string]: unknown
}

// AutomationModelView e a selecao atual por funcao (role), enriquecida com as flags.
export interface AutomationModelView {
  role: string
  provider: string
  modelId: string
  label: string
  requiresResponsesApi: boolean
  acceptsTemperature: boolean
  visionOk: boolean
  params: ModelParams
}

interface ModelsResponse {
  catalog: CatalogModelView[]
  selection: AutomationModelView[]
}

export function useAutomationModels() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const catalog = ref<CatalogModelView[]>([])
  const selection = ref<AutomationModelView[]>([])
  const loading = ref(false)
  const savingRole = ref('')
  const errorMessage = ref('')

  async function loadModels() {
    loading.value = true
    errorMessage.value = ''
    try {
      const res = (await apiRequest('/v1/automation/models', { method: 'GET' })) as ModelsResponse
      catalog.value = res.catalog ?? []
      selection.value = res.selection ?? []
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Falha ao carregar os modelos.')
    } finally {
      loading.value = false
    }
  }

  async function saveModel(
    role: string,
    provider: string,
    modelId: string,
    params: ModelParams,
  ): Promise<boolean> {
    savingRole.value = role
    errorMessage.value = ''
    try {
      const view = (await apiRequest('/v1/automation/models', {
        method: 'PUT',
        body: { role, provider, modelId, params },
      })) as AutomationModelView
      selection.value = selection.value.map((m) => (m.role === view.role ? view : m))
      return true
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Nao foi possivel salvar o modelo.')
      return false
    } finally {
      savingRole.value = ''
    }
  }

  return {
    catalog,
    selection,
    loading,
    savingRole,
    errorMessage,
    loadModels,
    saveModel,
  }
}
