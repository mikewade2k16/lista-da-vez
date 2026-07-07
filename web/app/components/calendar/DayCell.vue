<script setup lang="ts">
import { computed } from 'vue'
import EventChip from '~/components/calendar/EventChip.vue'
import { getApiBase } from '~/utils/api-client'
import { resolveMediaUrl } from '~/utils/media'
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
    /** URLs de midia usadas como fundo do dia (ate 4). Vazio = sem fundo. */
    bgUrls?: string[]
    /** Override de cor por tipo (config); `{ [type]: '#rrggbb' }`. Vazio = cor do cliente. */
    typeColors?: Record<string, string>
    maxChips?: number
    selected?: boolean
    dense?: boolean
  }>(),
  {
    holidays: () => [],
    bgUrls: () => [],
    typeColors: () => ({}),
    maxChips: 2,
    selected: false,
    dense: false,
  },
)

const emit = defineEmits<{
  'select-day': [dateKey: string]
  'select-event': [event: CalendarEvent]
}>()

const isToday = computed(() => props.day.dateKey === todayKey())
// maxChips <= 0 => SEM cap: mostra TODOS os itens do dia (a celula/linha do mes cresce e "vai
// diagramando"; decisao do dono). > 0 mantem o corte + "+N mais" (usado no denso da semana).
const visibleChips = computed(() =>
  props.maxChips > 0 ? props.events.slice(0, props.maxChips) : props.events,
)
const overflow = computed(() =>
  props.maxChips > 0 ? Math.max(0, props.events.length - props.maxChips) : 0,
)
// Limita a 4 tiles; o layout da grade e' escolhido por data-count no CSS.
// resolveMediaUrl absolutiza /uploads/* para a apiBase (dev roda web :3003 e
// api :9091 em portas diferentes; url relativa cairia no host errado).
const apiBase = getApiBase(useRuntimeConfig())
const tiles = computed(() => props.bgUrls.slice(0, 4).map((url) => resolveMediaUrl(url, apiBase)))
</script>

<template>
  <div
    class="calendar-cell"
    :class="{
      'calendar-cell--outside': !day.inMonth,
      'calendar-cell--today': isToday,
      'calendar-cell--selected': selected,
      'calendar-cell--week': dense,
      'calendar-cell--has-bg': tiles.length > 0,
    }"
    role="gridcell"
    :aria-selected="selected ? 'true' : 'false'"
    @click="emit('select-day', day.dateKey)"
  >
    <div
      v-if="tiles.length"
      class="calendar-cell__bg"
      :data-count="tiles.length"
      aria-hidden="true"
    >
      <span
        v-for="(url, index) in tiles"
        :key="index"
        class="calendar-cell__bg-tile"
        :style="{ backgroundImage: `url(${url})` }"
      ></span>
    </div>

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
        :type-color="typeColors[event.type]"
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
