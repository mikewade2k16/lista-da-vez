import { computed, onBeforeUnmount, ref, watch } from 'vue'
import {
  configureIntelligenceCredential,
  configureIntelligenceModel,
  createIntelligenceAgent,
  createIntelligenceAgentVersion,
  fetchIntelligenceAgents,
  fetchIntelligenceCredentials,
  fetchIntelligenceModels,
  publishIntelligenceAgentVersion,
  revokeIntelligenceCredential,
  updateIntelligenceAgent,
} from '~/domain/customer-intelligence/agent-admin-api'
import type {
  IntelligenceAgent,
  IntelligenceAgentPatchInput,
  IntelligenceAgentVersion,
  IntelligenceAgentVersionWriteInput,
  IntelligenceCredential,
  IntelligenceCredentialWriteInput,
  IntelligenceModel,
  IntelligenceModelWriteInput,
} from '~/domain/customer-intelligence/agent-admin-types'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import { useAuthStore } from '~/stores/auth'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { createApiRequest } from '~/utils/api-client'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'

export function useCustomerIntelligenceAgentAdmin() {
  const auth = useAuthStore()
  const scope = useCustomerIntelligenceStore()
  const access = useCustomerIntelligenceAccess()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)

  const models = ref<IntelligenceModel[]>([])
  const credentials = ref<IntelligenceCredential[]>([])
  const agents = ref<IntelligenceAgent[]>([])
  const sessionVersions = ref<Record<string, IntelligenceAgentVersion>>({})
  const loading = ref(false)
  const busyKey = ref('')
  const error = ref<CustomerApiErrorState | null>(null)
  const canLoad = computed(() => access.canManageAgents.value && access.clientScopeReady.value)

  let controller: AbortController | null = null
  let generation = 0

  function clear(): void {
    controller?.abort()
    controller = null
    generation += 1
    models.value = []
    credentials.value = []
    agents.value = []
    sessionVersions.value = {}
    loading.value = false
    busyKey.value = ''
    error.value = null
  }

  async function load(): Promise<void> {
    if (!canLoad.value) {
      clear()
      return
    }
    controller?.abort()
    const request = new AbortController()
    controller = request
    const current = ++generation
    const clientAccountId = scope.clientAccountId
    loading.value = true
    error.value = null
    try {
      const [nextModels, nextCredentials, nextAgents] = await Promise.all([
        fetchIntelligenceModels(api, request.signal),
        fetchIntelligenceCredentials(api, request.signal),
        fetchIntelligenceAgents(api, clientAccountId, request.signal),
      ])
      if (request.signal.aborted || current !== generation) return
      models.value = nextModels
      credentials.value = nextCredentials
      agents.value = nextAgents
    } catch (cause) {
      if (request.signal.aborted || current !== generation) return
      const classified = classifyCustomerApiError(
        cause,
        'Nao foi possivel carregar modelos, credenciais e agentes.',
      )
      if (classified.kind !== 'aborted') error.value = classified
    } finally {
      if (current === generation) loading.value = false
    }
  }

  function canMutate(key: string): boolean {
    if (!canLoad.value || busyKey.value) return false
    busyKey.value = key
    error.value = null
    return true
  }

  async function saveModel(input: IntelligenceModelWriteInput): Promise<boolean> {
    if (!canMutate(`model:${input.id || 'new'}`)) return false
    try {
      await configureIntelligenceModel(api, input)
      await load()
      return true
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel salvar o modelo.')
      return false
    } finally {
      busyKey.value = ''
    }
  }

  async function saveCredential(input: IntelligenceCredentialWriteInput): Promise<boolean> {
    if (!canMutate(`credential:${input.label}`)) return false
    try {
      await configureIntelligenceCredential(api, input)
      await load()
      return true
    } catch (cause) {
      error.value = classifyCustomerApiError(
        cause,
        'Nao foi possivel gravar ou rotacionar a credencial.',
      )
      return false
    } finally {
      // O composable nunca persiste a chave recebida; ela existe apenas no argumento.
      input.apiKey = ''
      busyKey.value = ''
    }
  }

  async function revokeCredential(credentialId: string): Promise<boolean> {
    if (!canMutate(`credential:${credentialId}`)) return false
    try {
      await revokeIntelligenceCredential(api, credentialId)
      await load()
      return true
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel revogar a credencial.')
      return false
    } finally {
      busyKey.value = ''
    }
  }

  async function addAgent(slug: string, name: string): Promise<boolean> {
    if (!canMutate('agent:new')) return false
    const clientAccountId = scope.clientAccountId
    try {
      await createIntelligenceAgent(api, { clientAccountId, slug, name })
      if (scope.clientAccountId === clientAccountId) await load()
      return true
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel criar o agente.')
      return false
    } finally {
      busyKey.value = ''
    }
  }

  async function saveAgent(agentId: string, input: IntelligenceAgentPatchInput): Promise<boolean> {
    if (!canMutate(`agent:${agentId}`)) return false
    try {
      await updateIntelligenceAgent(api, agentId, input)
      await load()
      return true
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel atualizar o agente.')
      return false
    } finally {
      busyKey.value = ''
    }
  }

  async function addAgentVersion(
    agentId: string,
    input: IntelligenceAgentVersionWriteInput,
  ): Promise<IntelligenceAgentVersion | null> {
    if (!canMutate(`version:${agentId}`)) return null
    try {
      const created = await createIntelligenceAgentVersion(api, agentId, input)
      sessionVersions.value = { ...sessionVersions.value, [agentId]: created }
      return created
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel criar o draft.')
      return null
    } finally {
      busyKey.value = ''
    }
  }

  async function publishAgentVersion(agentId: string, versionId: string): Promise<boolean> {
    if (!canMutate(`publish:${versionId}`)) return false
    try {
      const published = await publishIntelligenceAgentVersion(api, versionId)
      sessionVersions.value = { ...sessionVersions.value, [agentId]: published }
      await load()
      return true
    } catch (cause) {
      error.value = classifyCustomerApiError(cause, 'Nao foi possivel publicar o draft.')
      return false
    } finally {
      busyKey.value = ''
    }
  }

  watch(
    [() => scope.scopeKey, canLoad],
    () => {
      sessionVersions.value = {}
      void load()
    },
    { immediate: true },
  )
  onBeforeUnmount(clear)

  return {
    access,
    models,
    credentials,
    agents,
    sessionVersions,
    loading,
    busyKey,
    error,
    load,
    clear,
    saveModel,
    saveCredential,
    revokeCredential,
    addAgent,
    saveAgent,
    addAgentVersion,
    publishAgentVersion,
  }
}
