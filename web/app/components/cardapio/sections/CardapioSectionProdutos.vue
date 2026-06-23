<script setup lang="ts">
import { computed, ref } from 'vue'

import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniDataTable from '../../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import CardapioProductModal from '~/components/cardapio/product/CardapioProductModal.vue'
import {
  useCardapioProductColumns,
  type CardapioProductFilters,
  type CardapioProductRow,
} from '~/composables/useCardapioProductColumns'
import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import type { OmniTableCellUpdate, OmniTableImageUpload } from '~/types/omni/collection'

const store = useCardapioStore()
const ui = useUiStore()

const modalOpen = ref(false)
const editingId = ref('')
const busyId = ref('')
const selectedIds = ref<Array<string | number>>([])

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
