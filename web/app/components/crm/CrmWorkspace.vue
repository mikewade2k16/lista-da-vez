<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { CalendarDays } from 'lucide-vue-next'

import { useAuthStore } from '~/stores/auth'
import { useAppRuntimeStore } from '~/stores/app-runtime'
import { useCrmStore } from '~/stores/crm'
import { useCrmConsultantMetrics } from '~/composables/useCrmConsultantMetrics'
import { buildCrmListUsageSummary } from '~/domain/utils/crm-list-usage'
import {
  normalizeCrmGoalPayoutPolicy,
  normalizeCrmListUsageMinOrders,
  normalizeCrmListUsageTiers,
} from '~/domain/utils/crm-performance-policy'
import type { CRMSummary, QueueStats } from '~/stores/crm'

const crmStore = useCrmStore()
const runtime = useAppRuntimeStore()
const auth = useAuthStore()
const { overview, pending, ready, errorMessage, dateFrom, dateTo } = storeToRefs(crmStore)
const { state: runtimeState } = storeToRefs(runtime)

const managementStoreSlug = 'gerencia-multiloja'
const selectedStore = ref('')

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
      erpCancellations: 0,
      erpCancellationRate: 0,
    },
)
const canManageConsultantLinks = computed(() => auth.role === 'platform_admin')
const storeRows = computed(() => overview.value?.stores || [])
const consultantRows = computed(() => overview.value?.consultants || [])
const queueStats = computed(() => overview.value?.queueStats || null)
const warnings = computed(() => overview.value?.warnings || [])
const runtimeSettings = computed(() => runtimeState.value?.settings || {})

const commercialStoreRows = computed(() =>
  storeRows.value.filter((row) => row.storeSlug !== managementStoreSlug),
)
const managementStoreRows = computed(() =>
  storeRows.value.filter((row) => row.storeSlug === managementStoreSlug),
)

const storeOptions = computed(() => [
  { value: '', label: 'Todas as lojas' },
  ...commercialStoreRows.value.map((row) => ({
    value: row.storeSlug,
    label: row.storeLabel,
    meta: row.storeCode || '',
  })),
])

const filteredStoreRows = computed(() => {
  if (!selectedStore.value) return commercialStoreRows.value
  return commercialStoreRows.value.filter((row) => row.storeSlug === selectedStore.value)
})

const summaryRows = computed(() => {
  if (!selectedStore.value) return storeRows.value
  return storeRows.value.filter((row) => row.storeSlug === selectedStore.value)
})

const displaySummary = computed<CRMSummary>(() => {
  if (!selectedStore.value) return summary.value
  return buildSummaryFromStoreRows(summaryRows.value)
})

const displayQueueStats = computed<QueueStats | null>(() => {
  if (!selectedStore.value || !queueStats.value) return queueStats.value
  return buildQueueStatsForStore(queueStats.value, selectedStore.value)
})

const {
  erpStore,
  mergedConsultants,
  managementConsultantRows,
  unmatchedCount,
  consultantLinkOptions,
  consultantLinkDraftByRow,
  refreshConsultantLinks,
  autoLinkConsultants,
  saveConsultantLink,
  removeConsultantLink,
  updateConsultantLinkDraft,
  consultantLinkKey,
  linkStatusLabel,
  linkStatusClass,
} = useCrmConsultantMetrics({
  consultantRows,
  queueStats,
  ready,
  canManageConsultantLinks,
})

const displayMergedConsultants = computed(() => {
  if (!selectedStore.value) return mergedConsultants.value
  return mergedConsultants.value.filter((row) => row.storeSlug === selectedStore.value)
})

const listUsageTiers = computed(() =>
  normalizeCrmListUsageTiers(runtimeSettings.value.crmListUsageTiers),
)
const listUsageMinOrdersForHighlight = computed(() =>
  normalizeCrmListUsageMinOrders(runtimeSettings.value.crmListUsageMinOrdersForHighlight),
)
const goalPayoutPolicy = computed(() =>
  normalizeCrmGoalPayoutPolicy(runtimeSettings.value.crmGoalPayoutPolicy),
)
const listUsageSummary = computed(() =>
  buildCrmListUsageSummary(displayMergedConsultants.value, {
    tiers: listUsageTiers.value,
    minOrdersForHighlight: listUsageMinOrdersForHighlight.value,
  }),
)
const storeGoalProgressBySlug = computed(() =>
  Object.fromEntries(
    storeRows.value
      .map((row) => [String(row.storeSlug || '').trim(), Number(row.goalProgress || 0)])
      .filter(([slug]) => slug),
  ),
)

async function submitFilters() {
  await crmStore.applyFilters()
}

async function resetMonth() {
  crmStore.resetCurrentMonth()
  await crmStore.applyFilters()
}

async function setPreviousMonth() {
  const now = new Date()
  const start = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - 1, 1))
  const end = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 0))
  dateFrom.value = formatDateInput(start)
  dateTo.value = formatDateInput(end)
  await crmStore.applyFilters()
}

function clearLocalFilters() {
  selectedStore.value = ''
}

function updateSelectedStore(event: Event) {
  selectedStore.value = (event.target as HTMLSelectElement | null)?.value || ''
}

function formatDateInput(date: Date) {
  return [
    date.getUTCFullYear(),
    String(date.getUTCMonth() + 1).padStart(2, '0'),
    String(date.getUTCDate()).padStart(2, '0'),
  ].join('-')
}

function buildSummaryFromStoreRows(rows: Array<Record<string, unknown>>): CRMSummary {
  const next: CRMSummary = {
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
    erpCancellations: 0,
    erpCancellationRate: 0,
  }

  let productSalesCents = 0
  for (const row of rows) {
    const orders = Number(row.orders || 0)
    const units = Number(row.units || 0)
    const salesCents = Number(row.salesCents || 0)
    const valuePerProductCents = Number(row.valuePerProductCents || 0)

    next.orders += orders
    next.units += units
    next.salesCents += salesCents
    next.erpCancellations = Number(next.erpCancellations || 0) + Number(row.erpCancellations || 0)

    if (row.mapped !== false) {
      next.monthlyGoalCents += Number(row.monthlyGoalCents || 0)
    } else {
      next.unmappedSalesCents = Number(next.unmappedSalesCents || 0) + salesCents
    }

    productSalesCents +=
      valuePerProductCents > 0 && units > 0 ? valuePerProductCents * units : salesCents
  }

  next.ticketAverageCents = next.orders > 0 ? Math.round(next.salesCents / next.orders) : 0
  next.valuePerProductCents = next.units > 0 ? Math.round(productSalesCents / next.units) : 0
  next.paScore = next.orders > 0 ? Math.max(next.units, next.orders) / next.orders : 0
  next.remainingToGoalCents = Math.max(0, next.monthlyGoalCents - next.salesCents)
  next.goalProgress =
    next.monthlyGoalCents > 0 ? (next.salesCents / next.monthlyGoalCents) * 100 : 0

  const totalERP = next.orders + Number(next.erpCancellations || 0)
  next.erpCancellationRate =
    totalERP > 0 && Number(next.erpCancellations || 0) > 0
      ? (Number(next.erpCancellations || 0) / totalERP) * 100
      : 0

  return next
}

function buildQueueStatsForStore(stats: QueueStats, storeSlug: string): QueueStats | null {
  const byStore = (stats.byStore || []).filter((row) => row.storeSlug === storeSlug)
  const byConsultant = (stats.byConsultant || []).filter((row) => row.storeSlug === storeSlug)
  const totalAttendances = byStore.reduce((sum, row) => sum + Number(row.attendances || 0), 0)
  const totalConversions = byStore.reduce((sum, row) => sum + Number(row.conversions || 0), 0)
  const totalCancellations = byStore.reduce(
    (sum, row) => sum + Number(row.queueCancellations || 0),
    0,
  )

  return {
    totalAttendances,
    totalConversions,
    totalCancellations,
    conversionRate: totalAttendances > 0 ? (totalConversions / totalAttendances) * 100 : 0,
    cancellationRate: totalAttendances > 0 ? (totalCancellations / totalAttendances) * 100 : 0,
    byStore,
    byConsultant,
  }
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

        <button class="crm-btn crm-btn--ghost" type="button" @click="setPreviousMonth">
          Mes anterior
        </button>

        <button class="crm-btn crm-btn--ghost" type="button" @click="resetMonth">Mes atual</button>

        <div class="crm-filters__field crm-filters__store-wrap">
          <label class="crm-filters__label">Loja</label>
          <select
            class="crm-filters__store"
            :value="selectedStore"
            :disabled="!ready"
            @change="updateSelectedStore"
          >
            <option
              v-for="option in storeOptions"
              :key="option.value || 'all'"
              :value="option.value"
            >
              {{ option.label }}
            </option>
          </select>
        </div>

        <div class="crm-filters__actions">
          <button class="crm-btn" type="submit" :disabled="pending">
            {{ pending ? 'Atualizando...' : 'Atualizar' }}
          </button>
          <button
            v-if="selectedStore"
            class="crm-btn crm-btn--ghost"
            type="button"
            @click="clearLocalFilters"
          >
            Limpar
          </button>
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
      <CrmSummarySection
        :summary="displaySummary"
        :queue-stats="displayQueueStats"
        :list-usage-summary="listUsageSummary"
        :warnings="warnings"
        :unmatched-count="unmatchedCount"
      />

      <CrmStoresSection
        :filtered-store-rows="filteredStoreRows"
        :management-store-rows="managementStoreRows"
        :queue-stats="displayQueueStats"
        :summary="displaySummary"
        :date-from="overview?.dateFrom"
        :date-to="overview?.dateTo"
      />

      <CrmConsultantsSection
        :merged-consultants="displayMergedConsultants"
        :management-consultant-rows="managementConsultantRows"
        :store-goal-progress-by-slug="storeGoalProgressBySlug"
        :goal-payout-policy="goalPayoutPolicy"
        :list-usage-tiers="listUsageTiers"
        :can-manage-consultant-links="canManageConsultantLinks"
        :loading-consultant-links="erpStore.loadingConsultantLinks"
        :saving-consultant-link="erpStore.savingConsultantLink"
        :consultant-link-options="consultantLinkOptions"
        :consultant-link-drafts="consultantLinkDraftByRow"
        :queue-stats-available="!!queueStats"
        :consultant-link-key="consultantLinkKey"
        :link-status-label="linkStatusLabel"
        :link-status-class="linkStatusClass"
        @auto-link="autoLinkConsultants"
        @refresh-links="refreshConsultantLinks"
        @save-link="saveConsultantLink"
        @remove-link="removeConsultantLink"
        @update-draft="updateConsultantLinkDraft"
      />

      <CrmConsultantsManagementSection :rows="managementConsultantRows" />
    </section>
  </section>
</template>

<style scoped>
.crm-panel {
  gap: 1.25rem;
}

.crm-panel__header {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  align-items: flex-start;
  gap: 1rem;
}

.crm-panel__content {
  display: grid;
  gap: 1rem;
}

.crm-filters {
  display: flex;
  gap: 0.75rem;
  align-items: flex-end;
  flex-wrap: nowrap;
  width: 100%;
  max-width: 100%;
  overflow-x: auto;
  padding-bottom: 0.1rem;
}

.crm-filters__date-wrap,
.crm-filters__field {
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
  flex: 0 0 auto;
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

.crm-filters__store {
  width: 12.5rem;
  min-height: 42px;
  padding: 0 2rem 0 0.85rem;
  border-radius: 12px;
  border: 1px solid rgb(var(--border) / 0.9);
  background: rgb(var(--surface) / 0.95);
  color: rgb(var(--text));
  font-size: 0.88rem;
  font-weight: 700;
  cursor: pointer;
  outline: none;
}

.crm-filters__store-wrap {
  flex: 0 0 12.5rem;
}

.crm-filters__store:disabled {
  cursor: wait;
  opacity: 0.72;
}

.crm-btn {
  min-height: 38px;
  border: none;
  border-radius: 999px;
  padding: 0.55rem 0.9rem;
  background: rgb(var(--primary));
  color: rgb(255 255 255);
  font-size: 0.84rem;
  font-weight: 700;
  cursor: pointer;
  white-space: nowrap;
  flex: 0 0 auto;
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

@media (max-width: 860px) {
  .crm-panel__header,
  .crm-section__header {
    grid-template-columns: 1fr;
    display: grid;
  }

  .crm-filters {
    width: 100%;
  }

  .crm-filters__actions {
    width: auto;
  }

  .crm-btn {
    flex: 0 0 auto;
  }
}
</style>
