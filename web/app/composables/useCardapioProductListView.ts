import { computed, ref, watch, type Ref } from 'vue'

import type { CardapioProductFilters } from '~/domain/cardapio/product-filters'
import type { OmniFilterDefinition } from '~/types/omni/collection'

// Estado de exibicao da aba Produtos: barra de filtros enxuta, atalhos de foto e
// renderizacao em blocos. Nao filtra nada sozinho: usa o mesmo filtersState e as
// mesmas linhas filtradas de useCardapioProductColumns.
export function useCardapioProductListView<Row extends { imageUrl: string }>(
  filtersState: Ref<CardapioProductFilters & Record<string, unknown>>,
  filterDefinitions: Ref<OmniFilterDefinition[]>,
  tableRows: Ref<Row[]>,
  filteredRows: Ref<Row[]>,
) {
  // Barra enxuta: busca + categoria sempre visiveis; foto vira os atalhos acima da
  // tabela; disponibilidade, data e tags ficam em "Mais filtros" (continuam ativos
  // quando recolhidos e o botao mostra quantos estao aplicados).
  const PRIMARY_FILTERS = new Set(['search', 'categoryId'])
  const showMoreFilters = ref(false)
  const toolbarFilters = computed(() =>
    filterDefinitions.value.filter(
      (filter) =>
        filter.key !== 'image' && (showMoreFilters.value || PRIMARY_FILTERS.has(filter.key)),
    ),
  )
  const hiddenActiveFilters = computed(() => {
    const state = filtersState.value
    let count = 0
    if (typeof state.availability === 'boolean') count++
    if (state.createdPeriod) count++
    if (Array.isArray(state.tags) && state.tags.length) count++
    return count
  })

  // Atalhos de foto (mesmo filtro `image` do dominio).
  const noImageCount = computed(() => tableRows.value.filter((row) => !row.imageUrl?.trim()).length)
  const photoChips = computed(() => [
    { value: '', label: 'Todos', count: tableRows.value.length },
    { value: 'without', label: 'Sem foto', count: noImageCount.value },
    { value: 'with', label: 'Com foto', count: tableRows.value.length - noImageCount.value },
  ])
  function setPhotoFilter(value: string) {
    filtersState.value = { ...filtersState.value, image: value }
  }

  // Renderiza a lista em blocos: centenas de linhas editaveis de uma vez travam a
  // abertura da aba. Filtros voltam ao primeiro bloco.
  const PAGE_SIZE = 40
  const visibleCount = ref(PAGE_SIZE)
  watch(
    () => JSON.stringify(filtersState.value),
    () => {
      visibleCount.value = PAGE_SIZE
    },
  )
  const pagedRows = computed(() => filteredRows.value.slice(0, visibleCount.value))
  const remainingRows = computed(() => Math.max(0, filteredRows.value.length - visibleCount.value))
  function showMore() {
    visibleCount.value += PAGE_SIZE * 2
  }

  return {
    showMoreFilters,
    toolbarFilters,
    hiddenActiveFilters,
    photoChips,
    setPhotoFilter,
    PAGE_SIZE,
    pagedRows,
    remainingRows,
    showMore,
  }
}
