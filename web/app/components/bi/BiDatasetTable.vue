<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'

import AppEntityGrid from '~/components/ui/AppEntityGrid.vue'
import type { BiDataTable, BiSource } from '~/stores/bi'

const props = withDefaults(
  defineProps<{
    table: BiDataTable
    source?: BiSource | null
    loading?: boolean
  }>(),
  {
    source: null,
    loading: false,
  },
)

const emit = defineEmits<{
  refresh: []
}>()

const search = ref('')
const page = ref(1)
const pageSize = ref(50)
const filters = reactive<Record<string, string>>({})

const pageSizeOptions = [25, 50, 100, 200]

const rows = computed(() => (Array.isArray(props.table.rows) ? props.table.rows : []))
const tableFilters = computed(() => (Array.isArray(props.table.filters) ? props.table.filters : []))
const storageKey = computed(
  () => `bi-${props.table.key}-columns-v2-${props.table.columns.map((column) => column.id).join('-')}`,
)
const coverageLabel = computed(() => {
  if (props.source?.pending) {
    return 'carregando...'
  }
  if (props.table.total > props.table.fetched) {
    return `${props.table.fetched.toLocaleString('pt-BR')} de ${props.table.total.toLocaleString('pt-BR')} na base`
  }
  return `${filteredRows.value.length.toLocaleString('pt-BR')} registros`
})
const emptyTitle = computed(() =>
  props.source?.pending ? `${props.table.label} em carga` : `Nenhum registro em ${props.table.label}`,
)
const emptyText = computed(() => {
  if (props.source?.pending) {
    return 'Essa leitura e pesada na Perola BI e continua carregando em segundo plano.'
  }
  if (props.source?.error) {
    return props.source.error
  }
  return 'Ajuste os filtros ou atualize a leitura da Perola BI.'
})

const activeFilterCount = computed(
  () =>
    Object.values(filters).filter((value) => String(value || '').trim()).length +
    (search.value.trim() ? 1 : 0),
)

const filteredRows = computed(() => {
  const term = search.value.trim().toLowerCase()

  return rows.value.filter((row) => {
    if (term) {
      const haystack = Object.values(row)
        .map((value) => String(value ?? ''))
        .join(' ')
        .toLowerCase()
      if (!haystack.includes(term)) {
        return false
      }
    }

    return tableFilters.value.every((filter) => {
      const selectedValue = String(filters[filter.key] || '').trim()
      if (!selectedValue) {
        return true
      }
      return String(row[filter.key] ?? '').trim() === selectedValue
    })
  })
})

const totalPages = computed(() =>
  Math.max(1, Math.ceil(filteredRows.value.length / Math.max(1, pageSize.value))),
)

const visibleRows = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return filteredRows.value.slice(start, start + pageSize.value)
})

function rowKey(row: Record<string, unknown>, index: number) {
  return String(row.__rowId || row.id || `${props.table.key}-${index}`)
}

function updateSearch(value: string) {
  search.value = value
  page.value = 1
}

function updateFilter(key: string, event: Event) {
  const target = event.target as HTMLSelectElement | null
  filters[key] = String(target?.value || '')
  page.value = 1
}

function updatePageSize(event: Event) {
  const target = event.target as HTMLSelectElement | null
  const parsed = Number(target?.value || pageSize.value)
  pageSize.value = Number.isFinite(parsed) && parsed > 0 ? parsed : 50
  page.value = 1
}

function clearFilters() {
  search.value = ''
  for (const filter of tableFilters.value) {
    filters[filter.key] = ''
  }
  page.value = 1
}

function previousPage() {
  page.value = Math.max(1, page.value - 1)
}

function nextPage() {
  page.value = Math.min(totalPages.value, page.value + 1)
}

watch(
  () => props.table.key,
  () => {
    clearFilters()
  },
)
</script>

<template>
  <section class="bi-table">
    <header class="bi-table__summary">
      <div>
        <h3>{{ table.label }}</h3>
        <p>{{ table.description }}</p>
      </div>
      <strong>{{ coverageLabel }}</strong>
    </header>

    <AppEntityGrid
      :columns="table.columns"
      :rows="visibleRows"
      :row-key="rowKey"
      :search-value="search"
      :loading="loading"
      :search-placeholder="`Buscar em ${table.label.toLowerCase()}...`"
      :empty-title="emptyTitle"
      :empty-text="emptyText"
      :storage-key="storageKey"
      :testid="`bi-${table.key}-table`"
      @update:search-value="updateSearch"
    >
      <template #toolbar-filters>
        <label v-for="filter in tableFilters" :key="filter.key" class="bi-table__filter">
          <span>{{ filter.label }}</span>
          <select :value="filters[filter.key] || ''" @change="updateFilter(filter.key, $event)">
            <option value="">{{ filter.placeholder || 'Todos' }}</option>
            <option v-for="option in filter.options" :key="option" :value="option">
              {{ option }}
            </option>
          </select>
        </label>
      </template>

      <template #toolbar-actions>
        <div class="bi-table__actions">
          <button
            class="bi-table__button"
            type="button"
            :disabled="activeFilterCount === 0"
            @click="clearFilters"
          >
            Limpar filtros
          </button>
          <button class="bi-table__button bi-table__button--primary" type="button" @click="emit('refresh')">
            Atualizar
          </button>
        </div>
      </template>
    </AppEntityGrid>

    <footer class="bi-table__pagination">
      <span>
        Pagina {{ page }} de {{ totalPages }} - mostrando {{ visibleRows.length }} de
        {{ filteredRows.length.toLocaleString('pt-BR') }} carregados
        <template v-if="table.total > table.fetched">
          - {{ table.total.toLocaleString('pt-BR') }} na base Perola BI
        </template>
      </span>

      <div class="bi-table__pagination-controls">
        <label>
          <span>Por pagina</span>
          <select :value="pageSize" @change="updatePageSize">
            <option v-for="size in pageSizeOptions" :key="size" :value="size">{{ size }}</option>
          </select>
        </label>

        <button type="button" :disabled="page <= 1" @click="previousPage">Anterior</button>
        <button type="button" :disabled="page >= totalPages" @click="nextPage">Proxima</button>
      </div>
    </footer>
  </section>
</template>

<style scoped>
.bi-table {
  display: grid;
  gap: 0.75rem;
}

.bi-table__summary,
.bi-table__pagination,
.bi-table__pagination-controls,
.bi-table__actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.bi-table__summary h3 {
  margin: 0;
  color: var(--text-main);
  font-size: 1rem;
}

.bi-table__summary p {
  margin: 0.25rem 0 0;
  color: var(--text-muted);
  line-height: 1.45;
}

.bi-table__summary strong {
  color: #b9ffd2;
  font-size: 0.9rem;
}

.bi-table__filter {
  display: grid;
  gap: 0.2rem;
  min-width: 10rem;
  color: var(--text-muted);
  font-size: 0.72rem;
}

.bi-table__filter select,
.bi-table__pagination select,
.bi-table__pagination button,
.bi-table__button {
  min-height: 2.25rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.7rem;
  background: rgba(17, 24, 39, 0.92);
  color: var(--text-main);
}

.bi-table__filter select,
.bi-table__pagination select {
  padding: 0 0.65rem;
}

.bi-table__button,
.bi-table__pagination button {
  padding: 0 0.8rem;
  font-weight: 700;
  cursor: pointer;
}

.bi-table__button--primary {
  border-color: rgba(83, 198, 160, 0.34);
  background: linear-gradient(135deg, rgba(13, 102, 87, 0.95), rgba(14, 73, 67, 0.94));
}

.bi-table__button:disabled,
.bi-table__pagination button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.bi-table__pagination {
  color: var(--text-muted);
  font-size: 0.78rem;
}

.bi-table__pagination-controls label {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}

@media (max-width: 760px) {
  .bi-table__filter,
  .bi-table__actions,
  .bi-table__button {
    width: 100%;
  }
}
</style>
