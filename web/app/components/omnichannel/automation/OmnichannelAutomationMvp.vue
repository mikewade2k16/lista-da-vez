<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AutomationOverview from './AutomationOverview.vue'
import AutomationInterventionList from './AutomationInterventionList.vue'
import AutomationAiConfigDrawer from './AutomationAiConfigDrawer.vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useOmnichannelAutomationMvp } from '~/composables/omnichannel/useOmnichannelAutomationMvp'
import { useAuthStore } from '~/stores/auth'

type Tab = 'overview' | 'interventions'

const auth = useAuthStore()
const activeTab = ref<Tab>('overview')
const configOpen = ref(false)
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
  error,
  load,
  selectClient,
  loadInterventions,
  save,
  resumeInterventionWithAI,
  pauseAttendanceAI,
  replyAttendanceWithAI,
  startPolling,
} = useOmnichannelAutomationMvp()

const canManageAgents = computed(
  () =>
    auth.role === 'platform_admin' ||
    auth.effectivePermissionKeys.includes('omnichannel.agents.manage'),
)

const canManageInstances = computed(
  () =>
    auth.role === 'platform_admin' ||
    auth.effectivePermissionKeys.includes('omnichannel.instances.manage'),
)

const canManageSettings = computed(
  () =>
    auth.role === 'platform_admin' ||
    auth.effectivePermissionKeys.includes('omnichannel.settings.manage'),
)

const canConfigure = computed(
  () => canManageSettings.value || canManageInstances.value || canManageAgents.value,
)

const tabs: Array<{ id: Tab; label: string; icon: string }> = [
  { id: 'overview', label: 'Visão geral', icon: 'i-lucide-layout-dashboard' },
  { id: 'interventions', label: 'Atendimentos', icon: 'i-lucide-bot' },
]

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

onMounted(async () => {
  await load()
  startPolling()
})
</script>

<template>
  <section class="automation-mvp">
    <div class="automation-mvp__toolbar">
      <AppSelectField
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
            v-for="tab in tabs"
            :key="tab.id"
            type="button"
            :class="{ 'is-active': activeTab === tab.id }"
            @click="activeTab = tab.id"
          >
            <UIcon :name="tab.icon" />
            {{ tab.label }}
            <span v-if="tab.id === 'interventions' && interventions.length">
              {{ interventions.length }}
            </span>
          </button>
        </nav>

        <div v-if="loading" class="automation-mvp__loading">
          <UIcon name="i-lucide-loader-circle" />
          Carregando automação…
        </div>

        <AutomationOverview
          v-else-if="activeTab === 'overview'"
          :profiles="profiles"
          :interventions="interventions"
        />

        <AutomationInterventionList
          v-else
          :items="interventions"
          :loading="loading"
          :resuming-ids="resumingInterventionIds"
          :pausing-ids="pausingAttendanceIds"
          :replying-ids="replyingAttendanceIds"
          @refresh="loadInterventions"
          @resume-ai="resumeInterventionWithAI"
          @pause-ai="pauseAttendanceAI"
          @reply-ai="replyAttendanceWithAI"
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
      @save="save"
      @options-changed="load"
      @update:open="onConfigOpenChange"
    />
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
  padding: 1.5rem;
  overflow: hidden;
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
</style>
