<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  formatCurrencyBRL,
  formatDurationMinutes,
  formatPercent,
} from '~/domain/utils/admin-metrics'
import { computeScore360, useGamificationConfig } from '~/composables/useGamificationConfig'

interface TableRow {
  consultantId?: string
  consultantName?: string
  storeId?: string
  storeName?: string
  soldValue: number
  attendances: number
  conversions?: number
  conversionRate: number
  ticketAverage: number
  paScore: number
  qualityScore: number
  avgDurationMs?: number
  queueJumpServices: number
  score360?: number
  [key: string]: unknown
}

const props = withDefaults(
  defineProps<{
    title: string
    rows?: TableRow[]
    testid?: string
  }>(),
  {
    rows: () => [],
    testid: '',
  },
)

const { scoreWeights } = useGamificationConfig()

const sortBy = ref('soldValue')

const sortOptions = [
  { key: 'soldValue', label: 'Valor' },
  { key: 'conversionRate', label: 'Conversao' },
  { key: 'ticketAverage', label: 'Ticket' },
  { key: 'paScore', label: 'P.A.' },
  { key: 'qualityScore', label: 'Qualidade' },
  { key: 'score360', label: '360' },
  { key: 'queueJumpServices', label: 'Fora da vez' },
]

const rowsWith360 = computed(() => {
  const rows = props.rows
  const maxSold = Math.max(...rows.map((r) => r.soldValue), 1)
  const maxPa = Math.max(...rows.map((r) => r.paScore), 0.01)

  return rows.map((row) => ({
    ...row,
    score360:
      row.score360 ??
      computeScore360(
        {
          conversionRate: Number(row.conversionRate || 0),
          soldValue: Number(row.soldValue || 0),
          qualityScore: Number(row.qualityScore || 0),
          paScore: Number(row.paScore || 0),
          queueJumpServices: Number(row.queueJumpServices || 0),
          attendances: Number(row.attendances || 0),
        },
        { maxSold, maxPa, weights: scoreWeights.value },
      ),
  }))
})

const sortedRows = computed(() => {
  const key = sortBy.value
  return [...rowsWith360.value].sort((a, b) => {
    const aVal = Number((a as Record<string, unknown>)[key] || 0)
    const bVal = Number((b as Record<string, unknown>)[key] || 0)
    if (key === 'queueJumpServices') return aVal - bVal
    return bVal - aVal
  })
})
</script>

<template>
  <article class="ranking-card" :data-testid="testid || undefined">
    <header class="ranking-card__header">
      <h3 class="ranking-card__title">{{ title }}</h3>
      <div class="ranking-sort">
        <button
          v-for="opt in sortOptions"
          :key="opt.key"
          class="ranking-sort__btn"
          :class="{ 'is-active': sortBy === opt.key }"
          :data-testid="testid ? `${testid}-sort-${opt.key}` : undefined"
          type="button"
          @click="sortBy = opt.key"
        >
          {{ opt.label }}
        </button>
      </div>
    </header>
    <div class="ranking-card__table-wrap">
      <table class="ranking-table">
        <thead>
          <tr>
            <th>#</th>
            <th>Consultor</th>
            <th>Vendas</th>
            <th>Conv.</th>
            <th>Taxa</th>
            <th>Ticket</th>
            <th>P.A.</th>
            <th>Qualidade</th>
            <th>Tempo</th>
            <th>Fora da vez</th>
            <th>Score 360</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!sortedRows.length">
            <td colspan="11">Sem dados no periodo.</td>
          </tr>
          <tr
            v-for="(row, index) in sortedRows"
            :key="`${row.consultantId}-${row.storeId || 'store'}`"
          >
            <td>{{ index + 1 }}</td>
            <td>
              <div class="ranking-table__consultant">
                <span>{{ row.consultantName }}</span>
                <small v-if="row.storeName" class="ranking-table__store">{{ row.storeName }}</small>
              </div>
            </td>
            <td>{{ formatCurrencyBRL(row.soldValue) }}</td>
            <td>{{ row.conversions }}/{{ row.attendances }}</td>
            <td>{{ formatPercent(row.conversionRate) }}</td>
            <td>{{ formatCurrencyBRL(row.ticketAverage) }}</td>
            <td>{{ row.paScore.toFixed(2) }}</td>
            <td>{{ formatPercent(row.qualityScore) }}</td>
            <td>{{ formatDurationMinutes(row.avgDurationMs) }}</td>
            <td>{{ row.queueJumpServices }}</td>
            <td>
              <strong>{{ row.score360.toFixed(1) }}</strong>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </article>
</template>

<style scoped>
.ranking-table__consultant {
  display: grid;
  gap: 2px;
}

.ranking-table__store {
  color: rgb(var(--muted) / 0.88);
  font-size: 0.68rem;
  font-weight: 600;
}
</style>
