<script setup lang="ts">
import { computed } from 'vue'
import PerformanceRouteTable from './PerformanceRouteTable.vue'
import PerformanceRanking from './PerformanceRanking.vue'
import PerformanceWarmupNote from './PerformanceWarmupNote.vue'
import { usePerformanceData } from './usePerformanceData'

const {
  run,
  rows,
  inappSummaries,
  coldSummaries,
  totalRoutes,
  cappedCount,
  slowestInapp,
  slowestCold,
} = usePerformanceData()

function formatStamp(stamp: string): string {
  // stamp = YYYYMMDD-HHMMSS
  const match = /^(\d{4})(\d{2})(\d{2})-(\d{2})(\d{2})(\d{2})$/.exec(stamp)
  if (!match) {
    return stamp
  }
  const [, year, month, day, hour, minute] = match
  return `${day}/${month}/${year} ${hour}:${minute}`
}

function formatSeconds(value: number | null): string {
  if (value === null) {
    return '—'
  }
  return `${(value / 1000).toFixed(2)}s`
}

const runLabel = computed(() => formatStamp(run.stamp))

const slowestInappLabel = computed(() => {
  const peak = slowestInapp.value
  return peak ? `${peak.path} (${formatSeconds(peak.t3)})` : '—'
})

const slowestColdLabel = computed(() => {
  const peak = slowestCold.value
  return peak ? `${peak.path} (${formatSeconds(peak.t3)})` : '—'
})

const summaryCards = computed(() => [
  { id: 'routes', label: 'Rotas medidas', value: String(totalRoutes.value) },
  { id: 'inapp', label: 'Mais lenta in-app', value: slowestInappLabel.value },
  { id: 'cold', label: 'Mais lenta cold', value: slowestColdLabel.value },
  { id: 'capped', label: 'Realtime (cap 15s)', value: String(cappedCount.value) },
])
</script>

<template>
  <div class="performance-workspace">
    <AdminPageHeader
      eyebrow="Diagnostico"
      title="Performance"
      description="Resultados da auditoria de navegacao do painel: T1/T2/T3 por rota nos modos in-app e cold."
    />

    <div class="performance-workspace__run">
      <span class="performance-workspace__run-label">Ultima auditoria</span>
      <span class="performance-workspace__run-value">{{ runLabel }}</span>
      <span class="performance-workspace__run-sep">·</span>
      <span class="performance-workspace__run-base">{{ run.baseUrl }}</span>
    </div>

    <section class="performance-workspace__summary">
      <article
        v-for="card in summaryCards"
        :key="card.id"
        class="performance-workspace__summary-card"
      >
        <span class="performance-workspace__summary-label">{{ card.label }}</span>
        <span class="performance-workspace__summary-value">{{ card.value }}</span>
      </article>
    </section>

    <PerformanceRouteTable :rows="rows" />

    <div class="performance-workspace__rankings">
      <PerformanceRanking :routes="inappSummaries" mode="inapp" />
      <PerformanceRanking :routes="coldSummaries" mode="cold" />
    </div>

    <PerformanceWarmupNote />
  </div>
</template>

<style scoped>
.performance-workspace {
  display: grid;
  align-content: start;
  gap: 1.1rem;
  padding: 1.2rem;
  overflow-y: auto;
  flex: 1;
  min-height: 0;
}

.performance-workspace__run {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.82rem;
  color: var(--text-muted);
}

.performance-workspace__run-label {
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  font-size: 0.7rem;
}

.performance-workspace__run-value {
  color: var(--text-main);
  font-weight: 600;
}

.performance-workspace__run-base {
  font-variant-numeric: tabular-nums;
}

.performance-workspace__summary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.85rem;
}

.performance-workspace__summary-card {
  display: grid;
  gap: 0.35rem;
  padding: 0.9rem 1rem;
  background: var(--bg-panel);
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  box-shadow: var(--shadow-card);
}

.performance-workspace__summary-label {
  font-size: 0.72rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
}

.performance-workspace__summary-value {
  font-size: 1.05rem;
  font-weight: 600;
  color: var(--text-main);
}

.performance-workspace__rankings {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 1.1rem;
}
</style>
