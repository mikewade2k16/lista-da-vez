<script setup lang="ts">
import { computed } from 'vue'
import EventChip from '~/components/calendar/EventChip.vue'
import {
  todayKey,
  type CalendarClient,
  type CalendarEvent,
  type CalendarHoliday,
  type DayCellModel,
} from '~/utils/calendar'

const props = withDefaults(
  defineProps<{
    day: DayCellModel
    events: CalendarEvent[]
    clientsById: Map<string, CalendarClient>
    holidays?: CalendarHoliday[]
    maxChips?: number
    selected?: boolean
    dense?: boolean
  }>(),
  { holidays: () => [], maxChips: 2, selected: false, dense: false },
)

const emit = defineEmits<{
  'select-day': [dateKey: string]
  'select-event': [event: CalendarEvent]
}>()

const isToday = computed(() => props.day.dateKey === todayKey())
const visibleChips = computed(() => props.events.slice(0, props.maxChips))
const overflow = computed(() => Math.max(0, props.events.length - props.maxChips))
</script>

<template>
  <div
    class="calendar-cell"
    :class="{
      'calendar-cell--outside': !day.inMonth,
      'calendar-cell--today': isToday,
      'calendar-cell--selected': selected,
      'calendar-cell--week': dense,
    }"
    role="gridcell"
    :aria-selected="selected ? 'true' : 'false'"
    @click="emit('select-day', day.dateKey)"
  >
    <div class="calendar-cell__head">
      <span class="calendar-cell__num" :class="{ 'calendar-cell__num--today': isToday }">
        {{ day.day }}
      </span>
      <span v-if="isToday" class="calendar-cell__today">HOJE</span>
    </div>

    <div v-if="holidays.length" class="calendar-cell__holidays">
      <span
        v-for="holiday in holidays"
        :key="holiday.name"
        class="calendar-cell__holiday"
        :title="holiday.name"
      >
        {{ holiday.name }}
      </span>
    </div>

    <div class="calendar-cell__chips">
      <EventChip
        v-for="event in visibleChips"
        :key="event.id"
        :event="event"
        :client="clientsById.get(event.clientId)"
        @select="emit('select-event', $event)"
      />
      <button
        v-if="overflow > 0"
        type="button"
        class="calendar-cell__more"
        @click.stop="emit('select-day', day.dateKey)"
      >
        +{{ overflow }} mais
      </button>
    </div>
  </div>
</template>
