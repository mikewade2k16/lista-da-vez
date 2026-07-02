<script setup lang="ts">
import ErpDataTable from '~/components/erp/ErpDataTable.vue'
import type {
  ErpGridColumn,
  ErpImportedFile,
  ErpOrderStats,
  ErpRecord,
  ErpRun,
  ErpSpecificSearch,
  ExportRequest,
} from '~/domain/utils/erp-display'
import type { ErpRecordsFacetOption } from '~/stores/erp'

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
  recordsDateField: string
  recordsDateFrom: string
  recordsDateTo: string
  recordsEmployeeFilter: string
  recordsEmployeeOptions: ErpRecordsFacetOption[]
  recordsMinValue: string
  recordsRowKey: (row: ErpRecord, index: number) => string
  recordsSearchValue: string
  recordsSortBy: string
  recordsSortDir: string
  recordsSpecificSearchValue: string
  recordsStoreFilter: string
  recordsStoreOptions: ErpRecordsFacetOption[]
  rows: ErpRecord[]
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
      | 'update:recordsDateField'
      | 'update:recordsEmployeeFilter'
      | 'update:recordsMinValue'
      | 'update:recordsSearchValue'
      | 'update:recordsSpecificSearchValue'
      | 'update:recordsStoreFilter'
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
      v-if="showAdminCards && (activeTab === 'pedidos' || activeTab === 'cancelados')"
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

    <div v-else-if="showAdminCards" class="erp-panel__stats">
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
      :storage-key="`erp-${activeTab}-grid-columns-v8`"
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
    >
      <template v-if="activeTab === 'pedidos' || activeTab === 'cancelados'" #toolbar-filters>
        <input
          class="erp-records-filter"
          type="number"
          min="0"
          step="0.01"
          inputmode="decimal"
          placeholder="Valor min. (R$)"
          :value="recordsMinValue"
          @input="emit('update:recordsMinValue', ($event.target as HTMLInputElement).value)"
        />
        <select
          class="erp-records-filter erp-records-filter--select"
          :value="recordsStoreFilter"
          @change="emit('update:recordsStoreFilter', ($event.target as HTMLSelectElement).value)"
        >
          <option value="">Todas as lojas</option>
          <option v-for="opt in recordsStoreOptions" :key="opt.value" :value="opt.value">
            {{ opt.label }}
          </option>
        </select>
        <select
          class="erp-records-filter erp-records-filter--select"
          :value="recordsEmployeeFilter"
          @change="emit('update:recordsEmployeeFilter', ($event.target as HTMLSelectElement).value)"
        >
          <option value="">Todos os consultores</option>
          <option v-for="opt in recordsEmployeeOptions" :key="opt.value" :value="opt.value">
            {{ opt.label }}
          </option>
        </select>
      </template>

      <template v-if="activeTab === 'pedidos' || activeTab === 'cancelados'" #pagination-extra>
        <label
          class="erp-records-datefield"
          title="Base do periodo: ligado usa a data real da compra; desligado, a data de importacao do lote."
        >
          <USwitch
            :model-value="recordsDateField === 'order_date'"
            size="sm"
            @update:model-value="
              emit('update:recordsDateField', $event ? 'order_date' : 'batch_date')
            "
          />
          <span>
            {{ recordsDateField === 'order_date' ? 'Data da compra' : 'Data de importação' }}
          </span>
        </label>
      </template>
    </ErpDataTable>
  </div>
</template>

<style scoped>
/* Faz o grid PREENCHER a largura. O ErpDataTable usa min-width: max-content no
   canvas, que empacotava as colunas a esquerda e deixava um vazio a direita quando
   havia poucas colunas (e estourava a largura com texto longo). Com width:100% +
   colunas minmax(min, fr) a tabela estica para preencher e o texto longo quebra na
   celula. Escopado ao ErpRecordsTab (nao afeta a aba de produtos). */
.erp-panel__tab-body :deep(.app-entity-grid__canvas) {
  min-width: 0 !important;
  width: 100%;
}

/* Controles inline de uma linha so, na mesma altura dos demais filtros do toolbar. */
.erp-records-filter {
  min-height: 2.45rem;
  height: 2.45rem;
  padding: 0 0.7rem;
  border-radius: 0.8rem;
  border: 1px solid var(--erp-primary-border);
  background: var(--erp-control-bg);
  color: var(--text-main);
  font-size: 0.85rem;
}

.erp-records-filter[type='number'] {
  width: 9.5rem;
}

.erp-records-filter--select {
  cursor: pointer;
}

/* Switch sutil de base do periodo (compra x importacao), ao lado de "Por pagina".
   Discreto de proposito: e' de uso raro, so quando precisa auditar pela data do lote. */
.erp-records-datefield {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.74rem;
  color: var(--text-muted);
  cursor: pointer;
  white-space: nowrap;
}
</style>
