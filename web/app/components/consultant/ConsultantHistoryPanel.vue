<script setup lang="ts">
import { computed, ref } from 'vue'
import { formatCurrencyBRL, formatPercent } from '~/domain/utils/admin-metrics'

type RangeKey = 'today' | '7d' | '30d' | 'month'

interface HistoryEntry {
  personId?: string
  storeId?: string
  finishedAt?: number
  saleAmount?: number
  finishOutcome?: string
}

const DAY_IN_MS = 24 * 60 * 60 * 1000
const CHART_WIDTH = 480
const CHART_HEIGHT = 120

const RANGE_OPTIONS: Array<{ key: RangeKey; label: string }> = [
  { key: 'today', label: 'Hoje' },
  { key: '7d', label: '7 dias' },
  { key: '30d', label: '30 dias' },
  { key: 'month', label: 'Mês' },
]

const props = defineProps<{
  consultantId: string
  storeId?: string
  entries?: HistoryEntry[]
}>()

const activeRange = ref<RangeKey>('7d')

function startOfDay(timestamp: number) {
  const date = new Date(timestamp)
  date.setHours(0, 0, 0, 0)
  return date.getTime()
}

function buildDayKey(timestamp: number) {
  const date = new Date(timestamp)
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(
    date.getDate(),
  ).padStart(2, '0')}`
}

function formatShortDate(timestamp: number) {
  const date = new Date(timestamp)
  return `${String(date.getDate()).padStart(2, '0')}/${String(date.getMonth() + 1).padStart(2, '0')}`
}

const rangeStart = computed(() => {
  const now = Date.now()
  const today = startOfDay(now)

  switch (activeRange.value) {
    case 'today':
      return today
    case '30d':
      return today - 29 * DAY_IN_MS
    case 'month': {
      const date = new Date(now)
      date.setDate(1)
      date.setHours(0, 0, 0, 0)
      return date.getTime()
    }
    case '7d':
    default:
      return today - 6 * DAY_IN_MS
  }
})

const consultantEntries = computed(() =>
  (props.entries || [])
    .filter((entry) => String(entry.personId || '').trim() === props.consultantId)
    .filter(
      (entry) =>
        !String(props.storeId || '').trim() || String(entry.storeId || '').trim() === props.storeId,
    ),
)

const filteredEntries = computed(() => {
  const start = rangeStart.value
  const end = Date.now()

  return consultantEntries.value.filter((entry) => {
    const finishedAt = Number(entry.finishedAt || 0)
    return finishedAt >= start && finishedAt <= end
  })
})

const chartPoints = computed(() => {
  const start = rangeStart.value
  const end = startOfDay(Date.now())
  const totalDays = Math.max(1, Math.floor((end - start) / DAY_IN_MS) + 1)
  const totalsByDay = new Map<string, number>()

  filteredEntries.value.forEach((entry) => {
    const finishedAt = Number(entry.finishedAt || 0)
    if (!finishedAt) return
    const dayKey = buildDayKey(finishedAt)
    totalsByDay.set(dayKey, (totalsByDay.get(dayKey) || 0) + Number(entry.saleAmount || 0))
  })

  return Array.from({ length: totalDays }, (_, index) => {
    const timestamp = start + index * DAY_IN_MS
    const dayKey = buildDayKey(timestamp)

    return {
      label: formatShortDate(timestamp),
      value: totalsByDay.get(dayKey) || 0,
    }
  })
})

const hasHistory = computed(() => filteredEntries.value.length > 0)

const chartPath = computed(() => {
  const points = chartPoints.value
  if (!points.length) return ''

  const maxValue = Math.max(...points.map((point) => point.value), 1)
  const step = CHART_WIDTH / Math.max(points.length - 1, 1)

  return points
    .map((point, index) => {
      const x = Math.round(index * step)
      const y = Math.round(CHART_HEIGHT - (point.value / maxValue) * CHART_HEIGHT)
      return `${index === 0 ? 'M' : 'L'} ${x} ${y}`
    })
    .join(' ')
})

const totalSold = computed(() =>
  filteredEntries.value.reduce((sum, entry) => sum + Number(entry.saleAmount || 0), 0),
)

const attendances = computed(() => filteredEntries.value.length)

const conversions = computed(
  () =>
    filteredEntries.value.filter(
      (entry) => entry.finishOutcome === 'compra' || entry.finishOutcome === 'reserva',
    ).length,
)

const conversionRate = computed(() => {
  if (!attendances.value) return 0
  return (conversions.value / attendances.value) * 100
})

const periodLabel = computed(() => {
  const start = formatShortDate(rangeStart.value)
  const end = formatShortDate(Date.now())
  return `${start} - ${end}`
})
</script>

<template>
  <section class="history-panel" data-testid="consultant-history-panel">
    <header class="history-panel__header">
      <div class="history-panel__title-block">
        <h3 class="history-panel__title">Histórico</h3>
        <p class="history-panel__text">Vendas e conversões por período.</p>
      </div>

      <div class="history-panel__filters" role="tablist" aria-label="Filtros do histórico">
        <button
          v-for="option in RANGE_OPTIONS"
          :key="option.key"
          type="button"
          class="history-panel__filter"
          :class="{ 'history-panel__filter--active': activeRange === option.key }"
          @click="activeRange = option.key"
        >
          {{ option.label }}
        </button>
      </div>
    </header>

    <div v-if="hasHistory" class="history-panel__chart-shell">
      <svg viewBox="0 0 480 120" preserveAspectRatio="none" aria-hidden="true">
        <path :d="chartPath" fill="none" stroke="rgb(var(--primary))" stroke-width="2.5" />
      </svg>
    </div>
    <div v-else class="history-panel__empty">Sem dados no período selecionado.</div>

    <div class="history-panel__meta">
      <span>{{ periodLabel }}</span>
      <span>{{ chartPoints.length }} dias no intervalo</span>
    </div>

    <div class="history-panel__summary">
      <article class="history-panel__metric">
        <span class="history-panel__metric-label">Vendido</span>
        <strong class="history-panel__metric-value">{{ formatCurrencyBRL(totalSold) }}</strong>
      </article>
      <article class="history-panel__metric">
        <span class="history-panel__metric-label">Atendimentos</span>
        <strong class="history-panel__metric-value">{{ attendances }}</strong>
      </article>
      <article class="history-panel__metric">
        <span class="history-panel__metric-label">Conversões</span>
        <strong class="history-panel__metric-value">{{ conversions }}</strong>
      </article>
      <article class="history-panel__metric">
        <span class="history-panel__metric-label">Taxa</span>
        <strong class="history-panel__metric-value">{{ formatPercent(conversionRate) }}</strong>
      </article>
    </div>
  </section>
</template>

<style scoped>
.history-panel {
  display: grid;
  gap: 0.9rem;
  padding: 1rem;
  border-radius: 1rem;
  border: 1px solid rgb(var(--primary) / 0.16);
  background: rgb(var(--surface) / 0.78);
  box-shadow: var(--shadow-xs);
}

.history-panel__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.9rem;
}

.history-panel__title-block {
  display: grid;
  gap: 0.2rem;
}

.history-panel__title {
  margin: 0;
  font-size: 0.96rem;
  color: rgb(var(--text) / 0.96);
}

.history-panel__text {
  margin: 0;
  font-size: 0.78rem;
  color: rgb(var(--muted) / 0.9);
}

.history-panel__filters {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.45rem;
}

.history-panel__filter {
  padding: 0.42rem 0.72rem;
  border-radius: 999px;
  border: 1px solid rgb(var(--border) / 0.82);
  background: rgb(var(--surface-2) / 0.74);
  color: rgb(var(--muted) / 0.96);
  font-size: 0.74rem;
  font-weight: 600;
  cursor: pointer;
}

.history-panel__filter--active {
  border-color: rgb(var(--ring) / 0.4);
  background: rgb(var(--primary) / 0.18);
  color: rgb(var(--primary));
}

.history-panel__chart-shell {
  min-height: 8rem;
  padding: 0.75rem 0.5rem 0.25rem;
  border-radius: 0.85rem;
  background: rgb(var(--surface-2) / 0.74);
  border: 1px solid rgb(var(--border) / 0.72);
}

.history-panel__chart-shell svg {
  display: block;
  width: 100%;
  height: 7rem;
}

.history-panel__empty {
  display: grid;
  place-items: center;
  min-height: 8rem;
  padding: 1rem;
  border-radius: 0.85rem;
  background: rgb(var(--surface-2) / 0.74);
  border: 1px solid rgb(var(--border) / 0.72);
  color: rgb(var(--muted) / 0.9);
  font-size: 0.82rem;
}

.history-panel__meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  font-size: 0.76rem;
  color: rgb(var(--muted) / 0.9);
}

.history-panel__summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.55rem;
}

.history-panel__metric {
  display: grid;
  gap: 0.18rem;
  padding: 0.7rem 0.8rem;
  border-radius: 0.8rem;
  background: rgb(var(--surface-2) / 0.74);
  border: 1px solid rgb(var(--border) / 0.72);
}

.history-panel__metric-label {
  font-size: 0.68rem;
  color: rgb(var(--muted) / 0.88);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.history-panel__metric-value {
  font-size: 0.95rem;
  color: rgb(var(--text) / 0.96);
}

@media (max-width: 920px) {
  .history-panel__header,
  .history-panel__meta {
    flex-direction: column;
    align-items: stretch;
  }

  .history-panel__filters {
    justify-content: flex-start;
  }

  .history-panel__summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
