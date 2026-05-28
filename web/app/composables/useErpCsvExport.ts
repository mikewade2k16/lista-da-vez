import { ref, type ComputedRef, type Ref } from 'vue'

import {
  ERP_PRODUCT_COLUMNS,
  type ErpGridColumn,
  type ErpRecord,
  type ExportScope,
} from '~/domain/utils/erp-display'

type SnapshotResult = {
  ok: boolean
  message?: string
  data?: {
    items?: unknown[]
    total?: number | string | null
  } | null
}

type ErpExportStore = {
  products: ErpRecord[]
  records: ErpRecord[]
  fetchProductsSnapshot: (payload: Record<string, unknown>) => Promise<SnapshotResult>
  fetchRecordsSnapshot: (payload: Record<string, unknown>) => Promise<SnapshotResult>
}

type UiNotifier = {
  error: (message: string) => void
  success: (message: string) => void
}

type UseErpCsvExportOptions = {
  activeRecordsColumns: ComputedRef<ErpGridColumn[]>
  activeRecordsDataType: ComputedRef<string>
  activeTab: Ref<string>
  erpStore: ErpExportStore
  identifierPrefixValue: Ref<string>
  productsDateFrom: Ref<string>
  productsDateTo: Ref<string>
  productsSortBy: Ref<string>
  productsSortDir: Ref<string>
  recordsDateFrom: Ref<string>
  recordsDateTo: Ref<string>
  recordsSearchValue: Ref<string>
  recordsSortBy: Ref<string>
  recordsSortDir: Ref<string>
  recordsSpecificSearchValue: Ref<string>
  searchValue: Ref<string>
  ui: UiNotifier
}

function escapeCSVValue(v: unknown) {
  const s = String(v ?? '').replace(/"/g, '""')
  return /[",\n\r]/.test(s) ? `"${s}"` : s
}

function downloadRowsAsCSV(rows: ErpRecord[], tableColumns: ErpGridColumn[], filename: string) {
  const headers = tableColumns.map((c) => c.label)
  const colIds = tableColumns.map((c) => c.id)
  const lines = [
    headers.map(escapeCSVValue).join(','),
    ...rows.map((row) => colIds.map((id) => escapeCSVValue(row[id])).join(',')),
  ]
  const blob = new Blob(['\uFEFF' + lines.join('\n')], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}

function exportScopeSuffix(scope: ExportScope) {
  if (scope === 'all') return 'tudo'
  if (scope === 'filtered') return 'filtrado'
  return 'pagina'
}

function cloneRows(rows: ErpRecord[]) {
  return rows.map((row) => ({ ...row }))
}

export function useErpCsvExport(options: UseErpCsvExportOptions) {
  const exportingCsv = ref(false)

  async function collectProductsForExport(scope: ExportScope) {
    if (scope === 'page') {
      return cloneRows(options.erpStore.products)
    }

    const includeFilters = scope === 'filtered'
    const rows: ErpRecord[] = []
    let nextPage = 1
    let total = Number.POSITIVE_INFINITY

    while (rows.length < total) {
      const result = await options.erpStore.fetchProductsSnapshot({
        dateFrom: includeFilters ? options.productsDateFrom.value : '',
        dateTo: includeFilters ? options.productsDateTo.value : '',
        identifierPrefix: includeFilters ? options.identifierPrefixValue.value : '',
        page: nextPage,
        pageSize: 5000,
        search: includeFilters ? options.searchValue.value : '',
        sortBy: options.productsSortBy.value,
        sortDir: options.productsSortDir.value,
      })
      if (!result.ok || !result.data) {
        throw new Error(result.message || 'Erro ao exportar produtos.')
      }

      const items = Array.isArray(result.data.items) ? (result.data.items as ErpRecord[]) : []
      total = Number(result.data.total || 0)
      rows.push(...items)
      if (!items.length) break
      nextPage += 1
    }

    return rows
  }

  async function collectRecordsForExport(scope: ExportScope) {
    if (scope === 'page') {
      return cloneRows(options.erpStore.records)
    }

    const includeFilters = scope === 'filtered'
    const rows: ErpRecord[] = []
    let nextPage = 1
    let total = Number.POSITIVE_INFINITY

    while (rows.length < total) {
      const result = await options.erpStore.fetchRecordsSnapshot({
        dataType: options.activeRecordsDataType.value,
        dateFrom: includeFilters ? options.recordsDateFrom.value : '',
        dateTo: includeFilters ? options.recordsDateTo.value : '',
        dedup: true,
        page: nextPage,
        pageSize: 5000,
        search: includeFilters ? options.recordsSearchValue.value : '',
        sortBy: options.recordsSortBy.value,
        sortDir: options.recordsSortDir.value,
        specificSearch: includeFilters ? options.recordsSpecificSearchValue.value : '',
      })
      if (!result.ok || !result.data) {
        throw new Error(result.message || 'Erro ao exportar registros.')
      }

      const items = Array.isArray(result.data.items) ? (result.data.items as ErpRecord[]) : []
      total = Number(result.data.total || 0)
      rows.push(...items)
      if (!items.length) break
      nextPage += 1
    }

    return rows
  }

  async function exportCurrentDataAsCSV(scope: ExportScope = 'filtered') {
    const rawCols = options.activeRecordsColumns.value
    if (!rawCols.length || !options.activeRecordsDataType.value) return
    exportingCsv.value = true

    try {
      const rows =
        scope === 'page'
          ? cloneRows(options.erpStore.records)
          : await collectRecordsForExport(scope)
      if (!rows.length) {
        options.ui.error('Nenhum registro encontrado para exportar.')
        return
      }
      const suffix = exportScopeSuffix(scope)
      downloadRowsAsCSV(
        rows,
        rawCols,
        `erp-${options.activeTab.value}-${suffix}-${new Date().toISOString().slice(0, 10)}.csv`,
      )
      options.ui.success(`${rows.length.toLocaleString('pt-BR')} registros exportados.`)
    } catch (err) {
      options.ui.error(err instanceof Error ? err.message : 'Erro ao exportar os registros ERP.')
    } finally {
      exportingCsv.value = false
    }
  }

  async function exportProductsAsCSV(scope: ExportScope = 'filtered') {
    exportingCsv.value = true

    try {
      const rows =
        scope === 'page'
          ? cloneRows(options.erpStore.products)
          : await collectProductsForExport(scope)
      if (!rows.length) {
        options.ui.error('Nenhum produto encontrado para exportar.')
        return
      }
      const suffix = exportScopeSuffix(scope)
      downloadRowsAsCSV(
        rows,
        ERP_PRODUCT_COLUMNS,
        `erp-produtos-${suffix}-${new Date().toISOString().slice(0, 10)}.csv`,
      )
      options.ui.success(`${rows.length.toLocaleString('pt-BR')} produtos exportados.`)
    } catch (err) {
      options.ui.error(err instanceof Error ? err.message : 'Erro ao exportar os produtos ERP.')
    } finally {
      exportingCsv.value = false
    }
  }

  return {
    exportCurrentDataAsCSV,
    exportingCsv,
    exportProductsAsCSV,
  }
}
