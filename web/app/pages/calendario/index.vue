<script setup lang="ts">
import { computed, nextTick, onActivated, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import CalendarControls from '~/components/calendar/CalendarControls.vue'
import CalendarSearch from '~/components/calendar/CalendarSearch.vue'
import CalendarWeekRail from '~/components/calendar/CalendarWeekRail.vue'
import MonthNotesPanel from '~/components/calendar/MonthNotesPanel.vue'
import MonthGrid from '~/components/calendar/MonthGrid.vue'
import WeekView from '~/components/calendar/WeekView.vue'
import DayDrawer from '~/components/calendar/DayDrawer.vue'
import CalendarEventForm from '~/components/calendar/CalendarEventForm.vue'
import CalendarConfigDrawer from '~/components/calendar/config/CalendarConfigDrawer.vue'
import { useCalendarChat } from '~/composables/useCalendarChat'
import { useCalendarShortcuts } from '~/composables/useCalendarShortcuts'
import { useCalendarStore } from '~/stores/calendar'
import { useUiStore } from '~/stores/ui'
import { useCalendarLiveSync } from '~/composables/useCalendarLiveSync'
import {
  formatMonthTitle,
  monthKeyOf,
  todayKey,
  weekdayLabels,
  type CalendarEvent,
  type CalendarEventInput,
  type CalendarView,
} from '~/utils/calendar'
import {
  hasTaskSpanInMonth,
  taskSpansFrom,
  type CalendarTaskSpan,
} from '~/utils/calendar-task-spans'
// Store de tasks (layer): fonte das BARRAS multi-dia (precedente de import cross-layer:
// useCalendarChat). So carrega o board configurado; sem board = sem barras.
import { useTasksStore } from '../../../layers/tasks/stores/tasks'
import { useCoreAccountStore } from '../../../layers/core/stores/account'

definePageMeta({
  layout: 'dashboard',
  // Duas camadas de gating, iguais aos demais modulos (tasks/crm/meta-ads):
  //  - workspaceId 'calendar' → gate de PAPEL no auth.global.ts: papel sem o
  //    workspace volta pra auth.homePath (ex.: /operacao), como todo modulo.
  //  - MODULE_PATH_GUARDS (/calendario → 'calendar') no module-enabled.global.ts
  //    → gate de MODULO por conta (core.account_modules), espelha o back
  //    (/v1/calendar). Conta sem o modulo cai no fallback seguro (/perfil).
  workspaceId: 'calendar',
})

const store = useCalendarStore()
const ui = useUiStore()
const accountStore = useCoreAccountStore()
const canConfigureCalendar = computed(() => Boolean(accountStore.activeAccount?.isAgency))
// Chat singleton: permanece disponivel pelos gatilhos dedicados fora da barra compacta.
const chat = useCalendarChat()

// Coluna esquerda (anotacoes) minimizavel -> vira um sidebar SLIM (mes vertical + setas + clicar
// para reabrir). Persistido em localStorage. Libera espaco para o calendario quando nao esta usando.
const LEFT_MIN_KEY = 'omni.calendar.leftcol.min'
const leftMinimized = ref(false)
function toggleLeftMin(): void {
  leftMinimized.value = !leftMinimized.value
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(LEFT_MIN_KEY, leftMinimized.value ? '1' : '0')
  }
}

// Barras multi-dia (WAVE 11): tasks do board configurado com inicio->fim atravessando dias
// viram barra continua na grade (estilo Google). Toggle mostrar/ocultar persistido.
const SPANS_KEY = 'omni.calendar.spans.show'
const showTaskSpans = ref(true)
function toggleTaskSpans(): void {
  showTaskSpans.value = !showTaskSpans.value
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(SPANS_KEY, showTaskSpans.value ? '1' : '0')
  }
}
const tasksStore = useTasksStore()
const spansBoardId = computed(() => String(store.config.tasks?.boardId || ''))
// Carrega as tasks do board configurado para decidir se o toggle deve aparecer no mes.
watch(
  spansBoardId,
  async (boardId) => {
    if (!boardId) return
    await tasksStore.initialize({ allowAutoCreate: false }).catch(() => undefined)
    await tasksStore.ensureBoardTasksLoaded(boardId).catch(() => undefined)
  },
  { immediate: true },
)
const availableTaskSpans = computed<CalendarTaskSpan[]>(() => {
  if (!spansBoardId.value) return []
  const boardTasks = tasksStore.tasks.filter((t) => t.projectId === spansBoardId.value)
  const spans = taskSpansFrom(boardTasks)
  // Respeita o filtro de cliente da tela (mesma regra dos eventos).
  if (!effectiveClientId.value) return spans
  return spans.filter((s) => !s.clientId || s.clientId === effectiveClientId.value)
})
const hasTaskSpansInFocusedMonth = computed(() => {
  return hasTaskSpanInMonth(availableTaskSpans.value, focusMonthKey.value)
})
const taskSpans = computed<CalendarTaskSpan[]>(() =>
  showTaskSpans.value ? availableTaskSpans.value : [],
)
// Clique na barra abre o CARD da task no board (deep-link da WAVE 5, item 4).
function onSelectSpan(span: CalendarTaskSpan): void {
  void navigateTo({ path: '/tasks', query: { board: spansBoardId.value, task: span.id } })
}

// Atalhos de teclado da PAGINA (WAVE 11; mapa configuravel em config.shortcuts, aba
// Aparencia). Os do chat (gravar/parar/fechar) vivem no CalendarChatPanel.
useCalendarShortcuts([
  { action: 'calToday', handler: () => onToday() },
  { action: 'calMonthView', handler: () => onSetView('month') },
  { action: 'calWeekView', handler: () => onSetView('week') },
  { action: 'calNewItem', handler: () => onNew() },
  { action: 'calNotesSidebar', handler: () => toggleLeftMin() },
  { action: 'calSpans', handler: () => toggleTaskSpans() },
  { action: 'calPrev', handler: () => onPrev() },
  { action: 'calNext', handler: () => onNext() },
  { action: 'chatOpen', handler: () => chat.togglePanel() },
])
const {
  view,
  weekStartsOn,
  selectedClientId,
  canSelectClient,
  effectiveClientId,
  clients,
  clientsById,
  peopleById,
  config,
  focusMonthKey,
  focusWeekKey,
  currentMonthKey,
  currentWeekKey,
  renderedMonthKeys,
  renderedWeekKeys,
  weeksOfFocusedMonth,
  railActiveIndex,
  currentRailIndex,
  eventsByDate,
  holidaysByDate,
  selectedEvents,
  selectedDate,
  drawerOpen,
  periodTitle,
  activeNotes,
  activeNotesMonthKey,
} = storeToRefs(store)

const scrollContainer = ref<HTMLElement | null>(null)
const EXTEND_THRESHOLD = 800

let rafPending = false
let extending = false
let suppressFocusUpdate = false
let suppressTimer = 0

const weekdays = computed(() => weekdayLabels(weekStartsOn.value))
const clientNames = computed(() => clients.value.map((client) => client.name))
const peopleNames = computed(() => store.people.map((person) => person.name))
const peopleList = computed(() => store.people)
const notesTitle = computed(() => formatMonthTitle(activeNotesMonthKey.value))
// Overrides de cor por tipo (config); vazio = herda a cor do cliente no chip.
const typeColors = computed(() => config.value.typeColors || {})

// Formulario de criar/editar evento
const formOpen = ref(false)
const editingEvent = ref<CalendarEvent | null>(null)
const formDate = ref('')

// Drawer de configuracao (SPEC-F6): abre SEM sair do calendario (estado local).
// Deep-link ?config=<aba> abre direto na aba (o drawer le/escreve a query).
const configOpen = ref(false)
const route = useRoute()

// Abre o drawer sempre que ha ?config na URL: cobre o mount direto, o redirect de
// /calendario/config e a navegacao in-app (ex.: link "configurar" do modal de IA
// -> ?config=ia). Nunca FECHA por aqui (fechar e acao do usuario no drawer, que
// tira o ?config da URL); a aba certa e resolvida pelo proprio drawer via query.
watch(
  [() => route.query.config, canConfigureCalendar],
  ([value, canConfigure]) => {
    if (typeof value === 'string' && value && canConfigure) configOpen.value = true
    if (!canConfigure) configOpen.value = false
  },
  { immediate: true },
)

// Realtime + presenca + conflito 409 (SPEC-F9 / contratos C11-C12): wiring extraido para
// composable dedicado (mantem esta pagina < 450 linhas). 2 abas na mesma conta refletem
// create/edit/delete sem F5, indicam quem edita notas/evento e barram edicao concorrente.
const {
  presenceParticipants,
  notesPresenceLabel,
  eventPresenceLabel,
  onNotesFocus,
  onNotesBlur,
  handleEventConflict,
} = useCalendarLiveSync({ formOpen, editingEvent, formDate })

// Centraliza o bloco em foco (mes OU semana) no meio; o snap encaixa nele.
function scrollToFocus(smooth = false): void {
  const container = scrollContainer.value
  if (!container) return
  const focusEl = container.querySelector<HTMLElement>('[data-focus="true"]')
  if (!focusEl) return
  const cRect = container.getBoundingClientRect()
  const fRect = focusEl.getBoundingClientRect()
  const delta = fRect.top + fRect.height / 2 - (cRect.top + cRect.height / 2)
  container.scrollTo({
    top: Math.max(0, container.scrollTop + delta),
    behavior: smooth ? 'smooth' : 'auto',
  })
}

function centerFocus(smooth = true): void {
  suppressFocusUpdate = true
  if (suppressTimer) window.clearTimeout(suppressTimer)
  nextTick(() => {
    scrollToFocus(smooth)
    suppressTimer = window.setTimeout(
      () => {
        suppressFocusUpdate = false
      },
      smooth ? 650 : 80,
    )
  })
}

// Foco segue o scroll: o bloco (mes/semana) mais proximo do centro vira o foco.
function updateFocusFromScroll(container: HTMLElement): void {
  const blocks = container.querySelectorAll<HTMLElement>('[data-block-key]')
  const containerCenter = container.getBoundingClientRect().top + container.clientHeight / 2
  let bestKey = ''
  let bestDist = Number.POSITIVE_INFINITY
  for (const block of blocks) {
    const rect = block.getBoundingClientRect()
    const dist = Math.abs(rect.top + rect.height / 2 - containerCenter)
    if (dist < bestDist) {
      bestDist = dist
      bestKey = block.dataset.blockKey || ''
    }
  }
  if (!bestKey) return
  if (view.value === 'month') store.setFocusMonth(bestKey)
  else store.setFocusWeek(bestKey)
}

function maybeExtend(container: HTMLElement): void {
  if (extending) return
  const nearTop = container.scrollTop < EXTEND_THRESHOLD
  const nearBottom =
    container.scrollHeight - container.scrollTop - container.clientHeight < EXTEND_THRESHOLD
  const isMonth = view.value === 'month'

  if (nearTop) {
    extending = true
    const prevHeight = container.scrollHeight
    if (isMonth) store.prependMonths()
    else store.prependWeeks()
    nextTick(() => {
      container.scrollTop += container.scrollHeight - prevHeight
      extending = false
    })
  } else if (nearBottom) {
    extending = true
    if (isMonth) store.appendMonths()
    else store.appendWeeks()
    nextTick(() => {
      extending = false
    })
  }
}

function onScroll(): void {
  if (rafPending) return
  rafPending = true
  requestAnimationFrame(() => {
    rafPending = false
    const container = scrollContainer.value
    if (!container) return
    maybeExtend(container)
    if (!suppressFocusUpdate) updateFocusFromScroll(container)
  })
}

function onPrev(): void {
  store.goPrev()
  centerFocus(true)
}

function onNext(): void {
  store.goNext()
  centerFocus(true)
}

function onToday(): void {
  store.goToday()
  centerFocus(false)
}

function onSetView(next: CalendarView): void {
  store.setView(next)
  centerFocus(false)
}

function onSelectWeek(index: number): void {
  store.selectWeek(index) // entra na visao Semana e foca aquela semana
  centerFocus(false)
}

function onSelectDay(dateKey: string): void {
  store.selectDay(dateKey)
  centerFocus(true)
}

function onSelectEvent(event: CalendarEvent): void {
  onSelectDay(event.date)
}

function onNew(): void {
  editingEvent.value = null
  formDate.value = selectedDate.value || todayKey()
  formOpen.value = true
}

function onEditEvent(event: CalendarEvent): void {
  editingEvent.value = event
  formDate.value = event.date
  formOpen.value = true
}

// SEARCH (WAVE 14): clicar num resultado da busca navega ao mes/dia do item e ABRE o modal de
// edicao. Foca o mes (para o item entrar na janela renderizada) + seleciona o dia (abre o drawer)
// + abre o modal do evento. A busca vem de uma janela ampla no back, entao o item pode ser de
// outro mes que ainda nao estava carregado — setFocusMonth dispara o fetch da janela nova.
function onSearchOpen(event: CalendarEvent): void {
  store.setFocusMonth(monthKeyOf(event.date))
  onSelectDay(event.date)
  editingEvent.value = event
  formDate.value = event.date
  formOpen.value = true
}

// Engrenagem: abre o drawer de configuracao SEM sair do calendario. SPEC-F6.
function onConfig(): void {
  if (!canConfigureCalendar.value) return
  configOpen.value = true
}

async function onSubmitForm(input: CalendarEventInput): Promise<void> {
  const editing = editingEvent.value
  if (editing) {
    // C12: envia a version que ESTE form carregou (editing.version), nao a atual do store
    // (o realtime pode te-la atualizado). updateEvent devolve 'ok'|'conflict'|'error'.
    const outcome = await store.updateEvent(editing.id, input, editing.version)
    if (outcome === 'ok') {
      formOpen.value = false
      ui.success('Item atualizado.')
    } else if (outcome === 'conflict') {
      await handleEventConflict(editing.id)
    } else {
      ui.error('Não foi possível salvar o item.')
    }
    return
  }
  const ok = await store.createEvent(input)
  if (ok) {
    formOpen.value = false
    ui.success('Item criado.')
  } else {
    ui.error('Não foi possível salvar o item.')
  }
}

async function onRemoveEvent(id: string): Promise<void> {
  const ev = store.getEventById(id)
  // 1) Confirma a exclusao do evento.
  const del = await ui.confirm({
    title: 'Excluir item',
    message: `Excluir "${ev?.title || 'este item'}" do calendário?`,
    confirmLabel: 'Excluir',
    cancelLabel: 'Cancelar',
  })
  if (!del?.confirmed) return
  // 2) Politica "perguntar na hora": se tem task vinculada, pergunta se arquiva a task tambem.
  let archiveTask = false
  if (ev?.taskId) {
    const both = await ui.confirm({
      title: 'Task vinculada',
      message: 'Este item tem uma task no board. Arquivar a task também?',
      confirmLabel: 'Arquivar a task também',
      cancelLabel: 'Manter a task',
    })
    archiveTask = Boolean(both?.confirmed)
  }
  const ok = await store.deleteEvent(id, archiveTask)
  if (ok) {
    formOpen.value = false
    ui.success(archiveTask ? 'Item e task excluídos.' : 'Item excluído.')
  } else {
    ui.error('Não foi possível excluir o item.')
  }
}

onMounted(() => {
  if (typeof localStorage !== 'undefined') {
    leftMinimized.value = localStorage.getItem(LEFT_MIN_KEY) === '1'
    showTaskSpans.value = localStorage.getItem(SPANS_KEY) !== '0'
  }
  const first = store.init()
  // Ao (RE)entrar na pagina, refetcha a janela SEMPRE (menos no 1o load, que o init ja faz): pega
  // mudancas feitas com o calendario fechado — ex.: mexer na data/responsavel de uma TASK no board
  // sincroniza o evento-espelho no back, mas o WS do calendario so entrega com a pagina aberta.
  // Sem isto, voltar mostrava estado velho ("sumiu"/"precisa recarregar"). Cobre navegacao SPA.
  if (first === false) void store.refetchWindow()
  centerFocus(false)
})

// keepalive (se a rota for cacheada): onMounted nao re-dispara; onActivated cobre a re-entrada.
onActivated(() => {
  if (store.isInitialized) void store.refetchWindow()
})

onBeforeUnmount(() => {
  if (suppressTimer) window.clearTimeout(suppressTimer)
})
</script>

<template>
  <div class="calendar-page">
    <div class="calendar-body" :class="{ 'calendar-body--drawer': drawerOpen }">
      <CalendarWeekRail
        :weeks="weeksOfFocusedMonth"
        :view="view"
        :focused-week-index="railActiveIndex"
        :current-week-index="currentRailIndex"
        @select="onSelectWeek"
        @month="() => onSetView('month')"
      />

      <div class="calendar-leftcol" :class="{ 'calendar-leftcol--min': leftMinimized }">
        <!-- MINIMIZADA: sidebar slim (expandir + setas de mes + nome do mes vertical clicavel). -->
        <template v-if="leftMinimized">
          <button
            type="button"
            class="calendar-leftcol__expand"
            aria-label="Expandir anotações"
            title="Expandir as anotações"
            @click="toggleLeftMin"
          >
            <UIcon name="i-lucide-panel-left-open" aria-hidden="true" />
          </button>
          <button
            type="button"
            class="calendar-leftcol__mini-arrow"
            :aria-label="view === 'week' ? 'Semana anterior' : 'Mês anterior'"
            @click="onPrev"
          >
            <UIcon name="i-lucide-chevron-up" aria-hidden="true" />
          </button>
          <button
            type="button"
            class="calendar-leftcol__mini-month"
            title="Abrir anotações do mês"
            @click="toggleLeftMin"
          >
            {{ periodTitle }}
          </button>
          <button
            type="button"
            class="calendar-leftcol__mini-arrow"
            :aria-label="view === 'week' ? 'Próxima semana' : 'Próximo mês'"
            @click="onNext"
          >
            <UIcon name="i-lucide-chevron-down" aria-hidden="true" />
          </button>
        </template>

        <!-- EXPANDIDA: setas + controles (com botao minimizar) + editor de anotacoes. -->
        <template v-else>
          <button
            type="button"
            class="calendar-leftcol__arrow calendar-leftcol__arrow--prev"
            :aria-label="view === 'week' ? 'Semana anterior' : 'Mês anterior'"
            @click="onPrev"
          >
            <UIcon name="i-lucide-chevron-left" aria-hidden="true" />
          </button>
          <button
            type="button"
            class="calendar-leftcol__arrow calendar-leftcol__arrow--next"
            :aria-label="view === 'week' ? 'Próxima semana' : 'Próximo mês'"
            @click="onNext"
          >
            <UIcon name="i-lucide-chevron-right" aria-hidden="true" />
          </button>

          <CalendarControls
            :period-title="periodTitle"
            :clients="clients"
            :selected-client-id="selectedClientId"
            :can-select-client="canSelectClient"
            :can-configure="canConfigureCalendar"
            :view="view"
            :participants="presenceParticipants"
            :show-spans="showTaskSpans"
            :has-spans="hasTaskSpansInFocusedMonth"
            @today="onToday"
            @update:client="store.setClientFilter"
            @update:view="onSetView"
            @new-item="onNew"
            @config="onConfig"
            @minimize="toggleLeftMin"
            @toggle-spans="toggleTaskSpans"
          >
            <template #search>
              <CalendarSearch @open="onSearchOpen" />
            </template>
          </CalendarControls>
          <MonthNotesPanel
            :title="notesTitle"
            :model-value="activeNotes"
            :people-names="peopleNames"
            :client-names="clientNames"
            :editing-label="notesPresenceLabel"
            @update:model-value="store.setNotesForActiveMonth"
            @focus="onNotesFocus"
            @blur="onNotesBlur"
          />
        </template>
      </div>

      <div ref="scrollContainer" class="calendar-scroll" @scroll.passive="onScroll">
        <template v-if="view === 'month'">
          <MonthGrid
            v-for="monthKey in renderedMonthKeys"
            :key="monthKey"
            :month-key="monthKey"
            :week-starts-on="weekStartsOn"
            :weekdays="weekdays"
            :events-by-date="eventsByDate"
            :holidays-by-date="holidaysByDate"
            :clients-by-id="clientsById"
            :task-spans="taskSpans"
            :type-colors="typeColors"
            :is-focus="monthKey === focusMonthKey"
            :is-current="monthKey === currentMonthKey"
            :selected-date="selectedDate"
            @select-day="onSelectDay"
            @select-event="onSelectEvent"
            @select-span="onSelectSpan"
          />
        </template>
        <template v-else>
          <WeekView
            v-for="weekKey in renderedWeekKeys"
            :key="weekKey"
            :week-start-key="weekKey"
            :weekdays="weekdays"
            :events-by-date="eventsByDate"
            :holidays-by-date="holidaysByDate"
            :clients-by-id="clientsById"
            :type-colors="typeColors"
            :is-focus="weekKey === focusWeekKey"
            :is-current="weekKey === currentWeekKey"
            :selected-date="selectedDate"
            @select-day="onSelectDay"
            @select-event="onSelectEvent"
          />
        </template>
      </div>

      <DayDrawer
        v-if="drawerOpen"
        class="calendar-body__drawer"
        :date-key="selectedDate"
        :events="selectedEvents"
        :clients-by-id="clientsById"
        :people-by-id="peopleById"
        @close="store.closeDrawer"
        @new-item="onNew"
        @edit="onEditEvent"
        @remove="onRemoveEvent"
      />
    </div>

    <CalendarEventForm
      :open="formOpen"
      :event="editingEvent"
      :default-date="formDate"
      :clients="clients"
      :people="peopleList"
      :editing-label="eventPresenceLabel"
      @submit="onSubmitForm"
      @cancel="formOpen = false"
      @remove="onRemoveEvent"
    />

    <CalendarConfigDrawer v-model:open="configOpen" />
  </div>
</template>
