<script setup lang="ts">
import { computed, ref } from 'vue'

import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniDataTable from '../../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import CardapioProductImportModal from '~/components/cardapio/product/CardapioProductImportModal.vue'
import CardapioProductModal from '~/components/cardapio/product/CardapioProductModal.vue'
import {
  useCardapioProductColumns,
  type CardapioProductFilters,
  type CardapioProductRow,
} from '~/composables/useCardapioProductColumns'
import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import {
  serializeProductTransfer,
  type ProductImportInput,
  type ProductImportResult,
} from '~/domain/cardapio/product-transfer'
import type { ProductBulkAction } from '~/domain/cardapio/types'
import type { OmniTableCellUpdate, OmniTableImageUpload } from '~/types/omni/collection'

const store = useCardapioStore()
const ui = useUiStore()

const modalOpen = ref(false)
const editingId = ref('')
const busyId = ref('')
const selectedIds = ref<Array<string | number>>([])
const bulkBusy = ref(false)
const importOpen = ref(false)
const importBusy = ref(false)
const importResult = ref<ProductImportResult | null>(null)
const exportBusy = ref(false)

const filtersState = ref<CardapioProductFilters & Record<string, unknown>>({
  categoryId: '',
  availability: '',
})

const {
  tableRows,
  filteredRows,
  allTableColumns,
  filterDefinitions,
  onCellInput,
  applyImageUpload,
} = useCardapioProductColumns(() => filtersState.value, {
  onSuccess: () => ui.success('Produto atualizado.'),
  onError: (caught) =>
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel atualizar o produto.')),
})

// Sinaliza produtos sem imagem (ex.: import que ainda nao tem foto cadastrada).
const noImageCount = computed(() => tableRows.value.filter((row) => !row.imageUrl).length)
const selectedProductIds = computed(() =>
  selectedIds.value.map((id) => String(id).trim()).filter(Boolean),
)

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
      label: 'Excluir selecionados',
      icon: 'i-lucide-trash-2',
      color: 'error' as const,
      onSelect: () => runBulkAction('delete'),
    },
  ],
])

const exportItems = [
  [
    { label: 'Exportar JSON', icon: 'i-lucide-braces', onSelect: () => exportFile('json') },
    { label: 'Exportar CSV', icon: 'i-lucide-sheet', onSelect: () => exportFile('csv') },
  ],
]

const columnExcludeKeys = ['actions']
const { visibleColumnKeys, lockedColumnKeys, columnOrder, tableColumns, resetToDefaults } =
  useOmniVisibleColumns({
    preferenceKey: 'cardapio.editor.products',
    allColumns: allTableColumns,
    columnExcludeKeys,
  })

function toRow(row: Record<string, unknown>): CardapioProductRow {
  return row as unknown as CardapioProductRow
}

function rowId(row: Record<string, unknown>): string {
  return String(row.id ?? '').trim()
}

function openCreate() {
  editingId.value = ''
  modalOpen.value = true
}

function openEdit(row: Record<string, unknown>) {
  editingId.value = rowId(row)
  modalOpen.value = true
}

// Edicao inline: grava no overlay otimista e agenda o load-full-then-patch (PATCH
// full-replace). Salva no commit da celula (debounce p/ texto/select/money; switch
// `immediate`), nao a cada tecla. Toast vem dos callbacks do composable.
function onCellUpdate(payload: OmniTableCellUpdate) {
  onCellInput(String(payload.rowId).trim(), payload.key, payload.value, payload.immediate)
}

// Upload de imagem inline: sobe pela mesma API de midia e grava a url via PATCH
// full-replace (mecanica encapsulada no composable).
async function onImageUpload(payload: OmniTableImageUpload) {
  const id = String(payload.rowId).trim()
  if (!id) return
  busyId.value = id
  try {
    await applyImageUpload(id, payload.file)
    ui.success('Imagem atualizada.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel enviar a imagem.'))
  } finally {
    busyId.value = ''
  }
}

async function remove(row: Record<string, unknown>) {
  const product = toRow(row)
  const { confirmed } = (await ui.confirm({
    title: 'Remover produto',
    message: `Remover o produto "${product.name}"?`,
    confirmLabel: 'Remover',
  })) as { confirmed: boolean }
  if (!confirmed) return

  busyId.value = product.id
  try {
    await store.deleteProduct(product.id)
    ui.success('Produto removido.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel remover o produto.'))
  } finally {
    busyId.value = ''
  }
}

async function runBulkAction(action: ProductBulkAction) {
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
    const result = await store.bulkProducts(restaurantId, ids, action)
    selectedIds.value = []
    ui.success(`${result.affected} produto(s) atualizado(s).`)
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel aplicar a acao em lote.'))
  } finally {
    bulkBusy.value = false
  }
}

async function exportFile(format: 'json' | 'csv') {
  const restaurantId = store.restaurantId
  if (!restaurantId || exportBusy.value || import.meta.server) return
  exportBusy.value = true
  try {
    const document = await store.exportProducts(restaurantId)
    const content = serializeProductTransfer(document, format)
    const mime = format === 'json' ? 'application/json;charset=utf-8' : 'text/csv;charset=utf-8'
    const prefix = format === 'csv' ? '\uFEFF' : ''
    const blob = new Blob([prefix, content], { type: mime })
    const url = URL.createObjectURL(blob)
    const link = window.document.createElement('a')
    link.href = url
    link.download = `produtos-${store.restaurant?.slug || restaurantId}.${format}`
    link.click()
    URL.revokeObjectURL(url)
    ui.success(`${document.products.length} produto(s) exportado(s).`)
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel exportar os produtos.'))
  } finally {
    exportBusy.value = false
  }
}

function openImport() {
  importResult.value = null
  importOpen.value = true
}

function closeImport() {
  if (importBusy.value) return
  importOpen.value = false
}

async function importProducts(input: ProductImportInput) {
  const restaurantId = store.restaurantId
  if (!restaurantId || importBusy.value) return
  importBusy.value = true
  importResult.value = null
  try {
    const result = await store.importProducts(restaurantId, input)
    importResult.value = result
    selectedIds.value = []
    if (result.failed) {
      ui.error(`Importacao concluida com ${result.failed} produto(s) rejeitado(s).`)
    } else {
      ui.success(`${result.created + result.updated} produto(s) importado(s).`)
    }
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel importar os produtos.'))
  } finally {
    importBusy.value = false
  }
}

function onResetFilters() {
  filtersState.value = { categoryId: '', availability: '' }
}
</script>

<template>
  <section class="cardapio-prod">
    <div class="cardapio-prod__head">
      <p class="cardapio-prod__count">
        {{ tableRows.length }} produto(s)
        <span v-if="noImageCount" class="cardapio-prod__warn">· {{ noImageCount }} sem imagem</span>
      </p>
    </div>

    <OmniCollectionFilters
      v-model="filtersState"
      v-model:visible-columns="visibleColumnKeys"
      v-model:locked-columns="lockedColumnKeys"
      v-model:column-order="columnOrder"
      :filters="filterDefinitions"
      :table-columns="allTableColumns"
      :column-exclude-keys="columnExcludeKeys"
      @reset="onResetFilters"
      @reset-columns="resetToDefaults"
    >
      <template #actions>
        <UBadge v-if="selectedIds.length" color="neutral" variant="soft">
          {{ selectedIds.length }} selecionado(s)
        </UBadge>
        <UDropdownMenu :items="bulkActionItems">
          <UButton
            icon="i-lucide-list-checks"
            label="Acoes"
            color="neutral"
            variant="soft"
            :loading="bulkBusy"
            :disabled="!selectedIds.length || bulkBusy"
          />
        </UDropdownMenu>
        <UButton
          icon="i-lucide-upload"
          label="Importar"
          color="neutral"
          variant="soft"
          @click="openImport"
        />
        <UDropdownMenu :items="exportItems">
          <UButton
            icon="i-lucide-download"
            label="Exportar"
            color="neutral"
            variant="soft"
            :loading="exportBusy"
            :disabled="exportBusy || !tableRows.length"
          />
        </UDropdownMenu>
        <UButton icon="i-lucide-plus" label="Novo produto" color="primary" @click="openCreate" />
      </template>
    </OmniCollectionFilters>

    <OmniDataTable
      v-model="selectedIds"
      :rows="filteredRows"
      :columns="tableColumns"
      row-key="id"
      empty-text="Nenhum produto encontrado com os filtros atuais."
      @update:cell="onCellUpdate"
      @upload:image="onImageUpload"
    >
      <template #cell-actions="{ row }">
        <div class="cardapio-prod__actions">
          <UButton
            icon="i-lucide-pencil"
            color="neutral"
            variant="ghost"
            size="sm"
            title="Editar produto (variacoes, adicionais, galeria)"
            aria-label="Editar"
            @click="openEdit(row)"
          />
          <UButton
            icon="i-lucide-trash-2"
            color="error"
            variant="ghost"
            size="sm"
            title="Remover produto"
            aria-label="Remover"
            :loading="busyId === rowId(row)"
            @click="remove(row)"
          />
        </div>
      </template>
    </OmniDataTable>

    <CardapioProductModal
      :open="modalOpen"
      :product-id="editingId"
      :categories="store.categories"
      @close="modalOpen = false"
      @saved="modalOpen = false"
    />

    <CardapioProductImportModal
      v-if="importOpen"
      :restaurant-id="store.restaurantId"
      :importing="importBusy"
      :result="importResult"
      @close="closeImport"
      @reset="importResult = null"
      @submit="importProducts"
    />
  </section>
</template>

<style scoped>
.cardapio-prod {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
}

.cardapio-prod__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.cardapio-prod__count {
  font-size: 0.86rem;
  color: var(--text-muted);
}

.cardapio-prod__warn {
  color: rgb(var(--danger));
  font-weight: 600;
}

.cardapio-prod__actions {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.25rem;
}
</style>
