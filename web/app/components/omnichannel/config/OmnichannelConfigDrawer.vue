<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import ConfigNumbers from '~/components/omnichannel/config/ConfigNumbers.vue'
import ConfigDepartments from '~/components/omnichannel/config/ConfigDepartments.vue'
import ConfigQueues from '~/components/omnichannel/config/ConfigQueues.vue'
import ConfigRoutingRules from '~/components/omnichannel/config/ConfigRoutingRules.vue'
import ConfigHandoffPolicies from '~/components/omnichannel/config/ConfigHandoffPolicies.vue'
import ConfigAiAgent from '~/components/omnichannel/config/ConfigAiAgent.vue'
import ConfigAiToolsKnowledge from '~/components/omnichannel/config/ConfigAiToolsKnowledge.vue'
import ConfigInstagram from '~/components/omnichannel/config/ConfigInstagram.vue'
import { useAuthStore } from '~/stores/auth'

// Host das telas de config do omnichannel (F10). Abas + deep-link ?config=<aba>, no drawer
// canônico da casa (precedente do calendário). Cada aba salva sozinha (não há draft
// compartilhado / footer global). Substitui uma página /omnichannel/config — o drawer
// evita a armadilha da rota-pai (pages/omnichannel/ engole a filha).
//
// GATE: platform_admin tem has()=false no front, então cada aba é liberada por
// `isPlatformAdmin || effectivePermissionKeys.includes(<perm>)` — senão a config sumiria
// justamente para quem administra.
const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

type DrawerMode = 'side' | 'center' | 'fullscreen'
const mode = ref<DrawerMode>('side')

const isPlatformAdmin = computed(() => auth.role === 'platform_admin')
function can(perm: string): boolean {
  return isPlatformAdmin.value || auth.effectivePermissionKeys.includes(perm)
}

const ALL_TABS = [
  {
    key: 'numeros',
    label: 'Números',
    icon: 'i-lucide-smartphone',
    perm: 'omnichannel.instances.manage',
  },
  {
    key: 'setores',
    label: 'Setores',
    icon: 'i-lucide-layers',
    perm: 'omnichannel.settings.manage',
  },
  { key: 'filas', label: 'Filas', icon: 'i-lucide-users', perm: 'omnichannel.settings.manage' },
  {
    key: 'regras',
    label: 'Regras',
    icon: 'i-lucide-git-branch',
    perm: 'omnichannel.settings.manage',
  },
  {
    key: 'politicas',
    label: 'Handoff',
    icon: 'i-lucide-route',
    perm: 'omnichannel.settings.manage',
  },
  {
    key: 'agente',
    label: 'Agente IA',
    icon: 'i-lucide-sparkles',
    perm: 'omnichannel.agents.manage',
  },
  {
    key: 'tools',
    label: 'Tools e conhecimento',
    icon: 'i-lucide-database-zap',
    perm: 'omnichannel.agents.manage',
  },
  {
    key: 'instagram',
    label: 'Instagram',
    icon: 'i-lucide-instagram',
    perm: 'omnichannel.instances.manage',
  },
] as const
type ConfigTab = (typeof ALL_TABS)[number]['key']

const tabs = computed(() => ALL_TABS.filter((t) => can(t.perm)))

const activeTab = ref<ConfigTab>('numeros')
const visited = ref<Set<ConfigTab>>(new Set())

function isConfigTab(value: unknown): value is ConfigTab {
  return typeof value === 'string' && ALL_TABS.some((t) => t.key === value)
}

function firstAvailable(): ConfigTab {
  return tabs.value[0]?.key || 'numeros'
}

function tabFromRoute(): ConfigTab {
  const raw = route.query.config
  if (isConfigTab(raw) && tabs.value.some((t) => t.key === raw)) return raw
  return firstAvailable()
}

function syncQuery(tab: ConfigTab): void {
  if (route.query.config === tab) return
  void router.replace({ query: { ...route.query, config: tab } })
}

function setTab(tab: ConfigTab): void {
  if (tab === activeTab.value) return
  activeTab.value = tab
  visited.value = new Set([...visited.value, tab])
  syncQuery(tab)
}

function onOpen(): void {
  activeTab.value = tabFromRoute()
  visited.value = new Set([activeTab.value])
  syncQuery(activeTab.value)
}

function onClosed(): void {
  const { config: _omit, ...rest } = route.query
  void router.replace({ query: rest })
}

watch(
  () => props.open,
  (open, prev) => {
    if (open) onOpen()
    else if (prev) onClosed()
  },
  { immediate: true },
)

function onDrawerModel(value: boolean): void {
  if (value) return
  emit('update:open', false)
}
</script>

<template>
  <OmniEntityDrawer
    v-model:mode="mode"
    :model-value="open"
    title="Configurações do atendimento"
    subtitle="Números, setores, filas, regras de roteamento e agente de IA."
    @update:model-value="onDrawerModel"
  >
    <div class="omni-config">
      <nav class="omni-config__tabs" aria-label="Seções da configuração">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          class="omni-config__tab"
          :class="{ 'is-active': activeTab === tab.key }"
          @click="setTab(tab.key)"
        >
          <UIcon :name="tab.icon" aria-hidden="true" />
          <span>{{ tab.label }}</span>
        </button>
      </nav>

      <div class="omni-config__panel">
        <p v-if="tabs.length === 0" class="omni-config__empty">
          Você não tem permissão para configurar o atendimento nesta conta.
        </p>

        <template v-else>
          <div v-if="visited.has('numeros')" v-show="activeTab === 'numeros'">
            <ConfigNumbers :can-manage="can('omnichannel.instances.manage')" />
          </div>
          <div v-if="visited.has('setores')" v-show="activeTab === 'setores'">
            <ConfigDepartments :can-manage="can('omnichannel.settings.manage')" />
          </div>
          <div v-if="visited.has('filas')" v-show="activeTab === 'filas'">
            <ConfigQueues :can-manage="can('omnichannel.settings.manage')" />
          </div>
          <div v-if="visited.has('regras')" v-show="activeTab === 'regras'">
            <ConfigRoutingRules :can-manage="can('omnichannel.settings.manage')" />
          </div>
          <div v-if="visited.has('politicas')" v-show="activeTab === 'politicas'">
            <ConfigHandoffPolicies :can-manage="can('omnichannel.settings.manage')" />
          </div>
          <div v-if="visited.has('agente')" v-show="activeTab === 'agente'">
            <ConfigAiAgent :can-manage="can('omnichannel.agents.manage')" />
          </div>
          <div v-if="visited.has('tools')" v-show="activeTab === 'tools'">
            <ConfigAiToolsKnowledge
              :can-manage="can('omnichannel.agents.manage')"
              :can-audit="can('omnichannel.audit.view')"
            />
          </div>
          <div v-if="visited.has('instagram')" v-show="activeTab === 'instagram'">
            <ConfigInstagram :can-manage="can('omnichannel.instances.manage')" />
          </div>
        </template>
      </div>
    </div>
  </OmniEntityDrawer>
</template>

<style scoped>
.omni-config {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  min-height: 0;
}

.omni-config__tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  position: sticky;
  top: 0;
  z-index: 1;
  padding-bottom: 0.5rem;
  background: rgb(var(--surface));
}

.omni-config__tab {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.4rem 0.75rem;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  background: transparent;
  color: rgb(var(--muted));
  font-size: 0.8rem;
  font-weight: 600;
  cursor: pointer;
  transition:
    background 0.15s ease,
    color 0.15s ease;
}

.omni-config__tab:hover {
  color: rgb(var(--text));
}

.omni-config__tab.is-active {
  background: rgb(var(--primary) / 0.14);
  border-color: rgb(var(--primary) / 0.4);
  color: rgb(var(--primary));
}

.omni-config__panel {
  min-height: 0;
}

.omni-config__empty {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.85rem;
}
</style>
