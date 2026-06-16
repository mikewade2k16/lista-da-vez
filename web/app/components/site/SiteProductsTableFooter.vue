<script setup lang="ts">
// Rodape da tabela de produtos do site: alterna entre "Carregar tudo"
// (filtragem client-side) e "Paginado" (server-side), e mostra a paginacao
// quando paginado. O badge "X / total" fica no cabecalho (OmniCollectionFilters).
const props = defineProps<{
  mode: 'all' | 'paged'
  page: number
  perPage: number
  total: number
  loading?: boolean
}>()

const emit = defineEmits<{
  'update:mode': [value: 'all' | 'paged']
  'update:page': [value: number]
}>()

const isPaged = computed(() => props.mode === 'paged')

function onToggle(next: boolean) {
  emit('update:mode', next ? 'paged' : 'all')
}

const pageModel = computed({
  get: () => props.page,
  set: (value: number) => emit('update:page', value),
})

const rangeLabel = computed(() => {
  if (!isPaged.value || props.total === 0) return ''
  const start = (props.page - 1) * props.perPage + 1
  const end = Math.min(props.page * props.perPage, props.total)
  return `${start}-${end} de ${props.total}`
})
</script>

<template>
  <div class="site-products-footer">
    <div class="site-products-footer__mode">
      <USwitch
        :model-value="isPaged"
        :disabled="props.loading"
        label="Paginado"
        @update:model-value="onToggle"
      />
      <span class="site-products-footer__hint">
        {{ isPaged ? 'Paginacao no servidor' : 'Carregando tudo (filtro local)' }}
      </span>
    </div>

    <div v-if="isPaged" class="site-products-footer__pagination">
      <span v-if="rangeLabel" class="site-products-footer__range">{{ rangeLabel }}</span>
      <UPagination
        v-model:page="pageModel"
        :total="props.total"
        :items-per-page="props.perPage"
        :sibling-count="1"
        show-edges
        size="sm"
      />
    </div>
  </div>
</template>

<style scoped>
.site-products-footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.5rem 0.25rem;
}

.site-products-footer__mode {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.site-products-footer__hint {
  font-size: 0.75rem;
  color: var(--text-muted);
}

.site-products-footer__pagination {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.site-products-footer__range {
  font-size: 0.75rem;
  color: var(--text-muted);
}
</style>
