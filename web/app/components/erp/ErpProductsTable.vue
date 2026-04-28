<script setup lang="ts">
import { computed } from "vue";

import AppEntityGrid from "~/components/ui/AppEntityGrid.vue";

interface TableColumn {
  id: string;
  label: string;
  width?: string;
  align?: string;
  locked?: boolean;
  defaultVisible?: boolean;
}

interface GenericRow {
  [key: string]: unknown;
}

const props = withDefaults(defineProps<{
  columns: TableColumn[];
  rows: GenericRow[];
  rowKey?: string | ((row: GenericRow, index: number) => string);
  loading?: boolean;
  searchValue?: string;
  generalSearchPlaceholder?: string;
  identifierSearchValue?: string;
  identifierSearchLabel?: string;
  identifierSearchPlaceholder?: string;
  showIdentifierSearch?: boolean;
  showRefreshAction?: boolean;
  showBootstrapAction?: boolean;
  bootstrapLabel?: string;
  bootstrapBusyLabel?: string;
  syncing?: boolean;
  canBootstrap?: boolean;
  total?: number;
  page?: number;
  pageSize?: number;
  pageSizeOptions?: number[];
  showCounterColumn?: boolean;
  counterColumnLabel?: string;
  emptyTitle?: string;
  emptyText?: string;
  storageKey?: string;
  testid?: string;
}>(), {
  rowKey: "id",
  loading: false,
  searchValue: "",
  generalSearchPlaceholder: "Busca geral...",
  identifierSearchValue: "",
  identifierSearchLabel: "Busca por identificador (comeca com)",
  identifierSearchPlaceholder: "Ex: 153",
  showIdentifierSearch: true,
  showRefreshAction: true,
  showBootstrapAction: true,
  bootstrapLabel: "Bootstrap produtos 184",
  bootstrapBusyLabel: "Sincronizando...",
  syncing: false,
  canBootstrap: true,
  total: 0,
  page: 1,
  pageSize: 50,
  pageSizeOptions: () => [25, 50, 100, 200],
  showCounterColumn: true,
  counterColumnLabel: "#",
  emptyTitle: "Nenhum produto no ERP",
  emptyText: "Ajuste os filtros para preencher a grade.",
  storageKey: "erp-products-grid-columns",
  testid: "erp-products-grid"
});

const emit = defineEmits<{
  (e: "update:searchValue", value: string): void;
  (e: "update:identifierSearchValue", value: string): void;
  (e: "update:page", value: number): void;
  (e: "update:pageSize", value: number): void;
  (e: "refresh"): void;
  (e: "bootstrap"): void;
}>();

const totalPages = computed(() => {
  const size = Math.max(1, Number(props.pageSize || 1));
  const rawTotal = Math.max(0, Number(props.total || 0));
  return Math.max(1, Math.ceil(rawTotal / size));
});

const resolvedColumns = computed(() => {
  if (!props.showCounterColumn) {
    return props.columns;
  }

  return [
    { id: "__counter", label: props.counterColumnLabel, width: "84px", align: "center", locked: true },
    ...props.columns
  ];
});

const rowsWithCounter = computed(() => {
  const baseIndex = (Math.max(1, Number(props.page || 1)) - 1) * Math.max(1, Number(props.pageSize || 1));

  return (Array.isArray(props.rows) ? props.rows : []).map((row, index) => ({
    ...row,
    __counter: baseIndex + index + 1
  }));
});

function formatCurrencyFromCents(value: unknown) {
  const rawValue = String(value ?? "").trim();
  if (!rawValue) {
    return "-";
  }

  const parsed = Number(rawValue);
  if (!Number.isFinite(parsed)) {
    return rawValue;
  }

  return new Intl.NumberFormat("pt-BR", {
    style: "currency",
    currency: "BRL"
  }).format(parsed / 100);
}

function resolveInputValue(value: Event | string) {
  if (typeof value === "string") {
    return value;
  }

  const target = value.target as HTMLInputElement | null;
  return String(target?.value || "");
}

function updateGeneralSearch(value: Event | string) {
  emit("update:searchValue", resolveInputValue(value));
}

function updateIdentifierSearch(event: Event) {
  emit("update:identifierSearchValue", resolveInputValue(event));
}

function previousPage() {
  const nextPage = Math.max(1, Number(props.page || 1) - 1);
  if (nextPage !== props.page) {
    emit("update:page", nextPage);
  }
}

function nextPage() {
  const next = Math.min(totalPages.value, Number(props.page || 1) + 1);
  if (next !== props.page) {
    emit("update:page", next);
  }
}

function updatePageSize(event: Event) {
  const target = event.target as HTMLSelectElement;
  const parsed = Number(target?.value || props.pageSize);
  const nextSize = Number.isFinite(parsed) && parsed > 0 ? parsed : props.pageSize;
  emit("update:pageSize", nextSize);
}
</script>

<template>
  <div class="erp-products-table">
    <header class="erp-products-table__pagination erp-products-table__pagination--top">
      <div class="erp-products-table__pagination-summary">
        Mostrando {{ rowsWithCounter.length }} de {{ Number(total || 0).toLocaleString("pt-BR") }}
      </div>

      <div class="erp-products-table__pagination-controls">
        <label class="erp-products-table__page-size">
          <span>Por pagina</span>
          <select :value="pageSize" :disabled="loading" @change="updatePageSize">
            <option v-for="size in pageSizeOptions" :key="size" :value="size">{{ size === 99999 ? 'Todos' : size }}</option>
          </select>
        </label>

        <button
          class="erp-products-table__page-btn"
          type="button"
          :disabled="loading || Number(page || 1) <= 1"
          @click="previousPage"
        >
          Anterior
        </button>

        <strong class="erp-products-table__page-indicator">{{ Number(page || 1) }} / {{ totalPages }}</strong>

        <button
          class="erp-products-table__page-btn"
          type="button"
          :disabled="loading || Number(page || 1) >= totalPages"
          @click="nextPage"
        >
          Proxima
        </button>
      </div>
    </header>

    <AppEntityGrid
      :columns="resolvedColumns"
      :rows="rowsWithCounter"
      :row-key="rowKey"
      :search-value="searchValue"
      :loading="loading"
      :search-placeholder="generalSearchPlaceholder"
      :empty-title="emptyTitle"
      :empty-text="emptyText"
      :storage-key="storageKey"
      :testid="testid"
      @update:search-value="updateGeneralSearch"
    >
      <template #toolbar-filters>
        <label v-if="showIdentifierSearch" class="erp-products-table__prefix-filter">
          <span>{{ identifierSearchLabel }}</span>
          <input
            class="erp-products-table__prefix-input"
            type="search"
            :value="identifierSearchValue"
            :placeholder="identifierSearchPlaceholder"
            @input="updateIdentifierSearch"
          >
        </label>

        <slot name="toolbar-filters" />
      </template>

      <template #toolbar-actions>
        <div class="erp-products-table__actions">
          <button
            v-if="showRefreshAction"
            class="erp-products-table__action erp-products-table__action--ghost"
            type="button"
            :disabled="loading"
            @click="emit('refresh')"
          >
            Atualizar
          </button>

          <button
            v-if="showBootstrapAction"
            class="erp-products-table__action erp-products-table__action--primary"
            type="button"
            :disabled="!canBootstrap || syncing"
            @click="emit('bootstrap')"
          >
            {{ syncing ? bootstrapBusyLabel : bootstrapLabel }}
          </button>

          <slot name="toolbar-actions" />
        </div>
      </template>

      <template #cell-__counter="{ row }">
        <span class="erp-products-table__counter">{{ row.__counter }}</span>
      </template>

      <template #cell-name="slotProps">
        <slot name="cell-name" v-bind="slotProps">
          {{ slotProps.row?.name }}
        </slot>
      </template>

      <template #cell-priceRaw="slotProps">
        <slot name="cell-priceRaw" v-bind="slotProps">
          {{ slotProps.row?.priceRaw }}
        </slot>
      </template>

      <template #cell-sourceUpdatedAt="slotProps">
        <slot name="cell-sourceUpdatedAt" v-bind="slotProps">
          {{ slotProps.row?.sourceUpdatedAt }}
        </slot>
      </template>

      <template #cell-total_amount_raw="{ row }">
        <span class="erp-products-table__money">{{ formatCurrencyFromCents(row?.total_amount_raw) }}</span>
      </template>

      <template #cell-product_return_raw="{ row }">
        <span class="erp-products-table__money">{{ formatCurrencyFromCents(row?.product_return_raw) }}</span>
      </template>

      <template #cell-amount_raw="{ row }">
        <span class="erp-products-table__money">{{ formatCurrencyFromCents(row?.amount_raw) }}</span>
      </template>

      <template #cell-total_exclusion_raw="{ row }">
        <span class="erp-products-table__money">{{ formatCurrencyFromCents(row?.total_exclusion_raw) }}</span>
      </template>

      <template #cell-total_debit_raw="{ row }">
        <span class="erp-products-table__money">{{ formatCurrencyFromCents(row?.total_debit_raw) }}</span>
      </template>
    </AppEntityGrid>

    <footer class="erp-products-table__pagination">
      <div class="erp-products-table__pagination-summary">
        Mostrando {{ rowsWithCounter.length }} de {{ Number(total || 0).toLocaleString("pt-BR") }}
      </div>

      <div class="erp-products-table__pagination-controls">
        <label class="erp-products-table__page-size">
          <span>Por pagina</span>
          <select :value="pageSize" :disabled="loading" @change="updatePageSize">
            <option v-for="size in pageSizeOptions" :key="size" :value="size">{{ size === 99999 ? 'Todos' : size }}</option>
          </select>
        </label>

        <button
          class="erp-products-table__page-btn"
          type="button"
          :disabled="loading || Number(page || 1) <= 1"
          @click="previousPage"
        >
          Anterior
        </button>

        <strong class="erp-products-table__page-indicator">{{ Number(page || 1) }} / {{ totalPages }}</strong>

        <button
          class="erp-products-table__page-btn"
          type="button"
          :disabled="loading || Number(page || 1) >= totalPages"
          @click="nextPage"
        >
          Proxima
        </button>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.erp-products-table {
  display: grid;
  gap: 0.6rem;
}

.erp-products-table :deep(.app-entity-grid__toolbar-main) {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0.6rem;
}

.erp-products-table :deep(.app-entity-grid__search) {
  flex: 1 1 240px;
}

.erp-products-table :deep(.app-entity-grid__viewport) {
  overflow-x: auto;
  padding-bottom: 0.25rem;
}

.erp-products-table :deep(.app-entity-grid__canvas) {
  min-width: max-content;
}

.erp-products-table__prefix-filter {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  min-width: min(100%, 240px);
  flex: 1 1 240px;
}

.erp-products-table__prefix-filter span {
  font-size: 0.72rem;
  color: var(--text-muted);
}

.erp-products-table__prefix-input {
  width: 100%;
  min-height: 2.45rem;
  padding: 0 0.8rem;
  border-radius: 0.8rem;
  border: 1px solid rgba(129, 140, 248, 0.18);
  background: rgba(18, 25, 38, 0.9);
  color: var(--text-main);
}

.erp-products-table__actions {
  display: flex;
  gap: 0.55rem;
  flex-wrap: wrap;
}

.erp-products-table__action {
  min-height: 2.45rem;
  padding: 0 0.95rem;
  border-radius: 0.8rem;
  border: 1px solid var(--line-soft);
  background: rgba(17, 24, 39, 0.92);
  color: var(--text-main);
  font-weight: 600;
  cursor: pointer;
  transition: transform 0.16s ease, border-color 0.16s ease, background-color 0.16s ease;
}

.erp-products-table__action:hover:not(:disabled) {
  transform: translateY(-1px);
  border-color: rgba(98, 129, 255, 0.35);
}

.erp-products-table__action:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.erp-products-table__action--primary {
  border-color: rgba(83, 198, 160, 0.32);
  background: linear-gradient(135deg, rgba(13, 102, 87, 0.92), rgba(14, 73, 67, 0.94));
}

.erp-products-table__counter {
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

.erp-products-table__money {
  color: #b9ffd2;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.erp-products-table__pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.8rem;
  flex-wrap: wrap;
}

.erp-products-table__pagination-summary {
  color: var(--text-muted);
  font-size: 0.78rem;
}

.erp-products-table__pagination-controls {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  flex-wrap: wrap;
}

.erp-products-table__page-size {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: var(--text-muted);
  font-size: 0.74rem;
}

.erp-products-table__page-size select {
  min-height: 2rem;
  border-radius: 0.65rem;
  border: 1px solid var(--line-soft);
  background: rgba(17, 24, 39, 0.92);
  color: var(--text-main);
  padding: 0 0.55rem;
}

.erp-products-table__page-btn {
  min-height: 2rem;
  padding: 0 0.75rem;
  border-radius: 0.65rem;
  border: 1px solid var(--line-soft);
  background: rgba(17, 24, 39, 0.92);
  color: var(--text-main);
}

.erp-products-table__page-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.erp-products-table__page-indicator {
  min-width: 4.2rem;
  text-align: center;
  color: var(--text-main);
  font-size: 0.78rem;
}

@media (max-width: 1080px) {
  .erp-products-table :deep(.app-entity-grid__canvas) {
    min-width: 0;
  }
}

@media (max-width: 720px) {
  .erp-products-table__prefix-filter {
    width: 100%;
    min-width: 0;
  }

  .erp-products-table__actions {
    width: 100%;
  }

  .erp-products-table__action {
    flex: 1 1 11rem;
  }
}
</style>
