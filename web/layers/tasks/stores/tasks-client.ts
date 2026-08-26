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

  // Derived from real auth role. consultant = client viewer; everyone else = admin.
  const userType = computed<'admin' | 'client'>(() =>
    auth.role === 'consultant' ? 'client' : 'admin',
  )
  const userLevel = computed(() => auth.role || 'admin')

  // clientId selecionado por padrao (UUID; vazio = nenhum). Por ora puxa TODOS os tenants ativos;
  // futuramente filtrar por flag "aparece em tasks" na pagina de clientes.
  const clientId = ref('')
  const clientOptions = ref<TaskClientOption[]>([])
  const loadingClientOptions = ref(false)
  const clientOptionsSynced = ref(false)
  const clientOptionsError = ref('')

  const isAdmin = computed(() => userType.value === 'admin')
  const activeClientLabel = computed(
    () => clientOptions.value.find((client) => client.value === clientId.value)?.label || 'Cliente',
  )

  function initialize() {
    if (isAdmin.value && !clientOptionsSynced.value) {
      void refreshClientOptions()
    }
  }

  async function refreshClientOptions() {
    if (loadingClientOptions.value) {
      return
    }
    loadingClientOptions.value = true
    clientOptionsError.value = ''
    try {
      const response = await apiRequest('/v1/tenants/clients?includeInactive=true')
      const list = Array.isArray(response?.tenants) ? (response.tenants as BackendTenant[]) : []
      clientOptions.value = list
        .filter((tenant) => tenant?.id)
        .map((tenant) => ({
          label: String(tenant.name || '').trim() || `Cliente ${String(tenant.id).slice(0, 8)}`,
          value: String(tenant.id),
          active: Boolean(tenant.active ?? tenant.isActive ?? true),
        }))
      clientOptionsSynced.value = true
    } catch (error) {
      clientOptionsError.value = getApiErrorMessage(error, 'Nao foi possivel carregar os clientes.')
    } finally {
      loadingClientOptions.value = false
    }
  }

  function setClientId(nextClientId: number | string) {
    const parsed = String(nextClientId ?? '').trim()
    if (parsed) {
      clientId.value = parsed
    }
  }

  return {
    userType,
    userLevel,
    clientId,
    clientOptions,
    loadingClientOptions,
    clientOptionsSynced,
    clientOptionsError,
    isAdmin,
    activeClientLabel,
    initialize,
    refreshClientOptions,
    setClientId,
  }
})
