<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import CalendarControls from '~/components/calendar/CalendarControls.vue'
import CalendarWeekRail from '~/components/calendar/CalendarWeekRail.vue'
import MonthNotesPanel from '~/components/calendar/MonthNotesPanel.vue'
import MonthGrid from '~/components/calendar/MonthGrid.vue'
import WeekView from '~/components/calendar/WeekView.vue'
import DayDrawer from '~/components/calendar/DayDrawer.vue'
import CalendarEventForm from '~/components/calendar/CalendarEventForm.vue'
import CalendarConfigModal from '~/components/calendar/CalendarConfigModal.vue'
import { useCalendarStore } from '~/stores/calendar'
import { useUiStore } from '~/stores/ui'
import {
  formatMonthTitle,
  todayKey,
  weekdayLabels,
  type CalendarEvent,
  type CalendarEventInput,
  type CalendarView,
} from '~/utils/calendar'

definePageMeta({
  layout: 'dashboard',
  // Tela global (sem modulo): workspaceId vazio evita o gate de workspace do
  // auth.global.ts (igual /perfil). Preview front; o gating real entra no back.
  workspaceId: '',
})

const store = useCalendarStore()
const ui = useUiStore()
const {
  view,
  weekStartsOn,
  selectedClientId,
  clients,
  clientsById,
  peopleById,
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

// Formulario de criar/editar evento
const formOpen = ref(false)
const editingEvent = ref<CalendarEvent | null>(null)
const formDate = ref('')

// Modal de configuracao (responsaveis + feriados)
const configOpen = ref(false)

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

async function onSubmitForm(input: CalendarEventInput): Promise<void> {
  const editing = editingEvent.value
  const ok = editing ? await store.updateEvent(editing.id, input) : await store.createEvent(input)
  if (ok) {
    formOpen.value = false
    ui.success(editing ? 'Item atualizado.' : 'Item criado.')
  } else {
    ui.error('Não foi possível salvar o item.')
  }
}

async function onRemoveEvent(id: string): Promise<void> {
  const ok = await store.deleteEvent(id)
  if (ok) {
    formOpen.value = false
    ui.success('Item excluído.')
  } else {
    ui.error('Não foi possível excluir o item.')
  }
}

onMounted(() => {
  store.init()
  centerFocus(false)
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

      <div class="calendar-leftcol">
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
          :view="view"
          @today="onToday"
          @update:client="store.setClientFilter"
          @update:view="onSetView"
          @new-item="onNew"
          @config="configOpen = true"
        />
        <MonthNotesPanel
          :title="notesTitle"
          :model-value="activeNotes"
          :people-names="peopleNames"
          :client-names="clientNames"
          @update:model-value="store.setNotesForActiveMonth"
        />
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
            :is-focus="monthKey === focusMonthKey"
            :is-current="monthKey === currentMonthKey"
            :selected-date="selectedDate"
            @select-day="onSelectDay"
            @select-event="onSelectEvent"
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
      @submit="onSubmitForm"
      @cancel="formOpen = false"
      @remove="onRemoveEvent"
    />

    <CalendarConfigModal :open="configOpen" @close="configOpen = false" />
  </div>
</template>
