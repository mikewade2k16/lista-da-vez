<script setup lang="ts">
import { computed } from "vue";
import { storeToRefs } from "pinia";

import { formatCurrencyBRL, formatPercent } from "~/domain/utils/admin-metrics";
import { useCrmStore } from "~/stores/crm";

const crmStore = useCrmStore();
const { overview, pending, ready, errorMessage, dateFrom, dateTo } = storeToRefs(crmStore);

const summary = computed(() => overview.value?.summary || {
  orders: 0,
  units: 0,
  salesCents: 0,
  ticketAverageCents: 0,
  valuePerProductCents: 0,
  paScore: 0,
  monthlyGoalCents: 0,
  goalProgress: 0,
  remainingToGoalCents: 0,
  unmappedSalesCents: 0
});
const storeRows = computed(() => overview.value?.stores || []);
const consultantRows = computed(() => overview.value?.consultants || []);
const managementStoreSlug = "gerencia-multiloja";
const commercialStoreRows = computed(() => storeRows.value.filter((row) => row.storeSlug !== managementStoreSlug));
const managementStoreRows = computed(() => storeRows.value.filter((row) => row.storeSlug === managementStoreSlug));
const commercialConsultantRows = computed(() => consultantRows.value.filter((row) => row.storeSlug !== managementStoreSlug));
const managementConsultantRows = computed(() => consultantRows.value.filter((row) => row.storeSlug === managementStoreSlug));
const warnings = computed(() => overview.value?.warnings || []);
const summaryProgressWidth = computed(() => `${Math.min(100, Number(summary.value.goalProgress || 0)).toFixed(1)}%`);

function formatCurrencyFromCents(value?: number | null) {
  return formatCurrencyBRL((Number(value || 0) || 0) / 100);
}

function formatNumber(value?: number | null) {
  return Number(value || 0).toLocaleString("pt-BR");
}

function formatPA(value?: number | null) {
  return Number(value || 0).toFixed(2);
}

function progressWidth(value?: number | null) {
  return `${Math.min(100, Number(value || 0)).toFixed(1)}%`;
}

function progressClass(value?: number | null) {
  const normalized = Number(value || 0);
  if (normalized >= 100) {
    return "is-hit";
  }
  if (normalized >= 75) {
    return "is-near";
  }
  return "is-miss";
}

async function submitFilters() {
  await crmStore.applyFilters();
}

async function resetMonth() {
  crmStore.resetCurrentMonth();
  await crmStore.applyFilters();
}
</script>

<template>
  <section class="admin-panel crm-panel" data-testid="crm-panel">
    <header class="admin-panel__header crm-panel__header">
      <div>
        <h2 class="admin-panel__title">CRM comercial via ERP</h2>
        <p class="admin-panel__text">
          Metas cadastradas no sistema cruzadas com pedidos do ERP no escopo raiz. O foco aqui e leitura comercial por loja e consultor.
        </p>
      </div>

      <form class="crm-filters" @submit.prevent="submitFilters">
        <label class="crm-filters__field">
          <span>De</span>
          <input v-model="dateFrom" class="crm-filters__input" type="date">
        </label>

        <label class="crm-filters__field">
          <span>Ate</span>
          <input v-model="dateTo" class="crm-filters__input" type="date">
        </label>

        <div class="crm-filters__actions">
          <button class="crm-btn crm-btn--ghost" type="button" @click="resetMonth">Mes atual</button>
          <button class="crm-btn" type="submit" :disabled="pending">{{ pending ? "Atualizando..." : "Atualizar" }}</button>
        </div>
      </form>
    </header>

    <article v-if="errorMessage" class="settings-card">
      <p class="settings-card__text">{{ errorMessage }}</p>
    </article>

    <article v-else-if="pending && !ready" class="settings-card">
      <p class="settings-card__text">Carregando CRM do ERP...</p>
    </article>

    <section v-else class="crm-panel__content">
      <article class="crm-hero">
        <div class="crm-hero__copy">
          <span class="crm-hero__eyebrow">% Meta do periodo</span>
          <strong class="crm-hero__value">{{ formatPercent(summary.goalProgress) }}</strong>
          <p class="crm-hero__text">
            {{ formatCurrencyFromCents(summary.salesCents) }} vendidos sobre {{ formatCurrencyFromCents(summary.monthlyGoalCents) }} de meta consolidada.
          </p>
        </div>

        <div class="crm-progress-card">
          <div class="crm-progress-card__track">
            <div class="crm-progress-card__fill" :class="progressClass(summary.goalProgress)" :style="{ width: summaryProgressWidth }" />
          </div>
          <div class="crm-progress-card__meta">
            <span>Falta {{ formatCurrencyFromCents(summary.remainingToGoalCents) }}</span>
            <span v-if="summary.unmappedSalesCents">Nao mapeado: {{ formatCurrencyFromCents(summary.unmappedSalesCents) }}</span>
          </div>
        </div>
      </article>

      <section class="metric-grid crm-metrics">
        <article class="metric-card">
          <span class="metric-card__label">Vendas do periodo</span>
          <strong class="metric-card__value">{{ formatCurrencyFromCents(summary.salesCents) }}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Ticket medio</span>
          <strong class="metric-card__value">{{ formatCurrencyFromCents(summary.ticketAverageCents) }}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Valor por produto</span>
          <strong class="metric-card__value">{{ formatCurrencyFromCents(summary.valuePerProductCents) }}</strong>
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

      <article v-if="warnings.length" class="crm-warning-list">
        <p v-for="warning in warnings" :key="warning" class="crm-warning-list__item">{{ warning }}</p>
      </article>

      <article class="insight-card insight-card--wide">
        <header class="crm-section__header">
          <div>
            <h3 class="insight-card__title">Lojas mapeadas</h3>
            <p class="insight-card__text">Meta da loja cadastrada no sistema comparada com a venda vinda do ERP.</p>
          </div>
          <span class="crm-section__meta">{{ overview?.dateFrom }} ate {{ overview?.dateTo }}</span>
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
                <th>Valor por produto</th>
                <th>P.A.</th>
                <th>Pedidos</th>
                <th>Produtos</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in commercialStoreRows" :key="row.storeSlug">
                <td>
                  <div class="crm-row-heading">
                    <strong>{{ row.storeLabel }}</strong>
                    <small>{{ row.storeCode || "Sem codigo" }}</small>
                  </div>
                </td>
                <td>{{ formatCurrencyFromCents(row.monthlyGoalCents) }}</td>
                <td>{{ formatCurrencyFromCents(row.salesCents) }}</td>
                <td>
                  <div class="crm-table-progress">
                    <span class="crm-table-progress__track">
                      <span class="crm-table-progress__fill" :class="progressClass(row.goalProgress)" :style="{ width: progressWidth(row.goalProgress) }" />
                    </span>
                    <strong>{{ formatPercent(row.goalProgress) }}</strong>
                  </div>
                </td>
                <td>{{ formatCurrencyFromCents(row.remainingToGoalCents) }}</td>
                <td>{{ formatCurrencyFromCents(row.ticketAverageCents) }}</td>
                <td>{{ formatCurrencyFromCents(row.valuePerProductCents) }}</td>
                <td>{{ formatPA(row.paScore) }}</td>
                <td>{{ formatNumber(row.orders) }}</td>
                <td>{{ formatNumber(row.units) }}</td>
              </tr>
              <tr v-if="!commercialStoreRows.length">
                <td class="crm-empty" colspan="10">Nenhuma loja com vendas ERP no periodo selecionado.</td>
              </tr>
            </tbody>
          </table>
        </div>
      </article>

      <article v-if="managementStoreRows.length" class="insight-card insight-card--wide">
        <header class="crm-section__header">
          <div>
            <h3 class="insight-card__title">Gerencia / Multi-loja</h3>
            <p class="insight-card__text">Pedidos sem loja comercial confiavel para atribuicao direta dentro das lojas.</p>
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
                <th>Valor por produto</th>
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
                <td>{{ formatCurrencyFromCents(row.valuePerProductCents) }}</td>
                <td>{{ formatPA(row.paScore) }}</td>
                <td>{{ formatNumber(row.orders) }}</td>
                <td>{{ formatNumber(row.units) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </article>

      <article class="insight-card insight-card--wide">
        <header class="crm-section__header">
          <div>
            <h3 class="insight-card__title">Ticket medio e P.A. por consultor</h3>
            <p class="insight-card__text">Leitura comercial individual derivada diretamente dos pedidos ERP do periodo.</p>
          </div>
          <span class="crm-section__meta">{{ commercialConsultantRows.length }} consultor(es)</span>
        </header>

        <div class="insight-table-wrap">
          <table class="insight-table crm-table">
            <thead>
              <tr>
                <th>Consultor</th>
                <th>Loja</th>
                <th>Vendido</th>
                <th>Ticket medio</th>
                <th>Valor por produto</th>
                <th>P.A.</th>
                <th>Pedidos</th>
                <th>Produtos</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in commercialConsultantRows" :key="`${row.consultantId}-${row.storeSlug}-${row.storeCnpj || ''}`">
                <td>
                  <div class="crm-row-heading">
                    <strong>{{ row.consultantName }}</strong>
                    <small>{{ row.consultantId }}</small>
                  </div>
                </td>
                <td>{{ row.storeLabel }}</td>
                <td>{{ formatCurrencyFromCents(row.salesCents) }}</td>
                <td>{{ formatCurrencyFromCents(row.ticketAverageCents) }}</td>
                <td>{{ formatCurrencyFromCents(row.valuePerProductCents) }}</td>
                <td>{{ formatPA(row.paScore) }}</td>
                <td>{{ formatNumber(row.orders) }}</td>
                <td>{{ formatNumber(row.units) }}</td>
              </tr>
              <tr v-if="!commercialConsultantRows.length">
                <td class="crm-empty" colspan="8">Nenhum consultor com pedidos ERP no periodo selecionado.</td>
              </tr>
            </tbody>
          </table>
        </div>
      </article>

      <article v-if="managementConsultantRows.length" class="insight-card insight-card--wide">
        <header class="crm-section__header">
          <div>
            <h3 class="insight-card__title">Gerencia / Multi-loja por consultor</h3>
            <p class="insight-card__text">Consultores com pedidos sem loja comercial suficientemente confiavel no ERP.</p>
          </div>
          <span class="crm-section__meta">{{ managementConsultantRows.length }} consultor(es)</span>
        </header>

        <div class="insight-table-wrap">
          <table class="insight-table crm-table">
            <thead>
              <tr>
                <th>Consultor</th>
                <th>Grupo</th>
                <th>Vendido</th>
                <th>Ticket medio</th>
                <th>Valor por produto</th>
                <th>P.A.</th>
                <th>Pedidos</th>
                <th>Produtos</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in managementConsultantRows" :key="`${row.consultantId}-${row.storeSlug}-${row.storeCnpj || ''}`">
                <td>
                  <div class="crm-row-heading">
                    <strong>{{ row.consultantName }}</strong>
                    <small>{{ row.consultantId }}</small>
                  </div>
                </td>
                <td>{{ row.storeLabel }}</td>
                <td>{{ formatCurrencyFromCents(row.salesCents) }}</td>
                <td>{{ formatCurrencyFromCents(row.ticketAverageCents) }}</td>
                <td>{{ formatCurrencyFromCents(row.valuePerProductCents) }}</td>
                <td>{{ formatPA(row.paScore) }}</td>
                <td>{{ formatNumber(row.orders) }}</td>
                <td>{{ formatNumber(row.units) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </article>
    </section>
  </section>
</template>

<style scoped>
.crm-panel {
  gap: 1.25rem;
}

.crm-panel__header {
  align-items: flex-start;
  gap: 1rem;
}

.crm-panel__content {
  display: grid;
  gap: 1rem;
}

.crm-filters {
  display: grid;
  grid-template-columns: repeat(2, minmax(150px, 1fr)) auto;
  gap: 0.75rem;
  align-items: end;
  min-width: min(100%, 520px);
}

.crm-filters__field {
  display: grid;
  gap: 0.35rem;
}

.crm-filters__field span {
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: rgba(15, 23, 42, 0.65);
}

.crm-filters__input {
  width: 100%;
  min-height: 42px;
  border: 1px solid rgba(148, 163, 184, 0.35);
  border-radius: 12px;
  padding: 0.75rem 0.9rem;
  background: rgba(255, 255, 255, 0.95);
  color: #0f172a;
}

.crm-filters__actions {
  display: flex;
  gap: 0.5rem;
}

.crm-btn {
  min-height: 42px;
  border: none;
  border-radius: 12px;
  padding: 0.75rem 1rem;
  background: #0f766e;
  color: #f8fafc;
  font-weight: 700;
  cursor: pointer;
}

.crm-btn:disabled {
  cursor: wait;
  opacity: 0.72;
}

.crm-btn--ghost {
  background: rgba(15, 118, 110, 0.12);
  color: #115e59;
}

.crm-hero {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr);
  gap: 1rem;
  padding: 1.25rem;
  border-radius: 24px;
  background: linear-gradient(135deg, #082f49 0%, #164e63 50%, #0f766e 100%);
  color: #f8fafc;
}

.crm-hero__copy {
  display: grid;
  gap: 0.5rem;
}

.crm-hero__eyebrow {
  font-size: 0.78rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgba(226, 232, 240, 0.82);
}

.crm-hero__value {
  font-size: clamp(2rem, 4vw, 3.2rem);
  line-height: 1;
}

.crm-hero__text {
  max-width: 38rem;
  color: rgba(241, 245, 249, 0.88);
}

.crm-progress-card {
  align-self: center;
  display: grid;
  gap: 0.75rem;
  padding: 1rem;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.12);
  backdrop-filter: blur(10px);
}

.crm-progress-card__track,
.crm-table-progress__track {
  position: relative;
  display: block;
  width: 100%;
  height: 12px;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(226, 232, 240, 0.24);
}

.crm-progress-card__fill,
.crm-table-progress__fill {
  position: absolute;
  inset: 0 auto 0 0;
  border-radius: inherit;
  background: linear-gradient(90deg, #fb7185 0%, #fbbf24 52%, #34d399 100%);
}

.crm-progress-card__fill.is-hit,
.crm-table-progress__fill.is-hit {
  background: linear-gradient(90deg, #10b981 0%, #34d399 100%);
}

.crm-progress-card__fill.is-near,
.crm-table-progress__fill.is-near {
  background: linear-gradient(90deg, #f59e0b 0%, #fbbf24 100%);
}

.crm-progress-card__fill.is-miss,
.crm-table-progress__fill.is-miss {
  background: linear-gradient(90deg, #f97316 0%, #fb7185 100%);
}

.crm-progress-card__meta {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
  font-size: 0.92rem;
  color: rgba(248, 250, 252, 0.88);
}

.crm-metrics {
  grid-template-columns: repeat(6, minmax(0, 1fr));
}

.crm-warning-list {
  display: grid;
  gap: 0.5rem;
}

.crm-warning-list__item {
  margin: 0;
  padding: 0.85rem 1rem;
  border-radius: 14px;
  background: rgba(249, 115, 22, 0.12);
  color: #9a3412;
}

.crm-section__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
  margin-bottom: 1rem;
}

.crm-section__meta {
  color: rgba(15, 23, 42, 0.58);
  font-size: 0.88rem;
}

.crm-table {
  min-width: 1080px;
}

.crm-row-heading {
  display: grid;
  gap: 0.2rem;
}

.crm-row-heading small {
  color: rgba(15, 23, 42, 0.6);
}

.crm-empty {
  padding: 1rem;
  text-align: center;
  color: rgba(15, 23, 42, 0.62);
}

.crm-table-progress {
  display: grid;
  gap: 0.35rem;
  min-width: 120px;
}

.crm-table-progress strong {
  font-size: 0.85rem;
}

@media (max-width: 1100px) {
  .crm-metrics {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 860px) {
  .crm-panel__header,
  .crm-hero,
  .crm-section__header {
    grid-template-columns: 1fr;
    display: grid;
  }

  .crm-filters {
    grid-template-columns: 1fr;
  }

  .crm-filters__actions {
    width: 100%;
  }

  .crm-btn {
    flex: 1 1 0;
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