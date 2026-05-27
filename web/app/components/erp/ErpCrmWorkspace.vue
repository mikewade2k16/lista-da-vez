<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { CalendarDays } from 'lucide-vue-next'
import type { ErpCRMConsultantMetric, ErpCRMResponse, ErpQueueConsultantStats } from '~/stores/erp'
import ErpConsultantLinksPanel from '~/components/erp/ErpConsultantLinksPanel.vue'
import { useErpStore } from '~/stores/erp'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import ErpProductsTable from '~/components/erp/ErpProductsTable.vue'

const erpStore = useErpStore()
const auth = useAuthStore()
const ui = useUiStore()

const searchValue = ref('')
const cpfSearch = ref('')
const dedupEnabled = ref(true)
const dateFrom = ref('')
const dateTo = ref('')
const comparisonDateFrom = ref('')
const comparisonDateTo = ref('')
const comparisonCrm = ref<ErpCRMResponse | null>(null)
const showExportPanel = ref(false)
const exportingAll = ref(false)
const loadingComparison = ref(false)
let crmLoadTimer: ReturnType<typeof setTimeout> | null = null
let comparisonLoadTimer: ReturnType<typeof setTimeout> | null = null

const pageSizeOptions = [25, 50, 100, 200]

const crmColumns = [
  { id: 'name', label: 'Nome', width: 'minmax(240px, 1.8fr)', align: 'left', locked: true },
  { id: 'cpf', label: 'CPF', width: '155px', align: 'left' },
  { id: 'email', label: 'Email', width: 'minmax(220px, 1.5fr)', align: 'left' },
  { id: 'phone', label: 'Telefone', width: '150px', align: 'left' },
  { id: 'mobile', label: 'Celular', width: '150px', align: 'left' },
  { id: 'gender', label: 'Genero', width: '100px', align: 'center' },
  { id: 'birthday_raw', label: 'Nascimento', width: '135px', align: 'left' },
  { id: 'city', label: 'Cidade', width: '160px', align: 'left' },
  { id: 'uf', label: 'UF', width: '80px', align: 'center' },
  { id: 'registered_at_raw', label: 'Cadastro', width: '160px', align: 'left' },
  { id: 'tags', label: 'Tags', width: 'minmax(160px, 1fr)', align: 'left' },
  { id: 'identifier', label: 'Identificador', width: '135px', align: 'left' },
]

const crm = computed(() => erpStore.crm)
const summary = computed(() => crm.value?.summary || null)
const comparisonSummary = computed(() => comparisonCrm.value?.summary || null)
const queueStats = computed(() => crm.value?.queueStats || null)
const storeRows = computed(() => crm.value?.stores || [])
const consultantRows = computed(() => crm.value?.consultants || [])
const canManageConsultantLinks = computed(() => auth.role === 'platform_admin')

// merge fila + ERP via vinculo resolvido, com fallback por nome normalizado
type ConsultantRow = ErpCRMConsultantMetric & {
  queue?: ErpQueueConsultantStats
  matched: boolean
}

function normalizeConsultantLookupKey(value: unknown) {
  return String(value || '')
    .trim()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, ' ')
    .trim()
}

function linkStatusLabel(status?: string | null) {
  switch (status) {
    case 'manual':
      return 'manual'
    case 'employee_code':
      return 'codigo'
    case 'name_exact':
      return 'nome'
    case 'ambiguous':
      return 'ambiguo'
    case 'unmatched':
      return 'sem vinculo'
    default:
      return 'pendente'
  }
}

function linkStatusClass(status?: string | null) {
  switch (status) {
    case 'manual':
    case 'employee_code':
      return 'erp-crm__badge--ok'
    case 'name_exact':
      return 'erp-crm__badge--info'
    case 'ambiguous':
      return 'erp-crm__badge--warn'
    default:
      return 'erp-crm__badge--neutral'
  }
}

const queueById = computed(() => {
  const map = new Map<string, ErpQueueConsultantStats>()
  for (const q of queueStats.value?.byConsultant ?? []) {
    const key = String(q.personId || '').trim()
    if (key) map.set(key, q)
  }
  return map
})

const queueByName = computed(() => {
  const map = new Map<string, ErpQueueConsultantStats>()
  for (const q of queueStats.value?.byConsultant ?? []) {
    const key = normalizeConsultantLookupKey(q.personName)
    if (key) map.set(key, q)
  }
  return map
})

function findQueueForConsultant(c: ErpCRMConsultantMetric) {
  const linkedId = String(c.profileConsultantId || '').trim()
  if (linkedId) {
    const queueByLinkedId = queueById.value.get(linkedId)
    if (queueByLinkedId) return queueByLinkedId
  }

  const linkedName = normalizeConsultantLookupKey(c.profileConsultantName)
  if (linkedName) {
    const queueByLinkedName = queueByName.value.get(linkedName)
    if (queueByLinkedName) return queueByLinkedName
  }

  return queueByName.value.get(normalizeConsultantLookupKey(c.consultantName))
}
const mergedConsultants = computed<ConsultantRow[]>(() => {
  return consultantRows.value.map((c) => {
    const queue = findQueueForConsultant(c)
    return {
      ...c,
      queue,
      matched: !!queue,
    }
  })
})

function formatPct(value: number | null | undefined) {
  const n = Number(value || 0)
  return n ? `${n.toFixed(1)}%` : '-'
}

function formatCurrency(cents: number | null | undefined) {
  const n = Number(cents || 0)
  if (!n) return '-'
  return new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' }).format(n / 100)
}

function formatNumber(n: number | null | undefined) {
  return Number(n || 0).toLocaleString('pt-BR')
}

function formatDelta(
  current: number | null | undefined,
  previous: number | null | undefined,
  mode: 'number' | 'currency' = 'number',
) {
  if (!comparisonSummary.value) return ''
  const currentValue = Number(current || 0)
  const previousValue = Number(previous || 0)
  const delta = currentValue - previousValue
  const percent = previousValue
    ? ` (${delta >= 0 ? '+' : ''}${((delta / previousValue) * 100).toFixed(1)}%)`
    : ''
  const formatted =
    mode === 'currency' ? formatCurrency(Math.abs(delta)) : formatNumber(Math.abs(delta))
  return `${delta >= 0 ? '+' : '-'}${formatted}${percent}`
}

function deltaClass(current: number | null | undefined, previous: number | null | undefined) {
  const delta = Number(current || 0) - Number(previous || 0)
  if (delta > 0) return 'erp-crm__metric-delta--up'
  if (delta < 0) return 'erp-crm__metric-delta--down'
  return ''
}

function crmRowKey(row: Record<string, unknown>, index: number) {
  return String(row.id || row.cpf || row.identifier || row.original_id || index)
}

async function loadCRM() {
  const result = await erpStore.fetchCRM({ dateFrom: dateFrom.value, dateTo: dateTo.value })
  if (!result.ok && result.message) {
    ui.error(result.message)
  }
}

async function loadCRMComparison() {
  if (!comparisonDateFrom.value && !comparisonDateTo.value) {
    comparisonCrm.value = null
    return
  }

  loadingComparison.value = true
  const result = await erpStore.fetchCRMSnapshot({
    dateFrom: comparisonDateFrom.value,
    dateTo: comparisonDateTo.value,
  })
  loadingComparison.value = false
  if (!result.ok || !result.data) {
    ui.error(result.message || 'Erro ao carregar a comparacao CRM.')
    return
  }
  comparisonCrm.value = result.data
}

function clearCRMLoadTimer() {
  if (!crmLoadTimer) return
  clearTimeout(crmLoadTimer)
  crmLoadTimer = null
}

function clearComparisonLoadTimer() {
  if (!comparisonLoadTimer) return
  clearTimeout(comparisonLoadTimer)
  comparisonLoadTimer = null
}

function scheduleCRM() {
  clearCRMLoadTimer()
  crmLoadTimer = setTimeout(() => {
    crmLoadTimer = null
    void loadCRM()
  }, 250)
}

function scheduleCRMComparison() {
  clearComparisonLoadTimer()
  comparisonLoadTimer = setTimeout(() => {
    comparisonLoadTimer = null
    void loadCRMComparison()
  }, 250)
}

function clearComparison() {
  comparisonDateFrom.value = ''
  comparisonDateTo.value = ''
  comparisonCrm.value = null
}

async function loadCustomers(payload: { page?: number; pageSize?: number } = {}) {
  const result = await erpStore.fetchRecords({
    dataType: 'customer',
    search: searchValue.value,
    specificSearch: cpfSearch.value,
    dedup: dedupEnabled.value,
    page: payload.page || erpStore.recordsPage || 1,
    pageSize: payload.pageSize || erpStore.recordsPageSize || 50,
  })
  if (!result.ok && result.message) {
    ui.error(result.message)
  }
}

async function handlePageChange(nextPage: number) {
  await loadCustomers({ page: nextPage, pageSize: erpStore.recordsPageSize })
}

async function handlePageSizeChange(nextPageSize: number) {
  await loadCustomers({ page: 1, pageSize: nextPageSize })
}

function buildSearchParams() {
  return {
    dataType: 'customer',
    search: searchValue.value,
    specificSearch: cpfSearch.value,
    dedup: dedupEnabled.value,
  }
}

// --- Export helpers ---

function cellValue(val: unknown): string {
  if (val === null || val === undefined) return ''
  if (typeof val === 'object') return JSON.stringify(val)
  return String(val)
}

function exportCurrentPage(format: 'csv' | 'json' | 'md' | 'txt') {
  const rows = erpStore.records
  if (!rows.length) {
    ui.error('Nenhum dado na pagina atual para exportar.')
    return
  }
  triggerDownload(rows, format)
  showExportPanel.value = false
}

async function exportAllPages(format: 'csv' | 'json' | 'md' | 'txt') {
  exportingAll.value = true
  showExportPanel.value = false
  try {
    const allRows: Array<Record<string, unknown>> = []
    const pageSize = 5000
    let page = 1

    while (true) {
      const result = await erpStore.fetchRecordsSnapshot({
        ...buildSearchParams(),
        page,
        pageSize,
      })
      if (!result.ok || !result.data) {
        throw new Error(result.message || 'Erro ao exportar os dados.')
      }
      const items = Array.isArray(result.data.items) ? result.data.items : []
      allRows.push(...items)
      const total = Number(result.data.total || 0)
      if (allRows.length >= total || !items.length) break
      page++
    }

    if (!allRows.length) {
      ui.error('Nenhum dado encontrado para exportar.')
      return
    }

    triggerDownload(allRows, format)
    ui.success(`${allRows.length.toLocaleString('pt-BR')} registros exportados.`)
  } catch {
    ui.error('Erro ao exportar os dados.')
  } finally {
    exportingAll.value = false
  }
}

function triggerDownload(
  rows: Array<Record<string, unknown>>,
  format: 'csv' | 'json' | 'md' | 'txt',
) {
  const visibleCols = crmColumns.map((c) => c.id)
  const headers = crmColumns.map((c) => c.label)

  let content = ''
  let mime = 'text/plain'
  let ext = format

  if (format === 'json') {
    const exportRows = rows.map((row) => {
      const obj: Record<string, string> = {}
      for (const col of visibleCols) {
        obj[col] = cellValue(row[col])
      }
      return obj
    })
    content = JSON.stringify(exportRows, null, 2)
    mime = 'application/json'
  } else if (format === 'csv') {
    const escapeCSV = (v: string) =>
      v.includes(',') || v.includes('"') || v.includes('\n') ? `"${v.replace(/"/g, '""')}"` : v
    const lines = [headers.map(escapeCSV).join(',')]
    for (const row of rows) {
      lines.push(visibleCols.map((col) => escapeCSV(cellValue(row[col]))).join(','))
    }
    content = lines.join('\n')
    mime = 'text/csv'
  } else if (format === 'md') {
    const sep = headers.map(() => '---').join(' | ')
    const lines = [
      `| ${headers.join(' | ')} |`,
      `| ${sep} |`,
      ...rows.map(
        (row) =>
          `| ${visibleCols.map((col) => cellValue(row[col]).replace(/\|/g, '\\|')).join(' | ')} |`,
      ),
    ]
    content = lines.join('\n')
    mime = 'text/markdown'
    ext = 'md'
  } else if (format === 'txt') {
    const lines = [headers.join('\t')]
    for (const row of rows) {
      lines.push(visibleCols.map((col) => cellValue(row[col]).replace(/\t/g, ' ')).join('\t'))
    }
    content = lines.join('\n')
    mime = 'text/plain'
  }

  const blob = new Blob(['﻿' + content], { type: `${mime};charset=utf-8` })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `erp-clientes-${new Date().toISOString().slice(0, 10)}.${ext}`
  a.click()
  URL.revokeObjectURL(url)
}

// --- watchers ---

watch([searchValue, cpfSearch, dedupEnabled], () => {
  void loadCustomers({ page: 1 })
})

watch([dateFrom, dateTo], () => {
  scheduleCRM()
})

watch([comparisonDateFrom, comparisonDateTo], () => {
  scheduleCRMComparison()
})

onBeforeUnmount(() => {
  clearCRMLoadTimer()
  clearComparisonLoadTimer()
})

onMounted(() => {
  void loadCRM()
  void loadCustomers({ page: 1 })
})

defineExpose({ loadCRM, loadCRMComparison, loadCustomers })
</script>

<template>
  <section class="erp-crm">
    <!-- Metrics summary -->
    <div v-if="summary" class="erp-crm__metrics">
      <article class="erp-crm__metric-card">
        <span class="erp-crm__metric-label">Compras</span>
        <strong class="erp-crm__metric-value">{{ formatNumber(summary.orders) }}</strong>
        <small
          v-if="comparisonSummary"
          class="erp-crm__metric-delta"
          :class="deltaClass(summary.orders, comparisonSummary.orders)"
        >
          {{ formatDelta(summary.orders, comparisonSummary.orders) }}
        </small>
      </article>
      <article class="erp-crm__metric-card">
        <span class="erp-crm__metric-label">Unidades</span>
        <strong class="erp-crm__metric-value">{{ formatNumber(summary.units) }}</strong>
        <small
          v-if="comparisonSummary"
          class="erp-crm__metric-delta"
          :class="deltaClass(summary.units, comparisonSummary.units)"
        >
          {{ formatDelta(summary.units, comparisonSummary.units) }}
        </small>
      </article>
      <article class="erp-crm__metric-card">
        <span class="erp-crm__metric-label">Faturamento</span>
        <strong class="erp-crm__metric-value erp-crm__metric-value--money">
          {{ formatCurrency(summary.salesCents) }}
        </strong>
        <small
          v-if="comparisonSummary"
          class="erp-crm__metric-delta"
          :class="deltaClass(summary.salesCents, comparisonSummary.salesCents)"
        >
          {{ formatDelta(summary.salesCents, comparisonSummary.salesCents, 'currency') }}
        </small>
      </article>
      <article class="erp-crm__metric-card">
        <span class="erp-crm__metric-label">Ticket medio</span>
        <strong class="erp-crm__metric-value erp-crm__metric-value--money">
          {{ formatCurrency(summary.ticketAverageCents) }}
        </strong>
        <small
          v-if="comparisonSummary"
          class="erp-crm__metric-delta"
          :class="deltaClass(summary.ticketAverageCents, comparisonSummary.ticketAverageCents)"
        >
          {{
            formatDelta(
              summary.ticketAverageCents,
              comparisonSummary.ticketAverageCents,
              'currency',
            )
          }}
        </small>
      </article>
      <article class="erp-crm__metric-card">
        <span class="erp-crm__metric-label">PA</span>
        <strong class="erp-crm__metric-value">
          {{ summary.paScore ? summary.paScore.toFixed(2) : '-' }}
        </strong>
        <small
          v-if="comparisonSummary"
          class="erp-crm__metric-delta"
          :class="deltaClass(summary.paScore, comparisonSummary.paScore)"
        >
          {{ formatDelta(summary.paScore, comparisonSummary.paScore) }}
        </small>
      </article>
    </div>

    <!-- Fila: indicadores de atendimento -->
    <div v-if="queueStats" class="erp-crm__metrics erp-crm__metrics--queue">
      <article class="erp-crm__metric-card erp-crm__metric-card--queue">
        <span class="erp-crm__metric-label">Atendimentos</span>
        <strong class="erp-crm__metric-value">
          {{ formatNumber(queueStats.totalAttendances) }}
        </strong>
      </article>
      <article class="erp-crm__metric-card erp-crm__metric-card--queue">
        <span class="erp-crm__metric-label">Conversoes (fila)</span>
        <strong class="erp-crm__metric-value">
          {{ formatNumber(queueStats.totalConversions) }}
        </strong>
      </article>
      <article class="erp-crm__metric-card erp-crm__metric-card--queue">
        <span class="erp-crm__metric-label">Taxa de conversao</span>
        <strong class="erp-crm__metric-value erp-crm__metric-value--rate">
          {{ formatPct(queueStats.conversionRate) }}
        </strong>
        <div class="erp-crm__progress-bar">
          <div
            class="erp-crm__progress-fill erp-crm__progress-fill--green"
            :style="{ width: `${Math.min(queueStats.conversionRate, 100)}%` }"
          ></div>
        </div>
      </article>
      <article class="erp-crm__metric-card erp-crm__metric-card--queue">
        <span class="erp-crm__metric-label">Cancelamento (fila)</span>
        <strong
          class="erp-crm__metric-value erp-crm__metric-value--rate erp-crm__metric-value--warn"
        >
          {{ formatPct(queueStats.cancellationRate) }}
        </strong>
        <div class="erp-crm__progress-bar">
          <div
            class="erp-crm__progress-fill erp-crm__progress-fill--red"
            :style="{ width: `${Math.min(queueStats.cancellationRate, 100)}%` }"
          ></div>
        </div>
      </article>
      <article
        v-if="summary?.erpCancellations"
        class="erp-crm__metric-card erp-crm__metric-card--queue"
      >
        <span class="erp-crm__metric-label">Cancelamento ERP</span>
        <strong
          class="erp-crm__metric-value erp-crm__metric-value--rate erp-crm__metric-value--warn"
        >
          {{ formatPct(summary.erpCancellationRate) }}
        </strong>
        <small class="erp-crm__metric-sub">
          {{ formatNumber(summary.erpCancellations) }} pedidos
        </small>
      </article>
    </div>

    <ErpConsultantLinksPanel v-if="canManageConsultantLinks" @updated="loadCRM" />

    <!-- Indicadores por consultor -->
    <div v-if="mergedConsultants.length" class="erp-crm__section">
      <h3 class="erp-crm__section-title">Indicadores por consultor</h3>
      <div class="erp-crm__table-wrap">
        <table class="erp-crm__table">
          <thead>
            <tr>
              <th class="erp-crm__th erp-crm__th--name">Consultor</th>
              <th class="erp-crm__th">Vinculo</th>
              <th class="erp-crm__th">Loja</th>
              <th class="erp-crm__th erp-crm__th--num">Faturamento</th>
              <th class="erp-crm__th erp-crm__th--num">% Meta</th>
              <th class="erp-crm__th erp-crm__th--num">PA</th>
              <th class="erp-crm__th erp-crm__th--num">Ticket medio</th>
              <th class="erp-crm__th erp-crm__th--num">Atendimentos</th>
              <th class="erp-crm__th erp-crm__th--num">Conversao</th>
              <th class="erp-crm__th erp-crm__th--num">Canc. fila</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="c in mergedConsultants"
              :key="c.consultantId + c.storeCnpj"
              class="erp-crm__tr"
            >
              <td class="erp-crm__td erp-crm__td--name">
                <div class="erp-crm__consultant-cell">
                  <strong>{{ c.consultantName }}</strong>
                  <small>ERP {{ c.erpEmployeeId || c.consultantId }}</small>
                </div>
              </td>
              <td class="erp-crm__td">
                <span
                  class="erp-crm__badge"
                  :class="linkStatusClass(c.linkStatus)"
                  :title="
                    c.profileConsultantName
                      ? `Lista: ${c.profileConsultantName}`
                      : c.linkCandidates
                        ? `${c.linkCandidates} candidatos encontrados`
                        : 'Sem consultor vinculado'
                  "
                >
                  {{ linkStatusLabel(c.linkStatus) }}
                </span>
              </td>
              <td class="erp-crm__td erp-crm__td--store">{{ c.storeLabel }}</td>
              <td class="erp-crm__td erp-crm__td--num">{{ formatCurrency(c.salesCents) }}</td>
              <td class="erp-crm__td erp-crm__td--num erp-crm__td--goal">
                <span v-if="c.orders > 0">
                  <!-- sem meta individual por consultor no modelo atual; placeholder -->
                  -
                </span>
                <span v-else class="erp-crm__muted">-</span>
              </td>
              <td class="erp-crm__td erp-crm__td--num">
                {{ c.paScore ? c.paScore.toFixed(2) : '-' }}
              </td>
              <td class="erp-crm__td erp-crm__td--num">
                {{ formatCurrency(c.ticketAverageCents) }}
              </td>
              <td class="erp-crm__td erp-crm__td--num erp-crm__td--queue">
                {{ c.queue ? formatNumber(c.queue.attendances) : '-' }}
              </td>
              <td class="erp-crm__td erp-crm__td--num erp-crm__td--rate">
                <span :class="{ 'erp-crm__rate--good': (c.queue?.conversionRate ?? 0) >= 30 }">
                  {{ c.queue ? formatPct(c.queue.conversionRate) : '-' }}
                </span>
              </td>
              <td class="erp-crm__td erp-crm__td--num erp-crm__td--rate">
                <span :class="{ 'erp-crm__rate--bad': (c.queue?.queueCancellationRate ?? 0) > 10 }">
                  {{ c.queue ? formatPct(c.queue.queueCancellationRate) : '-' }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Indicadores por loja -->
    <div v-if="storeRows.length" class="erp-crm__section">
      <h3 class="erp-crm__section-title">Indicadores por loja</h3>
      <div class="erp-crm__table-wrap">
        <table class="erp-crm__table">
          <thead>
            <tr>
              <th class="erp-crm__th erp-crm__th--name">Loja</th>
              <th class="erp-crm__th erp-crm__th--num">Faturamento</th>
              <th class="erp-crm__th erp-crm__th--num">% Meta</th>
              <th class="erp-crm__th erp-crm__th--num">PA</th>
              <th class="erp-crm__th erp-crm__th--num">Ticket medio</th>
              <th class="erp-crm__th erp-crm__th--num">Canc. ERP</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in storeRows" :key="s.storeSlug" class="erp-crm__tr">
              <td class="erp-crm__td erp-crm__td--name">{{ s.storeLabel }}</td>
              <td class="erp-crm__td erp-crm__td--num">{{ formatCurrency(s.salesCents) }}</td>
              <td class="erp-crm__td erp-crm__td--num">
                <div v-if="s.monthlyGoalCents > 0" class="erp-crm__goal-cell">
                  <span>{{ s.goalProgress ? s.goalProgress.toFixed(1) : '0.0' }}%</span>
                  <div class="erp-crm__progress-bar erp-crm__progress-bar--inline">
                    <div
                      class="erp-crm__progress-fill erp-crm__progress-fill--green"
                      :style="{ width: `${Math.min(s.goalProgress, 100)}%` }"
                    ></div>
                  </div>
                </div>
                <span v-else class="erp-crm__muted">-</span>
              </td>
              <td class="erp-crm__td erp-crm__td--num">
                {{ s.paScore ? s.paScore.toFixed(2) : '-' }}
              </td>
              <td class="erp-crm__td erp-crm__td--num">
                {{ formatCurrency(s.ticketAverageCents) }}
              </td>
              <td class="erp-crm__td erp-crm__td--num">
                <span :class="{ 'erp-crm__rate--bad': (s.erpCancellationRate ?? 0) > 5 }">
                  {{ s.erpCancellations ? formatPct(s.erpCancellationRate) : '-' }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Filters row -->
    <div class="erp-crm__filters">
      <div class="erp-crm__filter-group erp-crm__filter-group--wide">
        <label class="erp-crm__filter-label">Periodo de vendas</label>
        <AppDatePicker
          :model-value="dateFrom"
          :end-date="dateTo"
          @update:model-value="dateFrom = $event"
          @update:end-date="dateTo = $event"
        >
          <template #default="{ label }">
            <button type="button" class="erp-crm__date-trigger">
              <CalendarDays :size="14" />
              <span>{{ label || 'Todas as vendas' }}</span>
            </button>
          </template>
        </AppDatePicker>
        <div class="erp-crm__date-range erp-crm__date-range--legacy">
          <input
            v-model="dateFrom"
            type="date"
            class="erp-crm__date-input"
            placeholder="De"
            title="Data inicial"
          />
          <span class="erp-crm__date-sep">até</span>
          <input
            v-model="dateTo"
            type="date"
            class="erp-crm__date-input"
            placeholder="Ate"
            title="Data final"
          />
        </div>
      </div>

      <div class="erp-crm__filter-group erp-crm__filter-group--wide">
        <label class="erp-crm__filter-label">Comparar com</label>
        <div class="erp-crm__compare-row">
          <AppDatePicker
            :model-value="comparisonDateFrom"
            :end-date="comparisonDateTo"
            @update:model-value="comparisonDateFrom = $event"
            @update:end-date="comparisonDateTo = $event"
          >
            <template #default="{ label }">
              <button type="button" class="erp-crm__date-trigger erp-crm__date-trigger--compare">
                <CalendarDays :size="14" />
                <span>{{ loadingComparison ? 'Comparando...' : label || 'Sem comparacao' }}</span>
              </button>
            </template>
          </AppDatePicker>
          <button
            v-if="comparisonDateFrom || comparisonDateTo"
            class="erp-crm__clear-compare"
            type="button"
            @click="clearComparison"
          >
            Limpar
          </button>
        </div>
      </div>

      <label
        class="erp-crm__dedup-toggle"
        :title="dedupEnabled ? 'Desativar deduplicacao' : 'Ativar deduplicacao por CPF'"
      >
        <input v-model="dedupEnabled" type="checkbox" class="erp-crm__dedup-checkbox" />
        <span
          class="erp-crm__dedup-track"
          :class="{ 'erp-crm__dedup-track--on': dedupEnabled }"
        ></span>
        <span class="erp-crm__dedup-text">Sem duplicatas</span>
      </label>

      <div class="erp-crm__export-wrap">
        <button
          class="erp-crm__export-btn"
          :disabled="exportingAll"
          @click="showExportPanel = !showExportPanel"
        >
          <span v-if="exportingAll">Exportando...</span>
          <span v-else>Exportar</span>
        </button>

        <div v-if="showExportPanel" class="erp-crm__export-panel">
          <p class="erp-crm__export-panel-title">Pagina atual</p>
          <button class="erp-crm__export-option" @click="exportCurrentPage('csv')">CSV</button>
          <button class="erp-crm__export-option" @click="exportCurrentPage('json')">JSON</button>
          <button class="erp-crm__export-option" @click="exportCurrentPage('md')">Markdown</button>
          <button class="erp-crm__export-option" @click="exportCurrentPage('txt')">
            TXT (tab)
          </button>
          <hr class="erp-crm__export-divider" />
          <p class="erp-crm__export-panel-title">Todos os registros</p>
          <button class="erp-crm__export-option" @click="exportAllPages('csv')">
            CSV completo
          </button>
          <button class="erp-crm__export-option" @click="exportAllPages('json')">
            JSON completo
          </button>
          <button class="erp-crm__export-option" @click="exportAllPages('md')">
            Markdown completo
          </button>
          <button class="erp-crm__export-option" @click="exportAllPages('txt')">
            TXT completo
          </button>
        </div>
      </div>
    </div>

    <!-- Customer table -->
    <ErpProductsTable
      :columns="crmColumns"
      :rows="erpStore.records"
      :row-key="crmRowKey"
      :total="erpStore.totalRecords"
      :page="erpStore.recordsPage"
      :page-size="erpStore.recordsPageSize"
      :page-size-options="pageSizeOptions"
      :search-value="searchValue"
      :identifier-search-value="cpfSearch"
      :loading="erpStore.loadingRecords || erpStore.loadingCrm"
      :show-identifier-search="true"
      identifier-search-label="CPF (comeca com)"
      identifier-search-placeholder="Ex: 123.456.789-00"
      general-search-placeholder="Busca geral (nome, email, cidade, telefone, tags...)"
      :show-refresh-action="true"
      :show-bootstrap-action="false"
      empty-title="Nenhum cliente encontrado"
      empty-text="Importe clientes pelo ERP ou ajuste os filtros para ver os registros."
      :storage-key="`erp-crm-grid-v1`"
      testid="erp-crm-grid"
      @update:search-value="searchValue = $event"
      @update:identifier-search-value="cpfSearch = $event"
      @update:page="handlePageChange"
      @update:page-size="handlePageSizeChange"
      @refresh="loadCustomers({ page: 1 })"
    />
  </section>
</template>

<style scoped>
.erp-crm {
  display: grid;
  gap: 1rem;
}

.erp-crm__metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 0.75rem;
}

.erp-crm__metric-card {
  display: grid;
  gap: 0.3rem;
  padding: 1rem;
  border-radius: 1rem;
  border: 1px solid var(--line-soft);
  background: var(--erp-card-bg);
  box-shadow: var(--shadow-card);
}

.erp-crm__metric-label {
  color: var(--text-muted);
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.erp-crm__metric-value {
  color: var(--text-main);
  font-size: 1.5rem;
  font-variant-numeric: tabular-nums;
  line-height: 1;
}

.erp-crm__metric-value--money {
  font-size: 1.1rem;
  color: var(--erp-success-text);
}

.erp-crm__metric-delta {
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 700;
}

.erp-crm__metric-delta--up {
  color: var(--erp-success-text);
}

.erp-crm__metric-delta--down {
  color: var(--erp-danger-text);
}

/* Filters */
.erp-crm__filters {
  display: flex;
  gap: 0.85rem;
  align-items: flex-end;
  flex-wrap: wrap;
}

.erp-crm__filter-group {
  display: grid;
  gap: 0.25rem;
}

.erp-crm__filter-group--wide {
  flex: 0 0 auto;
}

.erp-crm__filter-label {
  font-size: 0.72rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.07em;
  color: var(--text-muted);
}

.erp-crm__date-range {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.erp-crm__date-range--legacy {
  display: none;
}

.erp-crm__date-input {
  min-height: 2.35rem;
  padding: 0 0.7rem;
  border-radius: 0.75rem;
  border: 1px solid var(--line-soft);
  background: var(--erp-control-bg);
  color: var(--text-main);
  font-size: 0.88rem;
  color-scheme: dark;
}

.erp-crm__date-trigger {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 14rem;
  max-width: 22rem;
  min-height: 2.35rem;
  padding: 0 0.75rem;
  border-radius: 0.75rem;
  border: 1px solid var(--line-soft);
  background: var(--erp-control-bg);
  color: var(--text-main);
  font-size: 0.86rem;
  font-weight: 700;
  text-align: left;
  cursor: pointer;
}

.erp-crm__date-trigger span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.erp-crm__date-trigger--compare {
  border-color: var(--erp-primary-border);
}

.erp-crm__compare-row {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  flex-wrap: wrap;
}

.erp-crm__clear-compare {
  min-height: 2.35rem;
  padding: 0 0.7rem;
  border-radius: 0.75rem;
  border: 1px solid var(--line-soft);
  background: var(--erp-control-bg);
  color: var(--text-muted);
  font-size: 0.82rem;
  font-weight: 700;
  cursor: pointer;
}

.erp-crm__date-sep {
  color: var(--text-muted);
  font-size: 0.82rem;
  white-space: nowrap;
}

/* Dedup toggle */
.erp-crm__dedup-toggle {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
  user-select: none;
  padding-bottom: 0.2rem;
}

.erp-crm__dedup-checkbox {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
}

.erp-crm__dedup-track {
  position: relative;
  width: 2.4rem;
  height: 1.3rem;
  border-radius: 999px;
  background: var(--erp-track-off);
  border: 1px solid var(--line-soft);
  transition:
    background 0.18s ease,
    border-color 0.18s ease;
  flex-shrink: 0;
}

.erp-crm__dedup-track::after {
  content: '';
  position: absolute;
  top: 0.18rem;
  left: 0.18rem;
  width: 0.85rem;
  height: 0.85rem;
  border-radius: 50%;
  background: var(--erp-track-thumb);
  transition:
    transform 0.18s ease,
    background 0.18s ease;
}

.erp-crm__dedup-track--on {
  background: var(--erp-success-card-bg);
  border-color: var(--erp-success-border);
}

.erp-crm__dedup-track--on::after {
  transform: translateX(1.1rem);
  background: var(--erp-success-text);
}

.erp-crm__dedup-text {
  font-size: 0.84rem;
  color: var(--text-muted);
  white-space: nowrap;
}

/* Export */
.erp-crm__export-wrap {
  position: relative;
  margin-left: auto;
}

.erp-crm__export-btn {
  min-height: 2.35rem;
  padding: 0 1rem;
  border-radius: 0.75rem;
  border: 1px solid var(--erp-primary-border);
  background: var(--erp-primary-soft-bg);
  color: var(--erp-primary-soft-text);
  font-size: 0.88rem;
  font-weight: 600;
  cursor: pointer;
  transition:
    border-color 0.16s,
    transform 0.16s;
}

.erp-crm__export-btn:hover:not(:disabled) {
  border-color: var(--erp-hover-border);
  transform: translateY(-1px);
}

.erp-crm__export-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.erp-crm__export-panel {
  position: absolute;
  right: 0;
  top: calc(100% + 0.4rem);
  z-index: 50;
  min-width: 14rem;
  padding: 0.6rem;
  border-radius: 0.9rem;
  border: 1px solid var(--line-soft);
  background: var(--erp-strong-bg);
  box-shadow: var(--erp-shadow-dropdown);
  display: grid;
  gap: 0.25rem;
}

.erp-crm__export-panel-title {
  margin: 0.2rem 0 0.1rem;
  font-size: 0.7rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--text-muted);
}

.erp-crm__export-panel-title small {
  font-weight: 400;
  text-transform: none;
  letter-spacing: 0;
  opacity: 0.7;
}

.erp-crm__export-option {
  width: 100%;
  padding: 0.45rem 0.65rem;
  border-radius: 0.6rem;
  border: none;
  background: transparent;
  color: var(--text-main);
  font-size: 0.85rem;
  text-align: left;
  cursor: pointer;
  transition: background 0.14s;
}

.erp-crm__export-option:hover {
  background: var(--erp-primary-soft-bg);
}

.erp-crm__export-divider {
  margin: 0.3rem 0;
  border: none;
  border-top: 1px solid var(--line-soft);
}

/* Queue metrics row */
.erp-crm__metrics--queue {
  border-top: 1px solid var(--line-soft);
  padding-top: 0.75rem;
}

.erp-crm__metric-card--queue {
  border-color: var(--erp-primary-border);
}

.erp-crm__metric-value--rate {
  font-size: 1.3rem;
  color: var(--erp-success-text);
}

.erp-crm__metric-value--warn {
  color: var(--erp-warning-text);
}

.erp-crm__metric-sub {
  color: var(--text-muted);
  font-size: 0.72rem;
}

.erp-crm__progress-bar {
  height: 4px;
  border-radius: 2px;
  background: var(--erp-table-divider);
  overflow: hidden;
  margin-top: 0.25rem;
}

.erp-crm__progress-bar--inline {
  width: 100%;
  margin-top: 0.2rem;
}

.erp-crm__progress-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.3s ease;
}

.erp-crm__progress-fill--green {
  background: rgb(var(--success));
}

.erp-crm__progress-fill--red {
  background: rgb(var(--danger));
}

/* Sections */
.erp-crm__section {
  display: grid;
  gap: 0.6rem;
}

.erp-crm__section-title {
  font-size: 0.78rem;
  font-weight: 700;
  background: var(--erp-panel-bg);
  letter-spacing: 0.08em;
  color: var(--text-muted);
  margin: 0;
}

.erp-crm__table-wrap {
  background: var(--erp-hover-bg);
  overflow-x: auto;
  border-radius: 0.75rem;
  border: 1px solid var(--line-soft);
  border-top: 1px solid var(--erp-table-divider);
}

.erp-crm__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.84rem;
}

.erp-crm__th {
  padding: 0.55rem 0.75rem;
  text-align: left;
  font-size: 0.72rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-muted);
  background: var(--erp-panel-bg);
  white-space: nowrap;
  border-bottom: 1px solid var(--line-soft);
}

.erp-crm__th--num {
  text-align: right;
}

.erp-crm__th--name {
  min-width: 180px;
}

.erp-crm__tr:hover {
  background: var(--erp-hover-bg);
}

.erp-crm__tr + .erp-crm__tr .erp-crm__td {
  border-top: 1px solid var(--erp-table-divider);
}

.erp-crm__td {
  padding: 0.5rem 0.75rem;
  color: var(--text-main);
  white-space: nowrap;
}

.erp-crm__td--num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.erp-crm__td--name {
  font-weight: 600;
}

.erp-crm__consultant-cell {
  display: grid;
  gap: 0.15rem;
  min-width: 180px;
}

.erp-crm__consultant-cell small {
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 600;
}

.erp-crm__td--store {
  color: var(--text-muted);
  font-size: 0.8rem;
}

.erp-crm__td--queue {
  color: var(--erp-success-text);
}

.erp-crm__goal-cell {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 0.2rem;
  min-width: 80px;
}

.erp-crm__rate--good {
  color: var(--erp-success-text);
  font-weight: 700;
}

.erp-crm__rate--bad {
  color: var(--erp-danger-text);
  font-weight: 700;
}

.erp-crm__muted {
  color: var(--text-muted);
}

.erp-crm__badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 1.35rem;
  padding: 0.16rem 0.45rem;
  border-radius: 999px;
  border: 1px solid var(--line-soft);
  font-size: 0.68rem;
  font-weight: 700;
  line-height: 1;
  text-transform: uppercase;
  white-space: nowrap;
}

.erp-crm__badge--ok {
  border-color: rgb(var(--success) / 0.34);
  background: rgb(var(--success) / 0.12);
  color: var(--erp-success-text);
}

.erp-crm__badge--info {
  border-color: rgb(var(--primary) / 0.32);
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
}

.erp-crm__badge--warn {
  border-color: rgb(var(--primary) / 0.36);
  background: rgb(var(--primary) / 0.14);
  color: rgb(var(--primary));
}

.erp-crm__badge--neutral {
  border-color: var(--line-soft);
  background: rgb(var(--surface-2) / 0.78);
  color: var(--text-muted);
}
</style>
