import { computed, ref, type Ref } from 'vue'

import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import type { ProductBulkAction } from '~/domain/cardapio/types'

// Acoes em lote da aba Produtos. A selecao vale so para os produtos visiveis
// pelos filtros atuais (filteredRows); o back revalida conta e restaurante.
export function useCardapioProductBulk(
  selectedIds: Ref<Array<string | number>>,
  filteredRows: Ref<Array<{ id: string }>>,
) {
  const store = useCardapioStore()
  const ui = useUiStore()

  const bulkBusy = ref(false)
  const bulkEditor = ref<'add_tag' | 'set_category' | ''>('')
  const bulkTag = ref<string[]>([])
  const bulkCategory = ref('')

  const selectedProductIds = computed(() =>
    [...new Set(selectedIds.value.map(String))].filter((id) =>
      filteredRows.value.some((product) => product.id === id),
    ),
  )

  async function runBulkAction(action: ProductBulkAction, value?: string) {
    const ids = selectedProductIds.value
    const restaurantId = store.restaurantId
    if (!ids.length || !restaurantId || bulkBusy.value) return

    if (action === 'delete') {
      const { confirmed } = (await ui.confirm({
        title: 'Excluir produtos selecionados',
        message: `Excluir permanentemente ${ids.length} produto(s)? Esta acao nao pode ser desfeita.`,
        confirmLabel: `Excluir ${ids.length}`,
      })) as { confirmed: boolean }
      if (!confirmed) return
    }

    bulkBusy.value = true
    try {
      const result = await store.bulkProducts(restaurantId, ids, action, value)
      selectedIds.value = []
      bulkEditor.value = ''
      ui.success(
        `${result.affected} produto(s) ${action === 'delete' ? 'excluído(s)' : 'atualizado(s)'}.`,
      )
    } catch (caught) {
      ui.error(getApiErrorMessage(caught, 'Nao foi possivel aplicar a acao em lote.'))
    } finally {
      bulkBusy.value = false
    }
  }

  const bulkActionItems = computed(() => [
    [
      {
        label: 'Marcar como disponivel',
        icon: 'i-lucide-circle-check',
        onSelect: () => runBulkAction('enable'),
      },
      {
        label: 'Marcar como indisponivel',
        icon: 'i-lucide-circle-off',
        onSelect: () => runBulkAction('disable'),
      },
      {
        label: 'Adicionar destaque',
        icon: 'i-lucide-star',
        onSelect: () => runBulkAction('feature'),
      },
      {
        label: 'Remover destaque',
        icon: 'i-lucide-star-off',
        onSelect: () => runBulkAction('remove_feature'),
      },
    ],
    [
      {
        label: 'Adicionar tag',
        icon: 'i-lucide-tag',
        onSelect: () => {
          bulkTag.value = []
          bulkEditor.value = 'add_tag'
        },
      },
      {
        label: 'Mudar categoria',
        icon: 'i-lucide-folder-input',
        onSelect: () => {
          bulkCategory.value = ''
          bulkEditor.value = 'set_category'
        },
      },
    ],
    [
      {
        label: 'Excluir selecionados',
        icon: 'i-lucide-trash-2',
        color: 'error' as const,
        onSelect: () => runBulkAction('delete'),
      },
    ],
  ])

  return {
    bulkBusy,
    bulkEditor,
    bulkTag,
    bulkCategory,
    selectedProductIds,
    bulkActionItems,
    runBulkAction,
  }
}
