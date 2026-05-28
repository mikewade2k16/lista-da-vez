<script setup lang="ts">
import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import type { ProductFieldKey, ProductItem } from '~/types/products'
import type {
  OmniFilterDefinition,
  OmniFocusCell,
  OmniTableCellUpdate,
  OmniTableColumn,
  OmniTableImageUpload,
} from '~/types/omni/collection'

const {
  products,
  campaignOptions,
  categoryOptions,
  loading,
  creating,
  deletingId,
  errorMessage,
  savingMap,
  fetchProducts,
  updateField,
  uploadImage,
  createProduct,
  deleteProduct,
} = useProductsManager()
const sessionSimulation = useSessionSimulationStore()
const isAdminViewer = computed(() => sessionSimulation.userType === 'admin')

const selectedIds = ref<Array<string | number>>([])
const focusCell = ref<OmniFocusCell | null>(null)
const descriptionModalOpen = ref(false)
const descriptionDraft = ref('')
const descriptionRowId = ref<number | null>(null)
const descriptionProductName = ref('')

const filtersState = ref<Record<string, unknown>>({
  query: '',
  campaignFilter: '',
  categoryFilter: '',
  clientIdFilter: '',
  withDeletedFilter: false,
})

const campaignSelectOptions = computed(() =>
  campaignOptions.value.map((item) => ({ label: item, value: item })),
)
const categorySelectOptions = computed(() =>
  categoryOptions.value.map((item) => ({ label: item, value: item })),
)
const clientSelectOptions = computed(() => {
  const options = new Map<number, { label: string; value: number }>()
  products.value.forEach((item) => {
    const id = Number(item.clientId)
    if (!Number.isFinite(id) || id <= 0 || options.has(id)) {
      return
    }

    const label = String(item.clientName || '').trim() || `Cliente #${id}`
    options.set(id, { label, value: id })
  })

  return [...options.values()].sort((a, b) => a.label.localeCompare(b.label, 'pt-BR'))
})

function normalizeSearch(value: unknown) {
  return String(value ?? '')
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
}

function formatListValue(value: unknown) {
  if (!Array.isArray(value) || value.length === 0) {
    return '-'
  }

  return value
    .map((item) => String(item ?? ''))
    .filter(Boolean)
    .join(', ')
}

function descriptionPreview(value: unknown) {
  const text = String(value ?? '').trim()
  if (!text) {
    return 'Adicionar descricao'
  }
  if (text.length <= 74) {
    return text
  }

  return `${text.slice(0, 74)}...`
}

const filterDefinitions = computed<OmniFilterDefinition[]>(() => [
  {
    key: 'query',
    label: 'Buscar',
    type: 'text',
    placeholder: 'Pesquisar por nome ou codigo',
    mode: 'columns',
    columns: ['name', 'code', 'categoriesText', 'campaignsText', 'description', 'tipo'],
  },
  {
    key: 'campaignFilter',
    label: 'Campanhas',
    type: 'select',
    placeholder: 'Campanhas',
    options: campaignSelectOptions.value,
    customPredicate: (row, filterValue) => {
      const target = normalizeSearch(filterValue)
      const campaigns = Array.isArray(row.campaigns) ? row.campaigns : []
      return campaigns.some((item) => normalizeSearch(item) === target)
    },
  },
  {
    key: 'categoryFilter',
    label: 'Categorias',
    type: 'select',
    placeholder: 'Categorias',
    options: categorySelectOptions.value,
    customPredicate: (row, filterValue) => {
      const target = normalizeSearch(filterValue)
      const categories = Array.isArray(row.categories) ? row.categories : []
      return categories.some((item) => normalizeSearch(item) === target)
    },
  },
  {
    key: 'clientIdFilter',
    label: 'Cliente',
    adminOnly: true,
    type: 'select',
    placeholder: 'Cliente',
    options: clientSelectOptions.value,
    accessor: (row) => row.clientId,
  },
  {
    key: 'withDeletedFilter',
    label: 'Mostrar deletados',
    type: 'switch',
    switchOnValue: true,
    switchOffValue: false,
  },
])

const filterDefinitionsForApply = computed(() =>
  filterDefinitions.value.filter((filter) => filter.key !== 'withDeletedFilter'),
)

const allTableColumns = computed<OmniTableColumn[]>(() => [
  {
    key: 'clientName',
    label: 'Cliente',
    adminOnly: true,
    type: 'text',
    editable: false,
    minWidth: 170,
  },
  { key: 'name', label: 'Nome', type: 'text', editable: true, minWidth: 220, focusOnCreate: true },
  { key: 'code', label: 'Codigo', type: 'text', editable: true, minWidth: 170 },
  {
    key: 'categories',
    label: 'Categorias',
    type: 'multiselect',
    editable: true,
    minWidth: 180,
    maxWidth: 200,
    creatable: true,
    placeholder: 'Selecione ou crie categorias',
    options: categorySelectOptions.value,
    formatter: (value) => formatListValue(value),
  },
  {
    key: 'campaigns',
    label: 'Campanhas',
    type: 'multiselect',
    editable: true,
    minWidth: 180,
    maxWidth: 250,
    creatable: true,
    placeholder: 'Selecione ou crie campanhas',
    options: campaignSelectOptions.value,
    formatter: (value) => formatListValue(value),
  },
  { key: 'image', label: 'Imagem', type: 'image', editable: true, minWidth: 270, align: 'center' },
  {
    key: 'stock',
    label: 'Disponivel',
    type: 'switch',
    editable: true,
    immediate: true,
    switchOnValue: 1,
    switchOffValue: 0,
    minWidth: 90,
    align: 'center',
  },
  {
    key: 'status',
    label: 'Mostrar no Site',
    type: 'switch',
    editable: true,
    immediate: true,
    switchOnValue: 'active',
    switchOffValue: 'desactive',
    minWidth: 90,
    align: 'center',
  },
  { key: 'price', label: 'Preco', type: 'money', editable: true, minWidth: 140 },
  { key: 'fator', label: 'Fator', type: 'number', editable: true, minWidth: 120 },
  { key: 'tipo', label: 'Tipo', type: 'text', editable: true, minWidth: 130 },
  { key: 'description', label: 'Descricao', type: 'custom', editable: false, minWidth: 240 },
  {
    key: 'createdAt',
    label: 'Criado em',
    type: 'text',
    editable: false,
    minWidth: 170,
    formatter: (value) => formatDate(String(value ?? '')),
  },
  {
    key: 'updatedAt',
    label: 'Atualizado em',
    type: 'text',
    editable: false,
    minWidth: 170,
    formatter: (value) => formatDate(String(value ?? '')),
  },
  {
    key: 'deletedAt',
    label: 'Deletado em',
    type: 'text',
    editable: false,
    minWidth: 170,
    formatter: (value) => formatDate(String(value ?? '')),
  },
  { key: 'actions', label: 'Opcoes', type: 'custom', minWidth: 120, align: 'center' },
])

const columnExcludeKeys = ['actions']
const alwaysVisibleColumnKeys = new Set(['actions'])
const defaultVisibleColumnKeys = [
  'clientName',
  'name',
  'code',
  'categories',
  'campaigns',
  'image',
  'stock',
  'status',
]
const { visibleColumnKeys, tableColumns } = useOmniVisibleColumns({
  preferenceKey: 'admin.site.products',
  allColumns: allTableColumns,
  columnExcludeKeys,
  alwaysVisibleColumnKeys,
  defaultVisibleColumnKeys,
})

const filteredRows = computed(() => {
  const showDeleted = Boolean(filtersState.value.withDeletedFilter)
  const sourceRows = showDeleted
    ? products.value
    : products.value.filter((product) => !product.isDeleted)

  return applyOmniFilters(
    sourceRows as unknown as Array<Record<string, unknown>>,
    filtersState.value,
    filterDefinitionsForApply.value,
  )
})

const updatableFields = new Set<ProductFieldKey>([
  'name',
  'code',
  'categories',
  'categoriesText',
  'campaigns',
  'campaignsText',
  'image',
  'price',
  'fator',
  'tipo',
  'stock',
  'status',
  'description',
])

function toProduct(row: Record<string, unknown>) {
  return row as unknown as ProductItem
}

function rowIdValue(row: Record<string, unknown>) {
  const parsed = Number(row.id)
  return Number.isFinite(parsed) ? parsed : 0
}

function formatDate(value: string) {
  const raw = String(value ?? '').trim()
  if (!raw) {
    return '-'
  }

  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) {
    return raw
  }

  return date.toLocaleString('pt-BR')
}

function onCellUpdate(payload: OmniTableCellUpdate) {
  const field = String(payload.key) as ProductFieldKey
  if (!updatableFields.has(field)) {
    return
  }

  const id = Number(payload.rowId)
  if (!Number.isFinite(id) || id <= 0) {
    return
  }

  updateField(id, field, payload.value, { immediate: payload.immediate })
}

async function onImageUpload(payload: OmniTableImageUpload) {
  if (String(payload.key) !== 'image') {
    return
  }

  const id = Number(payload.rowId)
  if (!Number.isFinite(id) || id <= 0) {
    return
  }

  await uploadImage(id, payload.file)
}

function openDescriptionModal(row: Record<string, unknown>) {
  const product = toProduct(row)
  descriptionRowId.value = product.id
  descriptionProductName.value = product.name
  descriptionDraft.value = product.description || ''
  descriptionModalOpen.value = true
}

function onSaveDescription() {
  const id = Number(descriptionRowId.value ?? 0)
  if (!Number.isFinite(id) || id <= 0) {
    return
  }

  updateField(id, 'description', descriptionDraft.value, { immediate: true })
  descriptionModalOpen.value = false
}

async function onCreateProduct() {
  const createdId = await createProduct({ name: 'Novo Produto' })
  if (!createdId) {
    return
  }

  focusCell.value = { rowId: createdId, columnKey: 'name', token: Date.now() }
}

function onResetFilters() {
  filtersState.value = {
    query: '',
    campaignFilter: '',
    categoryFilter: '',
    clientIdFilter: '',
    withDeletedFilter: false,
  }
}

async function onDeleteProduct(id: number) {
  if (import.meta.client) {
    const confirmed = window.confirm('Excluir este produto? O item sera marcado como deletado.')
    if (!confirmed) {
      return
    }
  }

  await deleteProduct(id)
}

onMounted(() => {
  sessionSimulation.initialize()
  void fetchProducts()
})
</script>

<template>
  <section class="space-y-4">
    <AdminPageHeader
      eyebrow="Site"
      title="Produtos"
      description="Gestao de produtos em modo teste usando os componentes genericos de filtros e tabela."
    />

    <OmniCollectionFilters
      v-model="filtersState"
      v-model:visible-columns="visibleColumnKeys"
      :filters="filterDefinitions"
      :table-columns="allTableColumns"
      :viewer-user-type="isAdminViewer ? 'admin' : 'client'"
      :column-exclude-keys="columnExcludeKeys"
      :loading="loading"
      @reset="onResetFilters"
    >
      <template #actions>
        <UBadge color="neutral" variant="soft">Selecionados: {{ selectedIds.length }}</UBadge>
        <UButton
          icon="i-lucide-plus"
          label="Novo produto"
          color="success"
          :loading="creating"
          :disabled="creating"
          @click="onCreateProduct"
        />
      </template>
    </OmniCollectionFilters>

    <UAlert
      v-if="errorMessage"
      color="error"
      variant="soft"
      icon="i-lucide-alert-triangle"
      title="Erro"
      :description="errorMessage"
    />

    <OmniDataTable
      v-model="selectedIds"
      :rows="filteredRows"
      :columns="tableColumns"
      :viewer-user-type="isAdminViewer ? 'admin' : 'client'"
      row-key="id"
      :loading="loading"
      :focus-cell="focusCell"
      empty-text="Nenhum produto encontrado com os filtros atuais."
      @update:cell="onCellUpdate"
      @upload:image="onImageUpload"
    >
      <template #cell-description="{ row }">
        <button
          type="button"
          class="max-w-full text-left text-xs text-[rgb(var(--text))] underline-offset-2 hover:underline"
          :title="toProduct(row).description || 'Adicionar descricao'"
          @click="openDescriptionModal(row)"
        >
          <span class="line-clamp-2 break-words">
            {{ descriptionPreview(toProduct(row).description) }}
          </span>
        </button>
      </template>

      <template #cell-actions="{ row }">
        <div class="flex items-center justify-end gap-1">
          <UButton
            icon="i-lucide-message-square"
            color="neutral"
            variant="ghost"
            size="sm"
            aria-label="Editar descricao"
            @click="openDescriptionModal(row)"
          />

          <UPopover :content="{ align: 'end', side: 'bottom' }">
            <UButton
              icon="i-lucide-info"
              color="neutral"
              variant="ghost"
              size="sm"
              aria-label="Info"
            />
            <template #content>
              <div class="w-[320px] space-y-1 p-3 text-xs">
                <p>
                  <strong>ID:</strong>
                  {{ toProduct(row).id }}
                </p>
                <p>
                  <strong>Cliente:</strong>
                  {{ toProduct(row).clientName || `Cliente #${toProduct(row).clientId}` }}
                </p>
                <p>
                  <strong>Nome:</strong>
                  {{ toProduct(row).name }}
                </p>
                <p>
                  <strong>Codigo:</strong>
                  {{ toProduct(row).code }}
                </p>
                <p>
                  <strong>Status:</strong>
                  {{ toProduct(row).status }}
                </p>
                <p>
                  <strong>Stock:</strong>
                  {{ toProduct(row).stock ? 'Disponivel' : 'Indisponivel' }}
                </p>
                <p>
                  <strong>Categorias:</strong>
                  {{ toProduct(row).categoriesText || '-' }}
                </p>
                <p>
                  <strong>Campanhas:</strong>
                  {{ toProduct(row).campaignsText || '-' }}
                </p>
                <p>
                  <strong>Descricao:</strong>
                  {{ toProduct(row).description || '-' }}
                </p>
                <p>
                  <strong>Criado:</strong>
                  {{ formatDate(toProduct(row).createdAt) }}
                </p>
                <p>
                  <strong>Atualizado:</strong>
                  {{ formatDate(toProduct(row).updatedAt) }}
                </p>
                <p>
                  <strong>Deletado:</strong>
                  {{ formatDate(toProduct(row).deletedAt) }}
                </p>
              </div>
            </template>
          </UPopover>

          <UButton
            icon="i-lucide-trash-2"
            color="error"
            variant="ghost"
            size="sm"
            aria-label="Excluir"
            :disabled="toProduct(row).isDeleted"
            :loading="
              deletingId === rowIdValue(row) ||
              Boolean(savingMap[`${rowIdValue(row)}:delete`]) ||
              Boolean(savingMap[`${rowIdValue(row)}:image`])
            "
            @click="onDeleteProduct(rowIdValue(row))"
          />
        </div>
      </template>
    </OmniDataTable>

    <UModal
      v-model:open="descriptionModalOpen"
      title="Descricao do produto"
      :description="descriptionProductName || 'Editar descricao'"
      :ui="{ content: 'max-w-xl' }"
    >
      <template #body>
        <UTextarea
          v-model="descriptionDraft"
          :rows="7"
          class="w-full"
          placeholder="Digite a descricao..."
        />
      </template>

      <template #footer>
        <div class="flex w-full items-center justify-end gap-2">
          <UButton
            label="Cancelar"
            color="neutral"
            variant="ghost"
            @click="descriptionModalOpen = false"
          />
          <UButton
            label="Salvar descricao"
            color="primary"
            :loading="Boolean(descriptionRowId && savingMap[`${descriptionRowId}:description`])"
            @click="onSaveDescription"
          />
        </div>
      </template>
    </UModal>
  </section>
</template>
