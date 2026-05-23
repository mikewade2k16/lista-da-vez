<script setup lang="ts">
import { computed, ref, useSlots } from 'vue'
import { CalendarDays, Download } from 'lucide-vue-next'

import AppEntityGrid from '~/components/ui/AppEntityGrid.vue'

interface TableColumn {
  id: string
  label: string
  width?: string
  align?: string
  locked?: boolean
  defaultVisible?: boolean
  sortable?: boolean
}

interface GenericRow {
  [key: string]: unknown
}

type ExportScope = 'page' | 'filtered' | 'all'

const props = withDefaults(
  defineProps<{
    columns: TableColumn[]
    rows: GenericRow[]
    rowKey?: string | ((row: GenericRow, index: number) => string)
    loading?: boolean
    total?: number
    page?: number
    pageSize?: number
    pageSizeOptions?: number[]
    searchValue?: string
    searchPlaceholder?: string
    specificSearchValue?: string
    specificSearchLabel?: string
    specificSearchPlaceholder?: string
    showSpecificSearch?: boolean
    dateFrom?: string
    dateTo?: string
    sortBy?: string
    sortDir?: string
    dedup?: boolean
    showDedupToggle?: boolean
    showRefreshAction?: boolean
    showBootstrapAction?: boolean
    showExportAction?: boolean
    exporting?: boolean
    bootstrapLabel?: string
    bootstrapBusyLabel?: string
    syncing?: boolean
    canBootstrap?: boolean
    showColumnsManager?: boolean
    columnsLabel?: string
    showCounterColumn?: boolean
    counterColumnLabel?: string
    emptyTitle?: string
    emptyText?: string
    storageKey?: string
    testid?: string
  }>(),
  {
    rowKey: 'id',
    loading: false,
    total: 0,
    page: 1,
    pageSize: 50,
    pageSizeOptions: () => [25, 50, 100, 200],
    searchValue: '',
    searchPlaceholder: 'Busca geral...',
    specificSearchValue: '',
    specificSearchLabel: 'Código / ID',
    specificSearchPlaceholder: 'Ex: 153',
    showSpecificSearch: true,
    dateFrom: '',
    dateTo: '',
    sortBy: '',
    sortDir: 'desc',
    dedup: true,
    showDedupToggle: false,
    showRefreshAction: true,
    showBootstrapAction: false,
    showExportAction: false,
    exporting: false,
    bootstrapLabel: 'Bootstrap ERP',
    bootstrapBusyLabel: 'Sincronizando...',
    syncing: false,
    canBootstrap: true,
    showColumnsManager: true,
    columnsLabel: 'Colunas',
    showCounterColumn: true,
    counterColumnLabel: '#',
    emptyTitle: 'Nenhum registro encontrado',
    emptyText: 'Ajuste os filtros para preencher a grade.',
    storageKey: '',
    testid: 'erp-data-table',
  },
)

const emit = defineEmits<{
  (e: 'update:searchValue', value: string): void
  (e: 'update:specificSearchValue', value: string): void
  (e: 'update:dateFrom', value: string): void
  (e: 'update:dateTo', value: string): void
  (e: 'update:sortBy', value: string): void
  (e: 'update:sortDir', value: string): void
  (e: 'update:dedup', value: boolean): void
  (e: 'update:page', value: number): void
  (e: 'update:pageSize', value: number): void
  (e: 'refresh'): void
  (e: 'bootstrap'): void
  (e: 'export', scope: ExportScope): void
}>()

// Slots that ErpDataTable consumes internally and must NOT be forwarded to AppEntityGrid
const internalSlots = new Set([
  'toolbar-filters',
  'toolbar-actions',
  'cell-__counter',
  'cell-total_amount_raw',
  'cell-product_return_raw',
  'cell-amount_raw',
  'cell-total_exclusion_raw',
  'cell-total_debit_raw',
])

const slots = useSlots()
const exportMenuOpen = ref(false)

// All other slots from parent are forwarded to AppEntityGrid
const forwardableSlots = computed(() =>
  Object.fromEntries(Object.entries(slots).filter(([name]) => !internalSlots.has(name))),
)

const totalPages = computed(() => {
  const size = Math.max(1, Number(props.pageSize || 1))
  const rawTotal = Math.max(0, Number(props.total || 0))
  return Math.max(1, Math.ceil(rawTotal / size))
})

const resolvedColumns = computed(() => {
  if (!props.showCounterColumn) return props.columns
  return [
    {
      id: '__counter',
      label: props.counterColumnLabel,
      width: '72px',
      align: 'center',
      locked: true,
      sortable: false,
    },
    ...props.columns,
  ]
})

const rowsWithCounter = computed(() => {
  const baseIndex =
    (Math.max(1, Number(props.page || 1)) - 1) * Math.max(1, Number(props.pageSize || 1))
  return (Array.isArray(props.rows) ? props.rows : []).map((row, index) => ({
    ...row,
    __counter: baseIndex + index + 1,
  }))
})

function castRow(v: unknown): GenericRow {
  return v as GenericRow
}

function formatCurrencyFromCents(value: unknown) {
  const raw = String(value ?? '').trim()
  if (!raw) return '-'
  const parsed = Number(raw)
  if (!Number.isFinite(parsed)) return raw
  return new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' }).format(parsed / 100)
}

function handleSort(columnId: string) {
  if (columnId === props.sortBy) {
    emit('update:sortDir', props.sortDir === 'asc' ? 'desc' : 'asc')
  } else {
    emit('update:sortBy', columnId)
    emit('update:sortDir', 'desc')
  }
}

function previousPage() {
  const next = Math.max(1, Number(props.page || 1) - 1)
  if (next !== props.page) emit('update:page', next)
}

function nextPage() {
  const next = Math.min(totalPages.value, Number(props.page || 1) + 1)
  if (next !== props.page) emit('update:page', next)
}

function updatePageSize(event: Event) {
  const target = event.target as HTMLSelectElement
  const parsed = Number(target?.value || props.pageSize)
  const next = Number.isFinite(parsed) && parsed > 0 ? parsed : props.pageSize
  emit('update:pageSize', next)
}

function resolveInputValue(value: Event | string) {
  if (typeof value === 'string') return value
  const target = value.target as HTMLInputElement | null
  return String(target?.value || '')
}

function emitExport(scope: ExportScope) {
  exportMenuOpen.value = false
  emit('export', scope)
}
</script>

<template>
  <div class="erp-data-table" :data-testid="testid">
    <div class="erp-data-table__pagination erp-data-table__pagination--top">
      <div class="erp-data-table__pagination-summary">
        Mostrando
        <strong>{{ rowsWithCounter.length }}</strong>
        de
        <strong>{{ Number(total || 0).toLocaleString('pt-BR') }}</strong>
      </div>

      <div class="erp-data-table__pagination-controls">
        <label class="erp-data-table__page-size">
          <span>Por página</span>
          <select :value="pageSize" :disabled="loading" @change="updatePageSize">
            <option v-for="size in pageSizeOptions" :key="size" :value="size">
              {{ size === 99999 ? 'Todos' : size }}
            </option>
          </select>
        </label>

        <button
          class="erp-data-table__page-btn"
          type="button"
          :disabled="loading || Number(page || 1) <= 1"
          @click="previousPage"
        >
          Anterior
        </button>

        <strong class="erp-data-table__page-indicator">
          {{ Number(page || 1) }} / {{ totalPages }}
        </strong>

        <button
          class="erp-data-table__page-btn"
          type="button"
          :disabled="loading || Number(page || 1) >= totalPages"
          @click="nextPage"
        >
          Próxima
        </button>
      </div>
    </div>

    <AppEntityGrid
      :columns="resolvedColumns"
      :rows="rowsWithCounter"
      :row-key="rowKey"
      :search-value="searchValue"
      :search-placeholder="searchPlaceholder"
      :loading="loading"
      :empty-title="emptyTitle"
      :empty-text="emptyText"
      :storage-key="storageKey"
      :testid="testid + '-grid'"
      :show-columns-manager="showColumnsManager"
      :columns-label="columnsLabel"
      :sort-by="sortBy"
      :sort-dir="sortDir"
      @update:search-value="emit('update:searchValue', $event)"
      @sort="handleSort"
    >
      <template #toolbar-filters>
        <input
          v-if="showSpecificSearch"
          class="erp-data-table__filter-input"
          type="search"
          :value="specificSearchValue"
          :placeholder="specificSearchPlaceholder"
          @input="emit('update:specificSearchValue', resolveInputValue($event))"
        />

        <AppDatePicker
          :model-value="dateFrom"
          :end-date="dateTo"
          @update:model-value="emit('update:dateFrom', $event)"
          @update:end-date="emit('update:dateTo', $event)"
        >
          <template #default="{ label }">
            <button type="button" class="erp-data-table__date-trigger">
              <CalendarDays class="erp-data-table__date-trigger-icon" :size="14" />
              <span class="erp-data-table__date-trigger-label">
                {{ label || 'Período...' }}
              </span>
            </button>
          </template>
        </AppDatePicker>

        <label v-if="showDedupToggle" class="erp-data-table__dedup-toggle">
          <input
            type="checkbox"
            :checked="dedup"
            @change="emit('update:dedup', ($event.target as HTMLInputElement).checked)"
          />
          <span>Sem duplic.</span>
        </label>

        <slot name="toolbar-filters"></slot>
      </template>

      <template #toolbar-actions>
        <div class="erp-data-table__actions">
          <div v-if="showExportAction" class="erp-data-table__export-wrap">
            <button
              class="erp-data-table__action erp-data-table__action--export"
              type="button"
              :disabled="loading || exporting"
              @click="exportMenuOpen = !exportMenuOpen"
            >
              <Download :size="14" />
              {{ exporting ? 'Exportando...' : 'Exportar CSV' }}
            </button>

            <div v-if="exportMenuOpen" class="erp-data-table__export-menu">
              <button type="button" @click="emitExport('page')">
                <strong>Pagina atual</strong>
                <span>Somente as linhas visiveis agora.</span>
              </button>
              <button type="button" @click="emitExport('filtered')">
                <strong>Filtrado inteiro</strong>
                <span>Todos os resultados dos filtros atuais.</span>
              </button>
              <button type="button" @click="emitExport('all')">
                <strong>Tudo da aba</strong>
                <span>Ignora busca e periodo para baixar a base completa.</span>
              </button>
            </div>
          </div>

          <button
            v-if="showRefreshAction"
            class="erp-data-table__action erp-data-table__action--ghost"
            type="button"
            :disabled="loading"
            @click="emit('refresh')"
          >
            Atualizar
          </button>

          <button
            v-if="showBootstrapAction"
            class="erp-data-table__action erp-data-table__action--primary"
            type="button"
            :disabled="!canBootstrap || syncing"
            @click="emit('bootstrap')"
          >
            {{ syncing ? bootstrapBusyLabel : bootstrapLabel }}
          </button>

          <slot name="toolbar-actions"></slot>
        </div>
      </template>

      <template #cell-__counter="{ row }">
        <span class="erp-data-table__counter">{{ castRow(row).__counter }}</span>
      </template>

      <template #cell-total_amount_raw="{ row }">
        <span class="erp-data-table__money">
          {{ formatCurrencyFromCents(castRow(row).total_amount_raw) }}
        </span>
      </template>

      <template #cell-product_return_raw="{ row }">
        <span class="erp-data-table__money">
          {{ formatCurrencyFromCents(castRow(row).product_return_raw) }}
        </span>
      </template>

      <template #cell-amount_raw="{ row }">
        <span class="erp-data-table__money">
          {{ formatCurrencyFromCents(castRow(row).amount_raw) }}
        </span>
      </template>

      <template #cell-total_exclusion_raw="{ row }">
        <span class="erp-data-table__money">
          {{ formatCurrencyFromCents(castRow(row).total_exclusion_raw) }}
        </span>
      </template>

      <template #cell-total_debit_raw="{ row }">
        <span class="erp-data-table__money">
          {{ formatCurrencyFromCents(castRow(row).total_debit_raw) }}
        </span>
      </template>

      <!-- Forward all remaining parent slots to AppEntityGrid -->
      <template v-for="(_, name) in forwardableSlots" :key="name" #[name]="slotProps">
        <slot :name="name" v-bind="(slotProps as Record<string, unknown>) ?? {}"></slot>
      </template>
    </AppEntityGrid>

    <footer class="erp-data-table__pagination">
      <div class="erp-data-table__pagination-summary">
        Mostrando
        <strong>{{ rowsWithCounter.length }}</strong>
        de
        <strong>{{ Number(total || 0).toLocaleString('pt-BR') }}</strong>
      </div>

      <div class="erp-data-table__pagination-controls">
        <label class="erp-data-table__page-size">
          <span>Por página</span>
          <select :value="pageSize" :disabled="loading" @change="updatePageSize">
            <option v-for="size in pageSizeOptions" :key="size" :value="size">
              {{ size === 99999 ? 'Todos' : size }}
            </option>
          </select>
        </label>

        <button
          class="erp-data-table__page-btn"
          type="button"
          :disabled="loading || Number(page || 1) <= 1"
          @click="previousPage"
        >
          Anterior
        </button>

        <strong class="erp-data-table__page-indicator">
          {{ Number(page || 1) }} / {{ totalPages }}
        </strong>

        <button
          class="erp-data-table__page-btn"
          type="button"
          :disabled="loading || Number(page || 1) >= totalPages"
          @click="nextPage"
        >
          Próxima
        </button>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.erp-data-table {
  display: grid;
  gap: 0.6rem;
}

.erp-data-table :deep(.app-entity-grid__toolbar-main) {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}

.erp-data-table :deep(.app-entity-grid__search) {
  flex: 1 1 200px;
}

.erp-data-table :deep(.app-entity-grid__viewport) {
  overflow-x: auto;
  padding-bottom: 0.25rem;
}

.erp-data-table :deep(.app-entity-grid__canvas) {
  min-width: max-content;
}

.erp-data-table__filter-input {
  flex: 1 1 180px;
  min-width: min(100%, 180px);
  min-height: 2.45rem;
  padding: 0 0.8rem;
  border-radius: 0.8rem;
  border: 1px solid rgba(129, 140, 248, 0.18);
  background: rgba(18, 25, 38, 0.9);
  color: var(--text-main);
}

.erp-data-table__date-trigger {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  min-height: 2.45rem;
  padding: 0 0.8rem;
  border-radius: 0.8rem;
  border: 1px solid rgba(129, 140, 248, 0.18);
  background: rgba(18, 25, 38, 0.9);
  color: var(--text-main);
  cursor: pointer;
  min-width: 160px;
  max-width: 260px;
  text-align: left;
  font-size: 0.85rem;
  transition: border-color 0.16s ease;
  white-space: nowrap;
}

.erp-data-table__date-trigger:hover {
  border-color: rgba(98, 129, 255, 0.35);
}

.erp-data-table__date-trigger-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.erp-data-table__date-trigger-icon {
  flex-shrink: 0;
  opacity: 0.65;
}

.erp-data-table__dedup-toggle {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.8rem;
  color: var(--text-muted);
  cursor: pointer;
  white-space: nowrap;
}

.erp-data-table__actions {
  display: flex;
  gap: 0.55rem;
  flex-wrap: wrap;
}

.erp-data-table__export-wrap {
  position: relative;
}

.erp-data-table__action {
  min-height: 2.45rem;
  padding: 0 0.95rem;
  border-radius: 0.8rem;
  border: 1px solid var(--line-soft);
  background: rgba(17, 24, 39, 0.92);
  color: var(--text-main);
  font-weight: 600;
  cursor: pointer;
  transition:
    transform 0.16s ease,
    border-color 0.16s ease;
}

.erp-data-table__action:hover:not(:disabled) {
  transform: translateY(-1px);
  border-color: rgba(98, 129, 255, 0.35);
}

.erp-data-table__action:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.erp-data-table__action--primary {
  border-color: rgba(83, 198, 160, 0.32);
  background: linear-gradient(135deg, rgba(13, 102, 87, 0.92), rgba(14, 73, 67, 0.94));
}

.erp-data-table__action--export {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  border-color: rgba(129, 140, 248, 0.28);
}

.erp-data-table__export-menu {
  position: absolute;
  top: calc(100% + 0.45rem);
  right: 0;
  z-index: 40;
  width: 15rem;
  display: grid;
  gap: 0.25rem;
  padding: 0.45rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.8rem;
  background: rgba(14, 20, 35, 0.98);
  box-shadow: 0 14px 34px rgba(0, 0, 0, 0.36);
}

.erp-data-table__export-menu button {
  display: grid;
  gap: 0.12rem;
  width: 100%;
  padding: 0.5rem 0.6rem;
  border: none;
  border-radius: 0.65rem;
  background: transparent;
  color: var(--text-main);
  text-align: left;
  cursor: pointer;
}

.erp-data-table__export-menu button:hover {
  background: rgba(98, 129, 255, 0.12);
}

.erp-data-table__export-menu strong {
  font-size: 0.82rem;
}

.erp-data-table__export-menu span {
  color: var(--text-muted);
  font-size: 0.72rem;
  line-height: 1.35;
}

.erp-data-table__counter {
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

.erp-data-table__money {
  color: #b9ffd2;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.erp-data-table__pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.8rem;
  flex-wrap: wrap;
}

.erp-data-table__pagination-summary {
  color: var(--text-muted);
  font-size: 0.78rem;
}

.erp-data-table__pagination-controls {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  flex-wrap: wrap;
}

.erp-data-table__page-size {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: var(--text-muted);
  font-size: 0.74rem;
}

.erp-data-table__page-size select {
  min-height: 2rem;
  border-radius: 0.65rem;
  border: 1px solid var(--line-soft);
  background: rgba(17, 24, 39, 0.92);
  color: var(--text-main);
  padding: 0 0.55rem;
}

.erp-data-table__page-btn {
  min-height: 2rem;
  padding: 0 0.75rem;
  border-radius: 0.65rem;
  border: 1px solid var(--line-soft);
  background: rgba(17, 24, 39, 0.92);
  color: var(--text-main);
  cursor: pointer;
}

.erp-data-table__page-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.erp-data-table__page-indicator {
  min-width: 4.2rem;
  text-align: center;
  color: var(--text-main);
  font-size: 0.78rem;
}

@media (max-width: 1080px) {
  .erp-data-table :deep(.app-entity-grid__canvas) {
    min-width: 0;
  }
}

@media (max-width: 720px) {
  .erp-data-table__filter-input {
    flex: 1 1 100%;
    min-width: 0;
  }

  .erp-data-table__date-trigger {
    min-width: 0;
    max-width: 100%;
  }

  .erp-data-table__actions {
    width: 100%;
  }

  .erp-data-table__action {
    flex: 1 1 11rem;
  }
}
</style>
