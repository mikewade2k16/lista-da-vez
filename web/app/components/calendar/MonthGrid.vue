<script setup lang="ts">
import { computed } from 'vue'
import DayCell from '~/components/calendar/DayCell.vue'
import {
  NEUTRAL_COLOR,
  buildMonthMatrix,
  dayBackgroundUrls,
  formatMonthTitle,
  rgba,
  type CalendarClient,
  type CalendarEvent,
  type CalendarHoliday,
  type CalendarMediaItem,
  type WeekStart,
} from '~/utils/calendar'
import {
  weekSpanSegments,
  type CalendarSpanSegment,
  type CalendarTaskSpan,
} from '~/utils/calendar-task-spans'

const props = defineProps<{
  monthKey: string
  weekStartsOn: WeekStart
  weekdays: string[]
  eventsByDate: Map<string, CalendarEvent[]>
  holidaysByDate: Map<string, CalendarHoliday[]>
  dayMediaByDate: Map<string, CalendarMediaItem[]>
  clientsById: Map<string, CalendarClient>
  /** Barras multi-dia (WAVE 11): tasks com inicio->fim atravessando dias. Vazio = nada. */
  taskSpans?: CalendarTaskSpan[]
  /** Override de cor por tipo (config). Desce ate o EventChip. */
  typeColors?: Record<string, string>
  isFocus: boolean
  isCurrent: boolean
  selectedDate: string
}>()

const emit = defineEmits<{
  'select-day': [dateKey: string]
  'select-event': [event: CalendarEvent]
  'select-span': [span: CalendarTaskSpan]
}>()

const title = computed(() => formatMonthTitle(props.monthKey))
const weeks = computed(() => buildMonthMatrix(props.monthKey, props.weekStartsOn))
const days = computed(() => weeks.value.flat())

// Posicao EXPLICITA de cada celula no grid (linha da semana + coluna do dia): permite
// que as BARRAS multi-dia entrem no MESMO grid (mesma row, colunas start/end) sem
// desalinhar o auto-placement.
function cellStyle(index: number): Record<string, string> {
  return { gridRow: String(Math.floor(index / 7) + 1), gridColumn: String((index % 7) + 1) }
}

// Segmentos das barras por semana (lanes calculadas no util; teto MAX_SPAN_LANES).
const spanSegments = computed<{ row: number; seg: CalendarSpanSegment }[]>(() => {
  const spans = props.taskSpans || []
  if (!spans.length) return []
  const out: { row: number; seg: CalendarSpanSegment }[] = []
  weeks.value.forEach((week, weekIdx) => {
    const keys = week.map((cell) => cell.dateKey)
    for (const seg of weekSpanSegments(keys, spans)) {
      out.push({ row: weekIdx + 1, seg })
    }
  })
  return out
})

function spanStyle(row: number, seg: CalendarSpanSegment): Record<string, string> {
  const client = props.clientsById.get(seg.span.clientId)
  const color = client?.color ?? NEUTRAL_COLOR
  return {
    gridRow: String(row),
    gridColumn: `${seg.colStart} / ${seg.colEnd + 1}`,
    background: rgba(color, 0.85),
    // Lanes empilham de baixo para cima dentro da celula (align-self: end + margem).
    marginBottom: `${0.35 + seg.lane * 1.15}rem`,
  }
}

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
          v-for="(cell, index) in days"
          :key="cell.dateKey"
          :style="cellStyle(index)"
          :day="cell"
          :events="eventsFor(cell.dateKey)"
          :holidays="holidaysFor(cell.dateKey)"
          :bg-urls="bgUrlsFor(cell.dateKey)"
          :clients-by-id="clientsById"
          :type-colors="typeColors || {}"
          :max-chips="0"
          :selected="cell.dateKey === selectedDate"
          @select-day="emit('select-day', $event)"
          @select-event="emit('select-event', $event)"
        />
        <!-- Barras multi-dia (WAVE 11): mesma linha da semana, colunas start..end. -->
        <button
          v-for="({ row, seg }, idx) in spanSegments"
          :key="`${seg.span.id}-${row}-${idx}`"
          type="button"
          class="calendar-span-bar"
          :class="{ 'is-start': seg.startsHere, 'is-end': seg.endsHere }"
          :style="spanStyle(row, seg)"
          :title="`${seg.span.title} (tarefa com início e fim)`"
          @click.stop="emit('select-span', seg.span)"
        >
          <span v-if="seg.startsHere" class="calendar-span-bar__label">{{ seg.span.title }}</span>
        </button>
      </div>
    </div>
  </section>
</template>
