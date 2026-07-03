<script setup lang="ts">
import { computed } from 'vue'
import DayCell from '~/components/calendar/DayCell.vue'
import {
  buildMonthMatrix,
  dayBackgroundUrls,
  formatMonthTitle,
  type CalendarClient,
  type CalendarEvent,
  type CalendarHoliday,
  type CalendarMediaItem,
  type WeekStart,
} from '~/utils/calendar'

const props = defineProps<{
  monthKey: string
  weekStartsOn: WeekStart
  weekdays: string[]
  eventsByDate: Map<string, CalendarEvent[]>
  holidaysByDate: Map<string, CalendarHoliday[]>
  dayMediaByDate: Map<string, CalendarMediaItem[]>
  clientsById: Map<string, CalendarClient>
  /** Override de cor por tipo (config). Desce ate o EventChip. */
  typeColors?: Record<string, string>
  isFocus: boolean
  isCurrent: boolean
  selectedDate: string
}>()

const emit = defineEmits<{
  'select-day': [dateKey: string]
  'select-event': [event: CalendarEvent]
}>()

const title = computed(() => formatMonthTitle(props.monthKey))
const days = computed(() => buildMonthMatrix(props.monthKey, props.weekStartsOn).flat())

function eventsFor(dateKey: string): CalendarEvent[] {
  return props.eventsByDate.get(dateKey) || []
}

function holidaysFor(dateKey: string): CalendarHoliday[] {
  return props.holidaysByDate.get(dateKey) || []
}

// Fundo do dia: usa os eventos ja filtrados por cliente (eventsByDate) + anexos
// avulsos. Helper puro em utils/calendar (mesma regra na visao Semana).
function bgUrlsFor(dateKey: string): string[] {
  return dayBackgroundUrls(eventsFor(dateKey), props.dayMediaByDate.get(dateKey) || [])
}
</script>

<template>
  <section
    class="calendar-month calendar-snap"
    :class="{
      'calendar-month--focus': isFocus,
      'calendar-month--peek': !isFocus,
      'calendar-month--current': isCurrent,
    }"
    :data-block-key="monthKey"
    :data-focus="isFocus ? 'true' : 'false'"
  >
    <header class="calendar-month__header">
      <h3 class="calendar-month__title">{{ title }}</h3>
      <span v-if="isCurrent" class="calendar-month__tag">Mês atual</span>
    </header>

    <div class="calendar-grid">
      <div class="calendar-grid__weekdays">
        <span v-for="label in weekdays" :key="label" class="calendar-grid__weekday">
          {{ label }}
        </span>
      </div>
      <div class="calendar-grid__days" role="grid" :aria-label="title">
        <DayCell
          v-for="cell in days"
          :key="cell.dateKey"
          :day="cell"
          :events="eventsFor(cell.dateKey)"
          :holidays="holidaysFor(cell.dateKey)"
          :bg-urls="bgUrlsFor(cell.dateKey)"
          :clients-by-id="clientsById"
          :type-colors="typeColors || {}"
          :max-chips="2"
          :selected="cell.dateKey === selectedDate"
          @select-day="emit('select-day', $event)"
          @select-event="emit('select-event', $event)"
        />
      </div>
    </div>
  </section>
</template>
