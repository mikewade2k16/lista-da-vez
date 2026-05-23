<script setup lang="ts">
import ErpDataTable from '~/components/erp/ErpDataTable.vue'
import type {
  ErpGridColumn,
  ErpImportedFile,
  ErpOrderStats,
  ErpRecord,
  ErpRun,
  ErpSpecificSearch,
  ExportScope,
} from '~/domain/utils/erp-display'

defineProps<{
  activeRecordsBootstrapLabel: string
  activeRecordsColumns: ErpGridColumn[]
  activeRecordsGeneralSearchPlaceholder: string
  activeRecordsLastImportedFile: ErpImportedFile | null
  activeRecordsLastRun: ErpRun | null
  activeRecordsSpecificSearch: ErpSpecificSearch
  activeRecordsTotal: number
  activeTab: string
  canSync: boolean
  exporting: boolean
  formatCurrency: (cents: number | null | undefined) => string
  formatDateTime: (value?: string | null) => string
  formatNumber: (value: number | null | undefined) => string
  loading: boolean
  orderStats: ErpOrderStats | null
  page: number
  pageSize: number
  pageSizeOptions: number[]
  recordsDateFrom: string
  recordsDateTo: string
  recordsRowKey: (row: ErpRecord, index: number) => string
  recordsSearchValue: string
  recordsSortBy: string
  recordsSortDir: string
  recordsSpecificSearchValue: string
  rows: ErpRecord[]
  syncing: boolean
  total: number
}>()

const emit = defineEmits<{
  (e: 'bootstrap' | 'refresh'): void
  (e: 'export', scope: ExportScope): void
  (
    e:
      | 'update:dateFrom'
      | 'update:dateTo'
      | 'update:recordsSearchValue'
      | 'update:recordsSpecificSearchValue'
      | 'update:sortBy'
      | 'update:sortDir',
    value: string,
  ): void
  (e: 'update:page' | 'update:pageSize', value: number): void
}>()
</script>

<template>
  <div class="erp-panel__tab-body">
    <div
      v-if="activeTab === 'pedidos' || activeTab === 'cancelados'"
      class="erp-panel__stats erp-panel__stats--orders"
    >
      <article class="erp-panel__stat-card erp-panel__stat-card--accent">
        <span class="erp-panel__stat-label">Compras</span>
        <strong class="erp-panel__stat-value">
          {{ formatNumber(orderStats?.orderCount) }}
        </strong>
        <small>
          {{ recordsDateFrom || recordsDateTo ? 'compras unicas no periodo' : 'compras unicas' }}
        </small>
      </article>
      <article class="erp-panel__stat-card erp-panel__stat-card--green">
        <span class="erp-panel__stat-label">Faturamento</span>
        <strong class="erp-panel__stat-value erp-panel__stat-value--currency">
          {{ formatCurrency(orderStats?.totalAmountCents) }}
        </strong>
        <small>valor total das compras</small>
      </article>
      <article class="erp-panel__stat-card">
        <span class="erp-panel__stat-label">Ticket Medio</span>
        <strong class="erp-panel__stat-value erp-panel__stat-value--currency">
          {{ formatCurrency(orderStats?.avgAmountCents) }}
        </strong>
        <small>por compra</small>
      </article>
      <article class="erp-panel__stat-card">
        <span class="erp-panel__stat-label">PA</span>
        <strong class="erp-panel__stat-value">
          {{ orderStats?.pa != null ? orderStats.pa.toFixed(2) : '-' }}
        </strong>
        <small>pecas por compra</small>
      </article>
    </div>

    <div v-else class="erp-panel__stats">
      <article class="erp-panel__stat-card">
        <span class="erp-panel__stat-label">Registros atuais</span>
        <strong class="erp-panel__stat-value">{{ formatNumber(activeRecordsTotal) }}</strong>
        <small>linhas cadastradas nesta aba</small>
      </article>
      <article class="erp-panel__stat-card">
        <span class="erp-panel__stat-label">Ultimo lote importado</span>
        <strong class="erp-panel__stat-value erp-panel__stat-value--small">
          {{ formatNumber(activeRecordsLastImportedFile?.recordCount) }} registros
        </strong>
        <small>registrado {{ formatDateTime(activeRecordsLastImportedFile?.importedAt) }}</small>
      </article>
      <article class="erp-panel__stat-card">
        <span class="erp-panel__stat-label">Ultimo run</span>
        <strong class="erp-panel__stat-value erp-panel__stat-value--small">
          {{ activeRecordsLastRun?.status || 'sem execucao' }}
        </strong>
        <small>
          {{ formatDateTime(activeRecordsLastRun?.finishedAt || activeRecordsLastRun?.startedAt) }}
        </small>
      </article>
      <article class="erp-panel__stat-card">
        <span class="erp-panel__stat-label">Linhas importadas</span>
        <strong class="erp-panel__stat-value">
          {{ formatNumber(activeRecordsLastRun?.rowsImported) }}
        </strong>
        <small>{{ formatNumber(activeRecordsLastRun?.filesImported) }} lotes processados</small>
      </article>
    </div>

    <ErpDataTable
      :columns="activeRecordsColumns"
      :rows="rows"
      :row-key="recordsRowKey"
      :total="total"
      :page="page"
      :page-size="pageSize"
      :page-size-options="pageSizeOptions"
      :search-value="recordsSearchValue"
      :search-placeholder="activeRecordsGeneralSearchPlaceholder"
      :specific-search-value="recordsSpecificSearchValue"
      :specific-search-label="activeRecordsSpecificSearch.label"
      :specific-search-placeholder="activeRecordsSpecificSearch.placeholder"
      :date-from="recordsDateFrom"
      :date-to="recordsDateTo"
      :sort-by="recordsSortBy"
      :sort-dir="recordsSortDir"
      :loading="loading"
      :exporting="exporting"
      :show-bootstrap-action="canSync"
      :show-export-action="true"
      :can-bootstrap="canSync"
      :syncing="syncing"
      :bootstrap-label="activeRecordsBootstrapLabel"
      bootstrap-busy-label="Sincronizando..."
      empty-title="Nenhum registro encontrado"
      empty-text="Nao ha linhas importadas para este tipo no escopo consolidado do ERP. Use a sincronizacao da aba para carregar os dados."
      :storage-key="`erp-${activeTab}-grid-columns-v4`"
      :testid="`erp-${activeTab}-grid`"
      @update:search-value="emit('update:recordsSearchValue', $event)"
      @update:specific-search-value="emit('update:recordsSpecificSearchValue', $event)"
      @update:date-from="emit('update:dateFrom', $event)"
      @update:date-to="emit('update:dateTo', $event)"
      @update:sort-by="emit('update:sortBy', $event)"
      @update:sort-dir="emit('update:sortDir', $event)"
      @update:page="emit('update:page', $event)"
      @update:page-size="emit('update:pageSize', $event)"
      @refresh="emit('refresh')"
      @bootstrap="emit('bootstrap')"
      @export="emit('export', $event)"
    />
  </div>
</template>
