<script setup lang="ts">
import type { PerformanceFeedbackMetrics } from '~/types/performance-feedback'

const props = defineProps<{
  metrics: PerformanceFeedbackMetrics
}>()

const currency = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })
const number = new Intl.NumberFormat('pt-BR', { maximumFractionDigits: 2 })

function progress(value: number, goal: number): number | null {
  if (!goal) return null
  return Math.max(0, (value / goal) * 100)
}

function progressLabel(value: number, goal: number): string {
  const result = progress(value, goal)
  return result === null ? 'Meta não definida' : `${number.format(result)}% da meta`
}

function durationLabel(milliseconds: number): string {
  if (!milliseconds) return '0 min'
  return `${number.format(milliseconds / 60_000)} min`
}

const cards = computed(() => [
  {
    icon: 'i-lucide-badge-dollar-sign',
    label: 'Vendido',
    value: currency.format(props.metrics.soldValue),
    detail: props.metrics.salesGoal
      ? `${props.metrics.erpOrders ? `${number.format(props.metrics.erpOrders)} vendas ERP · ` : ''}${progressLabel(props.metrics.soldValue, props.metrics.salesGoal)} · meta ${currency.format(props.metrics.salesGoal)}`
      : 'Meta não definida',
    progress: progress(props.metrics.soldValue, props.metrics.salesGoal),
  },
  {
    icon: 'i-lucide-receipt-text',
    label: 'Ticket médio',
    value: currency.format(props.metrics.ticketAverage),
    detail: props.metrics.ticketGoal
      ? `Meta ${currency.format(props.metrics.ticketGoal)}`
      : 'Meta não definida',
    progress: progress(props.metrics.ticketAverage, props.metrics.ticketGoal),
  },
  {
    icon: 'i-lucide-shopping-basket',
    label: 'P.A.',
    value: number.format(props.metrics.paScore),
    detail: props.metrics.paGoal
      ? `Meta ${number.format(props.metrics.paGoal)}`
      : 'Meta não definida',
    progress: progress(props.metrics.paScore, props.metrics.paGoal),
  },
  {
    icon: 'i-lucide-chart-no-axes-combined',
    label: 'Conversão',
    value: `${number.format(props.metrics.conversionRate)}%`,
    detail: props.metrics.conversionGoal
      ? `Meta ${number.format(props.metrics.conversionGoal)}%`
      : `${props.metrics.conversions} conversões`,
    progress: progress(props.metrics.conversionRate, props.metrics.conversionGoal),
  },
  {
    icon: 'i-lucide-headphones',
    label: 'Nota da transcrição',
    value:
      props.metrics.transcriptionScore === undefined
        ? '—'
        : `${number.format(props.metrics.transcriptionScore)}/10`,
    detail: props.metrics.transcriptionSamples
      ? `${props.metrics.transcriptionSamples} atendimento(s) analisado(s)`
      : 'Aguardando análises com nota',
    progress:
      props.metrics.transcriptionScore === undefined ? null : props.metrics.transcriptionScore * 10,
  },
  {
    icon: 'i-lucide-users-round',
    label: 'Atendimentos',
    value: number.format(props.metrics.attendances),
    detail: `${props.metrics.conversions} convertidos · ${props.metrics.nonConversions} não convertidos`,
    progress: null,
  },
])
</script>

<template>
  <section class="pf-metrics" aria-label="Indicadores do consultor">
    <article v-for="card in cards" :key="card.label" class="pf-metric-card">
      <div class="pf-metric-card__head">
        <UIcon :name="card.icon" />
        <span>{{ card.label }}</span>
      </div>
      <strong>{{ card.value }}</strong>
      <p>{{ card.detail }}</p>
      <div v-if="card.progress !== null" class="pf-metric-card__track" aria-hidden="true">
        <span :style="{ width: `${Math.min(100, card.progress)}%` }"></span>
      </div>
    </article>

    <div class="pf-metrics__secondary">
      <span>
        <b>Qualidade dos dados</b>
        {{ number.format(metrics.qualityScore) }}%
      </span>
      <span>
        <b>Tempo médio</b>
        {{ durationLabel(metrics.avgDurationMs) }}
      </span>
      <span>
        <b>Não clientes convertidos</b>
        {{ metrics.nonClientConversions }}
      </span>
      <span>
        <b>Fora da vez</b>
        {{ number.format(metrics.queueJumpServices ?? metrics.queueJumpRate ?? 0) }}
      </span>
      <span v-if="metrics.cancellationRate !== undefined">
        <b>Cancelamento</b>
        {{ number.format(metrics.cancellationRate) }}%
      </span>
    </div>
  </section>
</template>
