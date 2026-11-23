<script setup lang="ts">
import type { ObsRunCompare } from '~/types/observability'
import { formatDate } from '~/utils/observability/format'

const props = defineProps<{ compare: ObsRunCompare }>()

const VERDICT_BAND = { regression: 'bad', improvement: 'good', same: 'none' } as const
const VERDICT_LABEL = { regression: 'regressao', improvement: 'melhora', same: 'igual' } as const

const ordered = computed(() => {
  const rank = { regression: 0, improvement: 1, same: 2 }
  return [...props.compare.changes].sort((x, y) => rank[x.verdict] - rank[y.verdict])
})
const regressions = computed(() => props.compare.changes.filter((c) => c.verdict === 'regression'))

function value(raw: number | null): string {
  return Number.isFinite(raw) ? String(Math.round((raw as number) * 100) / 100) : '-'
}
function delta(raw: number | null): string {
  if (!Number.isFinite(raw)) return '-'
  const n = Math.round((raw as number) * 10) / 10
  return `${n > 0 ? '+' : ''}${n}%`
}
</script>

<template>
  <section class="obs-section">
    <h3 class="obs-section__title">
      Antes ({{ compare.a.id }}, {{ formatDate(compare.a.startedAt) }}) x depois ({{
        compare.b.id
      }}, {{ formatDate(compare.b.startedAt) }})
    </h3>
    <p :class="['obs-hint', regressions.length ? 'obs-text--bad' : 'obs-text--good']">
      {{
        regressions.length
          ? `${regressions.length} regressao(oes) encontrada(s).`
          : 'Nenhuma regressao entre as duas execucoes.'
      }}
    </p>
    <div v-if="ordered.length" class="obs-tablewrap">
      <table class="obs-table">
        <thead>
          <tr>
            <th>Tipo</th>
            <th>Alvo</th>
            <th>Metrica</th>
            <th class="obs-num">Antes</th>
            <th class="obs-num">Depois</th>
            <th class="obs-num">Variacao</th>
            <th>Veredito</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(row, index) in ordered"
            :key="`${row.kind}-${row.target}-${row.metric}-${index}`"
            :class="{ 'obs-row--regression': row.verdict === 'regression' }"
          >
            <td>{{ row.kind }}</td>
            <td class="obs-mono">{{ row.target }}</td>
            <td>{{ row.metric }}</td>
            <td class="obs-num">{{ value(row.before) }}</td>
            <td class="obs-num">{{ value(row.after) }}</td>
            <td class="obs-num">{{ delta(row.deltaPct) }}</td>
            <td>
              <span :class="['obs-band', `obs-band--${VERDICT_BAND[row.verdict]}`]">
                {{ VERDICT_LABEL[row.verdict] }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
