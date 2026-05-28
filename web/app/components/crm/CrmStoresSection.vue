<script setup lang="ts">
import { formatCurrencyBRL, formatPercent } from '~/domain/utils/admin-metrics'
import type { CRMSummary, CRMStoreMetric, QueueStats } from '~/stores/crm'

const props = defineProps<{
  filteredStoreRows: CRMStoreMetric[]
  managementStoreRows: CRMStoreMetric[]
  queueStats: QueueStats | null
  summary: CRMSummary
  dateFrom?: string
  dateTo?: string
}>()

function formatCurrencyFromCents(value?: number | null) {
  return formatCurrencyBRL((Number(value || 0) || 0) / 100)
}

function formatNumber(value?: number | null) {
  return Number(value || 0).toLocaleString('pt-BR')
}

function formatPA(value?: number | null) {
  return Number(value || 0).toFixed(2)
}

function progressWidth(value?: number | null) {
  return `${Math.min(100, Number(value || 0)).toFixed(1)}%`
}

function progressClass(value?: number | null) {
  const normalized = Number(value || 0)
  if (normalized >= 100) return 'is-hit'
  if (normalized >= 75) return 'is-near'
  return 'is-miss'
}

function formatPct(value?: number | null) {
  const n = Number(value || 0)
  return n ? `${n.toFixed(1)}%` : '-'
}

function storeQueueRate(storeSlug: string) {
  const row = props.queueStats?.byStore?.find(
    (item) => item.storeSlug === storeSlug || item.storeId === storeSlug,
  )
  return row?.conversionRate
}
</script>

<template>
  <article class="insight-card insight-card--wide">
    <header class="crm-section__header">
      <div>
        <h3 class="insight-card__title">Lojas mapeadas</h3>
        <p class="insight-card__text">Meta da loja vs venda ERP no periodo.</p>
      </div>
      <span class="crm-section__meta">{{ dateFrom }} ate {{ dateTo }}</span>
    </header>

    <div class="insight-table-wrap">
      <table class="insight-table crm-table">
        <thead>
          <tr>
            <th>Loja</th>
            <th>Meta</th>
            <th>Vendido</th>
            <th>% Meta</th>
            <th>Falta</th>
            <th>Ticket medio</th>
            <th>P.A.</th>
            <th>Pedidos</th>
            <th>Produtos</th>
            <th v-if="queueStats">Conv. fila</th>
            <th v-if="summary.erpCancellations">Canc. ERP</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in filteredStoreRows" :key="row.storeSlug">
            <td>
              <div class="crm-row-heading">
                <strong>{{ row.storeLabel }}</strong>
                <small>{{ row.storeCode || 'Sem codigo' }}</small>
              </div>
            </td>
            <td>{{ formatCurrencyFromCents(row.monthlyGoalCents) }}</td>
            <td>{{ formatCurrencyFromCents(row.salesCents) }}</td>
            <td>
              <div class="crm-table-progress">
                <span class="crm-table-progress__track">
                  <span
                    class="crm-table-progress__fill"
                    :class="progressClass(row.goalProgress)"
                    :style="{ width: progressWidth(row.goalProgress) }"
                  ></span>
                </span>
                <strong>{{ formatPercent(row.goalProgress) }}</strong>
              </div>
            </td>
            <td>{{ formatCurrencyFromCents(row.remainingToGoalCents) }}</td>
            <td>{{ formatCurrencyFromCents(row.ticketAverageCents) }}</td>
            <td>{{ formatPA(row.paScore) }}</td>
            <td>{{ formatNumber(row.orders) }}</td>
            <td>{{ formatNumber(row.units) }}</td>
            <td v-if="queueStats">
              <span>{{ formatPct(storeQueueRate(row.storeSlug)) }}</span>
            </td>
            <td v-if="summary.erpCancellations">
              <span :class="{ 'crm-rate--bad': (row.erpCancellationRate ?? 0) > 5 }">
                {{ row.erpCancellations ? formatPct(row.erpCancellationRate) : '-' }}
              </span>
            </td>
          </tr>
          <tr v-if="!filteredStoreRows.length">
            <td class="crm-empty" colspan="11">
              Nenhuma loja com vendas ERP no periodo selecionado.
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </article>

  <article v-if="managementStoreRows.length" class="insight-card insight-card--wide">
    <header class="crm-section__header">
      <div>
        <h3 class="insight-card__title">Gerencia / Multi-loja</h3>
        <p class="insight-card__text">
          Pedidos sem loja comercial confiavel para atribuicao direta.
        </p>
      </div>
      <span class="crm-section__meta">Separado do consolidado por loja</span>
    </header>

    <div class="insight-table-wrap">
      <table class="insight-table crm-table">
        <thead>
          <tr>
            <th>Grupo</th>
            <th>Vendido</th>
            <th>Ticket medio</th>
            <th>P.A.</th>
            <th>Pedidos</th>
            <th>Produtos</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in managementStoreRows" :key="row.storeSlug">
            <td>
              <div class="crm-row-heading">
                <strong>{{ row.storeLabel }}</strong>
                <small>Sem loja unica confirmada</small>
              </div>
            </td>
            <td>{{ formatCurrencyFromCents(row.salesCents) }}</td>
            <td>{{ formatCurrencyFromCents(row.ticketAverageCents) }}</td>
            <td>{{ formatPA(row.paScore) }}</td>
            <td>{{ formatNumber(row.orders) }}</td>
            <td>{{ formatNumber(row.units) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </article>
</template>

<style scoped>
.crm-section__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
  margin-bottom: 1rem;
}

.crm-section__meta {
  color: rgb(var(--muted));
  font-size: 0.88rem;
}

.crm-table {
  min-width: 860px;
}

.crm-row-heading {
  display: grid;
  gap: 0.2rem;
}

.crm-row-heading small {
  color: rgb(var(--muted));
}

.crm-empty {
  padding: 1rem;
  text-align: center;
  color: rgb(var(--muted));
}

.crm-table-progress {
  display: grid;
  gap: 0.35rem;
  min-width: 120px;
}

.crm-table-progress strong {
  font-size: 0.85rem;
}

.crm-table-progress__track {
  position: relative;
  display: block;
  width: 100%;
  height: 12px;
  overflow: hidden;
  border-radius: 999px;
  background: rgb(255 255 255 / 0.24);
}

.crm-table-progress__fill {
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

.crm-table-progress__fill.is-hit {
  background: linear-gradient(90deg, rgb(var(--success) / 0.82) 0%, rgb(var(--success)) 100%);
}

.crm-table-progress__fill.is-near {
  background: linear-gradient(90deg, rgb(var(--primary-600)) 0%, rgb(var(--primary)) 100%);
}

.crm-table-progress__fill.is-miss {
  background: linear-gradient(90deg, rgb(var(--danger) / 0.82) 0%, rgb(var(--danger)) 100%);
}

.crm-rate--bad {
  color: rgb(var(--danger));
  font-weight: 700;
}

@media (max-width: 860px) {
  .crm-section__header {
    display: grid;
    grid-template-columns: 1fr;
  }
}
</style>
