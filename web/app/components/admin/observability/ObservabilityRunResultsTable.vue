<script setup lang="ts">
import type { ObsRunResult } from '~/types/observability'

defineProps<{ results: ObsRunResult[] }>()

const STATUS_BAND: Record<string, string> = {
  pass: 'good',
  warn: 'warn',
  fail: 'bad',
  skip: 'none',
}
const STATUS_LABEL: Record<string, string> = {
  pass: 'ok',
  warn: 'atencao',
  fail: 'falha',
  skip: 'ignorado',
}

function metricsSummary(metrics?: Record<string, unknown>): string {
  if (!metrics) return ''
  return Object.entries(metrics)
    .filter(([, value]) => ['string', 'number', 'boolean'].includes(typeof value))
    .slice(0, 4)
    .map(([key, value]) => `${key}: ${value}`)
    .join(' | ')
}
</script>

<template>
  <div class="obs-tablewrap">
    <table class="obs-table">
      <thead>
        <tr>
          <th>Alvo</th>
          <th>Modulo</th>
          <th>Status</th>
          <th>Motivo</th>
          <th>Metricas</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(row, index) in results" :key="`${row.kind}-${row.target}-${index}`">
          <td class="obs-mono">{{ row.target }}</td>
          <td>{{ row.module || '-' }}</td>
          <td>
            <span :class="['obs-band', `obs-band--${STATUS_BAND[row.status] || 'none'}`]">
              {{ STATUS_LABEL[row.status] || row.status }}
            </span>
          </td>
          <td class="obs-muted">{{ row.reason || '-' }}</td>
          <td class="obs-muted obs-mono">{{ metricsSummary(row.metrics) || '-' }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
