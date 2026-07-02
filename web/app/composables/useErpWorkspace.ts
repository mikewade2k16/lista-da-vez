import { computed, onBeforeUnmount, ref, watch } from 'vue'

import { useErpCsvExport } from '~/composables/useErpCsvExport'
import { useErpSyncActions } from '~/composables/useErpSyncActions'
import { useErpSyncNotifications } from '~/composables/useErpSyncNotifications'
import {
  ERP_BANCO_SECTION_BY_TAB,
  ERP_BANCO_TABS,
  ERP_PAGE_SIZE_OPTIONS,
  ERP_PRODUCT_COLUMNS,
  ERP_RECORDS_BOOTSTRAP_LABEL_BY_TAB,
  ERP_RECORDS_COLUMNS_BY_TAB,
  ERP_RECORDS_DATA_TYPE_BY_TAB,
  ERP_RECORDS_GENERAL_SEARCH_PLACEHOLDER_BY_TAB,
  ERP_RECORDS_SPECIFIC_SEARCH_BY_TAB,
  ERP_TABS,
  castRow,
  formatCurrency,
  formatDateTime,
  formatNumber,
  formatPrice,
  formatSourceFileName,
  productRowKey,
  recordsRowKey,
  type ErpImportedFile,
  type ErpOrderStats,
  type ErpRun,
} from '~/domain/utils/erp-display'
import { useAuthStore } from '~/stores/auth'
import { useErpStore, type ErpRecordsFacetOption } from '~/stores/erp'
import { useUiStore } from '~/stores/ui'

type ErpCrmWorkspaceHandle = {
  loadCRM: () => Promise<void> | void
  loadCustomers: (payload?: { page?: number }) => Promise<void> | void
}

type ErpTypeStatus = {
  dataType?: string | null
  lastImportedFile?: ErpImportedFile | null
  lastRun?: ErpRun | null
  totalRows?: number | string | null
}

export function useErpWorkspace() {
  const auth = useAuthStore()
  const erpStore = useErpStore()
  const ui = useUiStore()

  // Pagina inicial do ERP = Compras (pedidos).
  const activeTab = ref('pedidos')
  const activeBancoTab = ref('geral')
  const selectedSyncRunId = ref('')
  const searchValue = ref('')
  const identifierPrefixValue = ref('')
  const productsSortBy = ref('source_batch_date')
  const productsSortDir = ref('desc')
  const productsDateFrom = ref('')
  const productsDateTo = ref('')
  const recordsSearchValue = ref('')
  const recordsSpecificSearchValue = ref('')
  const recordsSortBy = ref('source_batch_date')
  const recordsSortDir = ref('desc')
  // Default = mes atual (aba inicial e Compras), para sempre cair no range scan
  // do indice order_date em vez de varrer o historico inteiro.
  const initialRecordsRange = currentMonthRecordsRange()
  const recordsDateFrom = ref(initialRecordsRange.from)
  const recordsDateTo = ref(initialRecordsRange.to)
  // Filtros do sorteio na aba Compras: data real da compra (order_date) por padrao
  // com toggle para o lote importado (batch_date), e valor minimo da compra em reais.
  const recordsDateField = ref('order_date')
  const recordsMinValue = ref('')
  // Filtros server-side de loja/consultor na aba Compras/Cancelados. Valor vazio =
  // todas as lojas / todos os consultores. As opcoes (facetas) vem do periodo real.
  const recordsStoreFilter = ref('')
  const recordsEmployeeFilter = ref('')
  const recordsStoreOptions = ref<ErpRecordsFacetOption[]>([])
  const recordsEmployeeOptions = ref<ErpRecordsFacetOption[]>([])
  const crmRef = ref<ErpCrmWorkspaceHandle | null>(null)
  let productsLoadTimer: ReturnType<typeof setTimeout> | null = null
  let recordsLoadTimer: ReturnType<typeof setTimeout> | null = null

  const status = computed(() => erpStore.status)
  const currentProductCount = computed(() => Number(status.value?.productCurrent || 0))
  const rawItemRows = computed(() => Number(status.value?.rawItemRows || 0))
  const lastRun = computed(() => status.value?.lastRun || null)
  const lastImportedFile = computed(() => status.value?.lastImportedFile || null)
  const overview = computed(() => erpStore.overview || null)
  const erpScopeLabel = computed(() => 'Sistema completo')
  const erpScopeHint = computed(
    () => 'Escopo consolidado da integracao ERP; nao segue a subloja operacional do topo.',
  )
  const isERPSystemAdmin = computed(() => auth.role === 'platform_admin')
  const canSync = computed(() => isERPSystemAdmin.value)
  const tabs = computed(() =>
    ERP_TABS.filter(
      (tab) => isERPSystemAdmin.value || (tab.id !== 'banco' && tab.id !== 'sincronizacao'),
    ),
  )
  const activeRecordsDataType = computed(() => ERP_RECORDS_DATA_TYPE_BY_TAB[activeTab.value] || '')
  const activeRecordsColumns = computed(() => ERP_RECORDS_COLUMNS_BY_TAB[activeTab.value] || [])
  const activeRecordsBootstrapLabel = computed(
    () => ERP_RECORDS_BOOTSTRAP_LABEL_BY_TAB[activeTab.value] || 'Bootstrap registros ERP',
  )
  const activeRecordsSpecificSearch = computed(
    () =>
      ERP_RECORDS_SPECIFIC_SEARCH_BY_TAB[activeTab.value] || {
        label: 'Campo especifico',
        placeholder: 'Digite para filtrar',
      },
  )
  const activeRecordsGeneralSearchPlaceholder = computed(
    () =>
      ERP_RECORDS_GENERAL_SEARCH_PLACEHOLDER_BY_TAB[activeTab.value] ||
      'Busca geral (campos do tipo selecionado)',
  )
  const typeStats = computed(() =>
    Array.isArray(status.value?.typeStats) ? (status.value.typeStats as ErpTypeStatus[]) : [],
  )
  const activeTypeStatus = computed(() => {
    const dataType = activeRecordsDataType.value
    return typeStats.value.find((item) => item?.dataType === dataType) || null
  })
  const activeRecordsTotal = computed(() =>
    erpStore.recordsDataType === activeRecordsDataType.value
      ? Number(erpStore.totalRecords || 0)
      : Number(activeTypeStatus.value?.totalRows || 0),
  )
  const activeRecordsLastRun = computed(() => activeTypeStatus.value?.lastRun || null)
  const activeRecordsLastImportedFile = computed(
    () => activeTypeStatus.value?.lastImportedFile || null,
  )
  const syncRuns = computed(() => {
    if (Array.isArray(erpStore.runs) && erpStore.runs.length) {
      return erpStore.runs as ErpRun[]
    }

    const runs: ErpRun[] = []
    const pushUnique = (run?: ErpRun | null) => {
      if (!run?.id || runs.some((item) => item.id === run.id)) {
        return
      }
      runs.push(run)
    }

    pushUnique((status.value?.lastRun || null) as ErpRun | null)
    for (const item of typeStats.value) {
      pushUnique(item?.lastRun || null)
    }

    return runs.sort((left, right) => {
      const leftTime = new Date(left.finishedAt || left.startedAt || 0).getTime()
      const rightTime = new Date(right.finishedAt || right.startedAt || 0).getTime()
      return rightTime - leftTime
    })
  })
  const selectedSyncRun = computed(
    () =>
      syncRuns.value.find((run) => run.id === selectedSyncRunId.value) || syncRuns.value[0] || null,
  )
  const activeBancoSection = computed(
    () => ERP_BANCO_SECTION_BY_TAB[activeBancoTab.value] || ERP_BANCO_SECTION_BY_TAB.geral,
  )
  // Centavos derivados do valor minimo digitado em reais (input numerico). 0 = sem filtro.
  const recordsMinValueCents = computed(() => {
    const value = Number(recordsMinValue.value)
    if (!Number.isFinite(value) || value <= 0) return 0
    return Math.round(value * 100)
  })
  // Deve espelhar EXATAMENTE o queryKey de fetchStats (erp.ts), inclusive ordem e os
  // filtros de loja/consultor — senao, com um filtro ativo, recordsStatsKey diverge e
  // orderStats vira null (cards zerados). normalizeText = trim.
  const currentRecordsStatsKey = computed(() =>
    [
      activeRecordsDataType.value,
      recordsSearchValue.value.trim(),
      recordsSpecificSearchValue.value.trim(),
      recordsDateFrom.value.trim(),
      recordsDateTo.value.trim(),
      recordsDateField.value,
      String(recordsMinValueCents.value),
      recordsStoreFilter.value.trim(),
      recordsEmployeeFilter.value.trim(),
    ].join('|'),
  )
  const orderStats = computed(() => {
    const stats = erpStore.recordsStats
    if (!stats || stats.dataType !== activeRecordsDataType.value) {
      return null
    }
    if (erpStore.recordsStatsKey !== currentRecordsStatsKey.value) {
      return null
    }
    return stats as ErpOrderStats
  })

  const { exportCurrentDataAsCSV, exportingCsv, exportProductsAsCSV } = useErpCsvExport({
    activeRecordsColumns,
    activeRecordsDataType,
    activeTab,
    erpStore,
    identifierPrefixValue,
    productsDateFrom,
    productsDateTo,
    productsSortBy,
    productsSortDir,
    recordsDateField,
    recordsDateFrom,
    recordsDateTo,
    recordsEmployeeFilter,
    recordsMinValueCents,
    recordsSearchValue,
    recordsSortBy,
    recordsSortDir,
    recordsSpecificSearchValue,
    recordsStoreFilter,
    searchValue,
    ui,
  })
  const { notifyAutomaticSyncRun } = useErpSyncNotifications({ erpStore, ui })

  async function loadStatus() {
    if (!isERPSystemAdmin.value) return
    const result = await erpStore.fetchStatus()
    if (!result.ok && result.message) {
      ui.error(result.message)
      return
    }
    notifyAutomaticSyncRun()
  }

  async function loadRuns() {
    if (!isERPSystemAdmin.value) return
    const result = await erpStore.fetchRuns({ page: 1, pageSize: 20 })
    if (!result.ok && result.message) {
      ui.error(result.message)
    }
  }

  async function loadOverview() {
    if (!isERPSystemAdmin.value) return
    const result = await erpStore.fetchOverview()
    if (!result.ok && result.message) {
      ui.error(result.message)
    }
  }

  async function loadProducts(payload: { page?: number; pageSize?: number } = {}) {
    if (import.meta.server || activeTab.value !== 'produtos') return
    const result = await erpStore.fetchProducts({
      identifierPrefix: identifierPrefixValue.value,
      search: searchValue.value,
      page: payload.page || erpStore.page || 1,
      pageSize: payload.pageSize || erpStore.pageSize || 50,
      sortBy: productsSortBy.value,
      sortDir: productsSortDir.value,
      dateFrom: productsDateFrom.value,
      dateTo: productsDateTo.value,
    })
    if (!result.ok && result.message) ui.error(result.message)
  }

  async function loadRecords(payload: { page?: number; pageSize?: number } = {}) {
    if (import.meta.server || !activeRecordsDataType.value) return
    const result = await erpStore.fetchRecords({
      dataType: activeRecordsDataType.value,
      search: recordsSearchValue.value,
      specificSearch: recordsSpecificSearchValue.value,
      page: payload.page || erpStore.recordsPage || 1,
      pageSize: payload.pageSize || erpStore.recordsPageSize || 50,
      dedup: true,
      sortBy: recordsSortBy.value,
      sortDir: recordsSortDir.value,
      dateFrom: recordsDateFrom.value,
      dateTo: recordsDateTo.value,
      dateField: recordsDateField.value,
      minValueCents: recordsMinValueCents.value,
      storeFilter: recordsStoreFilter.value,
      employeeFilter: recordsEmployeeFilter.value,
    })
    if (!result.ok && result.message) ui.error(result.message)
  }

  async function loadOrderStats() {
    if (
      import.meta.server ||
      !isERPSystemAdmin.value ||
      (activeTab.value !== 'pedidos' && activeTab.value !== 'cancelados')
    ) {
      return
    }
    const result = await erpStore.fetchStats({
      dataType: activeRecordsDataType.value,
      search: recordsSearchValue.value,
      specificSearch: recordsSpecificSearchValue.value,
      dateFrom: recordsDateFrom.value,
      dateTo: recordsDateTo.value,
      dateField: recordsDateField.value,
      minValueCents: recordsMinValueCents.value,
      storeFilter: recordsStoreFilter.value,
      employeeFilter: recordsEmployeeFilter.value,
    })
    if (!result.ok && result.message) ui.error(result.message)
  }

  async function loadRecordsFacets() {
    if (
      import.meta.server ||
      !isERPSystemAdmin.value ||
      (activeTab.value !== 'pedidos' && activeTab.value !== 'cancelados')
    ) {
      return
    }
    const result = await erpStore.fetchRecordsFacets({
      dataType: activeRecordsDataType.value,
      dateFrom: recordsDateFrom.value,
      dateTo: recordsDateTo.value,
      dateField: recordsDateField.value,
      // Cascata: as opcoes de consultor sao filtradas pela loja selecionada.
      storeFilter: recordsStoreFilter.value,
    })
    if (!result.ok) {
      if (result.message) ui.error(result.message)
      recordsStoreOptions.value = []
      recordsEmployeeOptions.value = []
      return
    }
    recordsStoreOptions.value = Array.isArray(result.data?.stores) ? result.data.stores : []
    recordsEmployeeOptions.value = Array.isArray(result.data?.employees)
      ? result.data.employees
      : []
  }

  function currentMonthRecordsRange() {
    const now = new Date()
    const start = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1))
    const end = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() + 1, 0))
    const toInput = (date: Date) =>
      [
        date.getUTCFullYear(),
        String(date.getUTCMonth() + 1).padStart(2, '0'),
        String(date.getUTCDate()).padStart(2, '0'),
      ].join('-')
    return { from: toInput(start), to: toInput(end) }
  }

  function handleProductsSortBy(col: string) {
    productsSortBy.value = col
    void loadProducts({ page: 1 })
  }

  function handleProductsSortDir(dir: string) {
    productsSortDir.value = dir
    void loadProducts({ page: 1 })
  }

  function handleRecordsSortBy(col: string) {
    recordsSortBy.value = col
    void loadRecords({ page: 1 })
  }

  function handleRecordsSortDir(dir: string) {
    recordsSortDir.value = dir
    void loadRecords({ page: 1 })
  }

  function clearProductsLoadTimer() {
    if (productsLoadTimer) clearTimeout(productsLoadTimer)
    productsLoadTimer = null
  }

  function clearRecordsLoadTimer() {
    if (recordsLoadTimer) clearTimeout(recordsLoadTimer)
    recordsLoadTimer = null
  }

  function scheduleProductsLoad() {
    if (import.meta.server || activeTab.value !== 'produtos') return
    clearProductsLoadTimer()
    productsLoadTimer = setTimeout(() => {
      productsLoadTimer = null
      void loadProducts({ page: 1 })
    }, 300)
  }

  function scheduleRecordsLoad() {
    if (import.meta.server || !activeRecordsDataType.value) return
    clearRecordsLoadTimer()
    recordsLoadTimer = setTimeout(() => {
      recordsLoadTimer = null
      void loadRecords({ page: 1 })
      void loadOrderStats()
    }, 300)
  }

  async function handlePageChange(nextPage: number) {
    await loadProducts({ page: nextPage, pageSize: erpStore.pageSize })
  }

  async function handlePageSizeChange(nextPageSize: number) {
    await loadProducts({ page: 1, pageSize: nextPageSize })
  }

  async function reloadWorkspace() {
    await loadStatus()
    if (activeTab.value === 'sincronizacao') {
      await loadOverview()
      await loadRuns()
      return
    }
    if (activeTab.value === 'produtos') {
      await loadProducts()
      return
    }
    if (activeTab.value === 'crm') {
      await crmRef.value?.loadCRM()
      return
    }
    if (activeTab.value !== 'banco') {
      await loadRecords()
      await loadOrderStats()
      await loadRecordsFacets()
    }
  }

  async function handleRecordsPageChange(nextPage: number) {
    await loadRecords({ page: nextPage, pageSize: erpStore.recordsPageSize })
  }

  async function handleRecordsPageSizeChange(nextPageSize: number) {
    await loadRecords({ page: 1, pageSize: nextPageSize })
  }

  const { handleBackfillSync, handleBootstrap, handleRecordsBootstrap, handleSyncNow } =
    useErpSyncActions({
      activeRecordsDataType,
      activeTab,
      erpStore,
      reloadWorkspace,
      ui,
    })

  watch(
    () => [auth.isAuthenticated, auth.activeTenantId, auth.activeStoreId],
    (currentScope, previousScope) => {
      const [isAuthenticated, tenantId, storeId] = currentScope
      const [previousAuthenticated, previousTenantId, previousStoreId] = previousScope ?? []
      if (!isAuthenticated) {
        erpStore.reset()
        return
      }
      if (
        isAuthenticated !== previousAuthenticated ||
        tenantId !== previousTenantId ||
        storeId !== previousStoreId
      ) {
        void reloadWorkspace()
      }
    },
    { immediate: true },
  )

  watch(activeTab, () => {
    if (!tabs.value.some((tab) => tab.id === activeTab.value)) {
      activeTab.value = tabs.value[0]?.id || 'produtos'
      return
    }
    if (activeTab.value === 'banco') {
      activeBancoTab.value = 'geral'
      return
    }
    if (activeTab.value === 'sincronizacao') {
      void loadOverview()
      void loadRuns()
      return
    }
    if (activeTab.value === 'produtos') {
      void loadProducts()
      return
    }
    if (activeTab.value === 'crm') {
      void crmRef.value?.loadCRM()
      void crmRef.value?.loadCustomers({ page: 1 })
      return
    }
    recordsSpecificSearchValue.value = ''
    recordsSortBy.value = 'source_batch_date'
    recordsSortDir.value = 'desc'
    recordsDateField.value = 'order_date'
    recordsMinValue.value = ''
    recordsStoreFilter.value = ''
    recordsEmployeeFilter.value = ''
    recordsStoreOptions.value = []
    recordsEmployeeOptions.value = []
    // Volume grande: abrir Compras/Cancelados ja no mes atual (range scan pelo
    // indice order_date) em vez de varrer todo o historico. Demais abas seguem
    // sem filtro de periodo.
    if (activeTab.value === 'pedidos' || activeTab.value === 'cancelados') {
      const range = currentMonthRecordsRange()
      recordsDateFrom.value = range.from
      recordsDateTo.value = range.to
    } else {
      recordsDateFrom.value = ''
      recordsDateTo.value = ''
    }
    void loadRecords({ page: 1 })
    void loadOrderStats()
    void loadRecordsFacets()
  })

  watch(searchValue, scheduleProductsLoad)
  watch(identifierPrefixValue, scheduleProductsLoad)
  watch(recordsSearchValue, scheduleRecordsLoad)
  watch(recordsSpecificSearchValue, scheduleRecordsLoad)
  watch(productsDateFrom, scheduleProductsLoad)
  watch(productsDateTo, scheduleProductsLoad)
  // O periodo (data de/ate e o campo de data) redefine as facetas de loja/consultor,
  // entao alem de reagendar a busca dos registros tambem recarrega as opcoes.
  watch(recordsDateFrom, () => {
    scheduleRecordsLoad()
    void loadRecordsFacets()
  })
  watch(recordsDateTo, () => {
    scheduleRecordsLoad()
    void loadRecordsFacets()
  })
  watch(recordsMinValue, scheduleRecordsLoad)
  // Loja em cascata: trocar a loja reseta o consultor (a lista muda) e recarrega as
  // facetas de consultor ja filtradas pela nova loja, alem de reagendar a busca.
  watch(recordsStoreFilter, () => {
    recordsEmployeeFilter.value = ''
    void loadRecordsFacets()
    scheduleRecordsLoad()
  })
  watch(recordsEmployeeFilter, scheduleRecordsLoad)
  watch(recordsDateField, () => {
    void loadRecords({ page: 1 })
    void loadOrderStats()
    void loadRecordsFacets()
  })
  watch(
    tabs,
    (availableTabs) => {
      if (!availableTabs.some((tab) => tab.id === activeTab.value)) {
        activeTab.value = availableTabs[0]?.id || 'produtos'
      }
    },
    { immediate: true },
  )
  watch(
    syncRuns,
    (runs) => {
      if (!runs.length) {
        selectedSyncRunId.value = ''
        return
      }
      if (!runs.some((run) => run.id === selectedSyncRunId.value)) {
        selectedSyncRunId.value = runs[0].id
      }
    },
    { immediate: true },
  )

  onBeforeUnmount(() => {
    clearProductsLoadTimer()
    clearRecordsLoadTimer()
  })

  return {
    activeBancoSection,
    activeBancoTab,
    activeRecordsBootstrapLabel,
    activeRecordsColumns,
    activeRecordsGeneralSearchPlaceholder,
    activeRecordsLastImportedFile,
    activeRecordsLastRun,
    activeRecordsSpecificSearch,
    activeRecordsTotal,
    activeTab,
    bancoTabs: ERP_BANCO_TABS,
    canSync,
    castRow,
    columns: ERP_PRODUCT_COLUMNS,
    crmRef,
    currentProductCount,
    erpScopeHint,
    erpScopeLabel,
    erpStore,
    exportCurrentDataAsCSV,
    exportingCsv,
    exportProductsAsCSV,
    formatCurrency,
    formatDateTime,
    formatNumber,
    formatPrice,
    formatSourceFileName,
    handleBackfillSync,
    handleBootstrap,
    handlePageChange,
    handlePageSizeChange,
    handleProductsSortBy,
    handleProductsSortDir,
    handleRecordsBootstrap,
    handleRecordsPageChange,
    handleRecordsPageSizeChange,
    handleRecordsSortBy,
    handleRecordsSortDir,
    handleSyncNow,
    identifierPrefixValue,
    isERPSystemAdmin,
    lastImportedFile,
    lastRun,
    orderStats,
    overview,
    pageSizeOptions: ERP_PAGE_SIZE_OPTIONS,
    productRowKey,
    productsDateFrom,
    productsDateTo,
    productsSortBy,
    productsSortDir,
    rawItemRows,
    recordsDateField,
    recordsDateFrom,
    recordsDateTo,
    recordsEmployeeFilter,
    recordsEmployeeOptions,
    recordsMinValue,
    recordsRowKey,
    recordsSearchValue,
    recordsSortBy,
    recordsSortDir,
    recordsSpecificSearchValue,
    recordsStoreFilter,
    recordsStoreOptions,
    reloadWorkspace,
    searchValue,
    selectedSyncRun,
    selectedSyncRunId,
    syncRuns,
    tabs,
  }
}
