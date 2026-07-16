<script setup lang="ts">
import { computed } from 'vue'
import EventChip from '~/components/calendar/EventChip.vue'
import {
  buildWeekDays,
  dayBackgroundUrls,
  formatWeekRangeTitle,
  todayKey,
  type CalendarClient,
  type CalendarEvent,
  type CalendarHoliday,
} from '~/utils/calendar'

const props = defineProps<{
  weekStartKey: string
  weekdays: string[]
  eventsByDate: Map<string, CalendarEvent[]>
  holidaysByDate: Map<string, CalendarHoliday[]>
  clientsById: Map<string, CalendarClient>
  /** Override de cor por tipo (config). Vazio = cor do cliente. */
  typeColors?: Record<string, string>
  isFocus: boolean
  isCurrent: boolean
  selectedDate: string
}>()

const emit = defineEmits<{
  'select-day': [dateKey: string]
  'select-event': [event: CalendarEvent]
}>()

const today = todayKey()
const title = computed(() => formatWeekRangeTitle(props.weekStartKey))
const days = computed(() => buildWeekDays(props.weekStartKey))

function eventsFor(dateKey: string): CalendarEvent[] {
  return props.eventsByDate.get(dateKey) || []
}

function holidaysFor(dateKey: string): CalendarHoliday[] {
  return props.holidaysByDate.get(dateKey) || []
}

// Fundo do dia (WAVE 13, mesma regra da visao Mes): 1a midia de cada evento filtrado.
function bgUrlsFor(dateKey: string): string[] {
  return dayBackgroundUrls(eventsFor(dateKey)).slice(0, 4)
}
</script>

<template>
  <section
    class="calendar-weekblock calendar-snap"
    :class="{
      'calendar-weekblock--focus': isFocus,
      'calendar-weekblock--peek': !isFocus,
    }"
    :data-block-key="weekStartKey"
    :data-focus="isFocus ? 'true' : 'false'"
  >
    <header class="calendar-weekblock__header">
      <h3 class="calendar-weekblock__title">{{ title }}</h3>
      <span v-if="isCurrent" class="calendar-weekblock__tag">Semana atual</span>
    </header>

    <div class="calendar-weekblock__grid">
      <div
        v-for="(cell, index) in days"
        :key="cell.dateKey"
        class="calendar-weekview__day"
        :class="{
          'calendar-weekview__day--today': cell.dateKey === today,
          'calendar-weekview__day--selected': cell.dateKey === selectedDate,
          'calendar-weekview__day--has-bg': bgUrlsFor(cell.dateKey).length > 0,
        }"
        @click="emit('select-day', cell.dateKey)"
      >
        <div
          v-if="bgUrlsFor(cell.dateKey).length"
          class="calendar-cell__bg"
          :data-count="bgUrlsFor(cell.dateKey).length"
          aria-hidden="true"
        >
          <span
            v-for="(url, i) in bgUrlsFor(cell.dateKey)"
            :key="i"
            class="calendar-cell__bg-tile"
            :style="{ backgroundImage: `url(${url})` }"
          ></span>
        </div>

        <div class="calendar-weekview__day-head">
          <span class="calendar-weekview__weekday">{{ weekdays[index] }}</span>
          <span
            class="calendar-weekview__num"
            :class="{ 'calendar-weekview__num--today': cell.dateKey === today }"
          >
            {{ cell.day }}
          </span>
        </div>

        <div v-if="holidaysFor(cell.dateKey).length" class="calendar-weekview__holidays">
          <span
            v-for="holiday in holidaysFor(cell.dateKey)"
            :key="holiday.name"
            class="calendar-cell__holiday"
            :title="holiday.name"
          >
            {{ holiday.name }}
          </span>
        </div>

        <div class="calendar-weekview__events">
          <EventChip
            v-for="event in eventsFor(cell.dateKey)"
            :key="event.id"
            :event="event"
            :client="clientsById.get(event.clientId)"
            :type-color="(typeColors || {})[event.type]"
            @select="emit('select-event', $event)"
          />
        </div>
      </div>
    </div>
  </section>
</template>
