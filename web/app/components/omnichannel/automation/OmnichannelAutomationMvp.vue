<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AutomationInterventionList from './AutomationInterventionList.vue'
import AutomationAiConfigDrawer from './AutomationAiConfigDrawer.vue'
import AutomationHiddenContacts from './AutomationHiddenContacts.vue'
import OmnichannelWorkspaceHeader from '~/components/omnichannel/OmnichannelWorkspaceHeader.vue'
import OmnichannelWhatsAppSessionModal from '~/components/omnichannel/inbox/OmnichannelWhatsAppSessionModal.vue'
import { useOmnichannelAutomationMvp } from '~/composables/omnichannel/useOmnichannelAutomationMvp'
import { useOmnichannelWhatsAppSession } from '~/composables/omnichannel/useOmnichannelWhatsAppSession'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../../../layers/core/stores/account'
import { CONVERSATION_PRIVACY_PERMISSION } from '~/domain/omnichannel/privacy-api'

type Tab = 'overview' | 'interventions' | 'hidden'

const auth = useAuthStore()
const accountStore = useCoreAccountStore()
const activeTab = ref<Tab>('interventions')
const configOpen = ref(false)
const whatsappSessionModalOpen = ref(false)
const {
  profiles,
  profile,
  interventions,
  instances,
  agents,
  selectedClientId,
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
  loadInterventions,
  save,
  resumeInterventionWithAI,
  pauseAttendanceAI,
  replyAttendanceWithAI,
  closeAttendanceConversation,
  loadHiddenContacts,
  restoreHiddenContact,
  startPolling,
} = useOmnichannelAutomationMvp()

const {
  activate: activateWhatsAppSession,
  deactivate: deactivateWhatsAppSession,
  connectionAlertTitle,
  connectionAlertDescription,
  connectionAlertColor,
  isConnected,
  hasExistingInstances,
  canManageChannel,
  loadingInstances,
} = useOmnichannelWhatsAppSession()

const showWhatsAppHeader = computed(() => canManageChannel.value && !loadingInstances.value)

const canManageAgents = computed(() =>
  auth.effectivePermissionKeys.includes('omnichannel.agents.manage'),
)

const canManageInstances = computed(() =>
  auth.effectivePermissionKeys.includes('omnichannel.instances.manage'),
)

const canManageSettings = computed(() =>
  auth.effectivePermissionKeys.includes('omnichannel.settings.manage'),
)

const canConfigure = computed(
  () => canManageSettings.value || canManageInstances.value || canManageAgents.value,
)

const canManagePrivacy = computed(() =>
  auth.effectivePermissionKeys.includes(CONVERSATION_PRIVACY_PERMISSION),
)

const canAudit = computed(() => auth.effectivePermissionKeys.includes('omnichannel.audit.view'))

const tabs: Array<{ id: Tab; label: string; icon: string }> = [
  { id: 'overview', label: 'Visão geral', icon: 'i-lucide-layout-dashboard' },
  { id: 'interventions', label: 'Atendimentos', icon: 'i-lucide-bot' },
]

const visibleTabs = computed(() =>
  (canManagePrivacy.value
    ? [...tabs, { id: 'hidden' as const, label: 'Pessoas ocultas', icon: 'i-lucide-eye-off' }]
    : tabs
  ).filter((tab) => tab.id !== 'overview'),
)

async function selectTab(tab: Tab): Promise<void> {
  activeTab.value = tab
  if (tab === 'hidden' && canManagePrivacy.value) await loadHiddenContacts()
}

async function handleClientSelect(clientId: string): Promise<void> {
  await selectClient(clientId)
  if (
    accountStore.accounts.some((account) => account.id === clientId) &&
    accountStore.activeAccountId !== clientId
  ) {
    await accountStore.switchAccount(clientId)
  }
}

const clientOptions = computed(() =>
  profiles.value.map((item) => ({
    value: item.client.id,
    label: item.client.name,
    meta: item.enabled ? 'IA ligada' : item.configured ? 'IA desligada' : 'Não configurada',
  })),
)

async function onConfigOpenChange(open: boolean): Promise<void> {
  configOpen.value = open
  if (!open) await load()
}

async function toggleAutomation(): Promise<void> {
  const current = profile.value
  if (!current?.whatsappInstance?.id || !current.aiAgent?.id || saving.value) return
  await save({
    whatsappInstanceId: current.whatsappInstance.id,
    aiAgentId: current.aiAgent.id,
    enabled: !current.enabled,
    closePolicy: {
      autoCloseEnabled: current.closePolicy.autoCloseEnabled,
      minimumConfidence: current.closePolicy.minimumConfidence,
      requireAllRequiredFields: current.closePolicy.requireAllRequiredFields,
      blockOnHumanRequest: current.closePolicy.blockOnHumanRequest,
      blockSensitiveTopics: current.closePolicy.blockSensitiveTopics,
    },
  })
}

onMounted(async () => {
  await load()
  startPolling()
  await activateWhatsAppSession()
})

watch(
  () => accountStore.activeAccountId,
  async (accountId) => {
    if (!accountId) return
    await load()
    if (profiles.value.some((item) => item.client.id === accountId)) {
      await selectClient(accountId)
    }
  },
)

onBeforeUnmount(() => {
  deactivateWhatsAppSession()
})
</script>

<template>
  <section class="automation-mvp">
    <OmnichannelWorkspaceHeader
      mode="automation"
      :visible="showWhatsAppHeader || canConfigure"
      :title="connectionAlertTitle"
      :description="connectionAlertDescription"
      :color="connectionAlertColor"
      :connected="isConnected"
      :configured="hasExistingInstances"
      :can-connect="canManageChannel"
      :client-value="selectedClientId"
      :client-options="clientOptions"
      :client-loading="loading"
      :client-disabled="clientOptions.length === 0"
      @connect="whatsappSessionModalOpen = true"
      @configure="configOpen = true"
      @update:client="handleClientSelect"
    />

    <div v-if="false" class="automation-mvp__toolbar">
      <AppSelectField
        v-if="false"
        :model-value="selectedClientId"
        :options="clientOptions"
        label="Cliente"
        placeholder="Selecione um cliente com acesso ao atendimento"
        empty-label="Nenhum cliente com o módulo de atendimento habilitado."
        search-placeholder="Buscar cliente"
        searchable
        :disabled="loading || clientOptions.length === 0"
        @update:model-value="selectClient"
      />

      <div class="automation-mvp__actions">
        <button
          type="button"
          class="automation-mvp__master-switch"
          :class="profile?.enabled ? 'is-enabled' : 'is-disabled'"
          :disabled="!canManageSettings || !profile?.configured || saving"
          :aria-label="
            profile?.enabled ? 'Desligar toda a automação de IA' : 'Ligar a automação de IA'
          "
          @click="toggleAutomation"
        >
          <span class="automation-mvp__master-status">
            <i aria-hidden="true"></i>
            {{ profile?.enabled ? 'IA ligada' : 'IA desligada' }}
          </span>
          <strong>
            {{ saving ? 'Salvando…' : profile?.enabled ? 'Desligar IA' : 'Ligar IA' }}
          </strong>
        </button>
        <AppPanelButton variant="secondary" :disabled="!canConfigure" @click="configOpen = true">
          <UIcon name="i-lucide-settings-2" aria-hidden="true" />
          Configurar atendimento
        </AppPanelButton>
        <NuxtLink to="/omnichannel" class="automation-mvp__inbox-link">
          <UIcon name="i-lucide-messages-square" />
          Ver Omnichannel
        </NuxtLink>
      </div>
    </div>

    <div v-if="error" class="automation-mvp__error">{{ error }}</div>

    <div class="automation-mvp__body">
      <main class="automation-mvp__content">
        <nav class="automation-mvp__tabs" aria-label="Seções da automação">
          <button
            v-for="tab in visibleTabs"
            :key="tab.id"
            type="button"
            :class="{ 'is-active': activeTab === tab.id }"
            @click="selectTab(tab.id)"
          >
            <UIcon :name="tab.icon" />
            {{ tab.label }}
            <span v-if="tab.id === 'interventions' && interventions.length">
              {{ interventions.length }}
            </span>
            <span v-if="tab.id === 'hidden' && hiddenContacts.length">
              {{ hiddenContacts.length }}
            </span>
          </button>
        </nav>

        <div v-if="loading" class="automation-mvp__loading">
          <UIcon name="i-lucide-loader-circle" />
          Carregando automação…
        </div>

        <AutomationInterventionList
          v-else-if="activeTab === 'interventions'"
          :items="interventions"
          :loading="loading"
          :resuming-ids="resumingInterventionIds"
          :pausing-ids="pausingAttendanceIds"
          :replying-ids="replyingAttendanceIds"
          :closing-ids="closingAttendanceIds"
          @refresh="loadInterventions"
          @resume-ai="resumeInterventionWithAI"
          @pause-ai="pauseAttendanceAI"
          @reply-ai="replyAttendanceWithAI"
          @close-conversation="closeAttendanceConversation"
        />

        <AutomationHiddenContacts
          v-else-if="canManagePrivacy"
          :items="hiddenContacts"
          :loading="loadingHiddenContacts"
          :restoring-ids="restoringHiddenContactIds"
          @refresh="loadHiddenContacts"
          @restore="restoreHiddenContact"
        />
      </main>
    </div>

    <AutomationAiConfigDrawer
      :open="configOpen"
      :profile="profile"
      :profiles="profiles"
      :instances="instances"
      :agents="agents"
      :loading-profile="loadingProfile"
      :saving="saving"
      :can-manage-settings="canManageSettings"
      :can-manage-instances="canManageInstances"
      :can-manage-agents="canManageAgents"
      :can-audit="canAudit"
      @save="save"
      @options-changed="load"
      @update:open="onConfigOpenChange"
    />

    <OmnichannelWhatsAppSessionModal v-model:open="whatsappSessionModalOpen" />
  </section>
</template>

<style scoped>
.automation-mvp {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  width: 100%;
  flex: 1;
  min-height: 0;
  padding: 0 0 1.5rem;
  overflow: hidden;
}

.automation-mvp__toolbar,
.automation-mvp__error,
.automation-mvp__body {
  margin-inline: 1rem;
}

.automation-mvp__toolbar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem;
}

.automation-mvp__toolbar :deep(.app-select-field) {
  width: min(420px, 100%);
}

.automation-mvp__actions {
  display: flex;
  align-items: center;
  gap: 0.55rem;
}

.automation-mvp__master-switch {
  display: grid;
  gap: 0.1rem;
  min-width: 9.5rem;
  padding: 0.5rem 0.75rem;
  border: 1px solid rgb(var(--border) / 0.62);
  border-radius: 0.9rem;
  background: rgb(var(--surface) / 0.5);
  box-shadow:
    inset 0 1px 0 rgb(255 255 255 / 0.08),
    0 8px 22px rgb(0 0 0 / 0.12);
  color: var(--text-main);
  text-align: left;
  backdrop-filter: blur(18px) saturate(135%);
  cursor: pointer;
}

.automation-mvp__master-switch.is-enabled {
  border-color: rgb(var(--success) / 0.38);
  background: linear-gradient(135deg, rgb(var(--success) / 0.13), rgb(var(--surface) / 0.5));
}

.automation-mvp__master-switch.is-disabled {
  border-color: rgb(var(--error) / 0.42);
  background: linear-gradient(135deg, rgb(var(--error) / 0.13), rgb(var(--surface) / 0.5));
}

.automation-mvp__master-switch:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.automation-mvp__master-status {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  color: var(--text-muted);
  font-size: 0.66rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.automation-mvp__master-status i {
  width: 0.42rem;
  height: 0.42rem;
  border-radius: 999px;
  background: rgb(var(--error));
}

.is-enabled .automation-mvp__master-status i {
  background: rgb(var(--success));
  box-shadow: 0 0 0 3px rgb(var(--success) / 0.12);
}

.automation-mvp__master-switch strong {
  font-size: 0.78rem;
}

.automation-mvp__inbox-link {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  min-height: 36px;
  padding: 0 0.8rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface));
  color: var(--text-main);
  font-size: 0.78rem;
  font-weight: 700;
  text-decoration: none;
}

.automation-mvp__error {
  padding: 0.65rem 0.8rem;
  border-radius: var(--radius-sm);
  background: rgb(var(--danger) / 0.12);
  color: rgb(var(--danger));
  font-size: 0.78rem;
}

.automation-mvp__body {
  display: flex;
  flex: 1;
  min-height: 0;
}

.automation-mvp__content {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  width: 100%;
  min-height: 0;
  padding: 0.9rem;
  overflow-y: auto;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.55);
}

.automation-mvp__tabs {
  display: flex;
  gap: 0.35rem;
  padding-bottom: 0.7rem;
  border-bottom: 1px solid var(--line-soft);
}

.automation-mvp__tabs button {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  min-height: 34px;
  padding: 0 0.7rem;
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-muted);
  font: inherit;
  font-size: 0.76rem;
  font-weight: 700;
  cursor: pointer;
}

.automation-mvp__tabs button.is-active {
  background: rgb(var(--primary) / 0.14);
  color: rgb(var(--primary));
}

.automation-mvp__tabs button span {
  min-width: 18px;
  padding: 0.05rem 0.3rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--accent-warning) 18%, transparent);
  color: var(--accent-warning);
  text-align: center;
}

.automation-mvp__loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  min-height: 220px;
  color: var(--text-muted);
  font-size: 0.8rem;
}

@media (max-width: 760px) {
  .automation-mvp {
    height: auto;
    min-height: calc(100vh - 5rem);
    overflow: visible;
  }

  .automation-mvp__toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .automation-mvp__actions {
    flex-wrap: wrap;
  }

  .automation-mvp__tabs {
    overflow-x: auto;
  }
}

/* A barra de ações agora é compartilhada com o inbox pelo header unificado. */
.automation-mvp__actions {
  display: none;
}
</style>
