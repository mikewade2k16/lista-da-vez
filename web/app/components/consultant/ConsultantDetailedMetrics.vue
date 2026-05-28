<script setup lang="ts">
import {
  formatCurrencyBRL,
  formatDurationMinutes,
  formatPercent,
} from '~/domain/utils/admin-metrics'

interface DetailedStats {
  monthlyGoal: number
  soldValue: number
  remainingToGoal: number
  estimatedCommission: number
  commissionRate: number
  ticketAverage: number
  paScore: number
  conversionRate: number
  conversions: number
  nonConversions: number
  averageDurationMs: number
  queueJumpServices: number
  nonClientConversions: number
  avgTicketGoal?: number
  paGoal?: number
  conversionGoal?: number
  cancellationRate?: number
}

defineProps<{
  stats: DetailedStats
}>()
</script>

<template>
  <section class="detailed-metrics" data-testid="consultant-detailed-metrics">
    <header class="detailed-metrics__header">
      <h3 class="detailed-metrics__title">Métricas detalhadas</h3>
    </header>

    <div class="detailed-metrics__grid">
      <article class="detailed-metrics__card">
        <span class="detailed-metrics__icon" aria-hidden="true">⏱</span>
        <span class="detailed-metrics__label">Tempo médio</span>
        <strong class="detailed-metrics__value">
          {{ formatDurationMinutes(stats.averageDurationMs) }}
        </strong>
      </article>
      <article class="detailed-metrics__card">
        <span class="detailed-metrics__icon" aria-hidden="true">💸</span>
        <span class="detailed-metrics__label">Comissão estimada</span>
        <strong class="detailed-metrics__value">
          {{ formatCurrencyBRL(stats.estimatedCommission) }}
        </strong>
        <span class="detailed-metrics__text">
          Taxa atual: {{ formatPercent(stats.commissionRate * 100) }}
        </span>
      </article>
      <article class="detailed-metrics__card">
        <span class="detailed-metrics__icon" aria-hidden="true">👥</span>
        <span class="detailed-metrics__label">Atendimentos</span>
        <strong class="detailed-metrics__value">
          {{ stats.conversions + stats.nonConversions }}
        </strong>
      </article>
      <article class="detailed-metrics__card">
        <span class="detailed-metrics__icon" aria-hidden="true">🔄</span>
        <span class="detailed-metrics__label">Conversões / Não convertidas</span>
        <strong class="detailed-metrics__value">
          {{ stats.conversions }} / {{ stats.nonConversions }}
        </strong>
      </article>
      <article class="detailed-metrics__card">
        <span class="detailed-metrics__icon" aria-hidden="true">⚡</span>
        <span class="detailed-metrics__label">Taxa de conversão</span>
        <strong class="detailed-metrics__value">{{ formatPercent(stats.conversionRate) }}</strong>
        <span
          v-if="stats.conversionGoal"
          class="detailed-metrics__text"
          :class="
            stats.conversionRate >= stats.conversionGoal
              ? 'detailed-metrics__text--hit'
              : 'detailed-metrics__text--miss'
          "
        >
          Meta: {{ formatPercent(stats.conversionGoal) }}
        </span>
      </article>
      <article class="detailed-metrics__card">
        <span class="detailed-metrics__icon" aria-hidden="true">🧲</span>
        <span class="detailed-metrics__label">Não-clientes convertidos</span>
        <strong class="detailed-metrics__value">{{ stats.nonClientConversions }}</strong>
      </article>
      <article class="detailed-metrics__card">
        <span class="detailed-metrics__icon" aria-hidden="true">↪</span>
        <span class="detailed-metrics__label">Fora da vez</span>
        <strong class="detailed-metrics__value">{{ stats.queueJumpServices }}</strong>
      </article>
      <article v-if="typeof stats.cancellationRate === 'number'" class="detailed-metrics__card">
        <span class="detailed-metrics__icon" aria-hidden="true">⛔</span>
        <span class="detailed-metrics__label">Taxa de cancelamento</span>
        <strong class="detailed-metrics__value">{{ formatPercent(stats.cancellationRate) }}</strong>
      </article>
    </div>
  </section>
</template>

<style scoped>
.detailed-metrics {
  display: grid;
  gap: 0.65rem;
  padding: 1rem;
  border-radius: 1rem;
  border: 1px solid rgb(var(--primary) / 0.16);
  background: rgb(var(--surface) / 0.78);
  box-shadow: var(--shadow-xs);
}

.detailed-metrics__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.detailed-metrics__title {
  margin: 0;
  font-size: 0.92rem;
  color: rgb(var(--text) / 0.96);
}

.detailed-metrics__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(11rem, 1fr));
  gap: 0.55rem;
}

.detailed-metrics__card {
  display: grid;
  gap: 0.15rem;
  padding: 0.7rem 0.8rem;
  border-radius: 0.7rem;
  background: rgb(var(--surface-2) / 0.74);
  border: 1px solid rgb(var(--border) / 0.72);
}

.detailed-metrics__icon {
  font-size: 0.95rem;
  line-height: 1;
}

.detailed-metrics__label {
  font-size: 0.68rem;
  color: rgb(var(--muted) / 0.88);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.detailed-metrics__value {
  font-size: 0.95rem;
  color: rgb(var(--text) / 0.96);
}

.detailed-metrics__text {
  font-size: 0.7rem;
  color: rgb(var(--muted) / 0.92);
}

.detailed-metrics__text--hit {
  color: rgb(var(--success));
}

.detailed-metrics__text--miss {
  color: rgb(var(--danger));
}
</style>
