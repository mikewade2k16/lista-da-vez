<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import ConfigResponsibles from '~/components/calendar/config/ConfigResponsibles.vue'
import ConfigHolidays from '~/components/calendar/config/ConfigHolidays.vue'
import ConfigAppearance from '~/components/calendar/config/ConfigAppearance.vue'
import ConfigAi from '~/components/calendar/config/ConfigAi.vue'
import ConfigTasks from '~/components/calendar/config/ConfigTasks.vue'
import ConfigClientProfiles from '~/components/calendar/config/ConfigClientProfiles.vue'
import ConfigMediaLimits from '~/components/calendar/config/ConfigMediaLimits.vue'
import { useCalendarStore } from '~/stores/calendar'
import { useUiStore } from '~/stores/ui'
import {
  defaultCalendarConfig,
  type CalendarAiConfig,
  type CalendarConfig,
  type CalendarHolidayFlags,
  type CalendarTasksConfig,
  type CalendarWhiteLabel,
  type WeekStart,
} from '~/utils/calendar'

// Drawer de configuracao do calendario (SPEC-F6, contrato C6). Substitui a antiga
// pagina /calendario/config: abre SEM sair do calendario, com abas. Reaproveita os
// componentes Config*.vue existentes. Modelos de salvar EXPLICITOS por aba:
//   responsaveis|feriados|aparencia|ia|integracoes -> draft compartilhado do
//     CalendarConfig + botao "Salvar configuracoes" no footer;
//   clientes|midia -> salvam com botao proprio dentro da aba (footer some).
// Deep-link ?config=<aba> lido/escrito na route (router.replace).
const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()

const store = useCalendarStore()
const ui = useUiStore()
const { config, members, clients } = storeToRefs(store)

const route = useRoute()
const router = useRouter()

type DrawerMode = 'side' | 'center' | 'fullscreen'
const mode = ref<DrawerMode>('side')

const TABS = [
  { key: 'responsaveis', label: 'Responsáveis', icon: 'i-lucide-users' },
  { key: 'feriados', label: 'Feriados', icon: 'i-lucide-calendar-heart' },
  { key: 'aparencia', label: 'Aparência', icon: 'i-lucide-palette' },
  { key: 'ia', label: 'IA', icon: 'i-lucide-sparkles' },
  { key: 'clientes', label: 'Clientes', icon: 'i-lucide-id-card' },
  { key: 'integracoes', label: 'Integrações', icon: 'i-lucide-plug' },
  { key: 'midia', label: 'Mídia', icon: 'i-lucide-image' },
] as const
type ConfigTab = (typeof TABS)[number]['key']

// Abas cujo salvar vive no footer do drawer (draft compartilhado).
const FOOTER_TABS: ConfigTab[] = ['responsaveis', 'feriados', 'aparencia', 'ia', 'integracoes']

// Rascunho editavel: re-hidrata da resposta do back sempre que a config chega
// (fonte unica = banco). So se preserva enquanto ha edicao pendente (touched).
const draft = ref<CalendarConfig>(defaultCalendarConfig())
const touched = ref(false)
const saving = ref(false)

const activeTab = ref<ConfigTab>('responsaveis')
// Abas ja visitadas ficam montadas (v-show), preservando o estado das abas de
// salvar-proprio (clientes/midia) ao trocar de aba; a 1a visita monta (lazy).
const visited = ref<Set<ConfigTab>>(new Set(['responsaveis']))

const showFooter = computed(() => FOOTER_TABS.includes(activeTab.value))
const footerSaveLabel = computed(() =>
  activeTab.value === 'ia' ? 'Salvar planejamento do calendário' : 'Salvar configurações',
)

watch(
  config,
  (cfg) => {
    if (touched.value) return
    draft.value = clone(cfg)
  },
  { immediate: true, deep: true },
)

function clone(cfg: CalendarConfig): CalendarConfig {
  return JSON.parse(JSON.stringify(cfg)) as CalendarConfig
}

function mark(): void {
  touched.value = true
}

function isConfigTab(value: unknown): value is ConfigTab {
  return typeof value === 'string' && TABS.some((tab) => tab.key === value)
}

function tabFromRoute(): ConfigTab {
  const raw = route.query.config
  return isConfigTab(raw) ? raw : 'responsaveis'
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

// Abrir: define a aba do deep-link, reseta o rascunho e busca config + membros
// (o store.init() da /calendario ja rodou; aqui so garantimos dados frescos).
function onOpen(): void {
  touched.value = false
  draft.value = clone(config.value)
  activeTab.value = tabFromRoute()
  visited.value = new Set([activeTab.value])
  void store.fetchConfig()
  void store.fetchMembers()
  syncQuery(activeTab.value)
}

// Fechar: remove ?config da URL preservando o resto da query.
function onClosed(): void {
  const { config: _omit, ...rest } = route.query
  void router.replace({ query: rest })
}

// Imediato: cobre o caso do drawer nascer JA aberto (deep-link direto / redirect de
// /calendario/config), onde o pai poe open=true antes deste filho montar — um watch
// nao-imediato perderia essa transicao inicial. `prev` evita disparar onClosed no
// primeiro tick quando o drawer nasce fechado (sem edicao pendente ainda).
watch(
  () => props.open,
  (open, prev) => {
    if (open) onOpen()
    else if (prev) onClosed()
  },
  { immediate: true },
)

// Dirty-guard unico: fechar com o rascunho compartilhado sujo pergunta via ui.confirm.
async function requestClose(): Promise<void> {
  if (touched.value) {
    const { confirmed } = await ui.confirm({
      title: 'Descartar alterações?',
      message: 'Há alterações não salvas nas configurações. Fechar vai descartá-las. Continuar?',
      confirmLabel: 'Descartar',
      cancelLabel: 'Continuar editando',
    })
    if (!confirmed) return
  }
  emit('update:open', false)
}

function onDrawerModel(value: boolean): void {
  // OmniEntityDrawer so emite fechamento (false); abrir vem sempre do pai.
  if (value) return
  void requestClose()
}

async function save(): Promise<void> {
  saving.value = true
  const ok = await store.saveConfig(draft.value)
  saving.value = false
  if (ok) {
    touched.value = false
    // Re-hidrata da resposta autoritativa do back (o back pode sanitizar `tasks`/
    // provider); fonte unica = banco (o watch de config nao dispara aqui pois a
    // config nao muda de novo apos o salvar).
    draft.value = clone(config.value)
    ui.success('Configurações salvas.')
  } else {
    ui.error('Não foi possível salvar as configurações.')
  }
}

// Handlers do rascunho compartilhado (marcam sujo ao editar).
function onResponsibles(value: string[]): void {
  draft.value.responsibleUserIds = value
  mark()
}
function onHolidays(value: CalendarHolidayFlags): void {
  draft.value.holidays = value
  mark()
}
function onWeekStart(value: WeekStart): void {
  draft.value.weekStartsOn = value
  mark()
}
// Atalhos de teclado (WAVE 11): { acao: tecla }; o back sanitiza no PUT.
function onShortcuts(value: Record<string, string>): void {
  draft.value.shortcuts = value
  mark()
}
function onClientColors(value: Record<string, string>): void {
  draft.value.clientColors = value
  mark()
}
function onTypeColors(value: Record<string, string>): void {
  draft.value.typeColors = value
  mark()
}
function onWhiteLabel(value: CalendarWhiteLabel): void {
  draft.value.whiteLabel = value
  mark()
}
function onAi(value: CalendarAiConfig): void {
  draft.value.ai = value
  mark()
}
function onTasks(value: CalendarTasksConfig): void {
  draft.value.tasks = value
  mark()
}
</script>

<template>
  <OmniEntityDrawer
    v-model:mode="mode"
    :model-value="open"
    title="Configurações do calendário"
    subtitle="Responsáveis, feriados, aparência, IA, clientes, integrações e mídia."
    @update:model-value="onDrawerModel"
  >
    <div class="calendar-config-drawer">
      <nav class="calendar-config__tabs" aria-label="Seções da configuração">
        <button
          v-for="tab in TABS"
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
        <div v-if="visited.has('responsaveis')" v-show="activeTab === 'responsaveis'">
          <ConfigResponsibles
            :model-value="draft.responsibleUserIds"
            :members="members"
            @update:model-value="onResponsibles"
          />
        </div>

        <div v-if="visited.has('feriados')" v-show="activeTab === 'feriados'">
          <ConfigHolidays :model-value="draft.holidays" @update:model-value="onHolidays" />
        </div>

        <div v-if="visited.has('aparencia')" v-show="activeTab === 'aparencia'">
          <ConfigAppearance
            :week-starts-on="draft.weekStartsOn"
            :client-colors="draft.clientColors"
            :type-colors="draft.typeColors"
            :white-label="draft.whiteLabel"
            :clients="clients"
            :shortcuts="draft.shortcuts"
            @update:week-starts-on="onWeekStart"
            @update:client-colors="onClientColors"
            @update:type-colors="onTypeColors"
            @update:white-label="onWhiteLabel"
            @update:shortcuts="onShortcuts"
          />
        </div>

        <div v-if="visited.has('ia')" v-show="activeTab === 'ia'">
          <ConfigAi :model-value="draft.ai" @update:model-value="onAi" />
        </div>

        <div v-if="visited.has('clientes')" v-show="activeTab === 'clientes'">
          <ConfigClientProfiles :clients="clients" />
        </div>

        <div v-if="visited.has('integracoes')" v-show="activeTab === 'integracoes'">
          <ConfigTasks :model-value="draft.tasks" @update:model-value="onTasks" />
        </div>

        <div v-if="visited.has('midia')" v-show="activeTab === 'midia'">
          <ConfigMediaLimits />
        </div>
      </div>
    </div>

    <template v-if="showFooter" #footer>
      <span v-if="touched" class="calendar-config-page__dirty">Alterações não salvas</span>
      <AppPanelButton variant="primary" :disabled="saving" @click="save">
        {{ footerSaveLabel }}
      </AppPanelButton>
    </template>
  </OmniEntityDrawer>
</template>
