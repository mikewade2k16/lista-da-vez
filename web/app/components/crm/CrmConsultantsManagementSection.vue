<script setup lang="ts">
import { formatCurrencyBRL } from '~/domain/utils/admin-metrics'
import type { CRMConsultantMetric } from '~/stores/crm'

defineProps<{
  rows: CRMConsultantMetric[]
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

function tableRowKey(row: CRMConsultantMetric) {
  return `${row.consultantId}-${row.storeSlug}-${row.storeCnpj || ''}`
}
</script>

<template>
  <article v-if="rows.length" class="insight-card insight-card--wide">
    <header class="crm-section__header">
      <div>
        <h3 class="insight-card__title">Gerencia / Multi-loja por consultor</h3>
        <p class="insight-card__text">
          Consultores com pedidos sem loja comercial suficientemente confiavel.
        </p>
      </div>
      <span class="crm-section__meta">{{ rows.length }} consultor(es)</span>
    </header>

    <div class="insight-table-wrap">
      <table class="insight-table crm-table">
        <thead>
          <tr>
            <th>Consultor</th>
            <th>Grupo</th>
            <th>Vendido</th>
            <th>Ticket medio</th>
            <th>P.A.</th>
            <th>Pedidos</th>
            <th>Produtos</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="tableRowKey(row)">
            <td>
              <div class="crm-row-heading">
                <strong>{{ row.consultantName }}</strong>
                <small>{{ row.consultantId }}</small>
              </div>
            </td>
            <td>{{ row.storeLabel }}</td>
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

@media (max-width: 860px) {
  .crm-section__header {
    display: grid;
    grid-template-columns: 1fr;
  }
}
</style>
