<script setup lang="ts">
import { computed } from 'vue'

import { formatCurrencyBRL, formatPercent } from '~/domain/utils/admin-metrics'
import type { CRMSummary, QueueStats } from '~/stores/crm'

const props = defineProps<{
  summary: CRMSummary
  queueStats: QueueStats | null
  warnings: string[]
  unmatchedCount: number
}>()

const summaryProgressWidth = computed(
  () => `${Math.min(100, Number(props.summary.goalProgress || 0)).toFixed(1)}%`,
)

function formatCurrencyFromCents(value?: number | null) {
  return formatCurrencyBRL((Number(value || 0) || 0) / 100)
}

function formatNumber(value?: number | null) {
  return Number(value || 0).toLocaleString('pt-BR')
}

function formatPct(value?: number | null) {
  const n = Number(value || 0)
  return n ? `${n.toFixed(1)}%` : '-'
}

function formatPA(value?: number | null) {
  return Number(value || 0).toFixed(2)
}

function progressClass(value?: number | null) {
  const normalized = Number(value || 0)
  if (normalized >= 100) return 'is-hit'
  if (normalized >= 75) return 'is-near'
  return 'is-miss'
}
</script>

<template>
  <article class="crm-hero">
    <div class="crm-hero__copy">
      <span class="crm-hero__eyebrow">% Meta do periodo</span>
      <strong class="crm-hero__value">{{ formatPercent(summary.goalProgress) }}</strong>
      <p class="crm-hero__text">
        {{ formatCurrencyFromCents(summary.salesCents) }} vendidos sobre
        {{ formatCurrencyFromCents(summary.monthlyGoalCents) }} de meta consolidada.
      </p>
    </div>

    <div class="crm-progress-card">
      <div class="crm-progress-card__track">
        <div
          class="crm-progress-card__fill"
          :class="progressClass(summary.goalProgress)"
          :style="{ width: summaryProgressWidth }"
        ></div>
      </div>
      <div class="crm-progress-card__meta">
        <span>Falta {{ formatCurrencyFromCents(summary.remainingToGoalCents) }}</span>
        <span v-if="summary.unmappedSalesCents">
          Nao mapeado: {{ formatCurrencyFromCents(summary.unmappedSalesCents) }}
        </span>
      </div>
    </div>
  </article>

  <section class="metric-grid crm-metrics">
    <article class="metric-card">
      <span class="metric-card__label">Vendas do periodo</span>
      <strong class="metric-card__value">
        {{ formatCurrencyFromCents(summary.salesCents) }}
      </strong>
    </article>
    <article class="metric-card">
      <span class="metric-card__label">Ticket medio</span>
      <strong class="metric-card__value">
        {{ formatCurrencyFromCents(summary.ticketAverageCents) }}
      </strong>
    </article>
    <article class="metric-card">
      <span class="metric-card__label">Valor por produto</span>
      <strong class="metric-card__value">
        {{ formatCurrencyFromCents(summary.valuePerProductCents) }}
      </strong>
    </article>
    <article class="metric-card">
      <span class="metric-card__label">P.A.</span>
      <strong class="metric-card__value">{{ formatPA(summary.paScore) }}</strong>
    </article>
    <article class="metric-card">
      <span class="metric-card__label">Pedidos</span>
      <strong class="metric-card__value">{{ formatNumber(summary.orders) }}</strong>
    </article>
    <article class="metric-card">
      <span class="metric-card__label">Produtos vendidos</span>
      <strong class="metric-card__value">{{ formatNumber(summary.units) }}</strong>
    </article>
  </section>

  <section v-if="queueStats" class="crm-queue-metrics">
    <h3 class="crm-section-label">Fila de atendimento - periodo selecionado</h3>
    <div class="metric-grid crm-queue-grid">
      <article class="metric-card crm-queue-card">
        <span class="metric-card__label">Atendimentos</span>
        <strong class="metric-card__value">{{ formatNumber(queueStats.totalAttendances) }}</strong>
      </article>
      <article class="metric-card crm-queue-card">
        <span class="metric-card__label">Conversoes (fila)</span>
        <strong class="metric-card__value">{{ formatNumber(queueStats.totalConversions) }}</strong>
      </article>
      <article class="metric-card crm-queue-card">
        <span class="metric-card__label">Taxa de conversao</span>
        <strong class="metric-card__value crm-rate--good">
          {{ formatPct(queueStats.conversionRate) }}
        </strong>
        <div class="crm-bar">
          <div
            class="crm-bar__fill crm-bar__fill--green"
            :style="{ width: `${Math.min(queueStats.conversionRate, 100)}%` }"
          ></div>
        </div>
      </article>
      <article class="metric-card crm-queue-card">
        <span class="metric-card__label">Cancelamento (fila)</span>
        <strong class="metric-card__value crm-rate--warn">
          {{ formatPct(queueStats.cancellationRate) }}
        </strong>
        <div class="crm-bar">
          <div
            class="crm-bar__fill crm-bar__fill--red"
            :style="{ width: `${Math.min(queueStats.cancellationRate, 100)}%` }"
          ></div>
        </div>
      </article>
      <article v-if="summary.erpCancellations" class="metric-card crm-queue-card">
        <span class="metric-card__label">Cancelamento ERP</span>
        <strong class="metric-card__value crm-rate--warn">
          {{ formatPct(summary.erpCancellationRate) }}
        </strong>
        <small class="crm-metric-sub">{{ formatNumber(summary.erpCancellations) }} pedidos</small>
      </article>
    </div>
  </section>

  <article v-if="warnings.length" class="crm-warning-list">
    <p v-for="warning in warnings" :key="warning" class="crm-warning-list__item">
      {{ warning }}
    </p>
  </article>

  <article v-if="unmatchedCount > 0" class="crm-warning-list">
    <p class="crm-warning-list__item crm-warning-list__item--info">
      {{ unmatchedCount }} consultor(es) ERP sem correspondente identificado na fila.
    </p>
  </article>
</template>

<style scoped>
.crm-hero {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr);
  gap: 1rem;
  padding: 1.25rem;
  border-radius: 24px;
  background: linear-gradient(
    135deg,
    rgb(var(--primary-600)) 0%,
    rgb(var(--primary)) 58%,
    rgb(var(--success)) 100%
  );
  color: rgb(255 255 255);
}

.crm-hero__copy {
  display: grid;
  gap: 0.5rem;
}

.crm-hero__eyebrow {
  font-size: 0.78rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgb(255 255 255 / 0.82);
}

.crm-hero__value {
  font-size: clamp(2rem, 4vw, 3.2rem);
  line-height: 1;
}

.crm-hero__text {
  max-width: 38rem;
  color: rgb(255 255 255 / 0.88);
}

.crm-progress-card {
  align-self: center;
  display: grid;
  gap: 0.75rem;
  padding: 1rem;
  border-radius: 18px;
  background: rgb(255 255 255 / 0.12);
  backdrop-filter: blur(10px);
}

.crm-progress-card__track {
  position: relative;
  display: block;
  width: 100%;
  height: 12px;
  overflow: hidden;
  border-radius: 999px;
  background: rgb(255 255 255 / 0.24);
}

.crm-progress-card__fill {
  position: absolute;
  inset: 0 auto 0 0;
  border-radius: inherit;
  background: linear-gradient(
    90deg,
    rgb(var(--danger)) 0%,
    rgb(var(--primary)) 52%,
    rgb(var(--success)) 100%
  );
}

.crm-progress-card__fill.is-hit {
  background: linear-gradient(90deg, rgb(var(--success) / 0.82) 0%, rgb(var(--success)) 100%);
}

.crm-progress-card__fill.is-near {
  background: linear-gradient(90deg, rgb(var(--primary-600)) 0%, rgb(var(--primary)) 100%);
}

.crm-progress-card__fill.is-miss {
  background: linear-gradient(90deg, rgb(var(--danger) / 0.82) 0%, rgb(var(--danger)) 100%);
}

.crm-progress-card__meta {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
  font-size: 0.92rem;
  color: rgb(255 255 255 / 0.88);
}

.crm-metrics {
  grid-template-columns: repeat(6, minmax(0, 1fr));
}

.crm-queue-metrics {
  display: grid;
  gap: 0.6rem;
}

.crm-section-label {
  margin: 0;
  font-size: 0.75rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: rgb(var(--muted));
}

.crm-queue-grid {
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
}

.crm-queue-card {
  border-left: 3px solid rgb(var(--primary) / 0.35);
}

.crm-bar {
  height: 4px;
  margin-top: 0.3rem;
  overflow: hidden;
  border-radius: 2px;
  background: rgb(var(--border) / 0.68);
}

.crm-bar__fill {
  height: 100%;
  border-radius: 2px;
}

.crm-bar__fill--green {
  background: rgb(var(--success));
}

.crm-bar__fill--red {
  background: rgb(var(--danger));
}

.crm-rate--good {
  color: rgb(var(--success));
  font-weight: 700;
}

.crm-rate--warn {
  color: rgb(var(--primary));
  font-weight: 700;
}

.crm-metric-sub {
  font-size: 0.72rem;
  color: rgb(var(--muted));
}

.crm-warning-list {
  display: grid;
  gap: 0.5rem;
}

.crm-warning-list__item {
  margin: 0;
  padding: 0.85rem 1rem;
  border-radius: 14px;
  background: rgb(var(--danger) / 0.12);
  color: rgb(var(--danger));
}

.crm-warning-list__item--info {
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
}

@media (max-width: 1100px) {
  .crm-metrics {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 860px) {
  .crm-hero {
    grid-template-columns: 1fr;
  }

  .crm-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 580px) {
  .crm-metrics {
    grid-template-columns: 1fr;
  }
}
</style>
