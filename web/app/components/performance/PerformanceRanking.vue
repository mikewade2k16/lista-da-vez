<script setup lang="ts">
import { computed } from 'vue'
import type { PerfMode, PerfRouteSummary } from './usePerformanceData'

interface PerformanceRankingProps {
  routes: PerfRouteSummary[]
  mode: PerfMode
  limit?: number
}

const props = withDefaults(defineProps<PerformanceRankingProps>(), {
  limit: 8,
})

const modeLabel = computed(() => (props.mode === 'cold' ? 'cold (1a visita / F5)' : 'in-app (SPA)'))

const ranked = computed<PerfRouteSummary[]>(() =>
  [...props.routes]
    .filter((route) => route.t3 !== null)
    .sort((left, right) => (right.t3 ?? 0) - (left.t3 ?? 0))
    .slice(0, props.limit),
)

const maxT3 = computed(() => ranked.value.reduce((peak, route) => Math.max(peak, route.t3 ?? 0), 0))

function formatSeconds(value: number | null): string {
  if (value === null) {
    return '—'
  }
  return `${(value / 1000).toFixed(2)}s`
}

function barWidth(value: number | null): string {
  if (value === null || maxT3.value <= 0) {
    return '0%'
  }
  return `${Math.max(4, Math.round((value / maxT3.value) * 100))}%`
}
</script>

<template>
  <section class="performance-ranking">
    <header class="performance-ranking__head">
      <h3 class="performance-ranking__title">Rotas mais lentas</h3>
      <p class="performance-ranking__subtitle">
        Ranking por T3 (carregamento final) no modo {{ modeLabel }}.
      </p>
    </header>

    <ol class="performance-ranking__list">
      <li v-for="(route, index) in ranked" :key="route.path" class="performance-ranking__row">
        <span class="performance-ranking__position">{{ index + 1 }}</span>
        <div class="performance-ranking__body">
          <div class="performance-ranking__route">
            <span class="performance-ranking__path">{{ route.path }}</span>
            <span v-if="route.capped" class="performance-ranking__flag">realtime (cap 15s)</span>
          </div>
          <div class="performance-ranking__track">
            <span class="performance-ranking__fill" :style="{ width: barWidth(route.t3) }"></span>
          </div>
        </div>
        <span class="performance-ranking__value">{{ formatSeconds(route.t3) }}</span>
      </li>
    </ol>
  </section>
</template>

<style scoped>
.performance-ranking {
  display: grid;
  gap: 0.85rem;
  padding: 1.1rem 1.2rem;
  background: var(--bg-panel);
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  box-shadow: var(--shadow-card);
}

.performance-ranking__head {
  display: grid;
  gap: 0.2rem;
}

.performance-ranking__title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-main);
}

.performance-ranking__subtitle {
  margin: 0;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.performance-ranking__list {
  display: grid;
  gap: 0.6rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.performance-ranking__row {
  display: grid;
  grid-template-columns: 1.6rem minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
}

.performance-ranking__position {
  display: grid;
  place-items: center;
  width: 1.6rem;
  height: 1.6rem;
  border-radius: var(--radius-soft);
  background: var(--bg-muted);
  color: var(--text-muted);
  font-size: 0.75rem;
  font-weight: 600;
}

.performance-ranking__body {
  display: grid;
  gap: 0.35rem;
  min-width: 0;
}

.performance-ranking__route {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
}

.performance-ranking__path {
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.performance-ranking__flag {
  flex: none;
  padding: 0.1rem 0.45rem;
  border-radius: var(--radius-soft);
  background: color-mix(in srgb, var(--accent-warning) 22%, transparent);
  color: var(--text-main);
  font-size: 0.68rem;
  font-weight: 600;
}

.performance-ranking__track {
  height: 0.45rem;
  border-radius: 999px;
  background: var(--bg-muted);
  overflow: hidden;
}

.performance-ranking__fill {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--accent-info);
}

.performance-ranking__value {
  font-variant-numeric: tabular-nums;
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--text-main);
}
</style>
