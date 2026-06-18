<script setup lang="ts">
import {
  formatCurrencyBRL,
  formatDurationMinutes,
  formatPercent,
} from '~/domain/utils/admin-metrics'
import type { PlayerCardStats } from './player-card-types'
import CardMetricTile from './CardMetricTile.vue'

withDefaults(
  defineProps<{
    stats: PlayerCardStats
    section: 'hero' | 'detail'
    mode?: 'full' | 'mini'
  }>(),
  { mode: 'full' },
)

// Cor do valor (verde bateu / vermelho nao bateu) quando ha meta cadastrada.
function metricHitClass(value?: number, goal?: number) {
  if (!goal || goal <= 0) return ''
  return Number(value || 0) >= goal ? 'card-metrics__value--hit' : 'card-metrics__value--miss'
}

function noteClass(value: number, goal: number) {
  return value >= goal ? 'card-metrics__note--hit' : 'card-metrics__note--miss'
}
</script>

<template>
  <!-- HERO (modo full): KPIs principais ao lado do gauge. -->
  <div v-if="section === 'hero'" class="card-metrics card-metrics--hero">
    <CardMetricTile
      icon="🎯"
      label="Ticket"
      :erp="stats.ticketAverageSource === 'erp'"
      :value-class="metricHitClass(stats.ticketAverage, stats.avgTicketGoal)"
      :note="stats.avgTicketGoal ? `Meta: ${formatCurrencyBRL(stats.avgTicketGoal)}` : null"
      :note-class="stats.avgTicketGoal ? noteClass(stats.ticketAverage, stats.avgTicketGoal) : ''"
    >
      {{ formatCurrencyBRL(stats.ticketAverage) }}
    </CardMetricTile>
    <CardMetricTile
      icon="📦"
      label="P.A."
      :erp="stats.paScoreSource === 'erp'"
      :value-class="metricHitClass(stats.paScore, stats.paGoal)"
      :note="stats.paGoal ? `Meta: ${stats.paGoal.toFixed(2)}` : null"
      :note-class="stats.paGoal ? noteClass(stats.paScore, stats.paGoal) : ''"
    >
      {{ stats.paScore.toFixed(2) }}
    </CardMetricTile>
    <CardMetricTile
      icon="⚡"
      label="Conversão"
      :note="stats.conversionGoal ? `Meta: ${formatPercent(stats.conversionGoal)}` : null"
      :note-class="
        stats.conversionGoal ? noteClass(stats.conversionRate, stats.conversionGoal) : ''
      "
    >
      {{ formatPercent(stats.conversionRate) }}
    </CardMetricTile>
    <CardMetricTile icon="⏱" label="Tempo médio">
      {{ formatDurationMinutes(stats.averageDurationMs || 0) }}
    </CardMetricTile>
  </div>

  <!-- DETALHE (modo full): comissao/atendimentos/etc. em faixa rolavel. -->
  <div v-else-if="mode === 'full'" class="card-metrics card-metrics--detail">
    <CardMetricTile
      icon="💸"
      label="Comissão estimada"
      :note="`Taxa atual: ${formatPercent((stats.commissionRate ?? 0) * 100)}`"
    >
      {{ formatCurrencyBRL(stats.estimatedCommission ?? 0) }}
    </CardMetricTile>
    <CardMetricTile icon="👥" label="Atendimentos">
      {{ (stats.conversions ?? 0) + (stats.nonConversions ?? 0) }}
    </CardMetricTile>
    <CardMetricTile icon="🔄" label="Conversões / Não convertidas">
      {{ stats.conversions ?? 0 }} / {{ stats.nonConversions ?? 0 }}
    </CardMetricTile>
    <CardMetricTile icon="🆕" label="Não-clientes convertidos">
      {{ stats.nonClientConversions ?? 0 }}
    </CardMetricTile>
    <CardMetricTile icon="↪" label="Fora da vez">
      {{ stats.queueJumpServices ?? 0 }}
    </CardMetricTile>
    <CardMetricTile
      v-if="typeof stats.cancellationRate === 'number'"
      icon="⛔"
      label="Taxa de cancelamento"
    >
      {{ formatPercent(stats.cancellationRate) }}
    </CardMetricTile>
  </div>

  <!-- DETALHE (modo mini): KPIs enxutos. -->
  <div v-else class="card-metrics card-metrics--mini">
    <CardMetricTile icon="⏱" label="Tempo">
      {{ formatDurationMinutes(stats.averageDurationMs || 0) }}
    </CardMetricTile>
    <CardMetricTile icon="⚡" label="Conversão">
      {{ formatPercent(stats.conversionRate) }}
    </CardMetricTile>
    <CardMetricTile
      icon="🎯"
      label="Ticket"
      :erp="stats.ticketAverageSource === 'erp'"
      :value-class="metricHitClass(stats.ticketAverage, stats.avgTicketGoal)"
      :note="stats.avgTicketGoal ? `Meta: ${formatCurrencyBRL(stats.avgTicketGoal)}` : null"
      :note-class="stats.avgTicketGoal ? noteClass(stats.ticketAverage, stats.avgTicketGoal) : ''"
    >
      {{ formatCurrencyBRL(stats.ticketAverage) }}
    </CardMetricTile>
    <CardMetricTile
      icon="📦"
      label="P.A."
      :erp="stats.paScoreSource === 'erp'"
      :value-class="metricHitClass(stats.paScore, stats.paGoal)"
      :note="stats.paGoal ? `Meta: ${stats.paGoal.toFixed(2)}` : null"
      :note-class="stats.paGoal ? noteClass(stats.paScore, stats.paGoal) : ''"
    >
      {{ stats.paScore.toFixed(2) }}
    </CardMetricTile>
  </div>
</template>

<style scoped>
.card-metrics {
  display: grid;
  gap: 0.5rem;
}

.card-metrics--hero {
  grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
}

.card-metrics--detail {
  grid-auto-flow: column;
  grid-auto-columns: minmax(11rem, 1fr);
  overflow-x: auto;
  padding-bottom: 0.1rem;
}

.card-metrics--mini {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

@media (max-width: 720px) {
  .card-metrics--hero {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 560px) {
  .card-metrics--hero {
    grid-template-columns: 1fr;
  }
}
</style>
