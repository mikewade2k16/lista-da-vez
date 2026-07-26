import { onBeforeUnmount, ref, watch } from 'vue'
import {
  buildIntelligenceSourceWriteInput,
  fetchIntelligenceSourceCatalog,
  fetchIntelligenceSources,
  saveIntelligenceSource,
  setIntelligenceSourceEnabled,
  syncIntelligenceSource,
  type IntelligenceSourceConfig,
  type IntelligenceSourceDescriptor,
  type IntelligenceSourceDraft,
} from '~/domain/customer-intelligence/sources'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'
import { useAuthStore } from '~/stores/auth'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { createApiRequest } from '~/utils/api-client'

export function useCustomerIntelligenceSources() {
  const auth = useAuthStore()
  const access = useCustomerIntelligenceAccess()
  const scopeStore = useCustomerIntelligenceStore()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
  const catalog = ref<IntelligenceSourceDescriptor[]>([])
  const sources = ref<IntelligenceSourceConfig[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const error = ref<CustomerApiErrorState | null>(null)
  let requestController: AbortController | null = null
  let generation = 0

  function clear(): void {
    requestController?.abort()
    requestController = null
    generation += 1
    catalog.value = []
    sources.value = []
    error.value = null
    loading.value = false
  }

  async function load(): Promise<void> {
    if (!access.canViewSources.value || !access.clientScopeReady.value) {
      clear()
      return
    }
    requestController?.abort()
    const controller = new AbortController()
    requestController = controller
    const current = ++generation
    loading.value = true
    error.value = null
    try {
      const [catalogResult, sourcesResult] = await Promise.all([
        fetchIntelligenceSourceCatalog(api, scopeStore.clientAccountId, controller.signal),
        fetchIntelligenceSources(api, scopeStore.clientAccountId, controller.signal),
      ])
      if (controller.signal.aborted || current !== generation) return
      catalog.value = catalogResult
      sources.value = sourcesResult
    } catch (cause) {
      if (controller.signal.aborted || current !== generation) return
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel carregar as fontes.')
    } finally {
      if (current === generation) loading.value = false
    }
  }

  async function save(
    descriptor: IntelligenceSourceDescriptor,
    existing: IntelligenceSourceConfig | null,
    draft: IntelligenceSourceDraft,
  ): Promise<boolean> {
    if (!access.canManageSources.value || saving.value) return false
    saving.value = true
    try {
      const input = buildIntelligenceSourceWriteInput(
        descriptor,
        existing,
        scopeStore.clientAccountId,
        draft,
      )
      await saveIntelligenceSource(api, input)
      await load()
      return true
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel salvar a fonte.')
      return false
    } finally {
      saving.value = false
    }
  }

  async function test(source: IntelligenceSourceConfig): Promise<void> {
    if (!access.canManageSources.value) return
    try {
      await syncIntelligenceSource(api, source.id, scopeStore.clientAccountId)
      await load()
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'O teste da fonte falhou.')
    }
  }

  async function toggle(source: IntelligenceSourceConfig, enabled: boolean): Promise<void> {
    if (!access.canManageSources.value) return
    try {
      await setIntelligenceSourceEnabled(api, source, enabled)
      await load()
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel alterar a fonte.')
    }
  }

  watch([() => scopeStore.scopeKey, () => access.canViewSources.value], () => void load(), {
    immediate: true,
  })
  onBeforeUnmount(clear)

  return { catalog, sources, loading, saving, error, load, save, test, toggle, clear }
}
