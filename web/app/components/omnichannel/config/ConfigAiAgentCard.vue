<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import ConfigAiAgentClientScope from '~/components/omnichannel/config/ConfigAiAgentClientScope.vue'
import ConfigAiAgentProviderKeys from '~/components/omnichannel/config/ConfigAiAgentProviderKeys.vue'
import ConfigAiAgentVersions from '~/components/omnichannel/config/ConfigAiAgentVersions.vue'
import ConfigAiAgentSimulator from '~/components/omnichannel/config/ConfigAiAgentSimulator.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { fetchAgent, fetchAgentVersions, updateAgent } from '~/domain/omnichannel/config-api'
import { saveAgentConfiguration } from '~/domain/omnichannel/ai-configuration-api'
import type { AutomationProfile } from '~/domain/omnichannel/automation-api'
import type {
  OmniAgent,
  OmniAgentVersion,
  OmniAgentVersionInput,
} from '~/domain/omnichannel/config-types'

const props = defineProps<{
  agent: OmniAgent
  profiles: AutomationProfile[]
  disabled?: boolean
}>()
const emit = defineEmits<{ changed: [] }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

const local = ref<OmniAgent>(props.agent)
const versions = ref<OmniAgentVersion[]>([])
const nameDraft = ref('')
const saving = ref(false)
const savingConfiguration = ref(false)
const providerKeysRevision = ref(0)

function hydrate(): void {
  local.value = props.agent
  nameDraft.value = props.agent.name
}
watch(() => props.agent, hydrate, { immediate: true })

async function reload(): Promise<void> {
  try {
    local.value = await fetchAgent(api, props.agent.id)
    nameDraft.value = local.value.name
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível recarregar o agente.'))
  }
}

async function loadVersions(): Promise<void> {
  try {
    versions.value = await fetchAgentVersions(api, props.agent.id)
  } catch {
    versions.value = []
  }
}

async function saveIdentity(): Promise<void> {
  const name = nameDraft.value.trim()
  if (!name) {
    ui.error('O nome do agente não pode ficar vazio.')
    return
  }
  saving.value = true
  try {
    await updateAgent(api, props.agent.id, { name })
    ui.success('Nome do agente salvo.')
    await reload()
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível salvar o agente.'))
  } finally {
    saving.value = false
  }
}

async function onSaveConfiguration(payload: OmniAgentVersionInput): Promise<void> {
  if (savingConfiguration.value) return
  savingConfiguration.value = true
  try {
    const saved = await saveAgentConfiguration(api, props.agent.id, payload)
    ui.success(`Configurações salvas. A IA já está usando a v${saved.version}.`)
    await Promise.all([reload(), loadVersions()])
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível salvar as configurações.'))
  } finally {
    savingConfiguration.value = false
  }
}

onMounted(() => void loadVersions())
</script>

<template>
  <ConfigAiAgentVersions
    :key="`${agent.id}-${local.providerKey.set}-${local.providerKey.last4}-${providerKeysRevision}`"
    :agent-id="agent.id"
    :versions="versions"
    :active-version-id="local.activeVersionId"
    :disabled="disabled || savingConfiguration"
    @save="onSaveConfiguration"
  >
    <template #client-scope>
      <ConfigAiAgentClientScope
        :agent-id="agent.id"
        :profiles="profiles"
        :disabled="disabled"
        @changed="emit('changed')"
      />
    </template>

    <template #api-key>
      <ConfigAiAgentProviderKeys
        :agent-id="agent.id"
        :disabled="disabled"
        @changed="providerKeysRevision++"
      />
    </template>

    <template #simulator>
      <ConfigAiAgentSimulator :agent-id="agent.id" :versions="versions" :disabled="disabled" />
    </template>

    <template #identity>
      <div class="cfg-identity">
        <label class="calendar-config__field">
          <span class="calendar-config__field-label">Nome do agente</span>
          <input
            v-model="nameDraft"
            class="calendar-config__input"
            type="text"
            :disabled="disabled || saving"
          />
        </label>
        <div class="calendar-config__section-actions">
          <AppPanelButton
            variant="secondary"
            :disabled="disabled || saving || !nameDraft.trim()"
            @click="saveIdentity"
          >
            Salvar nome
          </AppPanelButton>
        </div>
      </div>
    </template>
  </ConfigAiAgentVersions>
</template>

<style scoped>
.cfg-identity {
  display: grid;
  gap: 0.65rem;
}
</style>
