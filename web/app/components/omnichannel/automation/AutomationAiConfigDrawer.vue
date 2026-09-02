<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import ConfigNumbers from '~/components/omnichannel/config/ConfigNumbers.vue'
import ConfigAiAgent from '~/components/omnichannel/config/ConfigAiAgent.vue'
import ConfigAiCredentials from '~/components/omnichannel/config/ConfigAiCredentials.vue'
import ConfigDepartments from '~/components/omnichannel/config/ConfigDepartments.vue'
import ConfigQueues from '~/components/omnichannel/config/ConfigQueues.vue'
import ConfigRoutingRules from '~/components/omnichannel/config/ConfigRoutingRules.vue'
import ConfigHandoffPolicies from '~/components/omnichannel/config/ConfigHandoffPolicies.vue'
import ConfigAiToolsKnowledge from '~/components/omnichannel/config/ConfigAiToolsKnowledge.vue'
import ConfigInstagram from '~/components/omnichannel/config/ConfigInstagram.vue'
import ConfigChannelClientBindings from '~/components/omnichannel/config/ConfigChannelClientBindings.vue'
import ConfigOperations from '~/components/omnichannel/config/ConfigOperations.vue'
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
  canAudit?: boolean
  initialTab?: string
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  save: [input: AutomationProfileInput]
  optionsChanged: []
}>()

type DrawerMode = 'side' | 'center' | 'fullscreen'
type ConfigTab =
  | 'atendimento'
  | 'whatsapp'
  | 'credenciais'
  | 'ia'
  | 'setores'
  | 'filas'
  | 'regras'
  | 'politicas'
  | 'tools'
  | 'instagram'
  | 'clientes'
  | 'operacao'

const mode = ref<DrawerMode>('side')
const activeTab = ref<ConfigTab>('atendimento')
const visited = ref<Set<ConfigTab>>(new Set())
const credentialRevision = ref(0)
const clients = computed(() => {
  const byId = new Map(props.profiles.map((item) => [item.client.id, item.client]))
  return [...byId.values()]
})

const tabs = computed(() =>
  [
    {
      key: 'operacao' as const,
      label: 'Saúde e rollout',
      icon: 'i-lucide-activity',
      allowed: props.canManageSettings || Boolean(props.canAudit),
    },
    {
      key: 'credenciais' as const,
      label: 'Chaves de IA',
      icon: 'i-lucide-key-round',
      allowed: props.canManageAgents,
    },
    {
      key: 'atendimento' as const,
      label: 'Atendimento',
      icon: 'i-lucide-sliders-horizontal',
      allowed: props.canManageSettings,
    },
    {
      key: 'clientes' as const,
      label: 'Clientes por canal',
      icon: 'i-lucide-network',
      allowed: props.canManageInstances,
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
    {
      key: 'setores' as const,
      label: 'Setores',
      icon: 'i-lucide-layers',
      allowed: props.canManageSettings,
    },
    {
      key: 'filas' as const,
      label: 'Filas',
      icon: 'i-lucide-users',
      allowed: props.canManageSettings,
    },
    {
      key: 'regras' as const,
      label: 'Regras',
      icon: 'i-lucide-git-branch',
      allowed: props.canManageSettings,
    },
    {
      key: 'politicas' as const,
      label: 'Handoff',
      icon: 'i-lucide-route',
      allowed: props.canManageSettings,
    },
    {
      key: 'tools' as const,
      label: 'Tools e conhecimento',
      icon: 'i-lucide-database-zap',
      allowed: props.canManageAgents,
    },
    {
      key: 'instagram' as const,
      label: 'Instagram',
      icon: 'i-lucide-instagram',
      allowed: props.canManageInstances,
    },
  ].filter((tab) => tab.allowed),
)

function setTab(tab: ConfigTab): void {
  activeTab.value = tab
  visited.value = new Set([...visited.value, tab])
}

function requestedTab(): ConfigTab | null {
  const raw = String(props.initialTab || '')
    .trim()
    .toLowerCase()
  const aliases: Record<string, ConfigTab> = {
    'channel-client-bindings': 'clientes',
    clientes: 'clientes',
  }
  const requested = (aliases[raw] || raw) as ConfigTab
  return tabs.value.some((tab) => tab.key === requested) ? requested : null
}

function onCredentialsChanged(): void {
  credentialRevision.value += 1
  emit('optionsChanged')
}

watch(
  () => props.open,
  (open) => {
    if (!open) return
    const firstTab = requestedTab() || tabs.value[0]?.key || 'atendimento'
    activeTab.value = firstTab
    visited.value = new Set([firstTab])
  },
  { immediate: true },
)

watch(
  () => props.initialTab,
  () => {
    if (!props.open) return
    const requested = requestedTab()
    if (requested) setTab(requested)
  },
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

          <div v-if="visited.has('clientes')" v-show="activeTab === 'clientes'">
            <ConfigChannelClientBindings
              :clients="clients"
              :instances="instances"
              :can-manage="canManageInstances"
            />
          </div>

          <div v-if="visited.has('ia')" v-show="activeTab === 'ia'">
            <ConfigAiAgent
              :key="`agents-${credentialRevision}`"
              :can-manage="canManageAgents"
              :profiles="profiles"
              @changed="emit('optionsChanged')"
            />
          </div>

          <div v-if="visited.has('credenciais')" v-show="activeTab === 'credenciais'">
            <ConfigAiCredentials :disabled="!canManageAgents" @changed="onCredentialsChanged" />
          </div>

          <div v-if="visited.has('setores')" v-show="activeTab === 'setores'">
            <ConfigDepartments :can-manage="canManageSettings" />
          </div>

          <div v-if="visited.has('filas')" v-show="activeTab === 'filas'">
            <ConfigQueues :can-manage="canManageSettings" />
          </div>

          <div v-if="visited.has('regras')" v-show="activeTab === 'regras'">
            <ConfigRoutingRules :can-manage="canManageSettings" />
          </div>

          <div v-if="visited.has('politicas')" v-show="activeTab === 'politicas'">
            <ConfigHandoffPolicies :can-manage="canManageSettings" />
          </div>

          <div v-if="visited.has('tools')" v-show="activeTab === 'tools'">
            <ConfigAiToolsKnowledge :can-manage="canManageAgents" :can-audit="Boolean(canAudit)" />
          </div>

          <div v-if="visited.has('instagram')" v-show="activeTab === 'instagram'">
            <ConfigInstagram :can-manage="canManageInstances" />
          </div>

          <div v-if="visited.has('operacao')" v-show="activeTab === 'operacao'">
            <ConfigOperations :instances="instances" :can-manage="canManageSettings" />
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
