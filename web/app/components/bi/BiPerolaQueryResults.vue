<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { AlertCircle, ChevronLeft, ChevronRight, Clock3, ShieldAlert } from 'lucide-vue-next'

import AppEntityGrid from '~/components/ui/AppEntityGrid.vue'
import { perolaQueryResultColumns } from '~/domain/bi/perola-query'
import type { PerolaDatasetQueryResponse } from '~/domain/bi/perola-query'

const props = withDefaults(
  defineProps<{
    response?: PerolaDatasetQueryResponse | null
    loading?: boolean
    disabled?: boolean
    error?: string
  }>(),
  {
    response: null,
    loading: false,
    disabled: false,
    error: '',
  },
)

const emit = defineEmits<{
  page: [pageNumber: number]
}>()

const pageSearch = ref('')
const columns = computed(() => perolaQueryResultColumns(props.response?.records || []))
const totalPages = computed(() => Math.max(1, props.response?.totalPages || 0))
const isSensitiveDataset = computed(() =>
  ['nota', 'nota-item'].includes(props.response?.datasetId || ''),
)
const filteredRows = computed(() => {
  const term = pageSearch.value.trim().toLocaleLowerCase('pt-BR')
  const records = props.response?.records || []
  if (!term) return records

  return records.filter((record) =>
    Object.values(record)
      .map((value) => String(value ?? ''))
      .join(' ')
      .toLocaleLowerCase('pt-BR')
      .includes(term),
  )
})

function rowKey(row: Record<string, unknown>, index: number) {
  return String(row.id || `${props.response?.datasetId || 'dataset'}-${index}`)
}

watch(
  () => `${props.response?.datasetId || ''}:${props.response?.pageNumber || 0}`,
  () => {
    pageSearch.value = ''
  },
)
</script>

<template>
  <section class="bi-query-results" data-testid="bi-query-results">
    <div v-if="error" class="bi-query-results__error" role="alert">
      <AlertCircle :size="18" aria-hidden="true" />
      <div>
        <strong>Consulta não concluída</strong>
        <span>{{ error }}</span>
      </div>
    </div>

    <template v-if="response">
      <header class="bi-query-results__summary">
        <div>
          <span>Resultado paginado</span>
          <h4>{{ response.datasetLabel }}</h4>
        </div>
        <div class="bi-query-results__metrics">
          <span>
            <strong>{{ response.returned.toLocaleString('pt-BR') }}</strong>
            nesta página
          </span>
          <span>
            <strong>{{ response.totalRecords.toLocaleString('pt-BR') }}</strong>
            no total
          </span>
          <span>
            <Clock3 :size="14" aria-hidden="true" />
            <strong>{{ response.durationMs }}</strong>
            ms
          </span>
        </div>
      </header>

      <div v-if="isSensitiveDataset" class="bi-query-results__sensitive">
        <ShieldAlert :size="16" aria-hidden="true" />
        Esta entidade pode conter dados pessoais e fiscais. Use somente para a finalidade
        autorizada.
      </div>

      <AppEntityGrid
        :columns="columns"
        :rows="filteredRows"
        :row-key="rowKey"
        :search-value="pageSearch"
        :loading="loading"
        search-placeholder="Filtrar somente os registros desta página..."
        empty-title="Nenhum registro retornado"
        empty-text="A API não retornou registros para os filtros e a página informados."
        :storage-key="`bi-query-columns-${response.datasetId}`"
        testid="bi-query-result-grid"
        @update:search-value="pageSearch = $event"
      />

      <footer class="bi-query-results__pagination">
        <span>
          Página
          <strong>{{ response.pageNumber }}</strong>
          de
          <strong>{{ totalPages }}</strong>
          · limite {{ response.limit }} · {{ response.filterCount }} filtro(s)
        </span>
        <div>
          <button
            type="button"
            :disabled="loading || disabled || response.pageNumber <= 1"
            @click="emit('page', response.pageNumber - 1)"
          >
            <ChevronLeft :size="16" aria-hidden="true" />
            Anterior
          </button>
          <button
            type="button"
            :disabled="loading || disabled || !response.hasMore"
            @click="emit('page', response.pageNumber + 1)"
          >
            Próxima
            <ChevronRight :size="16" aria-hidden="true" />
          </button>
        </div>
      </footer>
    </template>

    <div v-else-if="loading" class="bi-query-results__loading" aria-live="polite">
      Consultando uma página na Pérola BI...
    </div>

    <div v-else-if="!error" class="bi-query-results__empty">
      <strong>Nenhuma consulta executada</strong>
      <span>Escolha os filtros e use “Consultar” para carregar somente uma página.</span>
    </div>
  </section>
</template>

<style scoped>
.bi-query-results {
  display: grid;
  gap: 0.85rem;
}

.bi-query-results__summary,
.bi-query-results__metrics,
.bi-query-results__pagination,
.bi-query-results__pagination > div,
.bi-query-results__error,
.bi-query-results__sensitive,
.bi-query-results__pagination button {
  display: flex;
  align-items: center;
  gap: 0.55rem;
}

.bi-query-results__summary,
.bi-query-results__pagination {
  justify-content: space-between;
  flex-wrap: wrap;
}

.bi-query-results__summary > div:first-child {
  display: grid;
  gap: 0.15rem;
}

.bi-query-results__summary span,
.bi-query-results__pagination,
.bi-query-results__empty span,
.bi-query-results__error span {
  color: var(--text-muted);
  font-size: 0.78rem;
}

.bi-query-results__summary h4 {
  margin: 0;
  color: var(--text-main);
  font-size: 1rem;
}

.bi-query-results__metrics {
  flex-wrap: wrap;
}

.bi-query-results__metrics > span {
  padding: 0.38rem 0.58rem;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
}

.bi-query-results__metrics strong,
.bi-query-results__pagination strong {
  color: var(--text-main);
}

.bi-query-results__sensitive,
.bi-query-results__error {
  align-items: flex-start;
  padding: 0.75rem;
  border-radius: var(--radius-md);
}

.bi-query-results__sensitive {
  color: var(--accent-warning);
  border: 1px solid color-mix(in srgb, var(--accent-warning) 28%, var(--line-soft));
  background: color-mix(in srgb, var(--accent-warning) 7%, transparent);
  font-size: 0.78rem;
}

.bi-query-results__error {
  color: var(--accent-danger);
  border: 1px solid color-mix(in srgb, var(--accent-danger) 32%, var(--line-soft));
  background: color-mix(in srgb, var(--accent-danger) 7%, transparent);
}

.bi-query-results__error div,
.bi-query-results__empty {
  display: grid;
  gap: 0.2rem;
}

.bi-query-results__error strong,
.bi-query-results__empty strong {
  color: var(--text-main);
}

.bi-query-results__pagination {
  padding-top: 0.75rem;
  border-top: 1px solid var(--line-soft);
}

.bi-query-results__pagination button {
  justify-content: center;
  min-height: 2.3rem;
  padding: 0 0.8rem;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  background: var(--bg-panel);
  color: var(--text-main);
  font-weight: 750;
  cursor: pointer;
}

.bi-query-results__pagination button:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.bi-query-results__loading,
.bi-query-results__empty {
  justify-items: center;
  padding: 2.5rem 1rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-md);
  color: var(--text-muted);
  text-align: center;
}

@media (max-width: 620px) {
  .bi-query-results__pagination > div,
  .bi-query-results__pagination button {
    width: 100%;
  }
}
</style>
