import { ref, type ComputedRef, type Ref } from 'vue'

import {
  ERP_PRODUCT_COLUMNS,
  type ErpGridColumn,
  type ErpRecord,
  type ExportFormat,
  type ExportRequest,
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
  recordsDateField: Ref<string>
  recordsDateFrom: Ref<string>
  recordsDateTo: Ref<string>
  recordsEmployeeFilter: Ref<string>
  recordsMinValueCents: ComputedRef<number>
  recordsSearchValue: Ref<string>
  recordsSortBy: Ref<string>
  recordsSortDir: Ref<string>
  recordsSpecificSearchValue: Ref<string>
  recordsStoreFilter: Ref<string>
  searchValue: Ref<string>
  ui: UiNotifier
}

// Colunas de valor (centavos no raw) exportadas formatadas em R$ para leitura.
const MONEY_COLUMN_IDS = new Set([
  'total_amount_raw',
  'amount_raw',
  'product_return_raw',
  'total_exclusion_raw',
  'total_debit_raw',
])

function formatExportValue(columnId: string, value: unknown) {
  if (MONEY_COLUMN_IDS.has(columnId)) {
    const cents = Number(value)
    if (Number.isFinite(cents)) {
      return new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' }).format(
        cents / 100,
      )
    }
  }
  return String(value ?? '')
}

function escapeCSVValue(value: unknown) {
  const text = String(value ?? '').replace(/"/g, '""')
  return /[",\n\r]/.test(text) ? `"${text}"` : text
}

function buildCSV(rows: ErpRecord[], columns: ErpGridColumn[]) {
  const lines = [
    columns.map((column) => escapeCSVValue(column.label)).join(','),
    ...rows.map((row) =>
      columns
        .map((column) => escapeCSVValue(formatExportValue(column.id, row[column.id])))
        .join(','),
    ),
  ]
  return lines.join('\n')
}

function buildJSON(rows: ErpRecord[], columns: ErpGridColumn[]) {
  const data = rows.map((row) => {
    const entry: Record<string, string> = {}
    for (const column of columns) {
      entry[column.label] = formatExportValue(column.id, row[column.id])
    }
    return entry
  })
  return JSON.stringify(data, null, 2)
}

function escapeMarkdownCell(value: string) {
  return value.replace(/\|/g, '\\|').replace(/\r?\n/g, ' ')
}

function buildMarkdown(rows: ErpRecord[], columns: ErpGridColumn[]) {
  const header = `| ${columns.map((column) => escapeMarkdownCell(column.label)).join(' | ')} |`
  const divider = `| ${columns.map(() => '---').join(' | ')} |`
  const body = rows.map(
    (row) =>
      `| ${columns
        .map((column) => escapeMarkdownCell(formatExportValue(column.id, row[column.id])))
        .join(' | ')} |`,
  )
  return [header, divider, ...body].join('\n')
}

function triggerDownload(content: string, mime: string, filename: string) {
  const blob = new Blob([content], { type: mime })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}

function downloadRows(
  rows: ErpRecord[],
  columns: ErpGridColumn[],
  format: ExportFormat,
  baseName: string,
) {
  if (format === 'json') {
    triggerDownload(buildJSON(rows, columns), 'application/json;charset=utf-8;', `${baseName}.json`)
    return
  }
  if (format === 'md') {
    triggerDownload(buildMarkdown(rows, columns), 'text/markdown;charset=utf-8;', `${baseName}.md`)
    return
  }
  // CSV com BOM (U+FEFF) para abrir corretamente no Excel.
  triggerDownload('﻿' + buildCSV(rows, columns), 'text/csv;charset=utf-8;', `${baseName}.csv`)
}

// Colunas a exportar = as VISIVEIS na tela (ids vindos do grid), preservando a
// ordem de exibicao. Fallback: as visiveis por padrao.
function resolveExportColumns(allColumns: ErpGridColumn[], visibleIds: string[]) {
  if (Array.isArray(visibleIds) && visibleIds.length) {
    const visible = new Set(visibleIds)
    const filtered = allColumns.filter((column) => visible.has(column.id))
    if (filtered.length) {
      return filtered
    }
  }
  return allColumns.filter((column) => column.defaultVisible !== false)
}

function cloneRows(rows: ErpRecord[]) {
  return rows.map((row) => ({ ...row }))
}

function exportFileBase(tab: string) {
  return `erp-${tab}-${new Date().toISOString().slice(0, 10)}`
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
        dateField: options.recordsDateField.value,
        dateFrom: includeFilters ? options.recordsDateFrom.value : '',
        dateTo: includeFilters ? options.recordsDateTo.value : '',
        dedup: true,
        employeeFilter: includeFilters ? options.recordsEmployeeFilter.value : '',
        minValueCents: includeFilters ? options.recordsMinValueCents.value : 0,
        page: nextPage,
        pageSize: 5000,
        search: includeFilters ? options.recordsSearchValue.value : '',
        sortBy: options.recordsSortBy.value,
        sortDir: options.recordsSortDir.value,
        specificSearch: includeFilters ? options.recordsSpecificSearchValue.value : '',
        storeFilter: includeFilters ? options.recordsStoreFilter.value : '',
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

  async function exportCurrentDataAsCSV(request: ExportRequest) {
    const allColumns = options.activeRecordsColumns.value
    if (!allColumns.length || !options.activeRecordsDataType.value) return
    const columns = resolveExportColumns(allColumns, request.columns)
    exportingCsv.value = true

    try {
      const rows =
        request.scope === 'page'
          ? cloneRows(options.erpStore.records)
          : await collectRecordsForExport(request.scope)
      if (!rows.length) {
        options.ui.error('Nenhum registro encontrado para exportar.')
        return
      }
      downloadRows(rows, columns, request.format, exportFileBase(options.activeTab.value))
      options.ui.success(`${rows.length.toLocaleString('pt-BR')} registros exportados.`)
    } catch (err) {
      options.ui.error(err instanceof Error ? err.message : 'Erro ao exportar os registros ERP.')
    } finally {
      exportingCsv.value = false
    }
  }

  async function exportProductsAsCSV(request: ExportRequest) {
    const columns = resolveExportColumns(ERP_PRODUCT_COLUMNS, request.columns)
    exportingCsv.value = true

    try {
      const rows =
        request.scope === 'page'
          ? cloneRows(options.erpStore.products)
          : await collectProductsForExport(request.scope)
      if (!rows.length) {
        options.ui.error('Nenhum produto encontrado para exportar.')
        return
      }
      downloadRows(rows, columns, request.format, exportFileBase('produtos'))
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
