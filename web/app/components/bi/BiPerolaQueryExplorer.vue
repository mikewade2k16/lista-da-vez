<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { AlertCircle, DatabaseZap, Filter, RefreshCw, Search } from 'lucide-vue-next'

import BiPerolaQueryFilters from '~/components/bi/BiPerolaQueryFilters.vue'
import BiPerolaQueryResults from '~/components/bi/BiPerolaQueryResults.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { buildPerolaQueryFilters, createInitialPerolaFilterDrafts } from '~/domain/bi/perola-query'
import type {
  PerolaDatasetCatalogItem,
  PerolaDatasetQueryInput,
  PerolaFilterDraft,
} from '~/domain/bi/perola-query'
import { useBiStore } from '~/stores/bi'

const biStore = useBiStore()
const {
  datasetCatalog,
  datasetCatalogLoading,
  datasetCatalogError,
  datasetQueryResponse,
  datasetQueryLoading,
  datasetQueryError,
  apiBlocked,
} = storeToRefs(biStore)

const selectedDatasetId = ref('')
const orderField = ref('')
const orderDirection = ref<'ASC' | 'DESC'>('DESC')
const pageLimit = ref('25')
const filterDrafts = ref<PerolaFilterDraft[]>([])
const validationError = ref('')
const lastSubmittedQuery = ref<PerolaDatasetQueryInput | null>(null)
const catalogRequested = ref(datasetCatalog.value.length > 0 || Boolean(datasetCatalogError.value))

const activeDataset = computed(
  () => datasetCatalog.value.find((dataset) => dataset.id === selectedDatasetId.value) || null,
)
const datasetOptions = computed(() =>
  datasetCatalog.value.map((dataset) => ({
    value: dataset.id,
    label: dataset.label,
    meta: dataset.description,
  })),
)
const orderFieldOptions = computed(() =>
  (activeDataset.value?.allowedOrderFields || []).map((field) => ({
    value: field,
    label: field,
  })),
)
const directionOptions = [
  { value: 'DESC', label: 'Mais recentes primeiro' },
  { value: 'ASC', label: 'Mais antigos primeiro' },
]
const pageLimitOptions = computed(() => {
  const dataset = activeDataset.value
  if (!dataset) return []
  const sizes = new Set([10, 15, 25, 50, 100, dataset.defaultLimit, dataset.maxLimit])
  return Array.from(sizes)
    .filter((size) => size > 0 && size <= dataset.maxLimit)
    .sort((left, right) => left - right)
    .map((size) => ({ value: String(size), label: `${size} por página` }))
})

function applyDatasetDefaults(dataset: PerolaDatasetCatalogItem) {
  orderField.value = dataset.defaultOrderBy.field
  orderDirection.value = dataset.defaultOrderBy.direction
  pageLimit.value = String(dataset.defaultLimit)
  filterDrafts.value = createInitialPerolaFilterDrafts(dataset)
  validationError.value = ''
  lastSubmittedQuery.value = null
  biStore.clearPerolaDatasetQuery()
}

async function loadCatalog(force = false) {
  if (apiBlocked.value) return
  catalogRequested.value = true
  const result = await biStore.loadPerolaDatasetCatalog(force)
  if (!result.ok || selectedDatasetId.value || !result.data?.[0]) return
  selectedDatasetId.value = result.data[0].id
}

async function submitQuery() {
  if (apiBlocked.value) return
  const dataset = activeDataset.value
  if (!dataset) return

  const built = buildPerolaQueryFilters(dataset, filterDrafts.value)
  validationError.value = built.error
  if (built.error) return

  const query: PerolaDatasetQueryInput = {
    pageNumber: 1,
    limit: Number(pageLimit.value) || dataset.defaultLimit,
    orderBy: {
      field: orderField.value,
      direction: orderDirection.value,
    },
    filters: built.filters,
  }
  lastSubmittedQuery.value = query
  await biStore.queryPerolaDataset(dataset.id, query)
}

async function changePage(pageNumber: number) {
  if (apiBlocked.value) return
  const dataset = activeDataset.value
  const previous = lastSubmittedQuery.value
  if (!dataset || !previous || pageNumber < 1) return

  const query: PerolaDatasetQueryInput = {
    ...previous,
    pageNumber,
    orderBy: { ...previous.orderBy },
    filters: previous.filters.map((filter) => ({ ...filter })),
  }
  const result = await biStore.queryPerolaDataset(dataset.id, query)
  if (result.ok) lastSubmittedQuery.value = query
}

function resetFilters() {
  if (!activeDataset.value) return
  applyDatasetDefaults(activeDataset.value)
}

watch(selectedDatasetId, () => {
  if (activeDataset.value) applyDatasetDefaults(activeDataset.value)
})

watch(
  datasetCatalog,
  (catalog) => {
    if (!selectedDatasetId.value && catalog[0]) selectedDatasetId.value = catalog[0].id
  },
  { immediate: true },
)
</script>

<template>
  <section class="bi-query" data-testid="bi-query-explorer">
    <header class="bi-query__hero omni-glass">
      <div>
        <span class="bi-query__eyebrow">
          <DatabaseZap :size="15" aria-hidden="true" />
          Consulta real e paginada
        </span>
        <h3>Pesquisar nas seis entidades da Pérola</h3>
        <p>
          Cada ação consulta somente uma página. Os filtros, limites e ordenações são validados pelo
          backend; abrir esta aba não faz nenhuma chamada do BI.
        </p>
      </div>
      <div class="bi-query__guard">
        <Filter :size="18" aria-hidden="true" />
        <span>Catálogo e registros somente após uma ação explícita.</span>
      </div>
    </header>

    <div v-if="apiBlocked" class="bi-query__blocked" role="status">
      <AlertCircle :size="18" aria-hidden="true" />
      <span>
        <strong>Consultas bloqueadas.</strong>
        Desative “Bloquear chamadas” no cabeçalho para liberar ações explícitas.
      </span>
    </div>

    <div
      v-if="!catalogRequested && !activeDataset"
      class="bi-query__catalog-idle"
      data-testid="bi-query-catalog-idle"
    >
      <DatabaseZap :size="20" aria-hidden="true" />
      <div>
        <strong>Nenhum dado solicitado</strong>
        <span>
          O catálogo contém apenas as regras permitidas de consulta. Ele só será carregado quando
          você autorizar.
        </span>
      </div>
      <button
        type="button"
        :disabled="apiBlocked"
        data-testid="bi-query-load-catalog"
        @click="loadCatalog()"
      >
        Carregar catálogo de consultas
      </button>
    </div>

    <div v-else-if="datasetCatalogError" class="bi-query__catalog-error" role="alert">
      <AlertCircle :size="18" aria-hidden="true" />
      <div>
        <strong>Catálogo indisponível</strong>
        <span>{{ datasetCatalogError }}</span>
      </div>
      <button
        type="button"
        :disabled="datasetCatalogLoading || apiBlocked"
        @click="loadCatalog(true)"
      >
        <RefreshCw :size="15" aria-hidden="true" />
        Tentar novamente
      </button>
    </div>

    <div v-else-if="datasetCatalogLoading && !activeDataset" class="bi-query__catalog-loading">
      Carregando regras seguras de consulta...
    </div>

    <template v-else-if="activeDataset">
      <section class="bi-query__form omni-glass">
        <div class="bi-query__selectors">
          <AppSelectField
            v-model="selectedDatasetId"
            label="Entidade"
            :options="datasetOptions"
            searchable
            testid="bi-query-dataset"
          />
          <AppSelectField
            v-model="orderField"
            label="Ordenar por"
            :options="orderFieldOptions"
            :disabled="datasetQueryLoading || apiBlocked"
          />
          <AppSelectField
            v-model="orderDirection"
            label="Direção"
            :options="directionOptions"
            :disabled="datasetQueryLoading || apiBlocked"
          />
          <AppSelectField
            v-model="pageLimit"
            label="Limite"
            :options="pageLimitOptions"
            :disabled="datasetQueryLoading || apiBlocked"
          />
        </div>

        <div class="bi-query__dataset-summary">
          <div>
            <strong>{{ activeDataset.label }}</strong>
            <span>{{ activeDataset.description }}</span>
          </div>
          <span>Máximo de {{ activeDataset.maxLimit }} registros por página</span>
        </div>

        <BiPerolaQueryFilters
          v-model="filterDrafts"
          :dataset="activeDataset"
          :disabled="datasetQueryLoading || apiBlocked"
        />

        <div v-if="validationError" class="bi-query__validation" role="alert">
          <AlertCircle :size="16" aria-hidden="true" />
          {{ validationError }}
        </div>

        <footer class="bi-query__actions">
          <button
            class="bi-query__reset"
            type="button"
            :disabled="datasetQueryLoading || apiBlocked"
            @click="resetFilters"
          >
            Restaurar filtros
          </button>
          <button
            class="bi-query__submit"
            type="button"
            :disabled="datasetQueryLoading || apiBlocked"
            data-testid="bi-query-submit"
            @click="submitQuery"
          >
            <Search :size="16" aria-hidden="true" />
            {{ datasetQueryLoading ? 'Consultando...' : 'Consultar' }}
          </button>
        </footer>
      </section>

      <BiPerolaQueryResults
        :response="datasetQueryResponse"
        :loading="datasetQueryLoading"
        :disabled="apiBlocked"
        :error="datasetQueryError"
        @page="changePage"
      />
    </template>
  </section>
</template>

<style scoped>
.bi-query {
  display: grid;
  gap: 1rem;
}

.bi-query__hero {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(260px, 0.6fr);
  gap: 1rem;
  padding: 1.1rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-lg);
}

.bi-query__eyebrow,
.bi-query__guard,
.bi-query__catalog-idle,
.bi-query__catalog-idle button,
.bi-query__catalog-error,
.bi-query__catalog-error button,
.bi-query__blocked,
.bi-query__validation,
.bi-query__actions,
.bi-query__submit {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.bi-query__eyebrow {
  color: var(--accent-info);
  font-size: 0.75rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.bi-query__hero h3 {
  margin: 0.45rem 0 0;
  color: var(--text-main);
  font-size: clamp(1.1rem, 2vw, 1.45rem);
}

.bi-query__hero p,
.bi-query__guard,
.bi-query__catalog-idle span,
.bi-query__dataset-summary span,
.bi-query__catalog-error span {
  color: var(--text-muted);
  line-height: 1.5;
}

.bi-query__hero p {
  margin: 0.45rem 0 0;
}

.bi-query__guard {
  align-self: stretch;
  padding: 0.9rem;
  color: var(--accent-success);
  border: 1px solid color-mix(in srgb, var(--accent-success) 28%, var(--line-soft));
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--accent-success) 7%, transparent);
  font-size: 0.8rem;
}

.bi-query__form {
  display: grid;
  gap: 1rem;
  padding: 1rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-lg);
}

.bi-query__selectors {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) repeat(3, minmax(160px, 0.55fr));
  gap: 0.7rem;
}

.bi-query__dataset-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.7rem 0;
  border-block: 1px solid var(--line-soft);
}

.bi-query__dataset-summary div {
  display: grid;
  gap: 0.2rem;
}

.bi-query__dataset-summary strong {
  color: var(--text-main);
}

.bi-query__dataset-summary span {
  font-size: 0.78rem;
}

.bi-query__catalog-idle,
.bi-query__catalog-error,
.bi-query__blocked,
.bi-query__validation {
  padding: 0.8rem;
  color: var(--accent-danger);
  border: 1px solid color-mix(in srgb, var(--accent-danger) 30%, var(--line-soft));
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--accent-danger) 7%, transparent);
}

.bi-query__blocked {
  color: var(--accent-warning);
  border-color: color-mix(in srgb, var(--accent-warning) 34%, var(--line-soft));
  background: color-mix(in srgb, var(--accent-warning) 7%, transparent);
}

.bi-query__blocked span {
  color: var(--text-muted);
  line-height: 1.45;
}

.bi-query__blocked strong {
  color: var(--text-main);
}

.bi-query__catalog-idle {
  align-items: flex-start;
  color: var(--accent-info);
  border-style: dashed;
  border-color: color-mix(in srgb, var(--accent-info) 35%, var(--line-soft));
  background: color-mix(in srgb, var(--accent-info) 5%, transparent);
}

.bi-query__catalog-idle div {
  display: grid;
  flex: 1;
  gap: 0.25rem;
}

.bi-query__catalog-idle strong {
  color: var(--text-main);
}

.bi-query__catalog-error {
  align-items: flex-start;
}

.bi-query__catalog-error div {
  display: grid;
  flex: 1;
  gap: 0.2rem;
}

.bi-query__catalog-error strong {
  color: var(--text-main);
}

.bi-query__catalog-error button,
.bi-query__catalog-idle button,
.bi-query__reset,
.bi-query__submit {
  min-height: 2.35rem;
  padding: 0 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  color: var(--text-main);
  font-weight: 750;
  cursor: pointer;
}

.bi-query__catalog-error button,
.bi-query__catalog-idle button,
.bi-query__reset {
  background: var(--bg-panel);
}

.bi-query__actions {
  justify-content: flex-end;
}

.bi-query__submit {
  justify-content: center;
  border-color: color-mix(in srgb, var(--accent-success) 38%, var(--line-soft));
  background: color-mix(in srgb, var(--accent-success) 18%, var(--bg-panel));
}

.bi-query__catalog-error button:disabled,
.bi-query__catalog-idle button:disabled,
.bi-query__reset:disabled,
.bi-query__submit:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.bi-query__catalog-loading {
  padding: 2rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-md);
  color: var(--text-muted);
  text-align: center;
}

@media (max-width: 960px) {
  .bi-query__hero,
  .bi-query__selectors {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 620px) {
  .bi-query__hero,
  .bi-query__selectors {
    grid-template-columns: 1fr;
  }

  .bi-query__dataset-summary,
  .bi-query__catalog-idle,
  .bi-query__catalog-error,
  .bi-query__actions {
    align-items: stretch;
    flex-direction: column;
  }

  .bi-query__reset,
  .bi-query__submit,
  .bi-query__catalog-idle button,
  .bi-query__catalog-error button {
    width: 100%;
  }
}
</style>
