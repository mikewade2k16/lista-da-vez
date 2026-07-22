<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import ConfigNumbers from '~/components/omnichannel/config/ConfigNumbers.vue'
import ConfigAiAgent from '~/components/omnichannel/config/ConfigAiAgent.vue'
import AutomationProfileConfig from './AutomationProfileConfig.vue'
import type { AutomationProfile, AutomationProfileInput } from '~/domain/omnichannel/automation-api'
import type { OmniAgent, OmniInstance } from '~/domain/omnichannel/config-types'

const props = defineProps<{
  open: boolean
  profile: AutomationProfile | null
  profiles: AutomationProfile[]
  instances: OmniInstance[]
  agents: OmniAgent[]
  loadingProfile: boolean
  saving: boolean
  canManageSettings: boolean
  canManageInstances: boolean
  canManageAgents: boolean
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  save: [input: AutomationProfileInput]
  optionsChanged: []
}>()

type DrawerMode = 'side' | 'center' | 'fullscreen'
type ConfigTab = 'atendimento' | 'whatsapp' | 'ia'

const mode = ref<DrawerMode>('side')
const activeTab = ref<ConfigTab>('atendimento')
const visited = ref<Set<ConfigTab>>(new Set())

const tabs = computed(() =>
  [
    {
      key: 'atendimento' as const,
      label: 'Atendimento',
      icon: 'i-lucide-sliders-horizontal',
      allowed: props.canManageSettings,
    },
    {
      key: 'whatsapp' as const,
      label: 'WhatsApp',
      icon: 'i-lucide-message-circle',
      allowed: props.canManageInstances,
    },
    {
      key: 'ia' as const,
      label: 'IA',
      icon: 'i-lucide-sparkles',
      allowed: props.canManageAgents,
    },
  ].filter((tab) => tab.allowed),
)

function setTab(tab: ConfigTab): void {
  activeTab.value = tab
  visited.value = new Set([...visited.value, tab])
}

watch(
  () => props.open,
  (open) => {
    if (!open) return
    const firstTab = tabs.value[0]?.key || 'atendimento'
    activeTab.value = firstTab
    visited.value = new Set([firstTab])
  },
  { immediate: true },
)
</script>

<template>
  <OmniEntityDrawer
    v-model:mode="mode"
    :model-value="open"
    title="Configurar atendimento"
    subtitle="Automação do cliente, conexão do WhatsApp e agente de IA em um único lugar."
    @update:model-value="emit('update:open', $event)"
  >
    <div class="calendar-config-drawer automation-ai-drawer">
      <nav class="calendar-config__tabs" aria-label="Seções da configuração">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          class="calendar-config__tab"
          :class="{ 'is-active': activeTab === tab.key }"
          @click="setTab(tab.key)"
        >
          <UIcon :name="tab.icon" aria-hidden="true" />
          <span>{{ tab.label }}</span>
        </button>
      </nav>

      <div class="calendar-config-drawer__panel">
        <p v-if="tabs.length === 0" class="automation-ai-drawer__empty">
          Você não tem permissão para configurar o atendimento nesta conta.
        </p>

        <template v-else>
          <div v-if="visited.has('atendimento')" v-show="activeTab === 'atendimento'">
            <AutomationProfileConfig
              :profile="profile"
              :instances="instances"
              :agents="agents"
              :loading="loadingProfile"
              :saving="saving"
              @save="emit('save', $event)"
            />
          </div>

          <div v-if="visited.has('whatsapp')" v-show="activeTab === 'whatsapp'">
            <ConfigNumbers :can-manage="canManageInstances" @changed="emit('optionsChanged')" />
          </div>

          <div v-if="visited.has('ia')" v-show="activeTab === 'ia'">
            <ConfigAiAgent
              :can-manage="canManageAgents"
              :profiles="profiles"
              @changed="emit('optionsChanged')"
            />
          </div>
        </template>
      </div>
    </div>
  </OmniEntityDrawer>
</template>

<style scoped>
.automation-ai-drawer {
  min-height: 100%;
}

.automation-ai-drawer__empty {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.82rem;
}
</style>
