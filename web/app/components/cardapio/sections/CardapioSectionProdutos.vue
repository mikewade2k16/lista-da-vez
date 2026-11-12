<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniTagsInput from '~/components/omni/OmniTagsInput.vue'
import OmniDataTable from '../../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import CardapioProductImportModal from '~/components/cardapio/product/CardapioProductImportModal.vue'
import CardapioProductDateRange from '~/components/cardapio/product/CardapioProductDateRange.vue'
import CardapioProductModal from '~/components/cardapio/product/CardapioProductModal.vue'
import CardapioProductImageCell from '~/components/cardapio/product/CardapioProductImageCell.vue'
import CardapioProductBulkEditor from '~/components/cardapio/product/CardapioProductBulkEditor.vue'
import CardapioProductMobileList from '~/components/cardapio/product/CardapioProductMobileList.vue'
import CardapioProductMobileToolbar from '~/components/cardapio/product/CardapioProductMobileToolbar.vue'
import CardapioProductPhotoViewer from '~/components/cardapio/product/CardapioProductPhotoViewer.vue'
import { useMediaMatch } from '~/composables/useMediaMatch'
import {
  useCardapioProductColumns,
  type CardapioProductFilters,
  type CardapioProductRow,
} from '~/composables/useCardapioProductColumns'
import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import { useCardapioProductTransfer } from '~/composables/useCardapioProductTransfer'
import { useCardapioProductBulk } from '~/composables/useCardapioProductBulk'
import { useCardapioProductListView } from '~/composables/useCardapioProductListView'
import type { OmniTableCellUpdate } from '~/types/omni/collection'

const store = useCardapioStore()
const ui = useUiStore()

const modalOpen = ref(false)
const editingId = ref('')
const busyId = ref('')
const uploadingIds = ref<string[]>([])
const selectedIds = ref<Array<string | number>>([])

const filtersState = ref<CardapioProductFilters & Record<string, unknown>>({
  search: '',
  categoryId: '',
  availability: '',
  image: '',
  tags: [],
})

const {
  tableRows,
  filteredRows,
  allTableColumns,
  filterDefinitions,
  tagOptions,
  onCellInput,
  applyImageUpload,
  applyImageRemoval,
} = useCardapioProductColumns(() => filtersState.value, {
  onSuccess: () => ui.success('Produto atualizado.'),
  onError: (caught) =>
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel atualizar o produto.')),
})

// Ao mudar os filtros, nenhuma selecao oculta participa das acoes em lote.
watch(filteredRows, (rows) => {
  const visible = new Set(rows.map((row) => row.id))
  selectedIds.value = selectedIds.value.filter((id) => visible.has(String(id)))
})

const {
  bulkBusy,
  bulkEditor,
  bulkTag,
  bulkCategory,
  selectedProductIds,
  bulkActionItems,
  runBulkAction,
} = useCardapioProductBulk(selectedIds, filteredRows)

const {
  importOpen,
  importBusy,
  importResult,
  exportBusy,
  settingsBusy,
  moreItems,
  closeImport,
  importProducts,
} = useCardapioProductTransfer(
  () => tableRows.value.length,
  () => {
    selectedIds.value = []
  },
)

const {
  showMoreFilters,
  toolbarFilters,
  hiddenActiveFilters,
  photoChips,
  setPhotoFilter,
  PAGE_SIZE,
  pagedRows,
  remainingRows,
  showMore,
} = useCardapioProductListView(filtersState, filterDefinitions, tableRows, filteredRows)
const columnExcludeKeys = ['actions']
const { visibleColumnKeys, lockedColumnKeys, columnOrder, tableColumns, resetToDefaults } =
  useOmniVisibleColumns({
    // v2: layout enxuto (foto em destaque). Tags, preco comparativo, destaque e
    // data continuam disponiveis em "Colunas".
    preferenceKey: 'cardapio.editor.products.v2',
    allColumns: allTableColumns,
    columnExcludeKeys,
    defaultVisibleColumnKeys: ['imageSrc', 'name', 'categoryId', 'priceReais', 'isAvailable'],
  })

// Mobile: cards no lugar da tabela. Os campos extras do card seguem uma
// preferencia de colunas propria (foto, nome e preco sempre aparecem).
const isMobile = useMediaMatch('(max-width: 720px)')
const { visibleColumnKeys: mobileFieldKeys } = useOmniVisibleColumns({
  preferenceKey: 'cardapio.editor.products.mobile',
  allColumns: allTableColumns,
  columnExcludeKeys,
  defaultVisibleColumnKeys: ['imageSrc', 'name', 'priceReais'],
})
const mobileExtraFilters = computed(() =>
  filterDefinitions.value.filter((filter) => !['search', 'image'].includes(filter.key)),
)
const mobileActiveFilters = computed(
  () => hiddenActiveFilters.value + (filtersState.value.categoryId ? 1 : 0),
)
const mobileSearch = computed({
  get: () => String(filtersState.value.search ?? ''),
  set: (value: string) => {
    filtersState.value = { ...filtersState.value, search: value }
  },
})

function openEditById(id: string) {
  editingId.value = id
  modalOpen.value = true
}

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
async function onImageUpload(id: string, file: File) {
  if (!id || bulkBusy.value || uploadingIds.value.includes(id)) return
  uploadingIds.value = [...uploadingIds.value, id]
  try {
    await applyImageUpload(id, file)
    ui.success('Imagem atualizada.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel enviar a imagem.'))
  } finally {
    uploadingIds.value = uploadingIds.value.filter((value) => value !== id)
  }
}

// Foto em tela cheia (conferir) + remover foto (o produto deixa de usar a foto;
// o arquivo e o original continuam guardados).
const viewerId = ref('')
const removingPhotoId = ref('')
const viewerRow = computed(() => tableRows.value.find((row) => row.id === viewerId.value))

async function removePhoto(id: string) {
  const product = tableRows.value.find((row) => row.id === id)
  if (!product || removingPhotoId.value) return
  const { confirmed } = (await ui.confirm({
    title: 'Remover foto',
    message: `Remover a foto de "${product.name}"? O produto fica sem foto no site.`,
    confirmLabel: 'Remover foto',
  })) as { confirmed: boolean }
  if (!confirmed) return
  removingPhotoId.value = id
  try {
    await applyImageRemoval(id)
    if (viewerId.value === id) viewerId.value = ''
    ui.success('Foto removida.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel remover a foto.'))
  } finally {
    removingPhotoId.value = ''
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

// "Limpar" so aparece quando ha filtro aplicado.
const hasActiveFilters = computed(() => {
  const state = filtersState.value
  return Boolean(
    String(state.search ?? '').trim() ||
    state.categoryId ||
    state.image ||
    typeof state.availability === 'boolean' ||
    state.createdPeriod ||
    (Array.isArray(state.tags) && state.tags.length),
  )
})

function onResetFilters() {
  filtersState.value = { search: '', categoryId: '', availability: '', image: '', tags: [] }
}
</script>

<template>
  <section class="cardapio-prod" :class="{ 'has-more': remainingRows > 0 }">
    <template v-if="isMobile">
      <CardapioProductMobileToolbar
        v-model:search="mobileSearch"
        v-model:filters-open="showMoreFilters"
        v-model:visible-columns="mobileFieldKeys"
        :photo-chips="photoChips"
        :photo-value="String(filtersState.image || '')"
        :active-filter-count="mobileActiveFilters"
        :columns="allTableColumns"
        :exclude-keys="columnExcludeKeys"
        :more-items="moreItems"
        :bulk-items="bulkActionItems"
        :selected-count="selectedProductIds.length"
        :busy="exportBusy || settingsBusy || bulkBusy"
        @create="openCreate"
        @photo="setPhotoFilter"
      />
      <OmniCollectionFilters
        v-if="showMoreFilters"
        v-model="filtersState"
        :filters="mobileExtraFilters"
        :show-column-filter="false"
        :show-reset="hasActiveFilters"
        @reset="onResetFilters"
      >
        <template #below>
          <CardapioProductDateRange
            v-if="filtersState.createdPeriod === 'custom'"
            v-model="filtersState"
          />
        </template>
      </OmniCollectionFilters>
      <CardapioProductMobileList
        v-model:selected="selectedIds"
        :rows="pagedRows"
        :visible-keys="mobileFieldKeys"
        :categories="store.categories"
        :uploading-ids="uploadingIds"
        :busy-id="busyId"
        :disabled="bulkBusy"
        :whole-amounts="store.restaurant?.settings.productPriceDecimals !== true"
        @upload="onImageUpload"
        @invalid="ui.error($event)"
        @view="viewerId = $event"
        @remove-photo="removePhoto"
        @edit="openEditById"
        @remove="remove($event as unknown as Record<string, unknown>)"
        @cell="onCellInput"
      />
    </template>

    <template v-else>
      <OmniCollectionFilters
        v-model="filtersState"
        v-model:visible-columns="visibleColumnKeys"
        v-model:locked-columns="lockedColumnKeys"
        v-model:column-order="columnOrder"
        :filters="toolbarFilters"
        :table-columns="allTableColumns"
        :column-exclude-keys="columnExcludeKeys"
        :show-reset="hasActiveFilters"
        @reset="onResetFilters"
        @reset-columns="resetToDefaults"
      >
        <template #below>
          <CardapioProductDateRange
            v-if="filtersState.createdPeriod === 'custom'"
            v-model="filtersState"
          />
        </template>
        <template #actions>
          <div class="cardapio-prod__photo" role="group" aria-label="Filtrar por foto">
            <button
              v-for="chip in photoChips"
              :key="chip.value"
              type="button"
              :class="{
                'is-active': (filtersState.image || '') === chip.value,
                'is-warn': chip.value === 'without' && chip.count > 0,
              }"
              :aria-pressed="(filtersState.image || '') === chip.value"
              :title="`${chip.label}: ${chip.count} produto(s)`"
              @click="setPhotoFilter(chip.value)"
            >
              {{ chip.label }}
              <strong>{{ chip.count }}</strong>
            </button>
          </div>
          <UButton
            :icon="showMoreFilters ? 'i-lucide-chevron-up' : 'i-lucide-sliders-horizontal'"
            :label="
              showMoreFilters
                ? 'Menos filtros'
                : hiddenActiveFilters
                  ? `Mais filtros (${hiddenActiveFilters})`
                  : 'Mais filtros'
            "
            color="neutral"
            variant="ghost"
            @click="showMoreFilters = !showMoreFilters"
          />
          <UDropdownMenu v-if="selectedIds.length" :items="bulkActionItems">
            <UButton
              icon="i-lucide-list-checks"
              :label="`Acoes (${selectedIds.length})`"
              color="neutral"
              variant="soft"
              :loading="bulkBusy"
              :disabled="!selectedProductIds.length || bulkBusy"
            />
          </UDropdownMenu>
          <div class="cardapio-prod__primary">
            <UDropdownMenu :items="moreItems" :content="{ align: 'end' }">
              <UButton
                icon="i-lucide-ellipsis"
                color="neutral"
                variant="ghost"
                aria-label="Mais opcoes"
                title="Importar, exportar e preferencias"
                :loading="exportBusy || settingsBusy"
              />
            </UDropdownMenu>
            <UButton
              icon="i-lucide-plus"
              label="Novo produto"
              color="primary"
              @click="openCreate"
            />
          </div>
        </template>
      </OmniCollectionFilters>

      <OmniDataTable
        v-model="selectedIds"
        :rows="pagedRows"
        :loading="bulkBusy"
        :columns="tableColumns"
        row-key="id"
        empty-text="Nenhum produto encontrado com os filtros atuais."
        @update:cell="onCellUpdate"
      >
        <template #cell-imageSrc="{ row }">
          <CardapioProductImageCell
            :name="toRow(row).name"
            :src="toRow(row).imageSrc"
            :uploading="uploadingIds.includes(rowId(row))"
            :disabled="bulkBusy"
            @upload="onImageUpload(rowId(row), $event)"
            @invalid="ui.error($event)"
            @view="viewerId = rowId(row)"
            @remove="removePhoto(rowId(row))"
          />
        </template>
        <template #cell-tags="{ row }">
          <div class="cardapio-prod__tags">
            <OmniTagsInput
              :model-value="toRow(row).tags ?? []"
              :options="tagOptions"
              size="sm"
              :disabled="bulkBusy"
              placeholder="Adicionar tags"
              creatable
              @update:model-value="onCellInput(rowId(row), 'tags', $event)"
            />
          </div>
        </template>
        <template #cell-actions="{ row }">
          <div class="cardapio-prod__actions">
            <UButton
              icon="i-lucide-pencil"
              color="neutral"
              variant="ghost"
              size="sm"
              title="Editar produto (descricao, variacoes, adicionais, galeria)"
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
    </template>

    <div v-if="remainingRows" class="cardapio-prod__more">
      <span>Mostrando {{ pagedRows.length }} de {{ filteredRows.length }}</span>
      <UButton
        icon="i-lucide-chevrons-down"
        :label="`Mostrar mais ${Math.min(remainingRows, PAGE_SIZE * 2)}`"
        color="neutral"
        variant="soft"
        @click="showMore"
      />
    </div>

    <CardapioProductPhotoViewer
      v-if="viewerRow && viewerRow.photoSrc"
      :name="viewerRow.name"
      :src="viewerRow.photoSrc"
      :uploading="uploadingIds.includes(viewerRow.id)"
      :removing="removingPhotoId === viewerRow.id"
      @close="viewerId = ''"
      @upload="onImageUpload(viewerRow.id, $event)"
      @invalid="ui.error($event)"
      @remove="removePhoto(viewerRow.id)"
    />

    <CardapioProductBulkEditor
      v-model:editor="bulkEditor"
      v-model:tag="bulkTag"
      v-model:category="bulkCategory"
      :count="selectedProductIds.length"
      :busy="bulkBusy"
      :tag-options="tagOptions"
      :categories="store.categories"
      @apply="runBulkAction"
    />

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
  position: relative;
  display: flex;
  flex-direction: column;
  flex: 1;
  gap: 0;
  min-width: 0;
  min-height: 0;
}

/* Acao principal sempre no fim da linha. */
.cardapio-prod__primary {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  margin-left: auto;
}

.cardapio-prod__actions {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.25rem;
}

:deep(.omni-data-table) {
  /* Cresce ate a altura disponivel do painel (editor) e rola por dentro. */
  flex: 0 1 auto;
  width: 100%;
  min-width: 0;
  max-width: 100%;
  min-height: 0;
  overflow: auto;
}

:deep(.omni-data-table__head-cell),
:deep(.omni-data-table__cell) {
  overflow: hidden;
}
</style>
<style scoped src="./cardapio-produtos.css"></style>
