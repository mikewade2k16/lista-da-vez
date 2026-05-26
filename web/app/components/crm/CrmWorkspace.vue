<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { CalendarDays } from 'lucide-vue-next'

import { formatCurrencyBRL, formatPercent } from '~/domain/utils/admin-metrics'
import { useCrmStore } from '~/stores/crm'
import type { QueueConsultantStats } from '~/stores/crm'

const crmStore = useCrmStore()
const { overview, pending, ready, errorMessage, dateFrom, dateTo } = storeToRefs(crmStore)

// filtros locais (sem nova requisição)
const selectedStore = ref('')
const consultantSearch = ref('')

const summary = computed(
  () =>
    overview.value?.summary || {
      orders: 0,
      units: 0,
      salesCents: 0,
      ticketAverageCents: 0,
      valuePerProductCents: 0,
      paScore: 0,
      monthlyGoalCents: 0,
      goalProgress: 0,
      remainingToGoalCents: 0,
      unmappedSalesCents: 0,
    },
)
const storeRows = computed(() => overview.value?.stores || [])
const consultantRows = computed(() => overview.value?.consultants || [])
const queueStats = computed(() => overview.value?.queueStats || null)
const warnings = computed(() => overview.value?.warnings || [])

const managementStoreSlug = 'gerencia-multiloja'

const commercialStoreRows = computed(() =>
  storeRows.value.filter((row) => row.storeSlug !== managementStoreSlug),
)
const managementStoreRows = computed(() =>
  storeRows.value.filter((row) => row.storeSlug === managementStoreSlug),
)

const storeOptions = computed(() =>
  commercialStoreRows.value.map((s) => ({ slug: s.storeSlug, label: s.storeLabel })),
)

const filteredStoreRows = computed(() => {
  if (!selectedStore.value) return commercialStoreRows.value
  return commercialStoreRows.value.filter((s) => s.storeSlug === selectedStore.value)
})

// merge ERP × fila por nome normalizado
type MergedConsultant = (typeof consultantRows.value)[number] & {
  queue?: QueueConsultantStats
  matched: boolean
}

const queueByName = computed(() => {
  const map = new Map<string, QueueConsultantStats>()
  for (const q of queueStats.value?.byConsultant ?? []) {
    map.set(q.personName.trim().toLowerCase(), q)
  }
  return map
})

const mergedConsultants = computed<MergedConsultant[]>(() => {
  const search = consultantSearch.value.trim().toLowerCase()
  return consultantRows.value
    .filter((c) => c.storeSlug !== managementStoreSlug)
    .filter((c) => !selectedStore.value || c.storeSlug === selectedStore.value)
    .filter((c) => !search || c.consultantName.toLowerCase().includes(search))
    .map((c) => {
      const queue = queueByName.value.get(c.consultantName.trim().toLowerCase())
      return { ...c, queue, matched: !!queue }
    })
})

const managementConsultantRows = computed(() =>
  consultantRows.value.filter((row) => row.storeSlug === managementStoreSlug),
)

const summaryProgressWidth = computed(
  () => `${Math.min(100, Number(summary.value.goalProgress || 0)).toFixed(1)}%`,
)

// consultores ERP sem correspondente na fila
const unmatchedCount = computed(
  () => mergedConsultants.value.filter((c) => !c.matched && queueStats.value).length,
)

function formatCurrencyFromCents(value?: number | null) {
  return formatCurrencyBRL((Number(value || 0) || 0) / 100)
}

function formatNumber(value?: number | null) {
  return Number(value || 0).toLocaleString('pt-BR')
}

function formatPA(value?: number | null) {
  return Number(value || 0).toFixed(2)
}

function formatPct(value?: number | null) {
  const n = Number(value || 0)
  return n ? `${n.toFixed(1)}%` : '-'
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

async function submitFilters() {
  await crmStore.applyFilters()
}

async function resetMonth() {
  crmStore.resetCurrentMonth()
  await crmStore.applyFilters()
}
</script>

<template>
  <section class="admin-panel crm-panel" data-testid="crm-panel">
    <header class="admin-panel__header crm-panel__header">
      <div>
        <h2 class="admin-panel__title">CRM comercial via ERP</h2>
        <p class="admin-panel__text">
          Metas cadastradas no sistema cruzadas com pedidos do ERP. Leitura comercial por loja e
          consultor.
        </p>
      </div>

      <!-- filtros de periodo -->
      <form class="crm-filters" @submit.prevent="submitFilters">
        <div class="crm-filters__date-wrap">
          <label class="crm-filters__label">Periodo</label>
          <AppDatePicker
            :model-value="dateFrom"
            :end-date="dateTo"
            @update:model-value="dateFrom = $event"
            @update:end-date="dateTo = $event"
          >
            <template #default="{ label }">
              <button type="button" class="crm-date-trigger">
                <CalendarDays :size="14" />
                <span>{{ label || 'Todas as vendas' }}</span>
              </button>
            </template>
          </AppDatePicker>
        </div>

        <div class="crm-filters__actions">
          <button class="crm-btn crm-btn--ghost" type="button" @click="resetMonth">
            Mes atual
          </button>
          <button class="crm-btn" type="submit" :disabled="pending">
            {{ pending ? 'Atualizando...' : 'Atualizar' }}
          </button>
        </div>
      </form>
    </header>

    <!-- filtros de loja e consultor (local, sem nova requisição) -->
    <div v-if="ready" class="crm-local-filters">
      <select v-model="selectedStore" class="crm-select" title="Filtrar por loja">
        <option value="">Todas as lojas</option>
        <option v-for="s in storeOptions" :key="s.slug" :value="s.slug">
          {{ s.label }}
        </option>
      </select>

      <input
        v-model="consultantSearch"
        class="crm-search"
        type="text"
        placeholder="Buscar consultor..."
        autocomplete="off"
      />

      <button
        v-if="selectedStore || consultantSearch"
        class="crm-btn crm-btn--ghost crm-btn--sm"
        type="button"
        @click="
          selectedStore = ''
          consultantSearch = ''
        "
      >
        Limpar filtros
      </button>
    </div>

    <article v-if="errorMessage" class="settings-card">
      <p class="settings-card__text">{{ errorMessage }}</p>
    </article>

    <article v-else-if="pending && !ready" class="settings-card">
      <p class="settings-card__text">Carregando CRM do ERP...</p>
    </article>

    <section v-else class="crm-panel__content">
      <!-- hero de meta -->
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

      <!-- métricas ERP -->
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

      <!-- indicadores de fila -->
      <section v-if="queueStats" class="crm-queue-metrics">
        <h3 class="crm-section-label">Fila de atendimento — periodo selecionado</h3>
        <div class="metric-grid crm-queue-grid">
          <article class="metric-card crm-queue-card">
            <span class="metric-card__label">Atendimentos</span>
            <strong class="metric-card__value">
              {{ formatNumber(queueStats.totalAttendances) }}
            </strong>
          </article>
          <article class="metric-card crm-queue-card">
            <span class="metric-card__label">Conversoes (fila)</span>
            <strong class="metric-card__value">
              {{ formatNumber(queueStats.totalConversions) }}
            </strong>
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
            <small class="crm-metric-sub">
              {{ formatNumber(summary.erpCancellations) }} pedidos
            </small>
          </article>
        </div>
      </section>

      <!-- aviso de não mapeados -->
      <article v-if="warnings.length" class="crm-warning-list">
        <p v-for="warning in warnings" :key="warning" class="crm-warning-list__item">
          {{ warning }}
        </p>
      </article>

      <!-- aviso de consultores sem match na fila -->
      <article v-if="unmatchedCount > 0" class="crm-warning-list">
        <p class="crm-warning-list__item crm-warning-list__item--info">
          {{ unmatchedCount }} consultor(es) ERP sem correspondente identificado na fila (nomes nao
          coincidem).
        </p>
      </article>

      <!-- tabela por loja -->
      <article class="insight-card insight-card--wide">
        <header class="crm-section__header">
          <div>
            <h3 class="insight-card__title">Lojas mapeadas</h3>
            <p class="insight-card__text">Meta da loja vs venda ERP no periodo.</p>
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
                  <span v-if="queueStats.byStore">
                    {{
                      formatPct(
                        queueStats.byStore.find((s) => s.storeId === row.storeSlug)?.conversionRate,
                      )
                    }}
                  </span>
                  <span v-else class="crm-muted">-</span>
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

      <!-- tabela por consultor com merge fila -->
      <article class="insight-card insight-card--wide">
        <header class="crm-section__header">
          <div>
            <h3 class="insight-card__title">Indicadores por consultor</h3>
            <p class="insight-card__text">
              ERP + fila de atendimento. Colunas de fila marcadas com (F) cruzadas por nome.
            </p>
          </div>
          <span class="crm-section__meta">{{ mergedConsultants.length }} consultor(es)</span>
        </header>

        <div class="insight-table-wrap">
          <table class="insight-table crm-table crm-table--consultants">
            <thead>
              <tr>
                <th>Consultor</th>
                <th>Loja</th>
                <th>Vendido</th>
                <th>Ticket medio</th>
                <th>P.A.</th>
                <th>Pedidos</th>
                <th>Atend. (F)</th>
                <th>Conversao (F)</th>
                <th>Canc. fila (F)</th>
                <th>Status fila</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in mergedConsultants"
                :key="`${row.consultantId}-${row.storeSlug}-${row.storeCnpj || ''}`"
                :class="{ 'crm-tr--unmatched': !row.matched && queueStats }"
              >
                <td>
                  <div class="crm-row-heading">
                    <strong>{{ row.consultantName }}</strong>
                    <small class="crm-muted">{{ row.consultantId }}</small>
                  </div>
                </td>
                <td>{{ row.storeLabel }}</td>
                <td>{{ formatCurrencyFromCents(row.salesCents) }}</td>
                <td>{{ formatCurrencyFromCents(row.ticketAverageCents) }}</td>
                <td>{{ formatPA(row.paScore) }}</td>
                <td>{{ formatNumber(row.orders) }}</td>
                <td :class="{ 'crm-td--queue': row.queue }">
                  {{ row.queue ? formatNumber(row.queue.attendances) : '-' }}
                </td>
                <td>
                  <span
                    v-if="row.queue"
                    :class="{ 'crm-rate--good': row.queue.conversionRate >= 30 }"
                  >
                    {{ formatPct(row.queue.conversionRate) }}
                  </span>
                  <span v-else class="crm-muted">-</span>
                </td>
                <td>
                  <span
                    v-if="row.queue"
                    :class="{ 'crm-rate--bad': row.queue.queueCancellationRate > 10 }"
                  >
                    {{ formatPct(row.queue.queueCancellationRate) }}
                  </span>
                  <span v-else class="crm-muted">-</span>
                </td>
                <td>
                  <span v-if="!queueStats" class="crm-badge crm-badge--neutral">
                    sem dados fila
                  </span>
                  <span v-else-if="row.matched" class="crm-badge crm-badge--ok">identificado</span>
                  <span v-else class="crm-badge crm-badge--warn">nao identificado</span>
                </td>
              </tr>
              <tr v-if="!mergedConsultants.length">
                <td class="crm-empty" colspan="10">
                  Nenhum consultor com pedidos ERP no periodo selecionado.
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </article>

      <!-- gerência multi-loja -->
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

      <article v-if="managementConsultantRows.length" class="insight-card insight-card--wide">
        <header class="crm-section__header">
          <div>
            <h3 class="insight-card__title">Gerencia / Multi-loja por consultor</h3>
            <p class="insight-card__text">
              Consultores com pedidos sem loja comercial suficientemente confiavel.
            </p>
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
                <th>P.A.</th>
                <th>Pedidos</th>
                <th>Produtos</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in managementConsultantRows"
                :key="`${row.consultantId}-${row.storeSlug}-${row.storeCnpj || ''}`"
              >
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

/* filtros de periodo */
.crm-filters {
  display: flex;
  gap: 0.75rem;
  align-items: flex-end;
  flex-wrap: wrap;
}

.crm-filters__date-wrap {
  display: grid;
  gap: 0.3rem;
}

.crm-filters__label {
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.07em;
  text-transform: uppercase;
  color: rgb(var(--muted));
}

.crm-filters__actions {
  display: flex;
  gap: 0.5rem;
}

.crm-date-trigger {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 13rem;
  min-height: 42px;
  padding: 0 0.85rem;
  border-radius: 12px;
  border: 1px solid rgb(var(--border) / 0.9);
  background: rgb(var(--surface) / 0.95);
  color: rgb(var(--text));
  font-size: 0.88rem;
  font-weight: 600;
  text-align: left;
  cursor: pointer;
  white-space: nowrap;
}

/* filtros locais */
.crm-local-filters {
  display: flex;
  gap: 0.75rem;
  align-items: center;
  flex-wrap: wrap;
}

.crm-select,
.crm-search {
  min-height: 38px;
  padding: 0 0.85rem;
  border-radius: 10px;
  border: 1px solid rgb(var(--border) / 0.88);
  background: rgb(var(--surface) / 0.95);
  color: rgb(var(--text));
  font-size: 0.88rem;
}

.crm-select {
  min-width: 160px;
  cursor: pointer;
}

.crm-search {
  min-width: 200px;
}

.crm-search::placeholder {
  color: rgb(var(--muted) / 0.72);
}

/* botoes */
.crm-btn {
  min-height: 42px;
  border: none;
  border-radius: 12px;
  padding: 0.75rem 1rem;
  background: rgb(var(--primary));
  color: rgb(255 255 255);
  font-weight: 700;
  cursor: pointer;
  white-space: nowrap;
}

.crm-btn:disabled {
  cursor: wait;
  opacity: 0.72;
}

.crm-btn--ghost {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.crm-btn--sm {
  min-height: 38px;
  padding: 0.45rem 0.75rem;
  font-size: 0.84rem;
  border-radius: 10px;
}

/* hero */
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

.crm-progress-card__track,
.crm-table-progress__track {
  position: relative;
  display: block;
  width: 100%;
  height: 12px;
  overflow: hidden;
  border-radius: 999px;
  background: rgb(255 255 255 / 0.24);
}

.crm-progress-card__fill,
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

.crm-progress-card__fill.is-hit,
.crm-table-progress__fill.is-hit {
  background: linear-gradient(90deg, rgb(var(--success) / 0.82) 0%, rgb(var(--success)) 100%);
}

.crm-progress-card__fill.is-near,
.crm-table-progress__fill.is-near {
  background: linear-gradient(90deg, rgb(var(--primary-600)) 0%, rgb(var(--primary)) 100%);
}

.crm-progress-card__fill.is-miss,
.crm-table-progress__fill.is-miss {
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

/* métricas ERP */
.crm-metrics {
  grid-template-columns: repeat(6, minmax(0, 1fr));
}

/* indicadores fila */
.crm-queue-metrics {
  display: grid;
  gap: 0.6rem;
}

.crm-section-label {
  font-size: 0.75rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: rgb(var(--muted));
  margin: 0;
}

.crm-queue-grid {
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
}

.crm-queue-card {
  border-left: 3px solid rgb(var(--primary) / 0.35);
}

.crm-bar {
  height: 4px;
  border-radius: 2px;
  background: rgb(var(--border) / 0.68);
  overflow: hidden;
  margin-top: 0.3rem;
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

.crm-rate--bad {
  color: rgb(var(--danger));
  font-weight: 700;
}

.crm-metric-sub {
  font-size: 0.72rem;
  color: rgb(var(--muted));
}

/* avisos */
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

/* section header */
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

/* tabelas */
.crm-table {
  min-width: 860px;
}

.crm-table--consultants {
  min-width: 1100px;
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

.crm-muted {
  color: rgb(var(--muted) / 0.72);
}

.crm-table-progress {
  display: grid;
  gap: 0.35rem;
  min-width: 120px;
}

.crm-table-progress strong {
  font-size: 0.85rem;
}

.crm-td--queue {
  color: rgb(var(--success));
  font-weight: 600;
}

.crm-tr--unmatched td {
  opacity: 0.75;
}

/* badges */
.crm-badge {
  display: inline-block;
  padding: 0.2rem 0.55rem;
  border-radius: 6px;
  font-size: 0.72rem;
  font-weight: 700;
  white-space: nowrap;
}

.crm-badge--ok {
  background: rgb(var(--success) / 0.12);
  color: rgb(var(--success));
}

.crm-badge--warn {
  background: rgb(var(--primary) / 0.14);
  color: rgb(var(--primary));
}

.crm-badge--neutral {
  background: rgb(var(--border) / 0.56);
  color: rgb(var(--muted));
}

/* responsive */
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
    flex-direction: column;
    align-items: flex-start;
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
