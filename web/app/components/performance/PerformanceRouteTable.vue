<script setup lang="ts">
import { computed, ref } from 'vue'
import type { PerfRow } from './perf-data'
import type { PerfRouteRow } from './usePerformanceData'

interface PerformanceRouteTableProps {
  rows: PerfRouteRow[]
}

const props = defineProps<PerformanceRouteTableProps>()

const query = ref('')

const filteredRows = computed<PerfRouteRow[]>(() => {
  const term = query.value.trim().toLowerCase()
  if (!term) {
    return props.rows
  }
  return props.rows.filter((row) => row.path.toLowerCase().includes(term))
})

function formatMs(value: number | null): string {
  if (value === null) {
    return '—'
  }
  if (value >= 1000) {
    return `${(value / 1000).toFixed(2)}s`
  }
  return `${Math.round(value)}ms`
}

function cellValue(mode: PerfRow | null, marco: 't1' | 't2' | 't3'): string {
  return formatMs(mode ? mode[marco] : null)
}

// Destaque visual de T3 lento: aviso a partir de 1.5s, critico a partir de 3s.
function severityClass(mode: PerfRow | null): string {
  const value = mode?.t3 ?? null
  if (value === null) {
    return ''
  }
  if (value >= 3000) {
    return 'performance-route-table__cell--critical'
  }
  if (value >= 1500) {
    return 'performance-route-table__cell--warn'
  }
  return ''
}
</script>

<template>
  <section class="performance-route-table">
    <header class="performance-route-table__head">
      <div class="performance-route-table__title-group">
        <h3 class="performance-route-table__title">Tempos por rota</h3>
        <p class="performance-route-table__subtitle">
          Media de T1/T2/T3 (3 rodadas) nos modos in-app e cold, ordenada pela mais lenta.
        </p>
      </div>
      <label class="performance-route-table__search">
        <span class="performance-route-table__search-label">Filtrar rota</span>
        <input
          v-model="query"
          class="performance-route-table__search-input"
          type="search"
          placeholder="/operacao"
        />
      </label>
    </header>

    <div class="performance-route-table__scroll">
      <table class="performance-route-table__table">
        <thead>
          <tr>
            <th class="performance-route-table__th performance-route-table__th--route" rowspan="2">
              Rota
            </th>
            <th class="performance-route-table__th performance-route-table__th--group" colspan="3">
              In-app (SPA)
            </th>
            <th class="performance-route-table__th performance-route-table__th--group" colspan="3">
              Cold (1a visita / F5)
            </th>
          </tr>
          <tr>
            <th class="performance-route-table__th">T1</th>
            <th class="performance-route-table__th">T2</th>
            <th class="performance-route-table__th">T3</th>
            <th class="performance-route-table__th">T1</th>
            <th class="performance-route-table__th">T2</th>
            <th class="performance-route-table__th">T3</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in filteredRows" :key="row.path" class="performance-route-table__row">
            <td class="performance-route-table__cell performance-route-table__cell--route">
              <span class="performance-route-table__path">{{ row.path }}</span>
              <span v-if="row.capped" class="performance-route-table__flag">realtime</span>
            </td>
            <td class="performance-route-table__cell">{{ cellValue(row.inapp, 't1') }}</td>
            <td class="performance-route-table__cell">{{ cellValue(row.inapp, 't2') }}</td>
            <td class="performance-route-table__cell" :class="severityClass(row.inapp)">
              {{ cellValue(row.inapp, 't3') }}
            </td>
            <td class="performance-route-table__cell">{{ cellValue(row.cold, 't1') }}</td>
            <td class="performance-route-table__cell">{{ cellValue(row.cold, 't2') }}</td>
            <td class="performance-route-table__cell" :class="severityClass(row.cold)">
              {{ cellValue(row.cold, 't3') }}
            </td>
          </tr>
          <tr v-if="!filteredRows.length">
            <td class="performance-route-table__empty" colspan="7">
              Nenhuma rota corresponde ao filtro.
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
.performance-route-table {
  display: grid;
  gap: 0.85rem;
  padding: 1.1rem 1.2rem;
  background: var(--bg-panel);
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  box-shadow: var(--shadow-card);
  min-width: 0;
}

.performance-route-table__head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  justify-content: space-between;
  gap: 0.75rem;
}

.performance-route-table__title-group {
  display: grid;
  gap: 0.2rem;
}

.performance-route-table__title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-main);
}

.performance-route-table__subtitle {
  margin: 0;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.performance-route-table__search {
  display: grid;
  gap: 0.25rem;
}

.performance-route-table__search-label {
  font-size: 0.7rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
}

.performance-route-table__search-input {
  min-width: 12rem;
  padding: 0.4rem 0.65rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: var(--bg-page);
  color: var(--text-main);
  font-size: 0.85rem;
}

.performance-route-table__search-input:focus {
  outline: 2px solid var(--accent-focus);
  outline-offset: 1px;
}

.performance-route-table__scroll {
  overflow-x: auto;
}

.performance-route-table__table {
  width: 100%;
  border-collapse: collapse;
  font-variant-numeric: tabular-nums;
}

.performance-route-table__th {
  padding: 0.5rem 0.6rem;
  font-size: 0.72rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  color: var(--text-muted);
  text-align: right;
  border-bottom: 1px solid var(--line-soft);
  white-space: nowrap;
}

.performance-route-table__th--route {
  text-align: left;
}

.performance-route-table__th--group {
  text-align: center;
  border-left: 1px solid var(--line-soft);
}

.performance-route-table__cell {
  padding: 0.45rem 0.6rem;
  font-size: 0.83rem;
  color: var(--text-main);
  text-align: right;
  border-bottom: 1px solid var(--line-soft);
  white-space: nowrap;
}

.performance-route-table__cell--route {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  text-align: left;
}

.performance-route-table__cell--warn {
  color: var(--text-main);
  background: color-mix(in srgb, var(--accent-warning) 16%, transparent);
}

.performance-route-table__cell--critical {
  color: var(--text-main);
  background: color-mix(in srgb, rgb(var(--danger)) 18%, transparent);
}

.performance-route-table__path {
  font-weight: 500;
}

.performance-route-table__flag {
  padding: 0.05rem 0.4rem;
  border-radius: var(--radius-soft);
  background: color-mix(in srgb, var(--accent-warning) 22%, transparent);
  color: var(--text-main);
  font-size: 0.66rem;
  font-weight: 600;
}

.performance-route-table__empty {
  padding: 1.5rem;
  text-align: center;
  color: var(--text-muted);
  font-size: 0.85rem;
}
</style>
