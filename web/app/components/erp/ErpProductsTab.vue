<script setup lang="ts">
import ErpDataTable from '~/components/erp/ErpDataTable.vue'
import type {
  ErpGridColumn,
  ErpImportedFile,
  ErpRecord,
  ErpRun,
  ExportRequest,
} from '~/domain/utils/erp-display'

defineProps<{
  canSync: boolean
  castRow: (value: unknown) => Record<string, unknown>
  columns: ErpGridColumn[]
  currentProductCount: number
  exporting: boolean
  formatDateTime: (value?: string | null) => string
  formatNumber: (value: number | null | undefined) => string
  formatPrice: (rawValue?: string, cents?: number | null) => string
  formatSourceFileName: (sourceName?: string | null) => string
  identifierPrefixValue: string
  lastImportedFile: ErpImportedFile | null
  lastRun: ErpRun | null
  loading: boolean
  page: number
  pageSize: number
  pageSizeOptions: number[]
  productRowKey: (row: ErpRecord, index: number) => string
  productsDateFrom: string
  productsDateTo: string
  productsSortBy: string
  productsSortDir: string
  rawItemRows: number
  rows: ErpRecord[]
  searchValue: string
  showAdminCards: boolean
  syncing: boolean
  total: number
}>()

const emit = defineEmits<{
  (e: 'bootstrap' | 'refresh'): void
  (e: 'export', request: ExportRequest): void
  (
    e:
      | 'update:dateFrom'
      | 'update:dateTo'
      | 'update:identifierPrefixValue'
      | 'update:searchValue'
      | 'update:sortBy'
      | 'update:sortDir',
    value: string,
  ): void
  (e: 'update:page' | 'update:pageSize', value: number): void
}>()
</script>

<template>
  <div class="erp-panel__tab-body">
    <div v-if="showAdminCards" class="erp-panel__stats">
      <article class="erp-panel__stat-card">
        <span class="erp-panel__stat-label">Produtos atuais</span>
        <strong class="erp-panel__stat-value">{{ formatNumber(currentProductCount) }}</strong>
        <small>
          projecao em
          <code>erp_item_current</code>
        </small>
      </article>
      <article class="erp-panel__stat-card">
        <span class="erp-panel__stat-label">Linhas raw</span>
        <strong class="erp-panel__stat-value">{{ formatNumber(rawItemRows) }}</strong>
        <small>historico bruto importado</small>
      </article>
      <article class="erp-panel__stat-card">
        <span class="erp-panel__stat-label">Ultimo lote importado</span>
        <strong class="erp-panel__stat-value erp-panel__stat-value--small">
          {{ formatSourceFileName(lastImportedFile?.sourceName) }}
        </strong>
        <small>registrado {{ formatDateTime(lastImportedFile?.importedAt) }}</small>
      </article>
      <article class="erp-panel__stat-card">
        <span class="erp-panel__stat-label">Ultimo run</span>
        <strong class="erp-panel__stat-value erp-panel__stat-value--small">
          {{ lastRun?.status || 'sem execucao' }}
        </strong>
        <small>{{ formatDateTime(lastRun?.finishedAt || lastRun?.startedAt) }}</small>
      </article>
    </div>

    <div class="erp-panel__run-meta">
      <div class="erp-panel__run-box">
        <span class="erp-panel__run-label">Arquivos processados</span>
        <strong>{{ formatNumber(lastRun?.filesImported) }}</strong>
        <small>{{ formatNumber(lastRun?.filesSkipped) }} ignorados por checksum</small>
      </div>
      <div class="erp-panel__run-box">
        <span class="erp-panel__run-label">Linhas importadas</span>
        <strong>{{ formatNumber(lastRun?.rowsImported) }}</strong>
        <small>{{ formatNumber(lastRun?.rowsRead) }} lidas do consolidado</small>
      </div>
      <div class="erp-panel__run-box">
        <span class="erp-panel__run-label">Concluido em</span>
        <strong class="erp-panel__run-date">{{ formatDateTime(lastRun?.finishedAt) }}</strong>
        <small>iniciado {{ formatDateTime(lastRun?.startedAt) }}</small>
      </div>
    </div>

    <ErpDataTable
      :columns="columns"
      :rows="rows"
      :row-key="productRowKey"
      :total="total"
      :page="page"
      :page-size="pageSize"
      :page-size-options="pageSizeOptions"
      :search-value="searchValue"
      search-placeholder="Busca geral (nome, descricao, SKU, identificador, valor, categoria...)"
      :specific-search-value="identifierPrefixValue"
      specific-search-label="Identificador (comeca com)"
      specific-search-placeholder="Ex: 153"
      :date-from="productsDateFrom"
      :date-to="productsDateTo"
      :sort-by="productsSortBy"
      :sort-dir="productsSortDir"
      :loading="loading"
      :exporting="exporting"
      :show-refresh-action="true"
      :show-export-action="true"
      :show-bootstrap-action="canSync"
      :can-bootstrap="canSync"
      :syncing="syncing"
      bootstrap-label="Bootstrap produtos ERP"
      bootstrap-busy-label="Sincronizando..."
      empty-title="Nenhum produto no ERP"
      empty-text="Dispare a sincronizacao ERP ou ajuste a busca para preencher a grade administrativa."
      storage-key="erp-products-grid-columns-v2"
      testid="erp-products-grid"
      @update:search-value="emit('update:searchValue', $event)"
      @update:specific-search-value="emit('update:identifierPrefixValue', $event)"
      @update:date-from="emit('update:dateFrom', $event)"
      @update:date-to="emit('update:dateTo', $event)"
      @update:sort-by="emit('update:sortBy', $event)"
      @update:sort-dir="emit('update:sortDir', $event)"
      @update:page="emit('update:page', $event)"
      @update:page-size="emit('update:pageSize', $event)"
      @refresh="emit('refresh')"
      @bootstrap="emit('bootstrap')"
      @export="emit('export', $event)"
    >
      <template #cell-name="{ row }">
        <div class="erp-panel__name-cell">
          <strong>{{ castRow(row).name }}</strong>
          <span>
            {{ castRow(row).description || castRow(row).brandName || 'Sem descricao complementar' }}
          </span>
        </div>
      </template>

      <template #cell-priceRaw="{ row }">
        <span class="erp-panel__price-cell">
          {{ formatPrice(castRow(row).priceRaw as string, castRow(row).priceCents as number) }}
        </span>
      </template>

      <template #cell-sourceUpdatedAt="{ row }">
        <span class="erp-panel__muted-cell">
          {{
            formatDateTime((castRow(row).sourceUpdatedAt || castRow(row).sourceCreatedAt) as string)
          }}
        </span>
      </template>
    </ErpDataTable>
  </div>
</template>
