import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

// Fonte de verdade: core.accounts, exposta pelo catalogo GET /v1/tenants/clients. O "cliente" de uma task e o id
// UUID do account (gravado em clientAccountId). NAO ha mais lista mock — ver docs/LEGADO.md §4 +
// memoria project_tasks_client_source.
interface TaskClientOption {
  label: string
  value: string
  active: boolean
}

interface BackendTenant {
  id?: string
  name?: string
  slug?: string
  active?: boolean
  isActive?: boolean
}

export const useTasksClientStore = defineStore('tasks-client', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  // clientId selecionado por padrao (UUID; vazio = nenhum). Por ora puxa TODOS os tenants ativos;
  // futuramente filtrar por flag "aparece em tasks" na pagina de clientes.
  const clientId = ref('')
  const clientOptions = ref<TaskClientOption[]>([])
  const loadingClientOptions = ref(false)
  const clientOptionsSynced = ref(false)
  const clientOptionsError = ref('')
  let clientOptionsRequest: Promise<void> | null = null
  let clientCatalogAllowed = false

  const activeClientLabel = computed(
    () => clientOptions.value.find((client) => client.value === clientId.value)?.label || 'Cliente',
  )

  function clearClientOptions() {
    clientCatalogAllowed = false
    clientOptions.value = []
    clientOptionsSynced.value = false
    clientOptionsError.value = ''
  }

  function initialize(canBrowseAgencyClients: boolean): Promise<void> {
    // Fail closed no contexto de uma conta-cliente. O endpoint tambem aplica o escopo no backend,
    // mas a UI nem solicita o catalogo da agencia fora do workspace da agencia.
    if (!canBrowseAgencyClients) {
      clearClientOptions()
      return Promise.resolve()
    }
    clientCatalogAllowed = true
    if (clientOptionsSynced.value) return Promise.resolve()
    return refreshClientOptions(canBrowseAgencyClients)
  }

  function refreshClientOptions(canBrowseAgencyClients: boolean): Promise<void> {
    if (!canBrowseAgencyClients) {
      clearClientOptions()
      return Promise.resolve()
    }
    clientCatalogAllowed = true
    if (clientOptionsRequest) return clientOptionsRequest

    clientOptionsRequest = (async () => {
      loadingClientOptions.value = true
      clientOptionsError.value = ''
      try {
        const response = await apiRequest('/v1/tenants/clients?includeInactive=true')
        const list = Array.isArray(response?.tenants) ? (response.tenants as BackendTenant[]) : []
        if (clientCatalogAllowed) {
          clientOptions.value = list
            .filter((tenant) => tenant?.id)
            .map((tenant) => ({
              label: String(tenant.name || '').trim() || `Cliente ${String(tenant.id).slice(0, 8)}`,
              value: String(tenant.id),
              active: Boolean(tenant.active ?? tenant.isActive ?? true),
            }))
          clientOptionsSynced.value = true
        }
      } catch (error) {
        clientOptionsError.value = getApiErrorMessage(
          error,
          'Nao foi possivel carregar os clientes.',
        )
      } finally {
        loadingClientOptions.value = false
        clientOptionsRequest = null
      }
    })()

    return clientOptionsRequest
  }

  function setClientId(nextClientId: number | string) {
    const parsed = String(nextClientId ?? '').trim()
    if (parsed) {
      clientId.value = parsed
    }
  }

  return {
    clientId,
    clientOptions,
    loadingClientOptions,
    clientOptionsSynced,
    clientOptionsError,
    activeClientLabel,
    initialize,
    refreshClientOptions,
    setClientId,
  }
})
