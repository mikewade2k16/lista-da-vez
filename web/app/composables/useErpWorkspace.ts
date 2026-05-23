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
import { hasPermission } from '~/domain/utils/permissions'
import { useAuthStore } from '~/stores/auth'
import { useErpStore } from '~/stores/erp'
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

  const activeTab = ref('sincronizacao')
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
  const recordsDateFrom = ref('')
  const recordsDateTo = ref('')
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
  const canSync = computed(() => {
    if (auth.permissionsResolved) {
      return hasPermission(auth.permissionKeys, 'workspace.erp.edit')
    }
    return auth.role === 'platform_admin' || auth.role === 'owner'
  })
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
  const currentRecordsStatsKey = computed(() =>
    [
      activeRecordsDataType.value,
      recordsSearchValue.value.trim(),
      recordsSpecificSearchValue.value.trim(),
      recordsDateFrom.value.trim(),
      recordsDateTo.value.trim(),
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
    recordsDateFrom,
    recordsDateTo,
    recordsSearchValue,
    recordsSortBy,
    recordsSortDir,
    recordsSpecificSearchValue,
    searchValue,
    ui,
  })
  const { notifyAutomaticSyncRun } = useErpSyncNotifications({ erpStore, ui })

  async function loadStatus() {
    const result = await erpStore.fetchStatus()
    if (!result.ok && result.message) {
      ui.error(result.message)
      return
    }
    notifyAutomaticSyncRun()
  }

  async function loadRuns() {
    const result = await erpStore.fetchRuns({ page: 1, pageSize: 20 })
    if (!result.ok && result.message) {
      ui.error(result.message)
    }
  }

  async function loadOverview() {
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
    })
    if (!result.ok && result.message) ui.error(result.message)
  }

  async function loadOrderStats() {
    if (import.meta.server || (activeTab.value !== 'pedidos' && activeTab.value !== 'cancelados')) {
      return
    }
    const result = await erpStore.fetchStats({
      dataType: activeRecordsDataType.value,
      search: recordsSearchValue.value,
      specificSearch: recordsSpecificSearchValue.value,
      dateFrom: recordsDateFrom.value,
      dateTo: recordsDateTo.value,
    })
    if (!result.ok && result.message) ui.error(result.message)
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
      await crmRef.value?.loadCustomers({ page: 1 })
      return
    }
    if (activeTab.value !== 'banco') {
      await loadRecords()
      await loadOrderStats()
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
    recordsDateFrom.value = ''
    recordsDateTo.value = ''
    void loadRecords({ page: 1 })
    void loadOrderStats()
  })

  watch(searchValue, scheduleProductsLoad)
  watch(identifierPrefixValue, scheduleProductsLoad)
  watch(recordsSearchValue, scheduleRecordsLoad)
  watch(recordsSpecificSearchValue, scheduleRecordsLoad)
  watch(productsDateFrom, scheduleProductsLoad)
  watch(productsDateTo, scheduleProductsLoad)
  watch(recordsDateFrom, scheduleRecordsLoad)
  watch(recordsDateTo, scheduleRecordsLoad)
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
    recordsDateFrom,
    recordsDateTo,
    recordsRowKey,
    recordsSearchValue,
    recordsSortBy,
    recordsSortDir,
    recordsSpecificSearchValue,
    reloadWorkspace,
    searchValue,
    selectedSyncRun,
    selectedSyncRunId,
    syncRuns,
    tabs: ERP_TABS,
  }
}
