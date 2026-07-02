<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { CalendarDate } from '@internationalized/date'
import { today, getLocalTimeZone, parseDate } from '@internationalized/date'
import { ChevronRight } from 'lucide-vue-next'

const props = defineProps<{
  modelValue?: string // ISO YYYY-MM-DD or YYYY-MM-DDTHH:mm (24h internally)
  endDate?: string
  open?: boolean // external popover control (used by draft cards for focusout guard)
  placement?: 'bottom' | 'top' | 'left' | 'right'
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  'update:endDate': [value: string]
  'update:open': [value: boolean]
}>()

// ─── Popover ─────────────────────────────────────────────────────────────────
// popoverOpen is authoritative; synced bidirectionally with optional `open` prop.
const popoverOpen = ref(props.open ?? false)
watch(
  () => props.open,
  (v) => {
    if (v !== undefined) popoverOpen.value = v
  },
)
watch(popoverOpen, (v) => emit('update:open', v))

// ─── Calendar state ───────────────────────────────────────────────────────────
const calStart = ref<CalendarDate | undefined>(undefined)
const calEnd = ref<CalendarDate | undefined>(undefined)
const viewPlaceholder = ref<CalendarDate>(today(getLocalTimeZone()))

// ─── Settings state ───────────────────────────────────────────────────────────
const showEndDate = ref(false)
const includeTime = ref(false)
type ViewMode = 'day' | 'month' | 'year'
type TimeTarget = 'start' | 'end'
const viewMode = ref<ViewMode>('day')
const manualStartDate = ref('')
const manualEndDate = ref('')
const manualStartTime = ref('')
const manualEndTime = ref('')
const monthStartValue = ref('')
const monthEndValue = ref('')
const yearStartValue = ref('')
const yearEndValue = ref('')
const startTimeHour = ref('00')
const startTimeMinute = ref('00')
const startTimePeriod = ref<'AM' | 'PM'>('AM')
const endTimeHour = ref('00')
const endTimeMinute = ref('00')
const endTimePeriod = ref<'AM' | 'PM'>('AM')
// dateFormat and timeFormat are shared singletons from useDateFormat —
// changing in one picker updates all other pickers and their trigger labels.
const { dateFormat, timeFormat } = useDateFormat()

const reminder = ref('none')

const reminderOpen = ref(false)
const formatOpen = ref(false)
const timeFormatOpen = ref(false)

function pickDateFormat(value: string) {
  dateFormat.value = value
  formatOpen.value = false
}
function pickTimeFormat(value: string) {
  timeFormat.value = value
  timeFormatOpen.value = false
}
function pickReminder(value: string) {
  reminder.value = value
  reminderOpen.value = false
}

// ─── Options ─────────────────────────────────────────────────────────────────
const REMINDER_OPTIONS = [
  { label: 'Nenhum', value: 'none' },
  { label: '5 minutos antes', value: '5min' },
  { label: '30 minutos antes', value: '30min' },
  { label: '1 hora antes', value: '1h' },
  { label: 'No dia', value: 'day' },
]

const FORMAT_OPTIONS = [
  { label: 'Dia/Mês/Ano', value: 'dmy' },
  { label: 'Mês/Dia/Ano', value: 'mdy' },
  { label: 'Ano/Mês/Dia', value: 'ymd' },
  { label: 'Data completa', value: 'full' },
  { label: 'Data curta', value: 'short' },
  { label: 'Relativo', value: 'rel' },
]

const TIME_FORMAT_OPTIONS = [
  { label: '24 horas', value: '24h' },
  { label: '12 horas', value: '12h' },
]

const VIEW_MODE_OPTIONS: Array<{ label: string; value: ViewMode }> = [
  { label: 'Dia', value: 'day' },
  { label: 'Mes', value: 'month' },
  { label: 'Ano', value: 'year' },
]

const QUICK_RANGE_OPTIONS = [
  { label: 'Hoje', value: 'today' },
  { label: 'Este mes', value: 'this-month' },
  { label: 'Mes passado', value: 'last-month' },
  { label: 'Este ano', value: 'this-year' },
  { label: '3 meses', value: 'last-3-months' },
  { label: '4 meses', value: 'last-4-months' },
  { label: 'Tudo', value: 'all' },
]

const initialDate = new Date()
const initialMonthValue = `${initialDate.getFullYear()}-${String(initialDate.getMonth() + 1).padStart(2, '0')}`
monthStartValue.value = initialMonthValue
monthEndValue.value = initialMonthValue
yearStartValue.value = String(initialDate.getFullYear())
yearEndValue.value = String(initialDate.getFullYear())

// ─── Weekday pt-BR ────────────────────────────────────────────────────────────
// UCalendar does not forward locale to Reka UI's Calendar.Root.
// Single-letter pt-BR labels are forced via manual slot + token mapping.
const WEEKDAY_PT: Record<string, string> = {
  Su: 'D',
  Sun: 'D',
  Mo: 'S',
  Mon: 'S',
  Tu: 'T',
  Tue: 'T',
  We: 'Q',
  Wed: 'Q',
  Th: 'Q',
  Thu: 'Q',
  Fr: 'S',
  Fri: 'S',
  Sa: 'S',
  Sat: 'S',
}
function ptWeekday(day: string) {
  return WEEKDAY_PT[day] ?? day[0].toUpperCase()
}

// ─── ISO ↔ CalendarDate ───────────────────────────────────────────────────────
function isoToCalendar(iso?: string): CalendarDate | undefined {
  if (!iso || iso.length < 10) return undefined
  try {
    return parseDate(iso.slice(0, 10))
  } catch {
    return undefined
  }
}

function calendarFromDate(date: Date): CalendarDate {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return parseDate(`${year}-${month}-${day}`)
}

function calendarFromParts(year: number, month: number, day: number): CalendarDate {
  const monthText = String(month).padStart(2, '0')
  const dayText = String(day).padStart(2, '0')
  return parseDate(`${year}-${monthText}-${dayText}`)
}

function lastDayOfMonth(year: number, month: number): number {
  return new Date(year, month, 0).getDate()
}

function dateInputFromIso(iso?: string): string {
  return iso && iso.length >= 10 ? iso.slice(0, 10) : ''
}

function timeInputFromIso(iso?: string): string {
  return iso && iso.length >= 16 ? iso.slice(11, 16) : ''
}

function monthInputFromCalendar(date?: CalendarDate): string {
  if (!date) return ''
  return `${date.year}-${String(date.month).padStart(2, '0')}`
}

function setTimeFromHHMM(target: TimeTarget, value: string) {
  const match = String(value || '').match(/^(\d{1,2}):(\d{2})/)
  if (!match) return
  const h24 = Math.min(23, Math.max(0, Number(match[1]) || 0))
  const minuteValue = Math.min(59, Math.max(0, Number(match[2]) || 0))
  const { hour, minute, period } = timeRefs(target)
  minute.value = String(minuteValue).padStart(2, '0')
  if (timeFormat.value === '12h') {
    period.value = h24 >= 12 ? 'PM' : 'AM'
    hour.value = String(h24 % 12 || 12)
    return
  }
  hour.value = String(h24).padStart(2, '0')
}

function syncManualFields(target: TimeTarget, iso?: string) {
  const parsed = isoToCalendar(iso)
  if (target === 'start') {
    manualStartDate.value = dateInputFromIso(iso)
    manualStartTime.value = timeInputFromIso(iso)
    monthStartValue.value = monthInputFromCalendar(parsed) || monthStartValue.value
    yearStartValue.value = parsed ? String(parsed.year) : yearStartValue.value
    if (manualStartTime.value) setTimeFromHHMM('start', manualStartTime.value)
    return
  }

  manualEndDate.value = dateInputFromIso(iso)
  manualEndTime.value = timeInputFromIso(iso)
  monthEndValue.value = monthInputFromCalendar(parsed) || monthEndValue.value
  yearEndValue.value = parsed ? String(parsed.year) : yearEndValue.value
  if (manualEndTime.value) setTimeFromHHMM('end', manualEndTime.value)
}

// ─── 24h hour from current input state ───────────────────────────────────────
function timeRefs(target: TimeTarget) {
  return target === 'start'
    ? { hour: startTimeHour, minute: startTimeMinute, period: startTimePeriod }
    : { hour: endTimeHour, minute: endTimeMinute, period: endTimePeriod }
}

function normalizedHour(target: TimeTarget) {
  const { hour } = timeRefs(target)
  const n = Number.parseInt(String(hour.value || '0'), 10)
  const min = timeFormat.value === '12h' ? 1 : 0
  const max = timeFormat.value === '12h' ? 12 : 23
  return String(Math.min(max, Math.max(min, Number.isFinite(n) ? n : min))).padStart(2, '0')
}

function normalizedMinute(target: TimeTarget) {
  const { minute } = timeRefs(target)
  const n = Number.parseInt(String(minute.value || '0'), 10)
  return String(Math.min(59, Math.max(0, Number.isFinite(n) ? n : 0))).padStart(2, '0')
}

function get24hHour(target: TimeTarget = 'start'): string {
  const { period } = timeRefs(target)
  if (timeFormat.value === '24h') return normalizedHour(target)
  let h = Number(normalizedHour(target))
  if (period.value === 'PM' && h !== 12) h += 12
  if (period.value === 'AM' && h === 12) h = 0
  return String(h).padStart(2, '0')
}

function syncTimeFromIso(iso: string | undefined, target: TimeTarget) {
  if (!iso || iso.length < 16) return
  includeTime.value = true
  const h24 = Number(iso.slice(11, 13))
  const { hour, minute, period } = timeRefs(target)
  minute.value = iso.slice(14, 16)
  if (timeFormat.value === '12h') {
    period.value = h24 >= 12 ? 'PM' : 'AM'
    hour.value = String(h24 % 12 || 12)
    return
  }
  hour.value = String(h24).padStart(2, '0')
}

// ─── Sync prop → state ────────────────────────────────────────────────────────
watch(
  () => props.modelValue,
  (val) => {
    const parsed = isoToCalendar(val)
    calStart.value = parsed
    if (parsed) viewPlaceholder.value = parsed
    syncManualFields('start', val)
    syncTimeFromIso(val, 'start')
  },
  { immediate: true },
)

watch(
  () => props.endDate,
  (val) => {
    calEnd.value = isoToCalendar(val)
    if (val) {
      showEndDate.value = true
      syncManualFields('end', val)
      syncTimeFromIso(val, 'end')
    } else {
      syncManualFields('end', '')
    }
  },
  { immediate: true },
)

watch(showEndDate, (enabled) => {
  if (enabled) return
  calEnd.value = undefined
  emit('update:endDate', '')
})

// ─── timeFormat change: convert hour value and clamp range ────────────────────
// Guard: only emit modelValue update from the currently open picker to avoid
// re-emitting unchanged ISO from every mounted instance on the page.
watch(timeFormat, (fmt, prev) => {
  ;(['start', 'end'] as const).forEach((target) => {
    const { hour, period } = timeRefs(target)
    const h = Number(hour.value)
    if (fmt === '12h' && prev === '24h') {
      period.value = h >= 12 ? 'PM' : 'AM'
      hour.value = String(h % 12 || 12)
    } else if (fmt === '24h' && prev === '12h') {
      let h24 = h
      if (period.value === 'PM' && h !== 12) h24 = h + 12
      if (period.value === 'AM' && h === 12) h24 = 0
      hour.value = String(h24).padStart(2, '0')
    }
  })
  if (includeTime.value && popoverOpen.value) {
    if (calStart.value) onTimeChange('start')
    if (calEnd.value) onTimeChange('end')
  }
})

// ─── Computed labels ──────────────────────────────────────────────────────────
const headingLabel = computed(() => {
  const d = new Date(viewPlaceholder.value.year, viewPlaceholder.value.month - 1, 1)
  const str = d.toLocaleDateString('pt-BR', { month: 'long', year: 'numeric' })
  return str.charAt(0).toUpperCase() + str.slice(1)
})

const reminderLabel = computed(
  () => REMINDER_OPTIONS.find((o) => o.value === reminder.value)?.label ?? 'Nenhum',
)
const formatLabel = computed(
  () => FORMAT_OPTIONS.find((o) => o.value === dateFormat.value)?.label ?? 'Dia/Mês/Ano',
)
const timeFormatLabel = computed(
  () => TIME_FORMAT_OPTIONS.find((o) => o.value === timeFormat.value)?.label ?? '24 horas',
)

// ─── Time display (for boxes inside picker) ───────────────────────────────────
// ─── Trigger labels (read from props, not internal cal state) ─────────────────
// Using props ensures labels reflect persisted parent data after popover closes.
// labelStart = start date + time; labelEnd = end date (empty when no range).
// Both recompute when dateFormat/timeFormat change (global singletons), so ALL
// trigger buttons on the page update simultaneously.
function formatTimeFromIso(iso: string): string {
  if (iso.length < 16) return ''
  const h24 = Number(iso.slice(11, 13))
  const m = iso.slice(14, 16)
  if (timeFormat.value === '12h') {
    const period = h24 >= 12 ? 'PM' : 'AM'
    const h12 = h24 % 12 || 12
    return `${h12}:${m} ${period}`
  }
  return `${String(h24).padStart(2, '0')}:${m}`
}

const labelStart = computed(() => {
  if (!props.modelValue) return ''
  let label = formatDisplay(props.modelValue)
  const t = formatTimeFromIso(props.modelValue)
  if (t) label += ` ${t}`
  return label
})

// labelEnd reads internal calEnd as fallback because parents don't always bind endDate prop.
// This means the end-date line shows while the component is mounted, even without endDate prop binding.
const labelEnd = computed(() => {
  const iso = props.endDate || (calEnd.value ? calEnd.value.toString() : undefined)
  if (!iso) return ''
  let label = formatDisplay(iso)
  const t = formatTimeFromIso(iso)
  if (t) label += ` ${t}`
  return label
})

const triggerLabel = computed(() => {
  if (!labelStart.value) return ''
  if (labelEnd.value) return `${labelStart.value} → ${labelEnd.value}`
  return labelStart.value
})

// ─── Date display formatting ──────────────────────────────────────────────────
function relativeDate(d: Date): string {
  const diff = Math.round((d.setHours(0, 0, 0, 0) - new Date().setHours(0, 0, 0, 0)) / 86400000)
  if (diff === 0) return 'hoje'
  if (diff === 1) return 'amanhã'
  if (diff === -1) return 'ontem'
  if (diff > 0) return `em ${diff} dias`
  return `há ${Math.abs(diff)} dias`
}

function formatDisplay(iso?: string): string {
  if (!iso || iso.length < 10) return ''
  const d = new Date(`${iso.slice(0, 10)}T00:00:00`)
  if (isNaN(d.getTime())) return iso
  const dd = String(d.getDate()).padStart(2, '0')
  const mm = String(d.getMonth() + 1).padStart(2, '0')
  const yyyy = d.getFullYear()
  switch (dateFormat.value) {
    case 'dmy':
      return `${dd}/${mm}/${yyyy}`
    case 'mdy':
      return `${mm}/${dd}/${yyyy}`
    case 'ymd':
      return `${yyyy}/${mm}/${dd}`
    case 'full':
      return d.toLocaleDateString('pt-BR', { day: 'numeric', month: 'long', year: 'numeric' })
    case 'short':
      return d.toLocaleDateString('pt-BR', { day: 'numeric', month: 'short' })
    case 'rel':
      return relativeDate(new Date(`${iso.slice(0, 10)}T00:00:00`))
    default:
      return `${dd}/${mm}/${yyyy}`
  }
}

const startDisplay = computed(() =>
  calStart.value ? formatDisplay(calStart.value.toString()) : '',
)
const endDisplay = computed(() => (calEnd.value ? formatDisplay(calEnd.value.toString()) : ''))

// ─── Calendar value ───────────────────────────────────────────────────────────
// Range mode always returns an object — RangeCalendar requires non-undefined initial value.
const calendarValue = computed(() => {
  if (showEndDate.value) return { start: calStart.value, end: calEnd.value }
  return calStart.value
})

// ─── Builders & handlers ──────────────────────────────────────────────────────
function buildIso(date: CalendarDate, withTime = false, h = '00', m = '00'): string {
  const iso = date.toString()
  return withTime ? `${iso}T${h.padStart(2, '0')}:${m.padStart(2, '0')}` : iso
}

function emitStartDate() {
  emit(
    'update:modelValue',
    calStart.value
      ? buildIso(calStart.value, includeTime.value, get24hHour('start'), normalizedMinute('start'))
      : '',
  )
}

function emitEndDate() {
  emit(
    'update:endDate',
    calEnd.value
      ? buildIso(calEnd.value, includeTime.value, get24hHour('end'), normalizedMinute('end'))
      : '',
  )
}

function onSelect(val: CalendarDate | undefined) {
  calStart.value = val
  if (val) viewPlaceholder.value = val
  emitStartDate()
}

function onRangeSelect(
  val: { start: CalendarDate | undefined; end: CalendarDate | undefined } | undefined,
) {
  calStart.value = val?.start
  calEnd.value = val?.end
  if (val?.start) viewPlaceholder.value = val.start
  emitStartDate()
  emitEndDate()
}

function onCalendarUpdate(val: unknown) {
  if (showEndDate.value)
    onRangeSelect(val as { start: CalendarDate | undefined; end: CalendarDate | undefined })
  else onSelect(val as CalendarDate | undefined)
}

function applyManualInput(target: TimeTarget) {
  const rawDate = target === 'start' ? manualStartDate.value : manualEndDate.value
  const rawTime = target === 'start' ? manualStartTime.value : manualEndTime.value
  const parsed = isoToCalendar(rawDate)

  if (!rawDate) {
    if (target === 'start') {
      calStart.value = undefined
      emit('update:modelValue', '')
    } else {
      calEnd.value = undefined
      emit('update:endDate', '')
    }
    return
  }

  if (!parsed) return
  if (rawTime) {
    includeTime.value = true
    setTimeFromHHMM(target, rawTime)
  }

  if (target === 'start') {
    calStart.value = parsed
    viewPlaceholder.value = parsed
    emitStartDate()
    return
  }

  showEndDate.value = true
  calEnd.value = parsed
  emitEndDate()
}

function setRange(start: CalendarDate, end?: CalendarDate) {
  calStart.value = start
  calEnd.value = end
  viewPlaceholder.value = start
  showEndDate.value = Boolean(end)
  includeTime.value = false
  syncManualFields('start', start.toString())
  syncManualFields('end', end?.toString())
  emitStartDate()
  emitEndDate()
}

function applyQuickRange(value: string) {
  const now = new Date()
  if (value === 'all') {
    clear()
    return
  }
  if (value === 'today') {
    const current = calendarFromDate(now)
    setRange(current, current)
    return
  }
  if (value === 'this-month') {
    const start = calendarFromParts(now.getFullYear(), now.getMonth() + 1, 1)
    const end = calendarFromParts(
      now.getFullYear(),
      now.getMonth() + 1,
      lastDayOfMonth(now.getFullYear(), now.getMonth() + 1),
    )
    setRange(start, end)
    return
  }
  if (value === 'last-month') {
    const previous = new Date(now.getFullYear(), now.getMonth() - 1, 1)
    const month = previous.getMonth() + 1
    const start = calendarFromParts(previous.getFullYear(), month, 1)
    const end = calendarFromParts(
      previous.getFullYear(),
      month,
      lastDayOfMonth(previous.getFullYear(), month),
    )
    setRange(start, end)
    return
  }
  if (value === 'this-year') {
    setRange(
      calendarFromParts(now.getFullYear(), 1, 1),
      calendarFromParts(now.getFullYear(), 12, 31),
    )
    return
  }

  const months = value === 'last-4-months' ? 4 : 3
  const startDate = new Date(now.getFullYear(), now.getMonth() - months + 1, 1)
  setRange(calendarFromDate(startDate), calendarFromDate(now))
}

function applyMonthRange() {
  const startMatch = monthStartValue.value.match(/^(\d{4})-(\d{2})$/)
  const endMatch = (monthEndValue.value || monthStartValue.value).match(/^(\d{4})-(\d{2})$/)
  if (!startMatch || !endMatch) return

  const startYear = Number(startMatch[1])
  const startMonth = Number(startMatch[2])
  const endYear = Number(endMatch[1])
  const endMonth = Number(endMatch[2])
  let start = calendarFromParts(startYear, startMonth, 1)
  let end = calendarFromParts(endYear, endMonth, lastDayOfMonth(endYear, endMonth))
  if (end.toString() < start.toString()) {
    const previousStart = start
    start = calendarFromParts(endYear, endMonth, 1)
    end = calendarFromParts(
      previousStart.year,
      previousStart.month,
      lastDayOfMonth(previousStart.year, previousStart.month),
    )
  }
  setRange(start, end)
}

function applyYearRange() {
  const startYear = Number(yearStartValue.value || new Date().getFullYear())
  const endYear = Number(yearEndValue.value || yearStartValue.value || startYear)
  if (!Number.isFinite(startYear) || !Number.isFinite(endYear)) return
  const firstYear = Math.min(startYear, endYear)
  const lastYear = Math.max(startYear, endYear)
  setRange(calendarFromParts(firstYear, 1, 1), calendarFromParts(lastYear, 12, 31))
}

function onTimeChange(target: TimeTarget = 'start') {
  if (target === 'start') {
    if (calStart.value) emitStartDate()
    return
  }
  if (calEnd.value) emitEndDate()
}

function onIncludeTimeChange(_enabled?: boolean) {
  emitStartDate()
  if (showEndDate.value) emitEndDate()
}

function togglePeriod(target: TimeTarget = 'start') {
  const { period } = timeRefs(target)
  period.value = period.value === 'AM' ? 'PM' : 'AM'
  onTimeChange(target)
}

function goToToday() {
  viewPlaceholder.value = today(getLocalTimeZone())
}

function clear() {
  calStart.value = undefined
  calEnd.value = undefined
  showEndDate.value = false
  manualStartDate.value = ''
  manualEndDate.value = ''
  manualStartTime.value = ''
  manualEndTime.value = ''
  emit('update:modelValue', '')
  emit('update:endDate', '')
}
</script>

<template>
  <UPopover v-model:open="popoverOpen" :content="{ side: placement ?? 'bottom', align: 'start' }">
    <!--
      Default slot = trigger.
      Parent renders whatever trigger button it needs.
      Slot exposes:
        - labelStart: formatted start date + time
        - labelEnd:   formatted end date (empty when no range)
        - label:      combined string (labelStart [ → labelEnd ])
    -->
    <slot :label="triggerLabel" :label-start="labelStart" :label-end="labelEnd"></slot>

    <template #content>
      <div class="app-datepicker">
        <!-- Date display boxes -->
        <div
          class="app-datepicker__inputs"
          :class="{ 'app-datepicker__inputs--range': showEndDate }"
        >
          <div class="app-datepicker__date-box" :class="{ 'is-set': !!calStart }">
            <span v-if="startDisplay" class="app-datepicker__date-text">{{ startDisplay }}</span>
            <span v-else class="app-datepicker__date-placeholder">
              {{ showEndDate ? 'Início' : 'Selecionar data...' }}
            </span>
            <span v-if="includeTime && calStart" class="app-datepicker__time-inline" @click.stop>
              <input
                v-model="startTimeHour"
                class="app-datepicker__time-input"
                type="number"
                :min="timeFormat === '12h' ? 1 : 0"
                :max="timeFormat === '12h' ? 12 : 23"
                placeholder="HH"
                @change="onTimeChange('start')"
              />
              <span class="app-datepicker__time-sep">:</span>
              <input
                v-model="startTimeMinute"
                class="app-datepicker__time-input"
                type="number"
                min="0"
                max="59"
                placeholder="MM"
                @change="onTimeChange('start')"
              />
              <button
                v-if="timeFormat === '12h'"
                class="app-datepicker__ampm-btn"
                type="button"
                @click="togglePeriod('start')"
              >
                {{ startTimePeriod }}
              </button>
            </span>
          </div>
          <div v-if="showEndDate" class="app-datepicker__date-box" :class="{ 'is-set': !!calEnd }">
            <span v-if="endDisplay" class="app-datepicker__date-text">{{ endDisplay }}</span>
            <span v-else class="app-datepicker__date-placeholder">Fim</span>
            <span v-if="includeTime && calEnd" class="app-datepicker__time-inline" @click.stop>
              <input
                v-model="endTimeHour"
                class="app-datepicker__time-input"
                type="number"
                :min="timeFormat === '12h' ? 1 : 0"
                :max="timeFormat === '12h' ? 12 : 23"
                placeholder="HH"
                @change="onTimeChange('end')"
              />
              <span class="app-datepicker__time-sep">:</span>
              <input
                v-model="endTimeMinute"
                class="app-datepicker__time-input"
                type="number"
                min="0"
                max="59"
                placeholder="MM"
                @change="onTimeChange('end')"
              />
              <button
                v-if="timeFormat === '12h'"
                class="app-datepicker__ampm-btn"
                type="button"
                @click="togglePeriod('end')"
              >
                {{ endTimePeriod }}
              </button>
            </span>
          </div>
        </div>

        <div class="app-datepicker__mode-tabs" role="tablist" aria-label="Visualizacao do periodo">
          <button
            v-for="option in VIEW_MODE_OPTIONS"
            :key="option.value"
            class="app-datepicker__mode-tab"
            :class="{ 'is-active': viewMode === option.value }"
            type="button"
            @click="viewMode = option.value"
          >
            {{ option.label }}
          </button>
        </div>

        <div v-if="viewMode === 'day'" class="app-datepicker__manual">
          <label class="app-datepicker__manual-field">
            <span>Inicio</span>
            <input
              v-model="manualStartDate"
              class="app-datepicker__native-input"
              type="date"
              @change="applyManualInput('start')"
            />
          </label>
          <label v-if="showEndDate" class="app-datepicker__manual-field">
            <span>Fim</span>
            <input
              v-model="manualEndDate"
              class="app-datepicker__native-input"
              type="date"
              @change="applyManualInput('end')"
            />
          </label>
          <label v-if="includeTime" class="app-datepicker__manual-field">
            <span>Hora inicio</span>
            <input
              v-model="manualStartTime"
              class="app-datepicker__native-input"
              type="time"
              @change="applyManualInput('start')"
            />
          </label>
          <label v-if="includeTime && showEndDate" class="app-datepicker__manual-field">
            <span>Hora fim</span>
            <input
              v-model="manualEndTime"
              class="app-datepicker__native-input"
              type="time"
              @change="applyManualInput('end')"
            />
          </label>
        </div>

        <div v-else-if="viewMode === 'month'" class="app-datepicker__manual">
          <label class="app-datepicker__manual-field">
            <span>Mes inicial</span>
            <input v-model="monthStartValue" class="app-datepicker__native-input" type="month" />
          </label>
          <label class="app-datepicker__manual-field">
            <span>Mes final</span>
            <input v-model="monthEndValue" class="app-datepicker__native-input" type="month" />
          </label>
          <button class="app-datepicker__apply" type="button" @click="applyMonthRange">
            Aplicar meses
          </button>
        </div>

        <div v-else class="app-datepicker__manual">
          <label class="app-datepicker__manual-field">
            <span>Ano inicial</span>
            <input
              v-model="yearStartValue"
              class="app-datepicker__native-input"
              type="number"
              min="2000"
              max="2100"
            />
          </label>
          <label class="app-datepicker__manual-field">
            <span>Ano final</span>
            <input
              v-model="yearEndValue"
              class="app-datepicker__native-input"
              type="number"
              min="2000"
              max="2100"
            />
          </label>
          <button class="app-datepicker__apply" type="button" @click="applyYearRange">
            Aplicar anos
          </button>
        </div>

        <div class="app-datepicker__quick-ranges" aria-label="Atalhos de periodo">
          <button
            v-for="option in QUICK_RANGE_OPTIONS"
            :key="option.value"
            class="app-datepicker__quick-range"
            type="button"
            @click="applyQuickRange(option.value)"
          >
            {{ option.label }}
          </button>
        </div>

        <!-- Calendar -->
        <UCalendar
          v-if="viewMode === 'day'"
          :model-value="calendarValue"
          :placeholder="viewPlaceholder"
          :range="showEndDate"
          :year-controls="false"
          :fixed-weeks="true"
          locale="pt-BR"
          class="app-datepicker__calendar"
          @update:model-value="onCalendarUpdate($event)"
          @update:placeholder="viewPlaceholder = $event as CalendarDate"
        >
          <template #heading>
            <span class="app-datepicker__heading-month">{{ headingLabel }}</span>
            <UButton
              size="xs"
              color="neutral"
              variant="ghost"
              class="app-datepicker__today-btn"
              @click="goToToday"
            >
              Hoje
            </UButton>
          </template>
          <template #week-day="{ day }">{{ ptWeekday(day) }}</template>
        </UCalendar>

        <!-- Settings -->
        <div class="app-datepicker__settings">
          <div class="app-datepicker__divider"></div>

          <!-- Data de término -->
          <div class="app-datepicker__setting-row">
            <span>Data de término</span>
            <USwitch v-model="showEndDate" size="sm" />
          </div>

          <!-- Formato de data -->
          <UPopover v-model:open="formatOpen" :content="{ side: 'left', align: 'start' }">
            <button
              class="app-datepicker__setting-row app-datepicker__setting-row--btn"
              type="button"
            >
              <span>Formato de data</span>
              <span class="app-datepicker__setting-value">
                {{ formatLabel }}
                <ChevronRight :size="13" :stroke-width="2.2" aria-hidden="true" />
              </span>
            </button>
            <template #content>
              <div class="app-datepicker__option-list">
                <button
                  v-for="opt in FORMAT_OPTIONS"
                  :key="opt.value"
                  class="app-datepicker__option"
                  :class="{ 'is-active': dateFormat === opt.value }"
                  type="button"
                  @click="pickDateFormat(opt.value)"
                >
                  {{ opt.label }}
                </button>
              </div>
            </template>
          </UPopover>

          <!-- Incluir hora -->
          <div class="app-datepicker__setting-row">
            <span>Incluir hora</span>
            <USwitch v-model="includeTime" size="sm" @update:model-value="onIncludeTimeChange" />
          </div>

          <!-- Formato de hora -->
          <UPopover v-model:open="timeFormatOpen" :content="{ side: 'left', align: 'start' }">
            <button
              class="app-datepicker__setting-row app-datepicker__setting-row--btn"
              type="button"
            >
              <span>Formato de hora</span>
              <span class="app-datepicker__setting-value">
                {{ timeFormatLabel }}
                <ChevronRight :size="13" :stroke-width="2.2" aria-hidden="true" />
              </span>
            </button>
            <template #content>
              <div class="app-datepicker__option-list">
                <button
                  v-for="opt in TIME_FORMAT_OPTIONS"
                  :key="opt.value"
                  class="app-datepicker__option"
                  :class="{ 'is-active': timeFormat === opt.value }"
                  type="button"
                  @click="pickTimeFormat(opt.value)"
                >
                  {{ opt.label }}
                </button>
              </div>
            </template>
          </UPopover>

          <!-- Lembrar -->
          <UPopover v-model:open="reminderOpen" :content="{ side: 'left', align: 'start' }">
            <button
              class="app-datepicker__setting-row app-datepicker__setting-row--btn"
              type="button"
            >
              <span>Lembrar</span>
              <span class="app-datepicker__setting-value">
                {{ reminderLabel }}
                <ChevronRight :size="13" :stroke-width="2.2" aria-hidden="true" />
              </span>
            </button>
            <template #content>
              <div class="app-datepicker__option-list">
                <button
                  v-for="opt in REMINDER_OPTIONS"
                  :key="opt.value"
                  class="app-datepicker__option"
                  :class="{ 'is-active': reminder === opt.value }"
                  type="button"
                  @click="pickReminder(opt.value)"
                >
                  {{ opt.label }}
                </button>
              </div>
            </template>
          </UPopover>

          <div class="app-datepicker__divider"></div>

          <!-- Limpar -->
          <button
            class="app-datepicker__clear"
            type="button"
            :disabled="!calStart && !calEnd"
            @click="clear"
          >
            Limpar
          </button>
        </div>
      </div>
    </template>
  </UPopover>
</template>

<style scoped>
.app-datepicker {
  width: 20rem;
  display: grid;
  background: var(--admin-header-panel-bg);
  border: 1px solid var(--admin-header-border);
  border-radius: 12px;
  /* Conteudo alto: limita a altura ao espaco real disponivel (o Floating UI/Reka
     expoe --reka-popper-available-height no lado escolhido) e rola por dentro. Sem
     isso, quando o gatilho fica alto na tela o popover vira pra cima e era CORTADO
     no topo da viewport (calendario sumia). Fallback 85vh se a var nao existir. */
  max-height: var(--reka-popper-available-height, 85vh);
  overflow-x: hidden;
  overflow-y: auto;
  overscroll-behavior: contain;
}

/* ── Date boxes ── */
.app-datepicker__inputs {
  display: grid;
  grid-template-columns: 1fr;
  gap: 0.35rem;
  padding: 0.62rem 0.72rem 0.3rem;
}

.app-datepicker__inputs--range {
  grid-template-columns: 1fr;
}

.app-datepicker__date-box {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.35rem;
  min-height: 2rem;
  padding: 0 0.55rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 8px;
  font-size: 0.875rem;
  color: var(--admin-header-text);
  transition: border-color 0.14s ease;
}

.app-datepicker__date-box.is-set {
  border-color: rgb(var(--primary) / 0.5);
}

.app-datepicker__date-placeholder {
  color: var(--admin-header-muted);
}

.app-datepicker__date-text {
  min-width: 0;
}

.app-datepicker__time-inline {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
}

.app-datepicker__mode-tabs {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 0.25rem;
  padding: 0.25rem 0.72rem 0.35rem;
}

.app-datepicker__mode-tab,
.app-datepicker__quick-range,
.app-datepicker__apply {
  min-height: 1.85rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 7px;
  background: transparent;
  color: var(--admin-header-muted);
  font-size: 0.76rem;
  font-weight: 700;
  cursor: pointer;
  transition:
    background 0.13s ease,
    border-color 0.13s ease,
    color 0.13s ease;
}

.app-datepicker__mode-tab:hover,
.app-datepicker__quick-range:hover,
.app-datepicker__apply:hover {
  background: var(--admin-header-hover-bg);
  color: var(--admin-header-text);
}

.app-datepicker__mode-tab.is-active {
  border-color: rgb(var(--primary) / 0.55);
  background: var(--admin-header-active-bg);
  color: rgb(var(--primary));
}

.app-datepicker__manual {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.45rem;
  padding: 0 0.72rem 0.45rem;
}

.app-datepicker__manual-field {
  display: grid;
  gap: 0.18rem;
  min-width: 0;
}

.app-datepicker__manual-field span {
  color: var(--admin-header-muted);
  font-size: 0.68rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.app-datepicker__native-input {
  width: 100%;
  min-height: 2rem;
  padding: 0 0.5rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 7px;
  background: rgba(12, 18, 30, 0.7);
  color: var(--admin-header-text);
  font-size: 0.8rem;
  outline: none;
  color-scheme: dark;
}

.app-datepicker__native-input:focus {
  border-color: rgb(var(--ring) / 0.5);
}

.app-datepicker__apply {
  grid-column: 1 / -1;
}

.app-datepicker__quick-ranges {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
  padding: 0 0.72rem 0.35rem;
}

.app-datepicker__quick-range {
  flex: 1 1 auto;
  min-height: 1.75rem;
  padding: 0 0.45rem;
  font-weight: 600;
}

/* ── Calendar ── */
.app-datepicker__calendar {
  padding: 0.3rem 0.4rem 0.5rem;
  border: none;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

:deep(.app-datepicker__calendar [data-slot='heading']) {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.app-datepicker__heading-month {
  flex: 1;
  font-size: 0.83rem;
  font-weight: 700;
  color: var(--admin-header-text);
}

.app-datepicker__today-btn {
  font-size: 0.72rem;
  height: 1.6rem;
  padding: 0 0.45rem;
}

/* ── Settings ── */
.app-datepicker__settings {
  display: grid;
  padding-bottom: 0.35rem;
}

.app-datepicker__divider {
  height: 1px;
  background: var(--admin-header-separator);
  margin: 0.2rem 0;
}

.app-datepicker__setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.42rem 0.75rem;
  font-size: 0.8rem;
  color: var(--admin-header-text);
}

.app-datepicker__setting-row--btn {
  width: 100%;
  background: transparent;
  border: none;
  cursor: pointer;
  text-align: left;
  transition: background 0.14s ease;
}

.app-datepicker__setting-row--btn:hover {
  background: var(--admin-header-hover-bg);
}

.app-datepicker__setting-value {
  display: inline-flex;
  align-items: center;
  gap: 0.15rem;
  color: var(--admin-header-muted);
  font-size: 0.78rem;
}

/* ── Time row ── */
.app-datepicker__time-row {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.25rem 0.75rem 0.35rem;
}

.app-datepicker__time-input {
  width: 2.7rem;
  padding: 0.2rem 0.35rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 6px;
  background: transparent;
  color: var(--admin-header-text);
  font-size: 0.82rem;
  text-align: center;
  outline: none;
  transition: border-color 0.14s ease;
}

.app-datepicker__time-input:focus {
  border-color: rgb(var(--ring) / 0.5);
}

.app-datepicker__time-sep {
  font-size: 0.9rem;
  font-weight: 700;
  color: var(--admin-header-muted);
}

.app-datepicker__ampm-btn {
  padding: 0.15rem 0.4rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 6px;
  background: transparent;
  color: var(--admin-header-text);
  font-size: 0.78rem;
  font-weight: 600;
  cursor: pointer;
  transition:
    background 0.13s ease,
    border-color 0.13s ease;
}

.app-datepicker__ampm-btn:hover {
  background: var(--admin-header-hover-bg);
  border-color: rgb(var(--primary) / 0.4);
}

/* ── Options popover ── */
.app-datepicker__option-list {
  min-width: 10rem;
  display: grid;
  padding: 0.3rem;
}

.app-datepicker__option {
  display: block;
  width: 100%;
  padding: 0.4rem 0.65rem;
  border: none;
  border-radius: 7px;
  background: transparent;
  color: var(--admin-header-text);
  font-size: 0.8rem;
  text-align: left;
  cursor: pointer;
  transition: background 0.13s ease;
}

.app-datepicker__option:hover {
  background: var(--admin-header-hover-bg);
}

.app-datepicker__option.is-active {
  background: var(--admin-header-active-bg);
  color: rgb(var(--primary));
  font-weight: 700;
}

/* ── Clear ── */
.app-datepicker__clear {
  margin: 0.15rem 0.72rem 0.25rem;
  padding: 0.3rem 0.5rem;
  border: none;
  border-radius: 7px;
  background: transparent;
  color: var(--admin-header-muted);
  font-size: 0.8rem;
  text-align: left;
  cursor: pointer;
  transition:
    background 0.13s ease,
    color 0.13s ease;
}

.app-datepicker__clear:hover:not(:disabled) {
  background: rgb(var(--color-error-500) / 0.08);
  color: rgb(var(--color-error-500));
}

.app-datepicker__clear:disabled {
  opacity: 0.4;
  cursor: default;
}

/* ── Transitions ── */
.datepicker-expand-enter-active,
.datepicker-expand-leave-active {
  transition:
    opacity 0.15s ease,
    transform 0.15s ease;
}

.datepicker-expand-enter-from,
.datepicker-expand-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
