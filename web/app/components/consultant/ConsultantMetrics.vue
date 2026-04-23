<script setup>
import { formatCurrencyBRL, formatDurationMinutes, formatPercent } from "~/domain/utils/admin-metrics";

defineProps({
  stats: {
    type: Object,
    required: true
  },
  goalPercent: {
    type: Number,
    required: true
  }
});
</script>

<template>
  <div class="metric-grid" data-testid="consultant-metrics">
    <article class="metric-card">
      <span class="metric-card__label">Meta mensal</span>
      <strong class="metric-card__value">{{ formatCurrencyBRL(stats.monthlyGoal) }}</strong>
      <span class="metric-card__text">
        Faltam {{ formatCurrencyBRL(stats.remainingToGoal) }} para fechar a meta.
      </span>
    </article>
    <article class="metric-card">
      <span class="metric-card__label">Vendido no mes</span>
      <strong class="metric-card__value">{{ formatCurrencyBRL(stats.soldValue) }}</strong>
      <span class="metric-card__text">{{ formatPercent(goalPercent) }} da meta.</span>
    </article>
    <article class="metric-card">
      <span class="metric-card__label">Comissao estimada</span>
      <strong class="metric-card__value">{{ formatCurrencyBRL(stats.estimatedCommission) }}</strong>
      <span class="metric-card__text">Taxa atual: {{ formatPercent(stats.commissionRate * 100) }}.</span>
    </article>
  </div>

  <div class="progress-block" data-testid="consultant-progress">
    <span class="progress-block__label">Progresso da meta</span>
    <div class="progress-bar">
      <span class="progress-bar__fill" :style="{ '--progress': `${Math.min(goalPercent, 100)}%` }"></span>
    </div>
    <span class="progress-block__text">{{ formatPercent(goalPercent) }} concluido</span>
  </div>

  <div class="metric-grid metric-grid--tight">
    <article class="metric-card metric-card--soft">
      <span class="metric-card__label">Atendimentos no mes</span>
      <strong class="metric-card__value">{{ stats.monthEntries.length }}</strong>
    </article>
    <article class="metric-card metric-card--soft">
      <span class="metric-card__label">Nao convertidas</span>
      <strong class="metric-card__value">{{ stats.nonConversions }}</strong>
    </article>
    <article class="metric-card metric-card--soft">
      <span class="metric-card__label">Taxa de conversao</span>
      <strong class="metric-card__value">{{ formatPercent(stats.conversionRate) }}</strong>
      <span v-if="stats.conversionGoal" class="metric-card__text" :class="stats.conversionRate >= stats.conversionGoal ? 'metric-card__text--hit' : 'metric-card__text--miss'">
        Meta: {{ formatPercent(stats.conversionGoal) }}
      </span>
    </article>
    <article class="metric-card metric-card--soft">
      <span class="metric-card__label">Ticket medio</span>
      <strong class="metric-card__value">{{ formatCurrencyBRL(stats.ticketAverage) }}</strong>
      <span v-if="stats.avgTicketGoal" class="metric-card__text" :class="stats.ticketAverage >= stats.avgTicketGoal ? 'metric-card__text--hit' : 'metric-card__text--miss'">
        Meta: {{ formatCurrencyBRL(stats.avgTicketGoal) }}
      </span>
    </article>
    <article class="metric-card metric-card--soft">
      <span class="metric-card__label">P.A. (pecas por atendimento)</span>
      <strong class="metric-card__value">{{ stats.paScore.toFixed(2) }}</strong>
      <span v-if="stats.paGoal" class="metric-card__text" :class="stats.paScore >= stats.paGoal ? 'metric-card__text--hit' : 'metric-card__text--miss'">
        Meta: {{ stats.paGoal.toFixed(2) }}
      </span>
    </article>
    <article class="metric-card metric-card--soft">
      <span class="metric-card__label">Tempo medio por atendimento</span>
      <strong class="metric-card__value">{{ formatDurationMinutes(stats.averageDurationMs) }}</strong>
    </article>
    <article class="metric-card metric-card--soft">
      <span class="metric-card__label">Nao clientes convertidos</span>
      <strong class="metric-card__value">{{ stats.nonClientConversions }}</strong>
    </article>
    <article class="metric-card metric-card--soft">
      <span class="metric-card__label">Atendimentos fora da vez</span>
      <strong class="metric-card__value">{{ stats.queueJumpServices }}</strong>
    </article>
  </div>
</template>
