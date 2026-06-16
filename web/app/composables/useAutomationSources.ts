import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

export interface AutomationSourcesView {
  catalogEnabled: boolean
  siteUrls: string[]
}

const DEFAULTS: AutomationSourcesView = {
  catalogEnabled: false,
  siteUrls: [],
}

export function useAutomationSources() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const catalogEnabled = ref(DEFAULTS.catalogEnabled)
  const siteUrls = ref<string[]>([...DEFAULTS.siteUrls])
  const loading = ref(false)
  const saving = ref(false)
  const saved = ref(false)
  const errorMessage = ref('')

  async function loadSources() {
    loading.value = true
    errorMessage.value = ''
    try {
      const res = (await apiRequest('/v1/automation/sources', {
        method: 'GET',
      })) as AutomationSourcesView
      catalogEnabled.value = res.catalogEnabled ?? DEFAULTS.catalogEnabled
      siteUrls.value = Array.isArray(res.siteUrls) ? [...res.siteUrls] : [...DEFAULTS.siteUrls]
    } catch {
      // Degrade limpo: backend pode ainda nao estar no ar (M5 backend em paralelo)
      catalogEnabled.value = DEFAULTS.catalogEnabled
      siteUrls.value = [...DEFAULTS.siteUrls]
    } finally {
      loading.value = false
    }
  }

  async function saveSources(): Promise<boolean> {
    saving.value = true
    saved.value = false
    errorMessage.value = ''
    // Otimista: captura estado antes do request
    const snapshot = {
      catalogEnabled: catalogEnabled.value,
      siteUrls: [...siteUrls.value],
    }
    try {
      await apiRequest('/v1/automation/sources', {
        method: 'PUT',
        body: snapshot,
      })
      saved.value = true
      setTimeout(() => {
        saved.value = false
      }, 2000)
      return true
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Nao foi possivel salvar as fontes.')
      return false
    } finally {
      saving.value = false
    }
  }

  function addUrl(url: string) {
    const trimmed = url.trim()
    if (!trimmed || siteUrls.value.includes(trimmed)) return
    siteUrls.value = [...siteUrls.value, trimmed]
  }

  function removeUrl(index: number) {
    siteUrls.value = siteUrls.value.filter((_, i) => i !== index)
  }

  return {
    catalogEnabled,
    siteUrls,
    loading,
    saving,
    saved,
    errorMessage,
    loadSources,
    saveSources,
    addUrl,
    removeUrl,
  }
}
