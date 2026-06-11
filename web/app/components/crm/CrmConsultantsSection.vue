<script setup lang="ts">
import { computed, ref } from 'vue'

import AppEntityGrid from '~/components/ui/AppEntityGrid.vue'
import {
  crmListUsageAttendances,
  crmListUsageCoverageRate,
  crmListUsageOrders,
  crmListUsageStatus,
  crmListUsageStatusLabel,
} from '~/domain/utils/crm-list-usage'
import {
  calculateCrmGoalPayout,
  classifyCrmListUsageRate,
  type CrmGoalPayoutPolicy,
  type CrmGoalPayoutRule,
  type CrmListUsageTier,
} from '~/domain/utils/crm-performance-policy'
import { formatCurrencyBRL } from '~/domain/utils/admin-metrics'
import type { CRMConsultantMetric } from '~/stores/crm'
import type { ErpConsultantLinkOption } from '~/stores/erp'
import type { MergedCrmConsultant } from '~/composables/useCrmConsultantMetrics'

const props = defineProps<{
  mergedConsultants: MergedCrmConsultant[]
  managementConsultantRows: CRMConsultantMetric[]
  storeGoalProgressBySlug: Record<string, number>
  goalPayoutPolicy: CrmGoalPayoutPolicy
  listUsageTiers: CrmListUsageTier[]
  canManageConsultantLinks: boolean
  loadingConsultantLinks: boolean
  savingConsultantLink: boolean
  consultantLinkOptions: ErpConsultantLinkOption[]
  consultantLinkDrafts: Record<string, string>
  queueStatsAvailable: boolean
  consultantLinkKey: (storeCode: unknown, employeeId: unknown) => string
  linkStatusLabel: (status?: string | null) => string
  linkStatusClass: (status?: string | null) => string
}>()

const emit = defineEmits<{
  (event: 'auto-link' | 'refresh-links'): void
  (event: 'save-link' | 'remove-link', row: MergedCrmConsultant): void
  (event: 'update-draft', payload: { key: string; value: string }): void
}>()

const expandedRowKey = ref('')
const sortBy = ref('salesCents')
const sortDir = ref<'asc' | 'desc'>('desc')
const linkFilter = ref('all')
const queueFilter = ref('all')
const storeFilter = ref('all')
const tableSearch = ref('')
const salesFilter = ref('')
const ticketFilter = ref('')
const ordersFilter = ref('')

const consultantGridColumns = [
  {
    id: 'consultantName',
    label: 'Consultor',
    width: 'minmax(190px, 1.3fr)',
    sortable: true,
    locked: true,
  },
  { id: 'linkStatusValue', label: 'Vinculo', width: 'minmax(190px, 1fr)', sortable: true },
  { id: 'storeLabel', label: 'Loja', width: '110px', sortable: true },
  { id: 'salesCents', label: 'Vendido', width: '130px', align: 'end', sortable: true },
  {
    id: 'ticketAverageCents',
    label: 'Ticket medio',
    width: '120px',
    align: 'end',
    sortable: true,
  },
  { id: 'paScore', label: 'P.A.', width: '78px', align: 'end', sortable: true },
  { id: 'orders', label: 'Pedidos', width: '82px', align: 'end', sortable: true },
  {
    id: 'attendancesValue',
    label: 'Atend. (F)',
    width: '92px',
    align: 'end',
    sortable: true,
  },
  {
    id: 'conversionRateValue',
    label: 'Conv. fila',
    width: '100px',
    align: 'end',
    sortable: true,
  },
  {
    id: 'queueUsageRateValue',
    label: 'Cobertura lista',
    width: '130px',
    align: 'end',
    sortable: true,
  },
  {
    id: 'queueCancellationRateValue',
    label: 'Canc. fila',
    width: '100px',
    align: 'end',
    sortable: true,
  },
  {
    id: 'goalPayoutCentsValue',
    label: 'Recebimento',
    width: '132px',
    align: 'end',
    sortable: true,
  },
  { id: 'queueStatusValue', label: 'Status fila', width: '150px', sortable: true },
]

const tableStoreOptions = computed(() => {
  const stores = new Map<string, string>()
  for (const row of props.mergedConsultants) {
    const storeSlug = String(row.storeSlug || '').trim()
    const storeLabel = String(row.storeLabel || '').trim()
    if (storeSlug && storeLabel && !stores.has(storeSlug)) {
      stores.set(storeSlug, storeLabel)
    }
  }

  return [
    { value: 'all', label: 'Loja: todas' },
    ...[...stores.entries()]
      .map(([value, label]) => ({ value, label }))
      .sort((left, right) => left.label.localeCompare(right.label)),
  ]
})

const decoratedConsultants = computed(() =>
  props.mergedConsultants.map((row) => {
    const attendances = queueAttendances(row)
    const conversionRate = queueInternalRate(row)
    const cancellationRate = queueCancellationRate(row)
    const queueStatus = queueStatusValue(row)
    const listStatus = queueUsageStatus(row)
    const usageRate = crmListUsageCoverageRate(row)
    const listTier = classifyCrmListUsageRate(usageRate, props.listUsageTiers)
    const storeGoalProgress = storeGoalProgressForRow(row)
    const goalPayout = calculateCrmGoalPayout(
      row.salesCents,
      storeGoalProgress,
      props.goalPayoutPolicy,
      'consultant',
    )

    return {
      ...row,
      rowKey: tableRowKey(row),
      linkStatusValue: props.linkStatusLabel(row.linkStatus),
      queueStatusKey: queueStatus,
      queueStatusValue: queueStatusLabel(row),
      attendancesValue: attendances,
      conversionRateValue: conversionRate,
      queueUsageRateValue: usageRate,
      queueUsageStatusValue: listStatus,
      queueUsageStatusLabel: crmListUsageStatusLabel(listStatus),
      queueUsageTierLabel: listTier.label,
      queueUsageOrdersValue: crmListUsageOrders(row),
      queueUsageAttendancesValue: crmListUsageAttendances(row),
      queueCancellationRateValue: cancellationRate,
      goalPayoutCentsValue: goalPayout.amountCents,
      goalPayoutRuleLabel: goalPayoutRuleLabel(goalPayout.rule),
      storeGoalProgressValue: storeGoalProgress,
      belowAverageQueueUse: listStatus === 'partial' || listStatus === 'unused',
    }
  }),
)

const filteredConsultants = computed(() =>
  decoratedConsultants.value.filter((row) => {
    if (storeFilter.value !== 'all' && String(row.storeSlug || '') !== storeFilter.value) {
      return false
    }
    if (linkFilter.value !== 'all' && String(row.linkStatus || 'pending') !== linkFilter.value) {
      return false
    }
    if (queueFilter.value !== 'all' && row.queueStatusKey !== queueFilter.value) {
      return false
    }
    if (
      tableSearch.value &&
      !matchesSearch(tableSearch.value, [
        row.consultantName,
        row.profileConsultantName,
        row.erpEmployeeId || row.consultantId,
        row.storeLabel,
      ])
    ) {
      return false
    }
    if (!matchesNumberFilter(row.salesCents, salesFilter.value, 100)) {
      return false
    }
    if (!matchesNumberFilter(row.ticketAverageCents, ticketFilter.value, 100)) {
      return false
    }
    if (!matchesNumberFilter(row.orders, ordersFilter.value)) {
      return false
    }
    return true
  }),
)

const sortedConsultants = computed(() => {
  const rows = [...filteredConsultants.value]
  rows.sort((left, right) => compareRows(left, right, sortBy.value, sortDir.value))
  return rows
})

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

function normalizeSearch(value: unknown) {
  return String(value || '')
    .trim()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
}

function matchesSearch(search: unknown, values: unknown[]) {
  const normalizedSearch = normalizeSearch(search)
  if (!normalizedSearch) return true
  return values.some((value) => normalizeSearch(value).includes(normalizedSearch))
}

function parseNumberToken(value: string) {
  const normalized = String(value || '').trim()
  if (!normalized) return null

  let numeric = normalized.replace(/[^\d,.-]/g, '')
  if (!numeric || numeric === '-' || numeric === '.' || numeric === ',') return null

  if (numeric.includes(',')) {
    numeric = numeric.replace(/\./g, '').replace(',', '.')
  } else if (/^-?\d{1,3}(\.\d{3})+$/.test(numeric)) {
    numeric = numeric.replace(/\./g, '')
  }

  const parsed = Number(numeric)
  return Number.isFinite(parsed) ? parsed : null
}

function parseRangeFilter(value: string, multiplier = 1) {
  const normalized = String(value || '').trim()
  if (!normalized) return null

  const tokens = normalized.match(/\d+(?:[.,]\d+)*/g) || []
  const parsed = tokens
    .map((token) => parseNumberToken(token))
    .filter((token): token is number => token !== null)

  if (!parsed.length) return null

  if (parsed.length === 1) {
    const exact = parsed[0] * multiplier
    return { exact, min: exact, max: exact }
  }

  const first = parsed[0] * multiplier
  const second = parsed[1] * multiplier
  return {
    exact: null,
    min: Math.min(first, second),
    max: Math.max(first, second),
  }
}

function matchesNumberFilter(value: unknown, filter: string, multiplier = 1) {
  const range = parseRangeFilter(filter, multiplier)
  if (!range) return true

  const numericValue = Number(value || 0)
  if (!Number.isFinite(numericValue)) return false

  if (range.exact !== null) {
    return Math.round(numericValue) === Math.round(range.exact)
  }

  return numericValue >= range.min && numericValue <= range.max
}

function hasTableFilters() {
  return (
    storeFilter.value !== 'all' ||
    linkFilter.value !== 'all' ||
    queueFilter.value !== 'all' ||
    tableSearch.value.trim() ||
    salesFilter.value.trim() ||
    ticketFilter.value.trim() ||
    ordersFilter.value.trim()
  )
}

function clearTableFilters() {
  storeFilter.value = 'all'
  linkFilter.value = 'all'
  queueFilter.value = 'all'
  tableSearch.value = ''
  salesFilter.value = ''
  ticketFilter.value = ''
  ordersFilter.value = ''
}

function tableRowKey(row: MergedCrmConsultant | CRMConsultantMetric) {
  return `${row.consultantId}-${row.storeSlug}-${row.storeCnpj || ''}`
}

function draftKeyForRow(row: MergedCrmConsultant) {
  return props.consultantLinkKey(row.storeCnpj, row.erpEmployeeId || row.consultantId)
}

function isExpanded(row: MergedCrmConsultant) {
  return expandedRowKey.value === tableRowKey(row)
}

function toggleEditor(row: MergedCrmConsultant) {
  const nextKey = tableRowKey(row)
  expandedRowKey.value = expandedRowKey.value === nextKey ? '' : nextKey
}

function updateDraft(row: MergedCrmConsultant, event: Event) {
  const target = event.target as HTMLSelectElement | null
  emit('update-draft', {
    key: draftKeyForRow(row),
    value: target?.value || '',
  })
}

function currentLinkedLabel(row: MergedCrmConsultant) {
  return (
    row.linkEmployee?.linkedConsultantName || row.profileConsultantName || 'Sem consultor vinculado'
  )
}

function queueStatusLabel(row: MergedCrmConsultant) {
  if (!props.queueStatsAvailable) return 'sem dados fila'
  if (row.queue) return 'identificado'
  if (String(row.profileConsultantId || row.profileConsultantName || '').trim()) {
    return 'sem atend. no periodo'
  }
  return 'nao identificado'
}

function queueStatusClass(row: MergedCrmConsultant) {
  if (!props.queueStatsAvailable) return 'crm-badge--neutral'
  if (row.queue) return 'crm-badge--ok'
  if (String(row.profileConsultantId || row.profileConsultantName || '').trim()) {
    return 'crm-badge--info'
  }
  return 'crm-badge--warn'
}

function queueStatusValue(row: MergedCrmConsultant) {
  if (!props.queueStatsAvailable) return 'no-data'
  if (row.queue) return 'identified'
  if (String(row.profileConsultantId || row.profileConsultantName || '').trim()) {
    return 'no-attendance'
  }
  return 'unidentified'
}

function queueAttendances(row: MergedCrmConsultant) {
  return Number(row.queue?.attendances || 0)
}

function queueInternalRate(row: MergedCrmConsultant) {
  return Number(row.queue?.conversionRate || 0)
}

function queueUsageStatus(row: MergedCrmConsultant) {
  return crmListUsageStatus(row)
}

function queueCancellationRate(row: MergedCrmConsultant) {
  return Number(row.queue?.queueCancellationRate || 0)
}

function storeGoalProgressForRow(row: MergedCrmConsultant) {
  return Number(props.storeGoalProgressBySlug?.[String(row.storeSlug || '').trim()] || 0)
}

function goalPayoutRuleLabel(rule: CrmGoalPayoutRule | null) {
  if (!rule) return 'sem faixa'
  if (rule.mode === 'amount') return formatCurrencyBRL(rule.value)
  return `${rule.value.toFixed(1)}%`
}

function sortValue(row: Record<string, unknown>, field: string) {
  const value = row?.[field]
  if (typeof value === 'number') return value
  if (value === null || value === undefined) return ''
  return String(value)
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
}

function compareRows(
  left: Record<string, unknown>,
  right: Record<string, unknown>,
  field: string,
  direction: 'asc' | 'desc',
) {
  const multiplier = direction === 'asc' ? 1 : -1
  const leftValue = sortValue(left, field)
  const rightValue = sortValue(right, field)
  if (leftValue < rightValue) return -1 * multiplier
  if (leftValue > rightValue) return 1 * multiplier
  return 0
}

function toggleSort(columnId: string) {
  if (sortBy.value === columnId) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortBy.value = columnId
  sortDir.value = columnId === 'consultantName' || columnId === 'storeLabel' ? 'asc' : 'desc'
}

function handleSave(row: MergedCrmConsultant) {
  emit('save-link', row)
  expandedRowKey.value = ''
}

function handleRemove(row: MergedCrmConsultant) {
  emit('remove-link', row)
  expandedRowKey.value = ''
}
</script>

<template>
  <article class="insight-card insight-card--wide">
    <header class="crm-section__header">
      <div>
        <h3 class="insight-card__title">Indicadores por consultor</h3>
        <p class="insight-card__text">
          ERP + fila de atendimento. Colunas de fila marcadas com (F) cruzadas pelo vinculo, codigo
          ou nome.
        </p>
      </div>
      <div class="crm-section__side">
        <span class="crm-section__meta">
          {{ sortedConsultants.length }} de {{ mergedConsultants.length }} consultor(es)
        </span>
        <div v-if="canManageConsultantLinks" class="crm-inline-link-toolbar">
          <button
            type="button"
            class="crm-btn crm-btn--ghost crm-btn--sm"
            :disabled="loadingConsultantLinks || savingConsultantLink"
            @click="emit('auto-link')"
          >
            Vincular automaticamente
          </button>
          <button
            type="button"
            class="crm-btn crm-btn--ghost crm-btn--sm"
            :disabled="loadingConsultantLinks || savingConsultantLink"
            @click="emit('refresh-links')"
          >
            Atualizar vinculos
          </button>
        </div>
      </div>
    </header>

    <AppEntityGrid
      :columns="consultantGridColumns"
      :rows="sortedConsultants"
      :row-key="(row) => row.rowKey"
      :search-value="''"
      :show-search="false"
      :sort-by="sortBy"
      :sort-dir="sortDir"
      storage-key="crm-consultants-columns-v4"
      empty-title="Nenhum consultor"
      empty-text="Nenhum consultor com pedidos ERP no periodo selecionado."
      testid="crm-consultants-grid"
      class="crm-consultants-grid"
      @sort="toggleSort"
    >
      <template #toolbar-filters>
        <input
          v-model="tableSearch"
          class="crm-table-filter crm-table-filter--input crm-table-filter--search"
          type="search"
          placeholder="Buscar consultor..."
          autocomplete="off"
        />

        <select v-model="storeFilter" class="crm-table-filter" title="Filtrar loja">
          <option v-for="option in tableStoreOptions" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>

        <select v-model="queueFilter" class="crm-table-filter" title="Filtrar status da fila">
          <option value="all">Status: todos</option>
          <option value="identified">Identificados</option>
          <option value="no-attendance">Sem atendimento</option>
          <option value="unidentified">Nao identificados</option>
        </select>

        <select v-model="linkFilter" class="crm-table-filter" title="Filtrar vinculo ERP">
          <option value="all">Vinculo: todos</option>
          <option value="manual">Manual</option>
          <option value="employee_code">Codigo</option>
          <option value="name_exact">Nome</option>
          <option value="ambiguous">Ambiguo</option>
          <option value="unmatched">Sem vinculo</option>
        </select>

        <input
          v-model="salesFilter"
          class="crm-table-filter crm-table-filter--input crm-table-filter--number"
          type="text"
          inputmode="decimal"
          placeholder="Vendido: 1000 ou 1000-5000"
          title="Filtrar valor vendido por numero unico ou faixa"
        />

        <input
          v-model="ticketFilter"
          class="crm-table-filter crm-table-filter--input crm-table-filter--number"
          type="text"
          inputmode="decimal"
          placeholder="Ticket: 1500 ou 1000-2000"
          title="Filtrar ticket medio por numero unico ou faixa"
        />

        <input
          v-model="ordersFilter"
          class="crm-table-filter crm-table-filter--input crm-table-filter--number"
          type="text"
          inputmode="numeric"
          placeholder="Pedidos: 10 ou 10-40"
          title="Filtrar pedidos por numero unico ou faixa"
        />

        <button
          v-if="hasTableFilters()"
          type="button"
          class="crm-table-filter crm-table-filter--button"
          @click="clearTableFilters"
        >
          Limpar
        </button>
      </template>

      <template #cell-consultantName="{ row }">
        <div class="crm-row-heading">
          <strong>{{ row.consultantName }}</strong>
          <small class="crm-muted">ERP {{ row.erpEmployeeId || row.consultantId }}</small>
        </div>
      </template>

      <template #cell-linkStatusValue="{ row }">
        <div class="crm-link-cell">
          <div class="crm-link-summary">
            <span class="crm-badge" :class="linkStatusClass(row.linkStatus)">
              {{ linkStatusLabel(row.linkStatus) }}
            </span>
            <button
              v-if="canManageConsultantLinks"
              type="button"
              class="crm-link-toggle"
              :disabled="loadingConsultantLinks || savingConsultantLink"
              @click="toggleEditor(row)"
            >
              {{ isExpanded(row) ? 'Fechar' : 'Editar' }}
            </button>
          </div>

          <small class="crm-link-caption">{{ currentLinkedLabel(row) }}</small>

          <template v-if="canManageConsultantLinks && isExpanded(row)">
            <select
              class="crm-link-select"
              :value="consultantLinkDrafts[draftKeyForRow(row)] || ''"
              :disabled="loadingConsultantLinks || savingConsultantLink"
              @change="updateDraft(row, $event)"
            >
              <option value="">Selecionar consultor</option>
              <option
                v-for="consultant in consultantLinkOptions"
                :key="consultant.consultantId"
                :value="consultant.consultantId"
              >
                {{ consultant.consultantName }} - {{ consultant.storeName }}
              </option>
            </select>

            <div class="crm-link-actions">
              <button
                type="button"
                class="crm-btn crm-btn--ghost crm-btn--xs"
                :disabled="savingConsultantLink"
                @click="handleSave(row)"
              >
                Salvar
              </button>
              <button
                v-if="row.linkEmployee?.linkId"
                type="button"
                class="crm-btn crm-btn--ghost crm-btn--xs"
                :disabled="savingConsultantLink"
                @click="handleRemove(row)"
              >
                Remover
              </button>
            </div>
          </template>
        </div>
      </template>

      <template #cell-salesCents="{ row }">
        <span>{{ formatCurrencyFromCents(row.salesCents) }}</span>
      </template>

      <template #cell-ticketAverageCents="{ row }">
        <span>{{ formatCurrencyFromCents(row.ticketAverageCents) }}</span>
      </template>

      <template #cell-paScore="{ row }">
        <span>{{ formatPA(row.paScore) }}</span>
      </template>

      <template #cell-orders="{ row }">
        <span>{{ formatNumber(row.orders) }}</span>
      </template>

      <template #cell-attendancesValue="{ row }">
        <span :class="{ 'crm-td--queue': row.queue }">
          {{ row.queue ? formatNumber(row.attendancesValue) : '-' }}
        </span>
      </template>

      <template #cell-conversionRateValue="{ row }">
        <span v-if="row.queue" :class="{ 'crm-rate--good': row.conversionRateValue >= 30 }">
          {{ formatPct(row.conversionRateValue) }}
        </span>
        <span v-else class="crm-muted">-</span>
      </template>

      <template #cell-queueUsageRateValue="{ row }">
        <div v-if="row.orders" class="crm-list-use-cell">
          <strong
            class="crm-list-use-cell__status"
            :class="`crm-list-use-cell__status--${row.queueUsageStatusValue}`"
          >
            {{ formatPct(row.queueUsageRateValue) }}
          </strong>
          <span class="crm-list-use-cell__tier">{{ row.queueUsageTierLabel }}</span>
          <small>
            {{ row.queueUsageStatusLabel }} - {{ formatNumber(row.queueUsageAttendancesValue) }} /
            {{ formatNumber(row.queueUsageOrdersValue) }} pedidos
          </small>
        </div>
        <span v-else class="crm-muted">-</span>
      </template>

      <template #cell-queueCancellationRateValue="{ row }">
        <span v-if="row.queue" :class="{ 'crm-rate--bad': row.queueCancellationRateValue > 10 }">
          {{ formatPct(row.queueCancellationRateValue) }}
        </span>
        <span v-else class="crm-muted">-</span>
      </template>

      <template #cell-goalPayoutCentsValue="{ row }">
        <div class="crm-payout-cell">
          <strong>{{ formatCurrencyFromCents(row.goalPayoutCentsValue) }}</strong>
          <small>
            {{ row.goalPayoutRuleLabel }} / meta
            {{ formatPct(row.storeGoalProgressValue) }}
          </small>
        </div>
      </template>

      <template #cell-queueStatusValue="{ row }">
        <div class="crm-status-cell">
          <span class="crm-badge" :class="queueStatusClass(row)">
            {{ queueStatusLabel(row) }}
          </span>
          <small v-if="row.belowAverageQueueUse" class="crm-usage-flag">cobertura incompleta</small>
        </div>
      </template>
    </AppEntityGrid>
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

.crm-section__side {
  display: grid;
  justify-items: flex-end;
  gap: 0.6rem;
}

.crm-section__meta {
  color: rgb(var(--muted));
  font-size: 0.88rem;
}

.crm-inline-link-toolbar {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.55rem;
}

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

.crm-btn:disabled,
.crm-link-toggle:disabled {
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

.crm-btn--xs {
  min-height: 34px;
  padding: 0.42rem 0.68rem;
  border-radius: 10px;
  font-size: 0.76rem;
}

.crm-consultants-grid {
  --crm-grid-min-width: 1420px;
  padding: 0.65rem;
  gap: 0.55rem;
}

.crm-consultants-grid :deep(.app-entity-grid__toolbar) {
  align-items: center;
  gap: 0.55rem;
}

.crm-consultants-grid :deep(.app-entity-grid__toolbar-main) {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  flex: 1 1 auto;
}

.crm-consultants-grid :deep(.app-entity-grid__filters) {
  flex-wrap: wrap;
  gap: 0.4rem;
}

.crm-consultants-grid :deep(.app-entity-grid__viewport) {
  overflow-x: auto;
}

.crm-consultants-grid :deep(.app-entity-grid__canvas) {
  min-width: var(--crm-grid-min-width);
}

.crm-consultants-grid :deep(.app-entity-grid__head) {
  background: rgb(var(--surface) / 0.98);
}

.crm-consultants-grid :deep(.app-entity-grid__row) {
  align-items: center;
  padding: 0.45rem 0.3rem;
  border-radius: 0;
  border-width: 0 0 1px;
  background: transparent;
}

.crm-consultants-grid :deep(.app-entity-grid__cell) {
  min-height: 2.35rem;
  font-size: 0.8rem;
}

.crm-table-filter {
  min-height: 2rem;
  padding: 0 0.65rem;
  border-radius: 999px;
  border: 1px solid rgb(var(--border) / 0.76);
  background: rgb(var(--surface) / 0.95);
  color: rgb(var(--text));
  font-size: 0.74rem;
  font-weight: 700;
}

.crm-table-filter--input {
  outline: none;
}

.crm-table-filter--input::placeholder {
  color: rgb(var(--muted) / 0.72);
}

.crm-table-filter--search {
  width: min(16rem, 100%);
}

.crm-table-filter--number {
  width: 12.5rem;
}

.crm-table-filter--button {
  cursor: pointer;
}

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

.crm-td--queue {
  color: rgb(var(--success));
  font-weight: 600;
}

.crm-tr--unmatched td {
  opacity: 0.82;
}

.crm-link-cell {
  display: grid;
  gap: 0.45rem;
  min-width: 240px;
}

.crm-link-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
}

.crm-link-toggle {
  padding: 0;
  border: none;
  background: none;
  color: rgb(var(--primary));
  font-size: 0.78rem;
  font-weight: 700;
  cursor: pointer;
}

.crm-link-caption {
  color: rgb(var(--muted));
  font-size: 0.74rem;
}

.crm-link-select {
  min-height: 34px;
  min-width: 220px;
  padding: 0 0.7rem;
  border-radius: 10px;
  border: 1px solid rgb(var(--border) / 0.88);
  background: rgb(var(--surface) / 0.95);
  color: rgb(var(--text));
  font-size: 0.82rem;
}

.crm-link-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

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

.crm-badge--info {
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
}

.crm-badge--neutral {
  background: rgb(var(--border) / 0.56);
  color: rgb(var(--muted));
}

.crm-rate--good {
  color: rgb(var(--success));
  font-weight: 700;
}

.crm-rate--bad {
  color: rgb(var(--danger));
  font-weight: 700;
}

.crm-list-use-cell {
  display: grid;
  justify-items: end;
  gap: 0.12rem;
}

.crm-list-use-cell small {
  color: rgb(var(--muted));
  font-size: 0.68rem;
}

.crm-list-use-cell__status {
  font-weight: 800;
}

.crm-list-use-cell__tier {
  color: rgb(var(--text));
  font-size: 0.72rem;
  font-weight: 700;
}

.crm-list-use-cell__status--covered {
  color: rgb(var(--success));
}

.crm-list-use-cell__status--partial {
  color: rgb(var(--primary));
}

.crm-list-use-cell__status--unused {
  color: rgb(var(--danger));
}

.crm-payout-cell {
  display: grid;
  justify-items: end;
  gap: 0.12rem;
}

.crm-payout-cell strong {
  color: rgb(var(--success));
  font-weight: 800;
}

.crm-payout-cell small {
  color: rgb(var(--muted));
  font-size: 0.68rem;
}

.crm-status-cell {
  display: grid;
  justify-items: start;
  gap: 0.25rem;
}

.crm-usage-flag {
  color: rgb(var(--danger));
  font-size: 0.66rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

@media (max-width: 860px) {
  .crm-section__header {
    display: grid;
    grid-template-columns: 1fr;
  }

  .crm-section__side,
  .crm-inline-link-toolbar {
    justify-items: flex-start;
    justify-content: flex-start;
  }
}
</style>
