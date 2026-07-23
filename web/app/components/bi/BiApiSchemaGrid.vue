<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Columns3, RotateCcw } from 'lucide-vue-next'

import AppEntityGrid from '~/components/ui/AppEntityGrid.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { BI_API_ENTITIES, biApiSchemaRows, filterBiApiSchemaRows } from '~/domain/bi/api-catalog'

const ALL_FILTER = 'all'
const allRows = biApiSchemaRows()

const search = ref('')
const entityFilter = ref(ALL_FILTER)
const groupFilter = ref(ALL_FILTER)
const typeFilter = ref(ALL_FILTER)
const pageSize = ref('25')
const page = ref(1)

const columns = [
  {
    id: 'entity',
    label: 'Entidade',
    width: 'minmax(150px, 0.8fr)',
    locked: true,
    defaultVisible: true,
  },
  {
    id: 'group',
    label: 'Grupo',
    width: 'minmax(180px, 0.9fr)',
    defaultVisible: true,
  },
  {
    id: 'field',
    label: 'Coluna da API',
    width: 'minmax(210px, 1fr)',
    defaultVisible: true,
  },
  {
    id: 'fieldLabel',
    label: 'Nome legível',
    width: 'minmax(210px, 1fr)',
    defaultVisible: true,
  },
  {
    id: 'typeLabel',
    label: 'Tipo observado',
    width: 'minmax(180px, 0.8fr)',
    defaultVisible: true,
  },
  {
    id: 'endpoint',
    label: 'Endpoint',
    width: 'minmax(210px, 0.9fr)',
    defaultVisible: true,
  },
]

const entityOptions = [
  { value: ALL_FILTER, label: 'Todas as entidades' },
  ...BI_API_ENTITIES.map((entity) => ({
    value: entity.id,
    label: `${entity.label} · ${biApiSchemaRows([entity]).length} colunas`,
  })),
]

const typeOptions: Array<{ value: string; label: string }> = [
  { value: ALL_FILTER, label: 'Todos os tipos' },
  { value: 'string', label: 'Texto' },
  { value: 'number', label: 'Número' },
  { value: 'boolean', label: 'Sim/não' },
  { value: 'null-observed', label: 'Opcional · tipo a confirmar' },
]

const pageSizeOptions = [
  { value: '25', label: '25 por página' },
  { value: '50', label: '50 por página' },
  { value: '100', label: '100 por página' },
]

const availableGroups = computed(() => {
  const entities =
    entityFilter.value === ALL_FILTER
      ? BI_API_ENTITIES
      : BI_API_ENTITIES.filter((entity) => entity.id === entityFilter.value)
  const groups = new Map<string, string>()
  for (const entity of entities) {
    for (const fieldGroup of entity.fieldGroups) {
      groups.set(fieldGroup.id, fieldGroup.label)
    }
  }
  return [...groups.entries()]
    .sort((left, right) => left[1].localeCompare(right[1], 'pt-BR'))
    .map(([value, label]) => ({ value, label }))
})

const groupOptions = computed(() => [
  { value: ALL_FILTER, label: 'Todos os grupos' },
  ...availableGroups.value,
])

const filteredRows = computed(() =>
  filterBiApiSchemaRows(allRows, {
    search: search.value,
    entityId: entityFilter.value,
    groupId: groupFilter.value,
    type: typeFilter.value,
  }),
)

const resolvedPageSize = computed(() => Number(pageSize.value) || 25)
const totalPages = computed(() =>
  Math.max(1, Math.ceil(filteredRows.value.length / resolvedPageSize.value)),
)
const visibleRows = computed(() => {
  const start = (page.value - 1) * resolvedPageSize.value
  return filteredRows.value.slice(start, start + resolvedPageSize.value)
})
const visibleRange = computed(() => {
  if (!filteredRows.value.length) return '0'
  const start = (page.value - 1) * resolvedPageSize.value + 1
  const end = Math.min(page.value * resolvedPageSize.value, filteredRows.value.length)
  return `${start}–${end}`
})
const hasFilters = computed(
  () =>
    Boolean(search.value) ||
    entityFilter.value !== ALL_FILTER ||
    groupFilter.value !== ALL_FILTER ||
    typeFilter.value !== ALL_FILTER,
)

function typeTone(value: unknown) {
  const label = String(value || '')
  if (label === 'Número') return 'number'
  if (label === 'Sim/não') return 'boolean'
  if (label.startsWith('Opcional')) return 'optional'
  return 'text'
}

function clearFilters() {
  search.value = ''
  entityFilter.value = ALL_FILTER
  groupFilter.value = ALL_FILTER
  typeFilter.value = ALL_FILTER
  page.value = 1
}

watch([search, entityFilter, groupFilter, typeFilter, pageSize], () => {
  page.value = 1
})

watch(entityFilter, () => {
  if (!groupOptions.value.some((option) => option.value === groupFilter.value)) {
    groupFilter.value = ALL_FILTER
  }
})

watch(totalPages, (nextTotal) => {
  if (page.value > nextTotal) page.value = nextTotal
})
</script>

<template>
  <section class="bi-schema" data-testid="bi-api-schema-grid">
    <header class="bi-schema__header">
      <div>
        <span>
          <Columns3 :size="15" aria-hidden="true" />
          Estrutura completa
        </span>
        <h4>Todas as colunas das seis entidades</h4>
        <p>
          {{ filteredRows.length }} de {{ allRows.length }} colunas observadas. Esta tabela é local
          e não chama a API.
        </p>
      </div>
    </header>

    <AppEntityGrid
      v-model:search-value="search"
      :columns="columns"
      :rows="visibleRows"
      row-key="id"
      search-placeholder="Pesquisar entidade, grupo, coluna ou endpoint..."
      storage-key="bi-api-schema-columns"
      columns-label="Colunas visíveis"
      empty-title="Nenhuma coluna encontrada"
      empty-text="Ajuste os filtros para voltar a exibir o contrato observado."
      testid="bi-api-schema-table"
    >
      <template #toolbar-filters>
        <div class="bi-schema__filters">
          <AppSelectField v-model="entityFilter" label="Entidade" :options="entityOptions" />
          <AppSelectField v-model="groupFilter" label="Grupo" :options="groupOptions" />
          <AppSelectField v-model="typeFilter" label="Tipo" :options="typeOptions" />
          <AppSelectField v-model="pageSize" label="Linhas" :options="pageSizeOptions" />
        </div>
      </template>

      <template #toolbar-actions>
        <button
          class="bi-schema__clear"
          type="button"
          :disabled="!hasFilters"
          @click="clearFilters"
        >
          <RotateCcw :size="14" aria-hidden="true" />
          Limpar filtros
        </button>
      </template>

      <template #cell-entity="{ value }">
        <strong class="bi-schema__entity">{{ value }}</strong>
      </template>

      <template #cell-field="{ value }">
        <code class="bi-schema__field">{{ value }}</code>
      </template>

      <template #cell-typeLabel="{ value }">
        <span class="bi-schema__type" :data-tone="typeTone(value)">
          {{ value }}
        </span>
      </template>

      <template #cell-endpoint="{ value }">
        <code class="bi-schema__endpoint">POST {{ value }}</code>
      </template>
    </AppEntityGrid>

    <footer class="bi-schema__pagination">
      <span>
        Exibindo {{ visibleRange }} de {{ filteredRows.length }} · página {{ page }} de
        {{ totalPages }}
      </span>
      <div>
        <button type="button" :disabled="page <= 1" @click="page -= 1">Anterior</button>
        <button type="button" :disabled="page >= totalPages" @click="page += 1">Próxima</button>
      </div>
    </footer>
  </section>
</template>

<style scoped>
.bi-schema {
  display: grid;
  gap: 0.75rem;
}

.bi-schema__header span,
.bi-schema__clear,
.bi-schema__pagination,
.bi-schema__pagination div {
  display: flex;
  align-items: center;
  gap: 0.45rem;
}

.bi-schema__header span {
  color: var(--accent-info);
  font-size: 0.72rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.bi-schema__header h4 {
  margin: 0.35rem 0 0;
  color: var(--text-main);
  font-size: 1rem;
}

.bi-schema__header p {
  margin: 0.25rem 0 0;
  color: var(--text-muted);
  font-size: 0.8rem;
}

.bi-schema__filters {
  display: grid;
  grid-template-columns: repeat(4, minmax(170px, 1fr));
  gap: 0.6rem;
}

.bi-schema__clear,
.bi-schema__pagination button {
  min-height: 2.35rem;
  padding: 0 0.8rem;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  color: var(--text-main);
  background: var(--bg-panel);
  font-weight: 750;
  cursor: pointer;
}

.bi-schema__clear:disabled,
.bi-schema__pagination button:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.bi-schema__entity {
  color: var(--text-main);
}

.bi-schema__field,
.bi-schema__endpoint {
  color: var(--accent-info);
  font-size: 0.75rem;
}

.bi-schema__type {
  display: inline-flex;
  padding: 0.18rem 0.48rem;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  color: var(--text-muted);
  font-size: 0.7rem;
}

.bi-schema__type[data-tone='number'] {
  color: var(--accent-info);
}

.bi-schema__type[data-tone='boolean'] {
  color: var(--accent-success);
}

.bi-schema__type[data-tone='optional'] {
  color: var(--accent-warning);
}

.bi-schema__pagination {
  justify-content: space-between;
  color: var(--text-muted);
  font-size: 0.75rem;
}

@media (max-width: 900px) {
  .bi-schema__filters {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 620px) {
  .bi-schema__filters {
    grid-template-columns: 1fr;
  }

  .bi-schema__pagination {
    align-items: stretch;
    flex-direction: column;
  }

  .bi-schema__pagination div,
  .bi-schema__pagination button {
    width: 100%;
  }
}
</style>
