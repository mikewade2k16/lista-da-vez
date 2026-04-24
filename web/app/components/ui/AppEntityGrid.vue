<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Search, SlidersHorizontal } from "lucide-vue-next";

const props = defineProps({
  columns: {
    type: Array,
    default: () => []
  },
  rows: {
    type: Array,
    default: () => []
  },
  rowKey: {
    type: [String, Function],
    default: "id"
  },
  searchValue: {
    type: String,
    default: ""
  },
  searchPlaceholder: {
    type: String,
    default: "Pesquisar por texto..."
  },
  showSearch: {
    type: Boolean,
    default: true
  },
  showColumnsManager: {
    type: Boolean,
    default: true
  },
  loading: {
    type: Boolean,
    default: false
  },
  emptyTitle: {
    type: String,
    default: "Nenhum resultado"
  },
  emptyText: {
    type: String,
    default: "Ajuste os filtros ou cadastre um novo item para preencher a listagem."
  },
  storageKey: {
    type: String,
    default: ""
  },
  columnsLabel: {
    type: String,
    default: "Colunas"
  },
  testid: {
    type: String,
    default: ""
  }
});

const emit = defineEmits(["update:searchValue", "visible-columns-change"]);

const columnsMenuOpen = ref(false);
const columnsMenuRef = ref(null);
const visibleColumnIds = ref([]);
const hydrated = ref(false);

const normalizedColumns = computed(() =>
  (Array.isArray(props.columns) ? props.columns : []).map((column, index) => ({
    id: String(column?.id || `column-${index}`).trim(),
    label: String(column?.label || column?.id || `Coluna ${index + 1}`).trim(),
    width: String(column?.width || "minmax(140px, 1fr)").trim(),
    align: String(column?.align || "start").trim(),
    locked: Boolean(column?.locked),
    defaultVisible: column?.defaultVisible !== false,
    description: String(column?.description || "").trim()
  }))
);

const visibleColumns = computed(() =>
  normalizedColumns.value.filter((column) => column.locked || visibleColumnIds.value.includes(column.id))
);

const gridTemplateColumns = computed(() => visibleColumns.value.map((column) => column.width).join(" "));
const selectedColumnsCount = computed(() => visibleColumns.value.length);

function buildDefaultVisibleColumns() {
  return normalizedColumns.value
    .filter((column) => column.defaultVisible && !column.locked)
    .map((column) => column.id);
}

function syncVisibleColumns(forceDefaults = false) {
  const availableIds = new Set(normalizedColumns.value.map((column) => column.id));
  const defaults = buildDefaultVisibleColumns();

  if (forceDefaults || visibleColumnIds.value.length === 0) {
    visibleColumnIds.value = defaults;
    return;
  }

  visibleColumnIds.value = visibleColumnIds.value.filter((columnId) => availableIds.has(columnId));

  if (visibleColumnIds.value.length === 0) {
    visibleColumnIds.value = defaults;
  }
}

function loadVisibleColumns() {
  if (!import.meta.client || !props.storageKey) {
    syncVisibleColumns(true);
    hydrated.value = true;
    return;
  }

  try {
    const rawValue = window.localStorage.getItem(props.storageKey);
    if (!rawValue) {
      syncVisibleColumns(true);
      hydrated.value = true;
      return;
    }

    const parsed = JSON.parse(rawValue);
    visibleColumnIds.value = Array.isArray(parsed)
      ? parsed.map((columnId) => String(columnId || "").trim()).filter(Boolean)
      : [];
    syncVisibleColumns(false);
  } catch {
    syncVisibleColumns(true);
  }

  hydrated.value = true;
}

function persistVisibleColumns() {
  if (!import.meta.client || !props.storageKey || !hydrated.value) {
    return;
  }

  window.localStorage.setItem(props.storageKey, JSON.stringify(visibleColumnIds.value));
}

function isColumnVisible(column) {
  return column.locked || visibleColumnIds.value.includes(column.id);
}

function toggleColumn(column) {
  if (column.locked) {
    return;
  }

  if (visibleColumnIds.value.includes(column.id)) {
    visibleColumnIds.value = visibleColumnIds.value.filter((columnId) => columnId !== column.id);
    return;
  }

  visibleColumnIds.value = [...visibleColumnIds.value, column.id];
}

function closeColumnsMenu() {
  columnsMenuOpen.value = false;
}

function handleOutsideClick(event) {
  if (!columnsMenuOpen.value) {
    return;
  }

  if (columnsMenuRef.value?.contains(event.target)) {
    return;
  }

  closeColumnsMenu();
}

function resolveRowKey(row, index) {
  if (typeof props.rowKey === "function") {
    return props.rowKey(row, index);
  }

  const key = String(props.rowKey || "id").trim();
  return row?.[key] || `row-${index}`;
}

function updateSearchValue(event) {
  emit("update:searchValue", String(event?.target?.value || ""));
}

function formatCellValue(value) {
  if (Array.isArray(value)) {
    return value.filter(Boolean).join(", ") || "-";
  }

  if (value === null || value === undefined || String(value).trim() === "") {
    return "-";
  }

  return String(value);
}

watch(
  normalizedColumns,
  () => {
    if (!hydrated.value) {
      return;
    }

    syncVisibleColumns(false);
  },
  { deep: true }
);

watch(
  visibleColumnIds,
  () => {
    persistVisibleColumns();
    emit("visible-columns-change", visibleColumns.value.map((column) => column.id));
  },
  { deep: true }
);

onMounted(() => {
  loadVisibleColumns();
  document.addEventListener("click", handleOutsideClick);
});

onBeforeUnmount(() => {
  document.removeEventListener("click", handleOutsideClick);
});
</script>

<template>
  <article class="app-entity-grid" :data-testid="testid || undefined">
    <header class="app-entity-grid__toolbar">
      <div class="app-entity-grid__toolbar-main">
        <label v-if="showSearch" class="app-entity-grid__search">
          <Search class="app-entity-grid__search-icon" :size="15" :stroke-width="2.1" />
          <input
            class="app-entity-grid__search-input"
            :value="searchValue"
            type="search"
            :placeholder="searchPlaceholder"
            @input="updateSearchValue"
          >
        </label>

        <div class="app-entity-grid__filters">
          <slot name="toolbar-filters" />
        </div>
      </div>

      <div class="app-entity-grid__toolbar-actions">
        <div v-if="showColumnsManager" ref="columnsMenuRef" class="app-entity-grid__columns-wrap">
          <button class="app-entity-grid__toolbar-btn" type="button" @click.stop="columnsMenuOpen = !columnsMenuOpen">
            <SlidersHorizontal class="app-entity-grid__toolbar-icon" :size="15" :stroke-width="2.1" />
            <span>{{ columnsLabel }}</span>
            <strong>{{ selectedColumnsCount }}</strong>
          </button>

          <div v-if="columnsMenuOpen" class="app-entity-grid__columns-menu">
            <header class="app-entity-grid__columns-header">
              <strong>{{ columnsLabel }}</strong>
              <span>{{ selectedColumnsCount }}/{{ normalizedColumns.length }}</span>
            </header>

            <label
              v-for="column in normalizedColumns"
              :key="column.id"
              class="app-entity-grid__columns-item"
              :class="{ 'is-locked': column.locked }"
            >
              <input
                :checked="isColumnVisible(column)"
                type="checkbox"
                :disabled="column.locked"
                @change="toggleColumn(column)"
              >
              <span class="app-entity-grid__columns-copy">
                <span>{{ column.label }}</span>
                <small v-if="column.description">{{ column.description }}</small>
              </span>
            </label>
          </div>
        </div>

        <slot name="toolbar-actions" />
      </div>
    </header>

    <div class="app-entity-grid__viewport">
      <div class="app-entity-grid__canvas">
        <div
          v-if="visibleColumns.length"
          class="app-entity-grid__head"
          :style="{ gridTemplateColumns }"
        >
          <div
            v-for="column in visibleColumns"
            :key="column.id"
            class="app-entity-grid__head-cell"
            :class="`is-${column.align}`"
          >
            {{ column.label }}
          </div>
        </div>

        <div v-if="loading" class="app-entity-grid__empty-state">
          <strong>{{ emptyTitle }}</strong>
          <span>Carregando dados...</span>
        </div>

        <div v-else-if="!rows.length" class="app-entity-grid__empty-state">
          <strong>{{ emptyTitle }}</strong>
          <span>{{ emptyText }}</span>
          <slot name="empty" />
        </div>

        <div v-else class="app-entity-grid__body">
          <article
            v-for="(row, index) in rows"
            :key="resolveRowKey(row, index)"
            class="app-entity-grid__row"
            :style="{ gridTemplateColumns }"
          >
            <div
              v-for="column in visibleColumns"
              :key="column.id"
              class="app-entity-grid__cell"
              :class="`is-${column.align}`"
              :data-column-label="column.label"
            >
              <slot
                :name="`cell-${column.id}`"
                :row="row"
                :column="column"
                :row-index="index"
                :value="row?.[column.id]"
              >
                {{ formatCellValue(row?.[column.id]) }}
              </slot>
            </div>
          </article>
        </div>
      </div>
    </div>
  </article>
</template>

<style scoped>
.app-entity-grid {
  display: grid;
  gap: 0.8rem;
  border: 1px solid var(--line-soft);
  border-radius: 1rem;
  background: rgba(13, 18, 29, 0.9);
  padding: 0.8rem;
  box-shadow: var(--shadow-card);
}

.app-entity-grid__toolbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.app-entity-grid__toolbar-main {
  flex: 1 1 42rem;
  display: grid;
  gap: 0.6rem;
}

.app-entity-grid__search {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  min-height: 2.45rem;
  padding: 0 0.8rem;
  border-radius: 0.8rem;
  border: 1px solid rgba(129, 140, 248, 0.18);
  background: rgba(18, 25, 38, 0.9);
}

.app-entity-grid__search:focus-within {
  border-color: rgba(129, 140, 248, 0.42);
  box-shadow: 0 0 0 3px rgba(129, 140, 248, 0.12);
}

.app-entity-grid__search-icon {
  color: var(--text-muted);
  width: 15px;
  height: 15px;
  flex-shrink: 0;
}

.app-entity-grid__search-input {
  flex: 1;
  border: none;
  outline: none;
  background: transparent;
  color: var(--text-main);
  font-size: 0.84rem;
}

.app-entity-grid__search-input::placeholder {
  color: rgba(148, 163, 184, 0.7);
}

.app-entity-grid__filters,
.app-entity-grid__toolbar-actions {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  flex-wrap: wrap;
}

.app-entity-grid__toolbar-actions {
  justify-content: flex-end;
  flex: 0 1 auto;
}

.app-entity-grid__columns-wrap {
  position: relative;
}

.app-entity-grid__toolbar-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  min-height: 2.35rem;
  padding: 0 0.8rem;
  border-radius: 999px;
  border: 1px solid rgba(129, 140, 248, 0.2);
  background: rgba(18, 25, 38, 0.92);
  color: var(--text-main);
  font-size: 0.76rem;
  font-weight: 700;
  cursor: pointer;
}

.app-entity-grid__toolbar-icon {
  width: 15px;
  height: 15px;
  flex-shrink: 0;
}

.app-entity-grid__toolbar-btn:hover {
  border-color: rgba(129, 140, 248, 0.4);
}

.app-entity-grid__toolbar-btn strong {
  color: #ffffff;
}

.app-entity-grid__columns-menu {
  position: absolute;
  right: 0;
  top: calc(100% + 0.45rem);
  z-index: 20;
  width: min(18rem, 82vw);
  padding: 0.8rem;
  display: grid;
  gap: 0.55rem;
  border-radius: 0.9rem;
  border: 1px solid var(--line-soft);
  background: rgba(9, 13, 21, 0.98);
  box-shadow: 0 24px 46px rgba(0, 0, 0, 0.38);
}

.app-entity-grid__columns-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  font-size: 0.76rem;
  color: var(--text-muted);
}

.app-entity-grid__columns-item {
  display: flex;
  align-items: flex-start;
  gap: 0.55rem;
  color: var(--text-main);
  font-size: 0.8rem;
}

.app-entity-grid__columns-item input {
  margin-top: 0.2rem;
  accent-color: var(--accent-focus);
}

.app-entity-grid__columns-item.is-locked {
  opacity: 0.72;
}

.app-entity-grid__columns-copy {
  display: grid;
  gap: 0.14rem;
}

.app-entity-grid__columns-copy small {
  color: var(--text-muted);
  line-height: 1.35;
  font-size: 0.72rem;
}

.app-entity-grid__viewport {
  overflow: visible;
}

.app-entity-grid__canvas {
  display: grid;
  gap: 0.45rem;
  min-width: 0;
}

.app-entity-grid__head,
.app-entity-grid__row {
  display: grid;
  gap: 0.55rem;
  width: 100%;
  min-width: 0;
}

.app-entity-grid__head {
  position: sticky;
  top: 0;
  z-index: 5;
  padding: 0 0.2rem 0.3rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  background: rgba(13, 18, 29, 0.96);
}

.app-entity-grid__head-cell {
  min-width: 0;
  padding: 0 0.3rem;
  color: rgba(226, 232, 240, 0.8);
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.app-entity-grid__body {
  display: grid;
  gap: 0.5rem;
}

.app-entity-grid__row {
  align-items: stretch;
  padding: 0.3rem;
  border-radius: 0.95rem;
  background: rgba(18, 25, 38, 0.7);
  border: 1px solid rgba(255, 255, 255, 0.04);
}

.app-entity-grid__cell {
  min-width: 0;
  min-height: 2.9rem;
  padding: 0.12rem;
  display: flex;
  align-items: center;
  color: var(--text-main);
  overflow-wrap: anywhere;
}

.app-entity-grid__cell.is-center,
.app-entity-grid__head-cell.is-center {
  justify-content: center;
  text-align: center;
}

.app-entity-grid__cell.is-end,
.app-entity-grid__head-cell.is-end {
  justify-content: flex-end;
  text-align: right;
}

.app-entity-grid__empty-state {
  min-height: 14rem;
  display: grid;
  place-items: center;
  gap: 0.35rem;
  padding: 2rem 1rem;
  text-align: center;
  color: var(--text-muted);
  font-size: 0.82rem;
}

.app-entity-grid__empty-state strong {
  color: var(--text-main);
  font-size: 0.92rem;
}

@media (max-width: 900px) {
  .app-entity-grid {
    padding: 0.72rem;
  }

  .app-entity-grid__toolbar {
    gap: 0.75rem;
  }

  .app-entity-grid__toolbar-main {
    flex-basis: 100%;
  }

  .app-entity-grid__toolbar-actions {
    width: 100%;
    justify-content: space-between;
  }
}

@media (max-width: 1080px) {
  .app-entity-grid__head {
    display: none;
  }

  .app-entity-grid__row {
    grid-template-columns: repeat(2, minmax(0, 1fr)) !important;
    gap: 0.4rem;
  }

  .app-entity-grid__cell {
    min-height: 0;
    align-items: flex-start;
  }

  .app-entity-grid__cell::before {
    content: attr(data-column-label);
    display: block;
    margin-bottom: 0.28rem;
    color: rgba(226, 232, 240, 0.64);
    font-size: 0.63rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }

  .app-entity-grid__cell,
  .app-entity-grid__cell.is-center,
  .app-entity-grid__cell.is-end {
    display: grid;
    justify-content: stretch;
    text-align: left;
  }
}

@media (max-width: 680px) {
  .app-entity-grid__row {
    grid-template-columns: minmax(0, 1fr) !important;
  }
}
</style>