import { computed, onBeforeUnmount, ref } from 'vue'
import { fetchAgents, fetchInstances } from '~/domain/omnichannel/config-api'
import type { OmniAgent, OmniInstance } from '~/domain/omnichannel/config-types'
import {
  closeAutomationConversation,
  fetchAutomationAttendances,
  fetchAutomationProfile,
  fetchAutomationProfiles,
  pauseAutomationAI,
  replyAutomationWithAI,
  saveAutomationProfile,
  type AutomationAttendance,
  type AutomationProfile,
  type AutomationProfileInput,
} from '~/domain/omnichannel/automation-api'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import {
  fetchHiddenOmnichannelContacts,
  restoreOmnichannelContact,
  type HiddenOmnichannelContact,
} from '~/domain/omnichannel/privacy-api'

export function useOmnichannelAutomationMvp() {
  const auth = useAuthStore()
  const ui = useUiStore()
  const runtimeConfig = useRuntimeConfig()
  const api = createApiRequest(runtimeConfig, () => auth.accessToken)

  const profiles = ref<AutomationProfile[]>([])
  const profile = ref<AutomationProfile | null>(null)
  const interventions = ref<AutomationAttendance[]>([])
  const instances = ref<OmniInstance[]>([])
  const agents = ref<OmniAgent[]>([])
  const selectedClientId = ref('')
  const loading = ref(true)
  const loadingProfile = ref(false)
  const saving = ref(false)
  const resumingInterventionIds = ref<string[]>([])
  const pausingAttendanceIds = ref<string[]>([])
  const replyingAttendanceIds = ref<string[]>([])
  const closingAttendanceIds = ref<string[]>([])
  const hiddenContacts = ref<HiddenOmnichannelContact[]>([])
  const loadingHiddenContacts = ref(false)
  const restoringHiddenContactIds = ref<string[]>([])
  const error = ref('')
  let pollTimer: ReturnType<typeof setInterval> | null = null

  const selectedClient = computed(
    () => profiles.value.find((item) => item.client.id === selectedClientId.value)?.client,
  )

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      profiles.value = await fetchAutomationProfiles(api)
      if (!selectedClientId.value && profiles.value.length) {
        selectedClientId.value = profiles.value[0]?.client.id || ''
      }
      await Promise.all([loadSelectedProfile(), loadInterventions(), loadOptions()])
    } catch (cause) {
      error.value = getApiErrorMessage(
        cause,
        'Não foi possível carregar a automação de atendimento.',
      )
    } finally {
      loading.value = false
    }
  }

  async function loadSelectedProfile(): Promise<void> {
    if (!selectedClientId.value) {
      profile.value = null
      return
    }
    loadingProfile.value = true
    try {
      profile.value = await fetchAutomationProfile(api, selectedClientId.value)
    } catch (cause) {
      error.value = getApiErrorMessage(cause, 'Não foi possível carregar o cliente selecionado.')
    } finally {
      loadingProfile.value = false
    }
  }

  async function selectClient(clientId: string): Promise<void> {
    if (selectedClientId.value === clientId) return
    selectedClientId.value = clientId
    await Promise.all([loadSelectedProfile(), loadInterventions()])
  }

  async function loadOptions(): Promise<void> {
    const [instanceResult, agentResult] = await Promise.allSettled([
      fetchInstances(api),
      fetchAgents(api),
    ])
    instances.value = instanceResult.status === 'fulfilled' ? instanceResult.value.instances : []
    agents.value = agentResult.status === 'fulfilled' ? agentResult.value : []
  }

  async function loadInterventions(): Promise<void> {
    try {
      interventions.value = await fetchAutomationAttendances(api, selectedClientId.value)
    } catch (cause) {
      error.value = getApiErrorMessage(cause, 'Não foi possível atualizar as intervenções.')
    }
  }

  async function save(input: AutomationProfileInput): Promise<boolean> {
    if (!selectedClientId.value || saving.value) return false
    saving.value = true
    try {
      const saved = await saveAutomationProfile(api, selectedClientId.value, input)
      profile.value = saved
      const index = profiles.value.findIndex((item) => item.client.id === saved.client.id)
      if (index >= 0) profiles.value[index] = saved
      ui.success('Automação do cliente salva.')
      return true
    } catch (cause) {
      ui.error(getApiErrorMessage(cause, 'Não foi possível salvar a automação.'))
      return false
    } finally {
      saving.value = false
    }
  }

  async function resumeInterventionWithAI(item: AutomationAttendance): Promise<void> {
    if (resumingInterventionIds.value.includes(item.id)) return
    resumingInterventionIds.value = [...resumingInterventionIds.value, item.id]
    try {
      await closeAutomationConversation(api, item.conversationId)
      await loadInterventions()
      ui.success('Intervenção encerrada. A IA atenderá a próxima mensagem deste contato.')
    } catch (cause) {
      ui.error(getApiErrorMessage(cause, 'Não foi possível retomar o atendimento com a IA.'))
    } finally {
      resumingInterventionIds.value = resumingInterventionIds.value.filter((id) => id !== item.id)
    }
  }

  async function loadHiddenContacts(): Promise<void> {
    loadingHiddenContacts.value = true
    try {
      hiddenContacts.value = await fetchHiddenOmnichannelContacts(api)
    } catch (cause) {
      error.value = getApiErrorMessage(cause, 'Não foi possível carregar as pessoas ocultas.')
    } finally {
      loadingHiddenContacts.value = false
    }
  }

  async function restoreHiddenContact(item: HiddenOmnichannelContact): Promise<void> {
    if (restoringHiddenContactIds.value.includes(item.contactId)) return
    restoringHiddenContactIds.value = [...restoringHiddenContactIds.value, item.contactId]
    try {
      await restoreOmnichannelContact(api, item.contactId)
      await Promise.all([loadHiddenContacts(), loadInterventions()])
      ui.success('Contato restaurado. As próximas mensagens voltarão a aparecer.')
    } catch (cause) {
      ui.error(getApiErrorMessage(cause, 'Não foi possível restaurar este contato.'))
    } finally {
      restoringHiddenContactIds.value = restoringHiddenContactIds.value.filter(
        (id) => id !== item.contactId,
      )
    }
  }

  async function closeAttendanceConversation(item: AutomationAttendance): Promise<void> {
    if (closingAttendanceIds.value.includes(item.id)) return
    closingAttendanceIds.value = [...closingAttendanceIds.value, item.id]
    try {
      await closeAutomationConversation(api, item.conversationId)
      await loadInterventions()
      ui.success('Conversa encerrada sem resposta da IA.')
    } catch (cause) {
      ui.error(getApiErrorMessage(cause, 'Não foi possível encerrar a conversa.'))
    } finally {
      closingAttendanceIds.value = closingAttendanceIds.value.filter((id) => id !== item.id)
    }
  }

  async function pauseAttendanceAI(item: AutomationAttendance): Promise<void> {
    if (pausingAttendanceIds.value.includes(item.id)) return
    pausingAttendanceIds.value = [...pausingAttendanceIds.value, item.id]
    try {
      await pauseAutomationAI(api, item.conversationId, automationActionKey())
      await loadInterventions()
      ui.success('Atendimento da IA pausado. A conversa ficou visível para intervenção.')
    } catch (cause) {
      ui.error(getApiErrorMessage(cause, 'Não foi possível pausar o atendimento da IA.'))
    } finally {
      pausingAttendanceIds.value = pausingAttendanceIds.value.filter((id) => id !== item.id)
    }
  }

  async function replyAttendanceWithAI(item: AutomationAttendance): Promise<void> {
    if (replyingAttendanceIds.value.includes(item.id)) return
    replyingAttendanceIds.value = [...replyingAttendanceIds.value, item.id]
    try {
      await replyAutomationWithAI(api, item.conversationId, automationActionKey())
      await loadInterventions()
      ui.success(
        item.mode === 'human_active'
          ? 'Conversa transferida. A IA vai responder as mensagens pendentes.'
          : 'Comando enviado. A IA vai responder agora, ignorando as regras automáticas de parada.',
      )
    } catch (cause) {
      ui.error(getApiErrorMessage(cause, 'Não foi possível pedir a resposta da IA.'))
    } finally {
      replyingAttendanceIds.value = replyingAttendanceIds.value.filter((id) => id !== item.id)
    }
  }

  function startPolling(): void {
    if (pollTimer) return
    pollTimer = setInterval(() => void loadInterventions(), 5_000)
  }

  onBeforeUnmount(() => {
    if (pollTimer) clearInterval(pollTimer)
  })

  return {
    profiles,
    profile,
    interventions,
    instances,
    agents,
    selectedClientId,
    selectedClient,
    loading,
    loadingProfile,
    saving,
    resumingInterventionIds,
    pausingAttendanceIds,
    replyingAttendanceIds,
    closingAttendanceIds,
    hiddenContacts,
    loadingHiddenContacts,
    restoringHiddenContactIds,
    error,
    load,
    selectClient,
    loadOptions,
    loadInterventions,
    loadHiddenContacts,
    restoreHiddenContact,
    save,
    resumeInterventionWithAI,
    pauseAttendanceAI,
    replyAttendanceWithAI,
    closeAttendanceConversation,
    startPolling,
  }
}

function automationActionKey(): string {
  if (typeof globalThis.crypto?.randomUUID === 'function') {
    return globalThis.crypto.randomUUID()
  }
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`
}
