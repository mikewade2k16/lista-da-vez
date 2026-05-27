<script setup>
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Sparkles, Download, RotateCcw, Trash2, UserPlus } from 'lucide-vue-next'

import AppEntityGrid from '~/components/ui/AppEntityGrid.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { formatCurrencyBRL, formatPercent } from '~/domain/utils/admin-metrics'
import { useAuthStore } from '~/stores/auth'
import { useCrmStore } from '~/stores/crm'
import { useOperationGoalsStore } from '~/stores/operation-goals'
import { useUiStore } from '~/stores/ui'

const props = defineProps({
  stores: {
    type: Array,
    default: () => [],
  },
  activeStoreId: {
    type: String,
    default: '',
  },
  canEditGoals: {
    type: Boolean,
    default: false,
  },
  allowAllStoreScope: {
    type: Boolean,
    default: false,
  },
  selectedMonth: {
    type: String,
    default: '',
  },
  selectedStoreId: {
    type: String,
    default: '',
  },
  showCards: {
    type: Boolean,
    default: true,
  },
  showTables: {
    type: Boolean,
    default: true,
  },
  manageDataLifecycle: {
    type: Boolean,
    default: true,
  },
})

const emit = defineEmits(['update:selectedMonth', 'update:selectedStoreId'])

const auth = useAuthStore()
const ui = useUiStore()
const crmStore = useCrmStore()
const operationGoals = useOperationGoalsStore()
const { goals, errorMessage, pending, ready, saving } = storeToRefs(operationGoals)
const { overview: crmOverview, pending: crmPending } = storeToRefs(crmStore)

const storeSearch = ref('')
const consultantSearch = ref('')
const selectedMonth = ref(normalizeMonthValue(props.selectedMonth) || currentMonth())
const selectedStoreId = ref(normalizeText(props.selectedStoreId))
const storeSortBy = ref('storeLabel')
const storeSortDir = ref('asc')
const consultantSortBy = ref('storeLabel')
const consultantSortDir = ref('asc')
const drafts = ref({})
const rowBusy = reactive({})
const rowSavePending = reactive({})
const rowSaveInFlight = new Map()
const bulkCreating = ref(false)
const bulkConsultantStoreId = ref('')
const bulkConsultantCreating = ref(false)

/* ===== Lojas e opções ===== */

const activeStores = computed(() =>
  (props.stores || []).filter((store) => normalizeText(store?.id) && store?.isActive !== false),
)

const fallbackStoreId = computed(() => {
  const activeStoreId = normalizeText(props.activeStoreId)
  if (activeStores.value.some((store) => normalizeText(store.id) === activeStoreId)) {
    return activeStoreId
  }
  return normalizeText(activeStores.value[0]?.id)
})

const storeFilterOptions = computed(() => {
  const options = activeStores.value.map((store) => ({
    value: normalizeText(store.id),
    label: normalizeText(store.name),
    description: normalizeText(store.code),
  }))
  if (props.allowAllStoreScope && activeStores.value.length > 1) {
    return [{ value: '', label: 'Loja: todas' }, ...options]
  }
  return options
})

const bulkConsultantStoreOptions = computed(() =>
  activeStores.value.map((store) => ({
    value: normalizeText(store.id),
    label: normalizeText(store.name),
    description: normalizeText(store.code),
  })),
)

/* ===== Particionamento por escopo ===== */

const storeGoals = computed(() => goals.value.filter((g) => g.scope === 'store'))
const consultantGoals = computed(() => goals.value.filter((g) => g.scope === 'consultant'))

const storesWithoutGoal = computed(() => {
  const existing = new Set(storeGoals.value.map((g) => g.storeId))
  return activeStores.value.filter((s) => !existing.has(normalizeText(s.id)))
})

const canBulkCreate = computed(
  () => !bulkCreating.value && !saving.value && storesWithoutGoal.value.length > 0,
)

const canBulkConsultant = computed(
  () =>
    !bulkConsultantCreating.value && !saving.value && !!normalizeText(bulkConsultantStoreId.value),
)

/* ===== Cards BI (performance vs realizado) ===== */

const isCurrentMonthSelected = computed(() => selectedMonth.value === currentMonth())
const selectedMonthRange = computed(() => buildMonthDateRange(selectedMonth.value))

const crmOverviewMatchesSelectedMonth = computed(() => {
  const overview = crmOverview.value
  if (!overview) return false
  const overviewTenantId = normalizeText(overview.store?.tenantId)
  if (overviewTenantId && normalizeText(auth.activeTenantId) !== overviewTenantId) return false
  return (
    normalizeText(overview.dateFrom) === selectedMonthRange.value.dateFrom &&
    normalizeText(overview.dateTo) === selectedMonthRange.value.dateTo
  )
})

const crmDataReady = computed(() => crmOverviewMatchesSelectedMonth.value)
const cardsLoadingCrm = computed(() => !crmDataReady.value && crmPending.value)

const crmStoreRows = computed(() =>
  crmDataReady.value && Array.isArray(crmOverview.value?.stores) ? crmOverview.value.stores : [],
)

const crmQueueStoreRows = computed(() =>
  crmDataReady.value && Array.isArray(crmOverview.value?.queueStats?.byStore)
    ? crmOverview.value.queueStats.byStore
    : [],
)

const crmStoreRowsByKey = computed(() => {
  const map = {}
  for (const row of crmStoreRows.value) {
    addStoreLookupKeys(map, row, {
      storeId: '',
      storeCode: row.storeCode,
      storeSlug: row.storeSlug,
      storeName: row.storeName || row.storeLabel,
    })
  }
  return map
})

const crmQueueRowsByKey = computed(() => {
  const map = {}
  for (const row of crmQueueStoreRows.value) {
    addStoreLookupKeys(map, row, {
      storeId: row.storeId,
      storeCode: '',
      storeSlug: row.storeSlug,
      storeName: row.storeLabel,
    })
  }
  return map
})

/** Lista de lojas que têm meta > 0 cruzada com realizado */
const performanceRows = computed(() => {
  if (!crmDataReady.value) return []
  const rows = []
  for (const goal of storeGoals.value) {
    const monthlyGoal = Number(goal.monthlyGoal) || 0
    if (monthlyGoal <= 0) continue
    const perf = buildStorePerformance(goal)
    if (!perf) continue
    const soldValue = Number(perf.soldValue) || 0
    rows.push({
      storeId: goal.storeId,
      storeName: perf.storeName || goal.storeName,
      monthlyGoal,
      soldValue,
      completionPct: (soldValue / monthlyGoal) * 100,
      gap: monthlyGoal - soldValue,
    })
  }
  return rows
})

const totalRealized = computed(() => performanceRows.value.reduce((sum, r) => sum + r.soldValue, 0))

const totalGoalWithRealized = computed(() =>
  performanceRows.value.reduce((sum, r) => sum + r.monthlyGoal, 0),
)

/** Card 1: % de cumprimento total */
const completionPct = computed(() => {
  if (!totalGoalWithRealized.value) return null
  return (totalRealized.value / totalGoalWithRealized.value) * 100
})

const completionRemaining = computed(() => {
  if (!totalGoalWithRealized.value) return 0
  return Math.max(0, totalGoalWithRealized.value - totalRealized.value)
})

/** Card 2: Projeção de fechamento (ritmo atual) */
const projectionPct = computed(() => {
  if (!isCurrentMonthSelected.value || !totalGoalWithRealized.value) return null
  const now = new Date()
  const currentDay = now.getDate()
  const totalDays = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate()
  if (!currentDay) return null
  const projected = (totalRealized.value / currentDay) * totalDays
  return (projected / totalGoalWithRealized.value) * 100
})

const projectionDelta = computed(() => {
  if (projectionPct.value === null) return null
  const now = new Date()
  const currentDay = now.getDate()
  const totalDays = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate()
  const projected = (totalRealized.value / currentDay) * totalDays
  return projected - totalGoalWithRealized.value
})

/** Card 3: Loja em maior gap (menor % de cumprimento) */
const worstGapStore = computed(() => {
  if (!performanceRows.value.length) return null
  return performanceRows.value.reduce((worst, row) =>
    !worst || row.completionPct < worst.completionPct ? row : worst,
  )
})

/** Card 4: Top loja (maior % de cumprimento) */
const topStore = computed(() => {
  if (!performanceRows.value.length) return null
  return performanceRows.value.reduce((top, row) =>
    !top || row.completionPct > top.completionPct ? row : top,
  )
})

function pctStatus(pct) {
  if (pct === null || pct === undefined) return 'neutral'
  if (pct >= 100) return 'success'
  if (pct >= 70) return 'warn'
  return 'danger'
}

/* ===== Colunas das grids ===== */

const storeGridColumns = computed(() => {
  const columns = [
    {
      id: 'month',
      label: 'Periodo',
      width: '88px',
      sortable: true,
      defaultVisible: false,
    },
    {
      id: 'storeLabel',
      label: 'Loja',
      width: 'minmax(180px, 1.3fr)',
      sortable: true,
      locked: true,
    },
    { id: 'monthlyGoal', label: 'Meta total', width: '140px', align: 'end', sortable: true },
    { id: 'realizedSales', label: 'Realizado', width: '130px', align: 'end', sortable: true },
    { id: 'goalCompletionPct', label: 'Ating.', width: '92px', align: 'end', sortable: true },
    { id: 'avgTicketGoal', label: 'Ticket medio', width: '140px', align: 'end', sortable: true },
    { id: 'conversionGoal', label: 'Conversao', width: '120px', align: 'end', sortable: true },
    { id: 'paGoal', label: 'P.A.', width: '100px', align: 'end', sortable: true },
    {
      id: 'updatedAtLabel',
      label: 'Atualizado',
      width: '130px',
      sortable: true,
      defaultVisible: false,
    },
  ]
  if (props.canEditGoals) {
    columns.push({ id: 'actions', label: '', width: '48px', align: 'end', locked: true })
  }
  return columns
})

const consultantGridColumns = computed(() => {
  const columns = [
    {
      id: 'month',
      label: 'Periodo',
      width: '88px',
      sortable: true,
      defaultVisible: false,
    },
    {
      id: 'storeLabel',
      label: 'Loja',
      width: 'minmax(150px, 1fr)',
      sortable: true,
    },
    {
      id: 'consultantLabel',
      label: 'Consultor',
      width: 'minmax(160px, 1.2fr)',
      sortable: true,
      locked: true,
    },
    { id: 'monthlyGoal', label: 'Meta total', width: '140px', align: 'end', sortable: true },
    { id: 'avgTicketGoal', label: 'Ticket medio', width: '140px', align: 'end', sortable: true },
    { id: 'conversionGoal', label: 'Conversao', width: '120px', align: 'end', sortable: true },
    { id: 'paGoal', label: 'P.A.', width: '100px', align: 'end', sortable: true },
    {
      id: 'updatedAtLabel',
      label: 'Atualizado',
      width: '130px',
      sortable: true,
      defaultVisible: false,
    },
  ]
  if (props.canEditGoals) {
    columns.push({ id: 'actions', label: '', width: '48px', align: 'end', locked: true })
  }
  return columns
})

/* ===== Filtros e ordenação ===== */

function applyFilter(rows, searchValue) {
  const selectedStore = normalizeText(selectedStoreId.value)
  const normalizedSearch = normalizeSearch(searchValue)

  return rows.filter((row) => {
    if (selectedStore && row.storeId !== selectedStore) return false
    if (!normalizedSearch) return true
    return normalizeSearch(
      [row.month, row.storeName, row.storeCode, row.consultantName].filter(Boolean).join(' '),
    ).includes(normalizedSearch)
  })
}

const filteredStoreRows = computed(() => applyFilter(storeGoals.value, storeSearch.value))
const filteredConsultantRows = computed(() =>
  applyFilter(consultantGoals.value, consultantSearch.value),
)

const sortedStoreRows = computed(() => {
  const rows = [...filteredStoreRows.value]
  rows.sort((a, b) => compareRows(a, b, storeSortBy.value, storeSortDir.value))
  return rows.map(decorateRow)
})

const sortedConsultantRows = computed(() => {
  const rows = [...filteredConsultantRows.value]
  rows.sort((a, b) => compareRows(a, b, consultantSortBy.value, consultantSortDir.value))
  return rows.map(decorateRow)
})

function decorateRow(row) {
  const performance = row.scope === 'store' ? buildStorePerformance(row) : null
  const monthlyGoal = Number(row.monthlyGoal) || 0
  const realizedSales = performance ? Number(performance.soldValue || 0) : null
  return {
    ...row,
    storeLabel: row.storeName,
    consultantLabel: row.consultantName || 'Meta da loja',
    realizedSales,
    goalCompletionPct:
      realizedSales !== null && monthlyGoal > 0 ? (realizedSales / monthlyGoal) * 100 : null,
    updatedAtLabel: formatUpdatedAt(row.updatedAt),
  }
}

/* ===== Drafts: preserva entre reloads, reseta só após save bem-sucedido ===== */

watch(
  () => goals.value,
  (rows) => {
    const next = {}
    for (const row of rows) {
      next[row.id] = drafts.value[row.id] ?? createMetricDraft(row)
    }
    drafts.value = next
  },
  { immediate: true, deep: true },
)

watch(
  () => props.selectedMonth,
  (value) => {
    const normalizedValue = normalizeMonthValue(value)
    if (normalizedValue && normalizedValue !== selectedMonth.value) {
      selectedMonth.value = normalizedValue
    }
  },
)

watch(
  () => props.selectedStoreId,
  (value) => {
    const normalizedValue = normalizeText(value)
    if (normalizedValue !== normalizeText(selectedStoreId.value)) {
      selectedStoreId.value = normalizedValue
    }
  },
)

watch(selectedMonth, (value) => {
  const normalizedValue = normalizeMonthValue(value) || currentMonth()
  if (normalizedValue !== value) {
    selectedMonth.value = normalizedValue
    return
  }
  emit('update:selectedMonth', normalizedValue)
})

watch(selectedStoreId, (value) => {
  emit('update:selectedStoreId', normalizeText(value))
})

watch(
  () => [activeStores.value, fallbackStoreId.value],
  () => {
    const availableStoreIds = new Set(activeStores.value.map((s) => normalizeText(s.id)))

    if (
      normalizeText(selectedStoreId.value) &&
      !availableStoreIds.has(normalizeText(selectedStoreId.value))
    ) {
      selectedStoreId.value = ''
    }

    if (
      (!props.allowAllStoreScope || activeStores.value.length <= 1) &&
      !normalizeText(selectedStoreId.value)
    ) {
      selectedStoreId.value = fallbackStoreId.value
    }

    if (
      normalizeText(bulkConsultantStoreId.value) &&
      !availableStoreIds.has(normalizeText(bulkConsultantStoreId.value))
    ) {
      bulkConsultantStoreId.value = ''
    }
  },
  { immediate: true, deep: true },
)

watch(
  () => [selectedMonth.value, selectedStoreId.value],
  () => {
    if (!props.manageDataLifecycle) return
    void refreshGoals()
  },
  { immediate: true },
)

watch(
  () => [auth.isAuthenticated, auth.activeTenantId, selectedMonth.value],
  () => {
    if (!props.manageDataLifecycle) return
    void refreshCrmOverview()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  for (const row of goals.value) {
    if (rowBusy[row.id]) continue
    const payload = buildRowPayload(row)
    if (!isRowDirty(row, payload)) continue
    void operationGoals.updateGoal(row.id, payload, { reload: false, skipLoadingIndicator: true })
  }
})

/* ===== Ações de servidor ===== */

async function refreshGoals() {
  try {
    await operationGoals.loadGoals({
      tenantId: auth.activeTenantId,
      storeId: normalizeText(selectedStoreId.value),
      month: selectedMonth.value,
    })
  } catch {
    return null
  }
  return true
}

async function refreshCrmOverview() {
  if (!auth.isAuthenticated || !normalizeText(auth.activeTenantId)) {
    return null
  }

  const range = selectedMonthRange.value
  crmStore.dateFrom = range.dateFrom
  crmStore.dateTo = range.dateTo

  try {
    await crmStore.ensureLoaded()
  } catch {
    return null
  }
  return true
}

async function createAllStoreGoals() {
  if (!canBulkCreate.value) return

  const month = selectedMonth.value
  const stores = storesWithoutGoal.value
  if (!stores.length) {
    ui.info(`Todas as lojas ja possuem meta para ${month}.`)
    return
  }

  bulkCreating.value = true
  let created = 0
  let failed = 0
  try {
    for (const store of stores) {
      const result = await operationGoals.createGoal({
        storeId: store.id,
        month,
        monthlyGoal: 0,
        avgTicketGoal: 0,
        conversionGoal: 0,
        paGoal: 0,
      })
      if (result?.ok === false) failed++
      else created++
    }
  } finally {
    bulkCreating.value = false
  }

  storeSearch.value = ''
  if (created && !failed) ui.success(`${created} meta(s) de loja criada(s) para ${month}.`)
  else if (created) ui.info(`${created} criada(s), ${failed} erro(s).`)
  else ui.error('Nao foi possivel criar as metas.')
}

async function createConsultantGoalsForStore() {
  if (!canBulkConsultant.value) return
  const storeId = normalizeText(bulkConsultantStoreId.value)
  const month = selectedMonth.value

  bulkConsultantCreating.value = true
  try {
    const consultants = await operationGoals.loadConsultants(storeId)
    if (!consultants.length) {
      ui.info('Nenhum consultor ativo nesta loja.')
      return
    }

    const existing = new Set(
      consultantGoals.value
        .filter((g) => g.storeId === storeId && g.month === month)
        .map((g) => g.consultantId),
    )

    const toCreate = consultants.filter((c) => !existing.has(c.id))
    if (!toCreate.length) {
      ui.info(`Todos os consultores desta loja ja possuem meta para ${month}.`)
      return
    }

    let created = 0
    let failed = 0
    for (const c of toCreate) {
      const result = await operationGoals.createGoal({
        storeId,
        consultantId: c.id,
        month,
        monthlyGoal: 0,
        avgTicketGoal: 0,
        conversionGoal: 0,
        paGoal: 0,
      })
      if (result?.ok === false) failed++
      else created++
    }

    consultantSearch.value = ''
    if (created && !failed) ui.success(`${created} meta(s) de consultor criada(s).`)
    else if (created) ui.info(`${created} criada(s), ${failed} erro(s).`)
    else ui.error('Nao foi possivel criar as metas de consultor.')
  } finally {
    bulkConsultantCreating.value = false
  }
}

async function saveRow(row) {
  const rowId = normalizeText(row?.id)
  if (!rowId) return { ok: true, noChange: true }

  rowSavePending[rowId] = true

  if (rowSaveInFlight.has(rowId)) {
    return rowSaveInFlight.get(rowId)
  }

  const promise = (async () => {
    rowBusy[rowId] = true
    let lastResult = { ok: true, noChange: true }

    try {
      while (rowSavePending[rowId]) {
        rowSavePending[rowId] = false

        const latestRow = goals.value.find((item) => item.id === rowId) || row
        const payload = buildRowPayload(latestRow)
        if (!isRowDirty(latestRow, payload)) {
          lastResult = { ok: true, noChange: true }
          continue
        }

        const result = await operationGoals.updateGoal(rowId, payload, {
          reload: false,
          skipLoadingIndicator: true,
        })
        if (result?.ok === false) {
          ui.error(result.message || 'Nao foi possivel atualizar a meta.')
          return result
        }
        if (result?.goal?.id && payloadMatchesDraft(result.goal.id, payload)) {
          drafts.value[result.goal.id] = createMetricDraft(result.goal)
        }
        crmStore.invalidateOverview()
        if (crmOverviewMatchesSelectedMonth.value) {
          void refreshCrmOverview()
        }
        lastResult = result
      }

      return lastResult
    } finally {
      rowSaveInFlight.delete(rowId)
      rowBusy[rowId] = false
      rowSavePending[rowId] = false
    }
  })()

  rowSaveInFlight.set(rowId, promise)
  return promise
}

async function removeRow(row) {
  const { confirmed } = await ui.confirm({
    title: 'Excluir meta',
    message: `A meta de ${row.consultantName || row.storeName} sera removida deste periodo. Deseja continuar?`,
    confirmLabel: 'Excluir',
  })
  if (!confirmed) return

  const result = await operationGoals.deleteGoal(row.id)
  if (result?.ok === false) {
    ui.error(result.message || 'Nao foi possivel excluir a meta.')
    return
  }
  ui.success('Meta excluida.')
}

/* ===== Edição inline com formatação ===== */

function updateMetricField(row, field, event) {
  const raw = String(event?.target?.value || '')
  const formatted = formatMoneyInput(raw)
  getDraft(row.id)[field] = formatted
}

async function handleInlineBlur(row) {
  const draft = getDraft(row.id)
  // força reformatação no blur
  draft.monthlyGoal = formatMoneyInput(draft.monthlyGoal)
  draft.avgTicketGoal = formatMoneyInput(draft.avgTicketGoal)
  draft.conversionGoal = formatMoneyInput(draft.conversionGoal)
  draft.paGoal = formatMoneyInput(draft.paGoal)

  await saveRow(row)
}

function handleInlineEnter(event) {
  event?.target?.blur?.()
}

function clearFilters() {
  storeSearch.value = ''
  consultantSearch.value = ''
  selectedMonth.value = currentMonth()
  selectedStoreId.value =
    props.allowAllStoreScope && activeStores.value.length > 1 ? '' : fallbackStoreId.value
}

function toggleSort(target, columnId) {
  if (target === 'store') {
    if (storeSortBy.value === columnId) {
      storeSortDir.value = storeSortDir.value === 'asc' ? 'desc' : 'asc'
    } else {
      storeSortBy.value = columnId
      storeSortDir.value = 'asc'
    }
  } else {
    if (consultantSortBy.value === columnId) {
      consultantSortDir.value = consultantSortDir.value === 'asc' ? 'desc' : 'asc'
    } else {
      consultantSortBy.value = columnId
      consultantSortDir.value = 'asc'
    }
  }
}

function exportCsv(target) {
  const rows = target === 'consultant' ? sortedConsultantRows.value : sortedStoreRows.value
  if (!rows.length) {
    ui.info('Nao ha metas para exportar.')
    return
  }

  const lines = [
    'Periodo;Escopo;Loja;Codigo loja;Consultor;Meta total;Ticket medio;Conversao;PA;Atualizado em',
  ]
  for (const row of rows) {
    lines.push(
      [
        row.month,
        row.scope === 'consultant' ? 'Consultor' : 'Loja',
        row.storeName,
        row.storeCode,
        row.consultantName,
        formatRawNumber(row.monthlyGoal),
        formatRawNumber(row.avgTicketGoal),
        formatRawNumber(row.conversionGoal),
        formatRawNumber(row.paGoal),
        row.updatedAt,
      ]
        .map(escapeCsvCell)
        .join(';'),
    )
  }

  const blob = new Blob(['﻿' + lines.join('\n')], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `metas-${target}-${selectedMonth.value}.csv`
  link.click()
  URL.revokeObjectURL(url)
  ui.success('CSV gerado.')
}

/* ===== Helpers de draft / parsing ===== */

function getDraft(rowId) {
  if (!drafts.value[rowId]) drafts.value[rowId] = createMetricDraft()
  return drafts.value[rowId]
}

function createMetricDraft(row = null) {
  return {
    monthlyGoal: formatEditableValue(row?.monthlyGoal),
    avgTicketGoal: formatEditableValue(row?.avgTicketGoal),
    conversionGoal: formatEditableValue(row?.conversionGoal),
    paGoal: formatEditableValue(row?.paGoal),
  }
}

function buildRowPayload(row) {
  const draft = getDraft(row.id)
  return {
    monthlyGoal: normalizeMetricNumber(draft.monthlyGoal),
    avgTicketGoal: normalizeMetricNumber(draft.avgTicketGoal),
    conversionGoal: clampPercent(draft.conversionGoal),
    paGoal: normalizeMetricNumber(draft.paGoal),
  }
}

function payloadsEqual(left, right) {
  return (
    normalizeMetricNumber(left?.monthlyGoal) === normalizeMetricNumber(right?.monthlyGoal) &&
    normalizeMetricNumber(left?.avgTicketGoal) === normalizeMetricNumber(right?.avgTicketGoal) &&
    clampPercent(left?.conversionGoal) === clampPercent(right?.conversionGoal) &&
    normalizeMetricNumber(left?.paGoal) === normalizeMetricNumber(right?.paGoal)
  )
}

function payloadMatchesDraft(rowId, payload) {
  return payloadsEqual(buildRowPayload({ id: rowId }), payload)
}

function isRowDirty(row, payload) {
  return (
    payload.monthlyGoal !== normalizeMetricNumber(row?.monthlyGoal) ||
    payload.avgTicketGoal !== normalizeMetricNumber(row?.avgTicketGoal) ||
    payload.conversionGoal !== clampPercent(row?.conversionGoal) ||
    payload.paGoal !== normalizeMetricNumber(row?.paGoal)
  )
}

function compareRows(left, right, field, direction) {
  const multiplier = direction === 'asc' ? 1 : -1
  const leftValue = sortValue(left, field)
  const rightValue = sortValue(right, field)
  if (leftValue < rightValue) return -1 * multiplier
  if (leftValue > rightValue) return 1 * multiplier
  return 0
}

function sortValue(row, field) {
  switch (field) {
    case 'storeLabel':
      return normalizeSearch(row.storeName)
    case 'consultantLabel':
      return normalizeSearch(row.consultantName || 'meta da loja')
    case 'updatedAtLabel':
    case 'updatedAt':
      return new Date(row.updatedAt || 0).getTime()
    default:
      if (
        [
          'monthlyGoal',
          'realizedSales',
          'goalCompletionPct',
          'avgTicketGoal',
          'conversionGoal',
          'paGoal',
        ].includes(field)
      ) {
        return Number(row[field] || 0) || 0
      }
      return normalizeSearch(row[field] || '')
  }
}

function normalizeSearch(value) {
  return String(value || '')
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .trim()
    .toLowerCase()
}

function normalizeText(value) {
  return String(value || '').trim()
}

function normalizeLookup(value) {
  return String(value || '')
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .trim()
    .toLowerCase()
}

function normalizeStoreCode(value) {
  return normalizeText(value).toUpperCase()
}

function buildMonthDateRange(month) {
  const normalized = /^\d{4}-\d{2}$/.test(normalizeText(month))
    ? normalizeText(month)
    : currentMonth()
  const [year, monthNumber] = normalized.split('-').map((part) => Number(part))
  const lastDay = new Date(Date.UTC(year, monthNumber, 0)).getUTCDate()

  return {
    dateFrom: `${normalized}-01`,
    dateTo: `${normalized}-${String(lastDay).padStart(2, '0')}`,
  }
}

function crmStoreSlugFromCodeName(code, name) {
  switch (normalizeStoreCode(code)) {
    case 'RIO':
    case 'PJ-RIO':
      return 'riomar'
    case 'JAR':
    case 'PJ-JAR':
      return 'jardins'
    case 'GAR':
    case 'PJ-GARCIA':
      return 'garcia'
    case 'TRE':
    case 'PJ-TRE':
      return 'treze'
    default:
      break
  }

  const normalizedName = normalizeLookup(name)
  if (normalizedName.includes('riomar')) return 'riomar'
  if (normalizedName.includes('jardins')) return 'jardins'
  if (normalizedName.includes('garcia')) return 'garcia'
  if (normalizedName.includes('treze')) return 'treze'
  return ''
}

function buildStoreLookupKeys(source = {}) {
  const keys = []
  const storeId = normalizeText(source.storeId)
  const storeCode = normalizeStoreCode(source.storeCode)
  const storeName = normalizeText(source.storeName)
  const storeSlug = normalizeText(source.storeSlug).toLowerCase()
  const resolvedSlug = storeSlug || crmStoreSlugFromCodeName(storeCode, storeName)

  if (storeId) keys.push(`id:${storeId}`)
  if (storeCode) keys.push(`code:${storeCode}`)
  if (resolvedSlug) keys.push(`slug:${resolvedSlug}`)
  if (storeName) keys.push(`name:${normalizeLookup(storeName)}`)

  return keys
}

function addStoreLookupKeys(map, row, source) {
  for (const key of buildStoreLookupKeys(source)) {
    if (!map[key]) {
      map[key] = row
    }
  }
}

function findStoreLookupRow(map, goal) {
  const keys = buildStoreLookupKeys({
    storeId: goal?.storeId,
    storeCode: goal?.storeCode,
    storeName: goal?.storeName,
  })

  for (const key of keys) {
    if (map[key]) {
      return map[key]
    }
  }
  return null
}

function moneyFromCents(value) {
  return Math.max(0, Number(value || 0) || 0) / 100
}

function buildStorePerformance(goal) {
  if (!crmDataReady.value) return null

  const storeRow = findStoreLookupRow(crmStoreRowsByKey.value, goal)
  const queueRow = findStoreLookupRow(crmQueueRowsByKey.value, goal)

  return {
    storeName: normalizeText(storeRow?.storeName || storeRow?.storeLabel || goal?.storeName),
    soldValue: moneyFromCents(storeRow?.salesCents),
    attendances: Math.max(0, Number(queueRow?.attendances ?? storeRow?.orders ?? 0) || 0),
    ticketAverage: moneyFromCents(storeRow?.ticketAverageCents),
    conversionRate: Math.max(0, Number(queueRow?.conversionRate || 0) || 0),
    paScore: Math.max(0, Number(storeRow?.paScore || 0) || 0),
  }
}

/* pt-BR: ponto = separador de milhar, vírgula = decimal */
function parseLocaleNumber(value) {
  const normalized = normalizeText(value)
  if (!normalized) return 0

  let sanitized = normalized
    .replace(/\s+/g, '')
    .replace(/[R$r$%]/gi, '')
    .replace(/[^0-9,.-]/g, '')

  sanitized = sanitized.replace(/\./g, '').replace(',', '.')

  const parsed = Number(sanitized)
  return Number.isFinite(parsed) ? parsed : 0
}

function normalizeMetricNumber(value) {
  return Math.max(0, parseLocaleNumber(value))
}

function clampPercent(value) {
  return Math.min(100, normalizeMetricNumber(value))
}

function currentMonth() {
  const now = new Date()
  return `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, '0')}`
}

function normalizeMonthValue(value) {
  const normalized = normalizeText(value)
  return /^\d{4}-\d{2}$/.test(normalized) ? normalized : ''
}

/* Recebe número OU string; devolve string formatada pt-BR (ex: "40.000" ou "1.750,5") */
function formatEditableValue(value) {
  if (value === null || value === undefined || value === '') return ''
  if (typeof value === 'number') {
    if (!Number.isFinite(value) || value === 0) return ''
    const fixed = Number.isInteger(value) ? String(value) : String(value).replace('.', ',')
    return formatMoneyInput(fixed)
  }
  return formatMoneyInput(String(value))
}

/* Formata input pt-BR: insere pontos de milhar, mantém vírgula como decimal */
function formatMoneyInput(raw) {
  const str = String(raw ?? '')
  const cleaned = str.replace(/[^\d,]/g, '')
  if (!cleaned) return ''

  const firstComma = cleaned.indexOf(',')
  let intPart = ''
  let decPart
  if (firstComma === -1) {
    intPart = cleaned
  } else {
    intPart = cleaned.slice(0, firstComma)
    decPart = cleaned
      .slice(firstComma + 1)
      .replace(/,/g, '')
      .slice(0, 2)
  }

  // remove zeros à esquerda mas mantém pelo menos "0" se vazio
  intPart = intPart.replace(/^0+(?=\d)/, '')
  const intFormatted = (intPart || '0').replace(/\B(?=(\d{3})+(?!\d))/g, '.')

  if (decPart !== undefined) return `${intFormatted},${decPart}`
  return intFormatted
}

function formatUpdatedAt(value) {
  if (!normalizeText(value)) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return new Intl.DateTimeFormat('pt-BR', {
    dateStyle: 'short',
    timeStyle: 'short',
  }).format(date)
}

function formatRawNumber(value) {
  return (Number(value || 0) || 0).toFixed(2)
}

function escapeCsvCell(value) {
  const text = String(value ?? '')
  if (/[";\n]/.test(text)) return `"${text.replace(/"/g, '""')}"`
  return text
}
</script>

<template>
  <section class="multistore-goals" data-testid="multistore-goals-section">
    <!-- Cards BI: performance vs realizado -->
    <div v-if="showCards" class="multistore-goals__cards">
      <!-- Card 1: % cumprimento da meta vigente -->
      <article
        class="multistore-goals__card"
        :class="`is-${pctStatus(completionPct)}`"
        :title="
          completionPct === null
            ? 'Sem dados de vendas para o periodo'
            : `R$ ${formatCurrencyBRL(totalRealized).replace('R$', '').trim()} de R$ ${formatCurrencyBRL(totalGoalWithRealized).replace('R$', '').trim()}`
        "
      >
        <span class="multistore-goals__card-label">Cumprimento da meta</span>
        <strong v-if="completionPct === null" class="multistore-goals__card-value is-muted">
          —
        </strong>
        <strong v-else class="multistore-goals__card-value">
          {{ Math.round(completionPct) }}
          <small>%</small>
        </strong>
        <small class="multistore-goals__card-meta">
          <template v-if="completionPct === null">
            {{ cardsLoadingCrm ? 'Carregando CRM...' : 'Sem vendas registradas' }}
          </template>
          <template v-else-if="completionPct >= 100">
            Bateu +{{ formatCurrencyBRL(totalRealized - totalGoalWithRealized) }}
          </template>
          <template v-else>Faltam {{ formatCurrencyBRL(completionRemaining) }}</template>
        </small>
        <div v-if="completionPct !== null" class="multistore-goals__card-bar">
          <div
            class="multistore-goals__card-bar-fill"
            :style="{ width: `${Math.min(100, completionPct)}%` }"
          ></div>
        </div>
      </article>

      <!-- Card 2: Projeção de fechamento -->
      <article
        class="multistore-goals__card"
        :class="`is-${pctStatus(projectionPct)}`"
        :title="
          projectionPct === null
            ? 'Projecao disponivel apenas no mes corrente'
            : 'Se manter o ritmo dos dias decorridos'
        "
      >
        <span class="multistore-goals__card-label">Projecao do mes</span>
        <strong v-if="projectionPct === null" class="multistore-goals__card-value is-muted">
          —
        </strong>
        <strong v-else class="multistore-goals__card-value">
          {{ Math.round(projectionPct) }}
          <small>%</small>
        </strong>
        <small class="multistore-goals__card-meta">
          <template v-if="projectionPct === null">
            {{
              cardsLoadingCrm
                ? 'Carregando CRM...'
                : isCurrentMonthSelected
                  ? 'Sem dados suficientes'
                  : 'Mes nao corrente'
            }}
          </template>
          <template v-else-if="projectionDelta >= 0">
            Super.: {{ formatCurrencyBRL(projectionDelta) }}
          </template>
          <template v-else>Gap proj.: {{ formatCurrencyBRL(Math.abs(projectionDelta)) }}</template>
        </small>
      </article>

      <!-- Card 3: Top loja -->
      <article
        class="multistore-goals__card"
        :class="topStore ? `is-${pctStatus(topStore.completionPct)}` : 'is-neutral'"
        title="Loja com maior % de cumprimento da meta"
      >
        <span class="multistore-goals__card-label">Top do mes</span>
        <strong v-if="!topStore" class="multistore-goals__card-value is-muted">—</strong>
        <strong v-else class="multistore-goals__card-value multistore-goals__card-value--name">
          {{ topStore.storeName }}
        </strong>
        <small class="multistore-goals__card-meta">
          <template v-if="!topStore">
            {{ cardsLoadingCrm ? 'Carregando CRM...' : 'Sem dados' }}
          </template>
          <template v-else>{{ Math.round(topStore.completionPct) }}% atingido</template>
        </small>
      </article>

      <!-- Card 4: Loja em maior gap -->
      <article
        class="multistore-goals__card"
        :class="worstGapStore ? `is-${pctStatus(worstGapStore.completionPct)}` : 'is-neutral'"
        title="Loja mais distante da meta"
      >
        <span class="multistore-goals__card-label">Em atencao</span>
        <strong v-if="!worstGapStore" class="multistore-goals__card-value is-muted">—</strong>
        <strong v-else class="multistore-goals__card-value multistore-goals__card-value--name">
          {{ worstGapStore.storeName }}
        </strong>
        <small class="multistore-goals__card-meta">
          <template v-if="!worstGapStore">
            {{ cardsLoadingCrm ? 'Carregando CRM...' : 'Sem dados' }}
          </template>
          <template v-else>
            {{ Math.round(worstGapStore.completionPct) }}% / faltam
            {{ formatCurrencyBRL(worstGapStore.gap) }}
          </template>
        </small>
      </article>
    </div>

    <!-- Tabela de lojas -->
    <div v-if="showTables" class="multistore-goals__block">
      <header class="multistore-goals__block-head">
        <h3 class="multistore-goals__block-title">Metas por loja</h3>
        <span class="multistore-goals__block-sub">{{ sortedStoreRows.length }} registros</span>
      </header>

      <AppEntityGrid
        :columns="storeGridColumns"
        :rows="sortedStoreRows"
        :row-key="(row) => row.id"
        :loading="pending && !ready"
        :search-value="storeSearch"
        :sort-by="storeSortBy"
        :sort-dir="storeSortDir"
        storage-key="multistore-goals-store-columns-v4"
        empty-title="Nenhuma meta de loja"
        :empty-text="
          errorMessage && !pending
            ? errorMessage
            : 'Clique no botao de gerar para criar as metas de todas as lojas do mes.'
        "
        testid="multistore-goals-store-grid"
        class="multistore-goals__grid"
        @update:search-value="storeSearch = $event"
        @sort="(col) => toggleSort('store', col)"
      >
        <template #toolbar-filters>
          <label class="multistore-goals__month-chip">
            <input v-model="selectedMonth" type="month" />
          </label>

          <AppSelectField
            class="multistore-goals__toolbar-select"
            :model-value="selectedStoreId"
            :options="storeFilterOptions"
            :show-leading-icon="false"
            compact
            @update:model-value="selectedStoreId = $event"
          />
        </template>

        <template #toolbar-actions>
          <button
            v-if="canEditGoals"
            class="multistore-goals__icon-btn multistore-goals__icon-btn--primary"
            type="button"
            :disabled="!canBulkCreate"
            :title="
              bulkCreating
                ? 'Criando metas...'
                : storesWithoutGoal.length
                  ? `Criar metas do mes para ${storesWithoutGoal.length} loja(s)`
                  : 'Todas as lojas ja possuem meta'
            "
            aria-label="Criar metas do mes"
            @click="createAllStoreGoals"
          >
            <Sparkles :size="14" :stroke-width="2.1" />
          </button>

          <button
            class="multistore-goals__icon-btn"
            type="button"
            :disabled="!sortedStoreRows.length"
            title="Exportar lojas (CSV)"
            aria-label="Exportar lojas"
            @click="exportCsv('store')"
          >
            <Download :size="14" :stroke-width="2.1" />
          </button>

          <button
            class="multistore-goals__icon-btn multistore-goals__icon-btn--ghost"
            type="button"
            title="Limpar filtros"
            aria-label="Limpar filtros"
            @click="clearFilters"
          >
            <RotateCcw :size="13" :stroke-width="2.1" />
          </button>
        </template>

        <template #cell-month="{ row }">
          <span>{{ row.month || selectedMonth }}</span>
        </template>

        <template #cell-storeLabel="{ row }">
          <div class="multistore-goals__identity">
            <strong>{{ row.storeName }}</strong>
            <small>{{ row.storeCode || 'Sem codigo' }}</small>
          </div>
        </template>

        <template #cell-monthlyGoal="{ row }">
          <label v-if="canEditGoals" class="multistore-goals__field">
            <span class="multistore-goals__affix multistore-goals__affix--prefix">R$</span>
            <input
              :value="getDraft(row.id).monthlyGoal"
              class="multistore-goals__inline-input"
              type="text"
              inputmode="decimal"
              placeholder="0"
              @input="updateMetricField(row, 'monthlyGoal', $event)"
              @blur="handleInlineBlur(row)"
              @keydown.enter.prevent="handleInlineEnter($event)"
            />
          </label>
          <span v-else>{{ formatCurrencyBRL(row.monthlyGoal) }}</span>
        </template>

        <template #cell-realizedSales="{ row }">
          <span v-if="row.realizedSales !== null" class="multistore-goals__metric">
            {{ formatCurrencyBRL(row.realizedSales) }}
          </span>
          <span v-else class="multistore-goals__muted">
            {{ crmPending ? 'CRM...' : '-' }}
          </span>
        </template>

        <template #cell-goalCompletionPct="{ row }">
          <span
            v-if="row.goalCompletionPct !== null"
            class="multistore-goals__metric"
            :class="`is-${pctStatus(row.goalCompletionPct)}`"
          >
            {{ formatPercent(row.goalCompletionPct) }}
          </span>
          <span v-else class="multistore-goals__muted">-</span>
        </template>

        <template #cell-avgTicketGoal="{ row }">
          <label v-if="canEditGoals" class="multistore-goals__field">
            <span class="multistore-goals__affix multistore-goals__affix--prefix">R$</span>
            <input
              :value="getDraft(row.id).avgTicketGoal"
              class="multistore-goals__inline-input"
              type="text"
              inputmode="decimal"
              placeholder="0"
              @input="updateMetricField(row, 'avgTicketGoal', $event)"
              @blur="handleInlineBlur(row)"
              @keydown.enter.prevent="handleInlineEnter($event)"
            />
          </label>
          <span v-else>{{ formatCurrencyBRL(row.avgTicketGoal) }}</span>
        </template>

        <template #cell-conversionGoal="{ row }">
          <label v-if="canEditGoals" class="multistore-goals__field">
            <input
              :value="getDraft(row.id).conversionGoal"
              class="multistore-goals__inline-input"
              type="text"
              inputmode="decimal"
              placeholder="0"
              @input="updateMetricField(row, 'conversionGoal', $event)"
              @blur="handleInlineBlur(row)"
              @keydown.enter.prevent="handleInlineEnter($event)"
            />
            <span class="multistore-goals__affix multistore-goals__affix--suffix">%</span>
          </label>
          <span v-else>{{ formatPercent(row.conversionGoal) }}</span>
        </template>

        <template #cell-paGoal="{ row }">
          <label v-if="canEditGoals" class="multistore-goals__field">
            <input
              :value="getDraft(row.id).paGoal"
              class="multistore-goals__inline-input"
              type="text"
              inputmode="decimal"
              placeholder="0,00"
              @input="updateMetricField(row, 'paGoal', $event)"
              @blur="handleInlineBlur(row)"
              @keydown.enter.prevent="handleInlineEnter($event)"
            />
          </label>
          <span v-else>{{ Number(row.paGoal || 0).toFixed(2) }}</span>
        </template>

        <template #cell-updatedAtLabel="{ row }">
          <span>{{ row.updatedAtLabel }}</span>
        </template>

        <template v-if="canEditGoals" #cell-actions="{ row }">
          <div class="multistore-goals__row-actions">
            <span v-if="rowBusy[row.id]" class="multistore-goals__pulse" aria-label="Salvando">
              <span></span>
              <span></span>
              <span></span>
            </span>
            <button
              v-else
              class="multistore-goals__icon-btn multistore-goals__icon-btn--ghost"
              type="button"
              tabindex="-1"
              :disabled="saving"
              title="Excluir meta"
              aria-label="Excluir meta"
              @click="removeRow(row)"
            >
              <Trash2 :size="13" :stroke-width="2.1" />
            </button>
          </div>
        </template>
      </AppEntityGrid>
    </div>

    <!-- Bulk consultor: seleciona loja → cria metas de todos os consultores -->
    <div v-if="showTables && canEditGoals" class="multistore-goals__bulk">
      <UserPlus class="multistore-goals__bulk-icon" :size="14" :stroke-width="2.1" />
      <span class="multistore-goals__bulk-label">Gerar metas para consultores de:</span>
      <AppSelectField
        class="multistore-goals__bulk-select"
        :model-value="bulkConsultantStoreId"
        :options="bulkConsultantStoreOptions"
        placeholder="Selecione a loja"
        :show-leading-icon="false"
        compact
        :disabled="bulkConsultantCreating"
        @update:model-value="bulkConsultantStoreId = $event"
      />
      <button
        class="multistore-goals__bulk-btn"
        type="button"
        :disabled="!canBulkConsultant"
        @click="createConsultantGoalsForStore"
      >
        {{ bulkConsultantCreating ? 'Gerando...' : 'Gerar' }}
      </button>
      <small class="multistore-goals__bulk-hint">
        Cria metas zeradas para todos os consultores ativos da loja. Edite inline.
      </small>
    </div>

    <!-- Tabela de consultores -->
    <div v-if="showTables" class="multistore-goals__block">
      <header class="multistore-goals__block-head">
        <h3 class="multistore-goals__block-title">Metas por consultor</h3>
        <span class="multistore-goals__block-sub">{{ sortedConsultantRows.length }} registros</span>
      </header>

      <AppEntityGrid
        :columns="consultantGridColumns"
        :rows="sortedConsultantRows"
        :row-key="(row) => row.id"
        :loading="pending && !ready"
        :search-value="consultantSearch"
        :sort-by="consultantSortBy"
        :sort-dir="consultantSortDir"
        storage-key="multistore-goals-consultant-columns-v3"
        empty-title="Nenhuma meta de consultor"
        :empty-text="
          errorMessage && !pending
            ? errorMessage
            : 'Selecione uma loja acima e clique em Gerar para criar metas para todos os consultores dela.'
        "
        testid="multistore-goals-consultant-grid"
        class="multistore-goals__grid"
        @update:search-value="consultantSearch = $event"
        @sort="(col) => toggleSort('consultant', col)"
      >
        <template #toolbar-actions>
          <button
            class="multistore-goals__icon-btn"
            type="button"
            :disabled="!sortedConsultantRows.length"
            title="Exportar consultores (CSV)"
            aria-label="Exportar consultores"
            @click="exportCsv('consultant')"
          >
            <Download :size="14" :stroke-width="2.1" />
          </button>
        </template>

        <template #cell-month="{ row }">
          <span>{{ row.month }}</span>
        </template>

        <template #cell-storeLabel="{ row }">
          <span class="multistore-goals__store-tag">{{ row.storeName }}</span>
        </template>

        <template #cell-consultantLabel="{ row }">
          <div class="multistore-goals__identity">
            <strong>{{ row.consultantName }}</strong>
            <small>{{ row.storeCode || 'Sem codigo' }}</small>
          </div>
        </template>

        <template #cell-monthlyGoal="{ row }">
          <label v-if="canEditGoals" class="multistore-goals__field">
            <span class="multistore-goals__affix multistore-goals__affix--prefix">R$</span>
            <input
              :value="getDraft(row.id).monthlyGoal"
              class="multistore-goals__inline-input"
              type="text"
              inputmode="decimal"
              placeholder="0"
              @input="updateMetricField(row, 'monthlyGoal', $event)"
              @blur="handleInlineBlur(row)"
              @keydown.enter.prevent="handleInlineEnter($event)"
            />
          </label>
          <span v-else>{{ formatCurrencyBRL(row.monthlyGoal) }}</span>
        </template>

        <template #cell-avgTicketGoal="{ row }">
          <label v-if="canEditGoals" class="multistore-goals__field">
            <span class="multistore-goals__affix multistore-goals__affix--prefix">R$</span>
            <input
              :value="getDraft(row.id).avgTicketGoal"
              class="multistore-goals__inline-input"
              type="text"
              inputmode="decimal"
              placeholder="0"
              @input="updateMetricField(row, 'avgTicketGoal', $event)"
              @blur="handleInlineBlur(row)"
              @keydown.enter.prevent="handleInlineEnter($event)"
            />
          </label>
          <span v-else>{{ formatCurrencyBRL(row.avgTicketGoal) }}</span>
        </template>

        <template #cell-conversionGoal="{ row }">
          <label v-if="canEditGoals" class="multistore-goals__field">
            <input
              :value="getDraft(row.id).conversionGoal"
              class="multistore-goals__inline-input"
              type="text"
              inputmode="decimal"
              placeholder="0"
              @input="updateMetricField(row, 'conversionGoal', $event)"
              @blur="handleInlineBlur(row)"
              @keydown.enter.prevent="handleInlineEnter($event)"
            />
            <span class="multistore-goals__affix multistore-goals__affix--suffix">%</span>
          </label>
          <span v-else>{{ formatPercent(row.conversionGoal) }}</span>
        </template>

        <template #cell-paGoal="{ row }">
          <label v-if="canEditGoals" class="multistore-goals__field">
            <input
              :value="getDraft(row.id).paGoal"
              class="multistore-goals__inline-input"
              type="text"
              inputmode="decimal"
              placeholder="0,00"
              @input="updateMetricField(row, 'paGoal', $event)"
              @blur="handleInlineBlur(row)"
              @keydown.enter.prevent="handleInlineEnter($event)"
            />
          </label>
          <span v-else>{{ Number(row.paGoal || 0).toFixed(2) }}</span>
        </template>

        <template #cell-updatedAtLabel="{ row }">
          <span>{{ row.updatedAtLabel }}</span>
        </template>

        <template v-if="canEditGoals" #cell-actions="{ row }">
          <div class="multistore-goals__row-actions">
            <span v-if="rowBusy[row.id]" class="multistore-goals__pulse" aria-label="Salvando">
              <span></span>
              <span></span>
              <span></span>
            </span>
            <button
              v-else
              class="multistore-goals__icon-btn multistore-goals__icon-btn--ghost"
              type="button"
              tabindex="-1"
              :disabled="saving"
              title="Excluir meta"
              aria-label="Excluir meta"
              @click="removeRow(row)"
            >
              <Trash2 :size="13" :stroke-width="2.1" />
            </button>
          </div>
        </template>
      </AppEntityGrid>
    </div>
  </section>
</template>

<style scoped>
.multistore-goals {
  display: grid;
  gap: 0.85rem;
}

/* ===== Cards inteligentes ===== */
.multistore-goals__cards {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.55rem;
}

.multistore-goals__card {
  position: relative;
  display: grid;
  gap: 0.22rem;
  padding: 0.7rem 0.85rem 0.75rem;
  border-radius: 0.7rem;
  border: 1px solid rgb(var(--ring) / 0.14);
  background: rgb(var(--surface-2) / 0.55);
  min-width: 0;
  overflow: hidden;
}

.multistore-goals__card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  width: 3px;
  height: 100%;
  background: rgb(var(--ring) / 0.18);
}

.multistore-goals__card.is-success::before {
  background: rgb(34 197 94);
}

.multistore-goals__card.is-warn::before {
  background: rgb(234 179 8);
}

.multistore-goals__card.is-danger::before {
  background: rgb(239 68 68);
}

.multistore-goals__card.is-neutral::before {
  background: rgb(var(--ring) / 0.16);
}

.multistore-goals__card-label {
  font-size: 0.62rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.multistore-goals__card-value {
  font-size: 1.4rem;
  font-weight: 800;
  color: var(--text-main);
  line-height: 1;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.01em;
}

.multistore-goals__card-value--name {
  font-size: 0.95rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.multistore-goals__card-value small {
  font-size: 0.78rem;
  font-weight: 700;
  color: var(--text-muted);
  margin-left: 0.05rem;
}

.multistore-goals__card-value.is-muted {
  color: var(--text-muted);
  font-size: 1.6rem;
}

.multistore-goals__card.is-success .multistore-goals__card-value {
  color: rgb(34 197 94);
}

.multistore-goals__card.is-warn .multistore-goals__card-value {
  color: rgb(234 179 8);
}

.multistore-goals__card.is-danger .multistore-goals__card-value {
  color: rgb(239 68 68);
}

.multistore-goals__card-meta {
  font-size: 0.68rem;
  color: var(--text-muted);
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.multistore-goals__card-bar {
  margin-top: 0.3rem;
  height: 3px;
  border-radius: 999px;
  background: rgb(var(--ring) / 0.12);
  overflow: hidden;
}

.multistore-goals__card-bar-fill {
  height: 100%;
  background: rgb(var(--ring) / 0.4);
  transition: width 0.3s ease;
}

.multistore-goals__card.is-success .multistore-goals__card-bar-fill {
  background: rgb(34 197 94);
}

.multistore-goals__card.is-warn .multistore-goals__card-bar-fill {
  background: rgb(234 179 8);
}

.multistore-goals__card.is-danger .multistore-goals__card-bar-fill {
  background: rgb(239 68 68);
}

/* ===== Bloco (cabeçalho + grid) ===== */
.multistore-goals__block {
  display: grid;
  gap: 0.4rem;
}

.multistore-goals__block-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.6rem;
  padding-inline: 0.15rem;
}

.multistore-goals__block-title {
  margin: 0;
  font-size: 0.84rem;
  font-weight: 800;
  color: var(--text-main);
  letter-spacing: -0.005em;
}

.multistore-goals__block-sub {
  font-size: 0.68rem;
  font-weight: 600;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

/* ===== Toolbar de cada grid em linha única ===== */
.multistore-goals__grid :deep(.app-entity-grid) {
  padding: 0.6rem;
  gap: 0.5rem;
}

.multistore-goals__grid :deep(.app-entity-grid__toolbar) {
  align-items: center;
  flex-wrap: nowrap;
  overflow-x: auto;
  gap: 0.4rem;
  padding-bottom: 0.1rem;
}

.multistore-goals__grid :deep(.app-entity-grid__toolbar-main) {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex: 1 1 auto;
  min-width: 0;
}

.multistore-goals__grid :deep(.app-entity-grid__search) {
  flex: 0 1 12rem;
  min-width: 10rem;
  min-height: 2rem;
}

.multistore-goals__grid :deep(.app-entity-grid__search-input) {
  font-size: 0.74rem;
}

.multistore-goals__grid :deep(.app-entity-grid__filters) {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  flex-wrap: nowrap;
  flex: 0 0 auto;
}

.multistore-goals__grid :deep(.app-entity-grid__toolbar-actions) {
  gap: 0.3rem;
  flex-wrap: nowrap;
  flex: 0 0 auto;
}

.multistore-goals__grid :deep(.app-entity-grid__toolbar-btn) {
  min-height: 2rem;
  padding: 0 0.6rem;
  font-size: 0.7rem;
}

.multistore-goals__grid :deep(.app-entity-grid__head-cell) {
  font-size: 0.6rem;
  letter-spacing: 0.05em;
  padding: 0 0.25rem;
}

.multistore-goals__grid :deep(.app-entity-grid__row) {
  padding: 0.28rem 0;
  gap: 0.35rem;
}

.multistore-goals__grid :deep(.app-entity-grid__cell) {
  padding: 0 0.25rem;
  font-size: 0.76rem;
}

/* Filtros e botões */
.multistore-goals__toolbar-select {
  min-width: 8.5rem;
}

.multistore-goals__month-chip {
  display: inline-flex;
  align-items: center;
  min-height: 2rem;
  padding: 0 0.6rem;
  border-radius: 0.55rem;
  border: 1px solid rgb(var(--ring) / 0.16);
  background: rgb(var(--surface-2) / 0.88);
  flex-shrink: 0;
}

.multistore-goals__month-chip input {
  border: none;
  outline: none;
  background: transparent;
  color: var(--text-main);
  font-size: 0.72rem;
  font-weight: 600;
  color-scheme: dark light;
}

.multistore-goals__icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border-radius: 0.55rem;
  border: 1px solid rgb(var(--ring) / 0.18);
  background: rgb(var(--surface-2) / 0.88);
  color: var(--text-main);
  cursor: pointer;
  flex-shrink: 0;
  transition:
    border-color 0.12s ease,
    background 0.12s ease,
    color 0.12s ease;
}

.multistore-goals__icon-btn:hover:not(:disabled) {
  border-color: rgb(var(--ring) / 0.4);
  color: rgb(var(--text));
}

.multistore-goals__icon-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.multistore-goals__icon-btn--primary {
  border-color: rgb(var(--primary) / 0.4);
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.multistore-goals__icon-btn--primary:hover:not(:disabled) {
  background: rgb(var(--primary) / 0.24);
}

.multistore-goals__icon-btn--ghost {
  background: transparent;
  border-color: rgb(var(--ring) / 0.12);
  color: var(--text-muted);
}

/* ===== Bulk consultor: faixa horizontal ===== */
.multistore-goals__bulk {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  padding: 0.55rem 0.75rem;
  border-radius: 0.65rem;
  border: 1px dashed rgb(var(--ring) / 0.22);
  background: rgb(var(--surface-2) / 0.35);
}

.multistore-goals__bulk-icon {
  color: rgb(var(--primary));
  flex-shrink: 0;
}

.multistore-goals__bulk-label {
  font-size: 0.74rem;
  font-weight: 600;
  color: var(--text-main);
  flex-shrink: 0;
}

.multistore-goals__bulk-select {
  min-width: 11rem;
  flex: 0 1 14rem;
}

.multistore-goals__bulk-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2rem;
  padding: 0 0.85rem;
  border-radius: 0.55rem;
  border: 1px solid rgb(var(--primary) / 0.4);
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
  font-size: 0.72rem;
  font-weight: 700;
  cursor: pointer;
  flex-shrink: 0;
  transition: background 0.12s ease;
}

.multistore-goals__bulk-btn:hover:not(:disabled) {
  background: rgb(var(--primary) / 0.24);
}

.multistore-goals__bulk-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.multistore-goals__bulk-hint {
  font-size: 0.66rem;
  color: var(--text-muted);
  flex: 1 1 auto;
  text-align: right;
  min-width: 12rem;
}

/* ===== Cells / inputs ===== */
.multistore-goals__identity {
  display: grid;
  gap: 0;
  min-width: 0;
}

.multistore-goals__identity strong {
  font-size: 0.8rem;
  font-weight: 700;
  color: var(--text-main);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.25;
}

.multistore-goals__identity small {
  font-size: 0.62rem;
  color: var(--text-muted);
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.multistore-goals__store-tag {
  display: inline-flex;
  align-items: center;
  min-height: 1.5rem;
  padding: 0 0.55rem;
  border-radius: 999px;
  border: 1px solid rgb(var(--ring) / 0.14);
  background: rgb(var(--surface-2) / 0.7);
  font-size: 0.66rem;
  font-weight: 700;
  color: var(--text-muted);
  letter-spacing: 0.02em;
}

.multistore-goals__metric {
  font-size: 0.78rem;
  font-weight: 700;
  color: var(--text-main);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.multistore-goals__metric.is-success {
  color: rgb(34 197 94);
}

.multistore-goals__metric.is-warn {
  color: rgb(234 179 8);
}

.multistore-goals__metric.is-danger {
  color: rgb(239 68 68);
}

.multistore-goals__muted {
  font-size: 0.74rem;
  color: var(--text-muted);
  font-weight: 600;
}

.multistore-goals__field {
  display: inline-flex;
  align-items: center;
  width: 100%;
  min-height: 1.9rem;
  border-radius: 0.5rem;
  border: 1px solid rgb(var(--ring) / 0.14);
  background: rgb(var(--surface-2) / 0.85);
  overflow: hidden;
  transition:
    border-color 0.12s ease,
    box-shadow 0.12s ease;
}

.multistore-goals__field:focus-within {
  border-color: rgb(var(--ring) / 0.4);
  box-shadow: 0 0 0 2px rgb(var(--ring) / 0.1);
}

.multistore-goals__affix {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 0.66rem;
  font-weight: 700;
  color: var(--text-muted);
  flex-shrink: 0;
  user-select: none;
}

.multistore-goals__affix--prefix {
  padding: 0 0.05rem 0 0.5rem;
}

.multistore-goals__affix--suffix {
  padding: 0 0.5rem 0 0.05rem;
}

.multistore-goals__inline-input {
  flex: 1 1 auto;
  width: 100%;
  min-width: 0;
  height: 1.9rem;
  padding: 0 0.45rem;
  border: none;
  background: transparent;
  color: var(--text-main);
  font-size: 0.8rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  text-align: right;
  outline: none;
}

.multistore-goals__inline-input::placeholder {
  color: rgb(var(--muted) / 0.5);
  font-weight: 500;
}

.multistore-goals__inline-input:disabled {
  opacity: 0.55;
}

.multistore-goals__row-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
}

.multistore-goals__pulse {
  display: inline-flex;
  gap: 3px;
  padding: 0 0.3rem;
}

.multistore-goals__pulse span {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: rgb(var(--primary) / 0.6);
  animation: multistore-goals-pulse 1s infinite ease-in-out;
}

.multistore-goals__pulse span:nth-child(2) {
  animation-delay: 0.15s;
}

.multistore-goals__pulse span:nth-child(3) {
  animation-delay: 0.3s;
}

@keyframes multistore-goals-pulse {
  0%,
  80%,
  100% {
    transform: scale(0.7);
    opacity: 0.4;
  }
  40% {
    transform: scale(1);
    opacity: 1;
  }
}

/* ===== Responsivo ===== */
@media (max-width: 960px) {
  .multistore-goals__cards {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .multistore-goals__grid :deep(.app-entity-grid__search) {
    flex: 1 1 8rem;
    min-width: 7rem;
  }

  .multistore-goals__toolbar-select {
    min-width: 7rem;
  }

  .multistore-goals__bulk-hint {
    text-align: left;
    flex-basis: 100%;
  }
}

@media (max-width: 560px) {
  .multistore-goals__cards {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
