<script setup lang="ts">
import { computed, ref } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import {
  saveAutomationProfile,
  type AutomationProfile,
  type AutomationProfileInput,
} from '~/domain/omnichannel/automation-api'

const props = defineProps<{
  agentId: string
  profiles: AutomationProfile[]
  disabled?: boolean
}>()
const emit = defineEmits<{ changed: [] }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

const selectedClientId = ref('')
const saving = ref(false)

const selectedProfile = computed(
  () => props.profiles.find((profile) => profile.client.id === selectedClientId.value) || null,
)
const assignedProfiles = computed(() =>
  props.profiles.filter((profile) => profile.aiAgent?.id === props.agentId),
)
const clientOptions = computed(() =>
  props.profiles.map((profile) => ({
    value: profile.client.id,
    label: profile.client.name,
    meta: profile.whatsappInstance
      ? profile.aiAgent?.id === props.agentId
        ? 'Já vinculado'
        : 'Pronto para vincular'
      : 'Configure o WhatsApp primeiro',
  })),
)
const blockedReason = computed(() => {
  if (!selectedProfile.value) return 'Selecione um cliente.'
  if (!selectedProfile.value.whatsappInstance)
    return 'Este cliente ainda não possui um número definido na aba Atendimento.'
  if (selectedProfile.value.aiAgent?.id === props.agentId) return 'Este cliente já usa este agente.'
  return ''
})

function profileInput(profile: AutomationProfile): AutomationProfileInput {
  return {
    whatsappInstanceId: profile.whatsappInstance!.id,
    aiAgentId: props.agentId,
    enabled: profile.enabled,
    closePolicy: {
      autoCloseEnabled: profile.closePolicy.autoCloseEnabled,
      minimumConfidence: profile.closePolicy.minimumConfidence,
      requireAllRequiredFields: profile.closePolicy.requireAllRequiredFields,
      blockOnHumanRequest: profile.closePolicy.blockOnHumanRequest,
      blockSensitiveTopics: profile.closePolicy.blockSensitiveTopics,
    },
  }
}

async function assign(): Promise<void> {
  const profile = selectedProfile.value
  if (!profile || blockedReason.value || saving.value) return
  saving.value = true
  try {
    await saveAutomationProfile(api, profile.client.id, profileInput(profile))
    ui.success(`${profile.client.name} vinculado a este agente.`)
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível vincular o cliente ao agente.'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="calendar-config__client-scope">
    <div class="calendar-config__block">
      <span class="calendar-config__field-label">Clientes vinculados</span>
      <div v-if="assignedProfiles.length" class="calendar-config__members">
        <span
          v-for="profile in assignedProfiles"
          :key="profile.client.id"
          class="cfg-scope__client"
        >
          {{ profile.client.name }}
        </span>
      </div>
      <p v-else class="calendar-config__empty">Nenhum cliente usa este agente.</p>
    </div>

    <div class="calendar-config__block">
      <AppSelectField
        label="Vincular cliente"
        :model-value="selectedClientId"
        :options="clientOptions"
        placeholder="Selecione um cliente"
        empty-label="Nenhum cliente com acesso ao módulo."
        searchable
        :disabled="disabled || saving || clientOptions.length === 0"
        @update:model-value="selectedClientId = $event"
      />
      <span v-if="blockedReason" class="calendar-config__hint">{{ blockedReason }}</span>
      <div class="calendar-config__section-actions">
        <AppPanelButton
          variant="secondary"
          :disabled="disabled || saving || !!blockedReason"
          @click="assign"
        >
          Vincular a este agente
        </AppPanelButton>
      </div>
    </div>
  </div>
</template>

<style scoped>
.cfg-scope__client {
  display: inline-flex;
  padding: 0.3rem 0.55rem;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  color: rgb(var(--text));
  font-size: 0.76rem;
}
</style>
