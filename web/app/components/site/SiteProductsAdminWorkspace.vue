<script setup lang="ts">
import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniMinimalPopover from '~/components/omni/overlay/OmniMinimalPopover.vue'
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import WebhookSourcesDrawer from '~/components/site/WebhookSourcesDrawer.vue'
import type { ProductCreateInput, ProductFieldKey, ProductItem } from '~/types/products'
import type {
  OmniFilterDefinition,
  OmniFocusCell,
  OmniTableCellUpdate,
  OmniTableColumn,
} from '~/types/omni/collection'

const {
  products,
  loading,
  creating,
  deletingId,
  errorMessage,
  savingMap,
  fetchProducts,
  updateField,
  createProduct,
  deleteProduct,
} = useProductsManager()

const auth = useAuthStore()
const canCreate = computed(
  () =>
    auth.role === 'platform_admin' ||
    auth.role === 'owner' ||
    auth.role === 'director' ||
    auth.role === 'manager',
)
const canDelete = computed(() => canCreate.value)

const selectedIds = ref<Array<string | number>>([])
const focusCell = ref<OmniFocusCell | null>(null)

const filtersState = ref<Record<string, unknown>>({
  query: '',
  statusFilter: '',
  categoryFilter: '',
})

const filterDefinitions = computed<OmniFilterDefinition[]>(() => [
  {
    key: 'query',
    label: 'Buscar',
    type: 'text',
    placeholder: 'Nome, codigo, descricao...',
    mode: 'all',
  },
  {
    key: 'statusFilter',
    label: 'Status',
    type: 'select',
    placeholder: 'Status',
    options: [
      { label: 'Ativo', value: 'active' },
      { label: 'Inativo', value: 'inactive' },
    ],
    accessor: (row) => row.status,
  },
])

const allTableColumns = computed<OmniTableColumn[]>(() => [
  {
    key: 'name',
    label: 'Nome',
    type: 'text',
    editable: true,
    minWidth: 220,
    focusOnCreate: true,
    locked: true,
    defaultOrder: 10,
  },
  { key: 'code', label: 'Codigo', type: 'text', editable: true, minWidth: 140, defaultOrder: 20 },
  {
    key: 'status',
    label: 'Status',
    type: 'select',
    editable: true,
    immediate: true,
    minWidth: 120,
    defaultOrder: 30,
    options: [
      { label: 'Ativo', value: 'active' },
      { label: 'Inativo', value: 'inactive' },
    ],
  },
  { key: 'price', label: 'Preco', type: 'money', editable: true, minWidth: 140, defaultOrder: 40 },
  { key: 'fator', label: 'Fator', type: 'number', editable: true, minWidth: 100, defaultOrder: 50 },
  {
    key: 'stock',
    label: 'Estoque',
    type: 'number',
    editable: true,
    minWidth: 110,
    defaultOrder: 60,
  },
  { key: 'tipo', label: 'Tipo', type: 'text', editable: true, minWidth: 130, defaultOrder: 70 },
  {
    key: 'sourceLabel',
    label: 'Fonte',
    type: 'text',
    editable: false,
    minWidth: 140,
    defaultOrder: 80,
  },
  {
    key: 'actions',
    label: 'Opcoes',
    type: 'custom',
    minWidth: 150,
    align: 'center',
    defaultOrder: 1000,
  },
])

const columnExcludeKeys = ['actions']
const { visibleColumnKeys, lockedColumnKeys, columnOrder, tableColumns, resetToDefaults } =
  useOmniVisibleColumns({
    preferenceKey: 'admin.site.products',
    allColumns: allTableColumns,
    columnExcludeKeys,
  })

const filteredRows = computed(() => {
  const rows = products.value as unknown as Array<Record<string, unknown>>
  return applyOmniFilters(rows, filtersState.value, filterDefinitions.value)
})

const tableRows = computed(() => {
  const seen = new Set<string>()
  return filteredRows.value.filter((row) => {
    const id = String((row as Record<string, unknown>).id ?? '').trim()
    if (!id || seen.has(id)) return false
    seen.add(id)
    return true
  })
})

const updatableFields = new Set<ProductFieldKey>([
  'name',
  'code',
  'description',
  'image',
  'price',
  'fator',
  'tipo',
  'stock',
  'status',
])

function toProduct(row: Record<string, unknown>) {
  return row as unknown as ProductItem
}

function rowId(row: Record<string, unknown>) {
  return String(row.id ?? '').trim()
}

function onCellUpdate(payload: OmniTableCellUpdate) {
  const id = String(payload.rowId).trim()
  if (!id) return
  const field = String(payload.key) as ProductFieldKey
  if (!updatableFields.has(field)) return
  updateField(id, field, payload.value, { immediate: payload.immediate })
}

const createDialogOpen = ref(false)
const createForm = reactive<ProductCreateInput>({
  name: '',
  code: '',
  description: '',
  image: '',
  categories: [],
  campaigns: [],
  price: 0,
  fator: 1,
  tipo: '',
  stock: 0,
})

function openCreate() {
  createForm.name = ''
  createForm.code = ''
  createForm.description = ''
  createForm.image = ''
  createForm.categories = []
  createForm.campaigns = []
  createForm.price = 0
  createForm.fator = 1
  createForm.tipo = ''
  createForm.stock = 0
  createDialogOpen.value = true
}

async function submitCreate() {
  if (!canCreate.value) return
  const createdId = await createProduct({ ...createForm })
  if (!createdId) return
  createDialogOpen.value = false
  focusCell.value = { rowId: createdId, columnKey: 'name', token: Date.now() }
}

function onResetFilters() {
  filtersState.value = { query: '', statusFilter: '', categoryFilter: '' }
}

async function onDeleteProduct(id: string) {
  if (!canDelete.value) return
  if (import.meta.client && !window.confirm('Excluir este produto?')) return
  await deleteProduct(id)
}

const openInfoFor = ref<string | null>(null)
function infoOpen(id: string) {
  return openInfoFor.value === id
}
function setInfoOpen(id: string, v: boolean) {
  openInfoFor.value = v ? id : null
}

const sourcesDrawerOpen = ref(false)

onMounted(() => {
  void fetchProducts()
})
</script>

<template>
  <section class="site-products-workspace flex h-full min-h-0 flex-col gap-4 overflow-hidden">
    <AdminPageHeader
      eyebrow="Site"
      title="Produtos"
      description="Catalogo de produtos do site (manual ou ingerido por webhook). Ligado a API real /v1/admin/products."
    />

    <OmniCollectionFilters
      v-model="filtersState"
      v-model:visible-columns="visibleColumnKeys"
      v-model:locked-columns="lockedColumnKeys"
      v-model:column-order="columnOrder"
      :viewer-user-type="canCreate ? 'admin' : 'client'"
      :filters="filterDefinitions"
      :table-columns="allTableColumns"
      :column-exclude-keys="columnExcludeKeys"
      :loading="loading"
      @reset="onResetFilters"
      @reset-columns="resetToDefaults"
    >
      <template #actions>
        <UBadge color="neutral" variant="soft">Selecionados: {{ selectedIds.length }}</UBadge>
        <UButton
          v-if="canCreate"
          icon="i-lucide-webhook"
          label="Fontes"
          color="neutral"
          variant="soft"
          @click="sourcesDrawerOpen = true"
        />
        <UButton
          icon="i-lucide-plus"
          label="Novo produto"
          color="primary"
          :loading="creating"
          :disabled="creating || !canCreate"
          @click="openCreate"
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

    <div class="site-products-workspace__scroll flex-1 min-h-0 overflow-y-auto">
      <OmniDataTable
        v-model="selectedIds"
        :rows="tableRows"
        :columns="tableColumns"
        row-key="id"
        :loading="loading"
        :focus-cell="focusCell"
        empty-text="Nenhum produto encontrado."
        @update:cell="onCellUpdate"
      >
        <template #cell-actions="{ row }">
          <div class="flex items-center justify-end gap-1">
            <OmniMinimalPopover
              :open="infoOpen(rowId(row))"
              title="Detalhes do produto"
              width-class="w-[320px] max-w-[90vw]"
              @update:open="setInfoOpen(rowId(row), $event)"
            >
              <template #trigger>
                <UButton
                  icon="i-lucide-info"
                  color="neutral"
                  variant="ghost"
                  size="sm"
                  title="Detalhes"
                  aria-label="Info"
                />
              </template>
              <div class="space-y-1 text-xs">
                <p>
                  <strong>Nome:</strong>
                  {{ toProduct(row).name }}
                </p>
                <p>
                  <strong>Codigo:</strong>
                  {{ toProduct(row).code || '-' }}
                </p>
                <p>
                  <strong>Status:</strong>
                  {{ toProduct(row).status }}
                </p>
                <p>
                  <strong>Preco:</strong>
                  R$ {{ toProduct(row).price.toFixed(2) }}
                </p>
                <p>
                  <strong>Fator:</strong>
                  {{ toProduct(row).fator }}
                </p>
                <p>
                  <strong>Estoque:</strong>
                  {{ toProduct(row).stock }}
                </p>
                <p>
                  <strong>Tipo:</strong>
                  {{ toProduct(row).tipo || '-' }}
                </p>
                <p>
                  <strong>Categorias:</strong>
                  {{ toProduct(row).categories.join(', ') || '-' }}
                </p>
                <p>
                  <strong>Campanhas:</strong>
                  {{ toProduct(row).campaigns.join(', ') || '-' }}
                </p>
                <p>
                  <strong>Fonte:</strong>
                  {{ toProduct(row).sourceLabel || '-' }}
                </p>
                <p v-if="toProduct(row).description">
                  <strong>Descricao:</strong>
                  {{ toProduct(row).description }}
                </p>
              </div>
            </OmniMinimalPopover>

            <UButton
              v-if="canDelete"
              icon="i-lucide-trash-2"
              color="error"
              variant="ghost"
              size="sm"
              title="Excluir produto"
              aria-label="Excluir"
              :loading="deletingId === rowId(row)"
              @click="onDeleteProduct(rowId(row))"
            />
          </div>
        </template>
      </OmniDataTable>
    </div>

    <UModal v-model:open="createDialogOpen">
      <template #content>
        <UCard>
          <template #header>
            <h3 class="text-base font-semibold">Novo produto</h3>
          </template>
          <div class="space-y-3">
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Nome</label>
              <UInput
                :model-value="createForm.name"
                @update:model-value="createForm.name = String($event ?? '')"
              />
            </div>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Codigo</label>
              <UInput
                :model-value="createForm.code"
                @update:model-value="createForm.code = String($event ?? '')"
              />
            </div>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Descricao</label>
              <UInput
                :model-value="createForm.description"
                @update:model-value="createForm.description = String($event ?? '')"
              />
            </div>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Image URL</label>
              <UInput
                :model-value="createForm.image"
                placeholder="https://..."
                @update:model-value="createForm.image = String($event ?? '')"
              />
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-xs text-[rgb(var(--muted))] mb-1">Preco</label>
                <UInput
                  type="number"
                  :model-value="createForm.price"
                  @update:model-value="createForm.price = Number($event ?? 0)"
                />
              </div>
              <div>
                <label class="block text-xs text-[rgb(var(--muted))] mb-1">Fator</label>
                <UInput
                  type="number"
                  :model-value="createForm.fator"
                  @update:model-value="createForm.fator = Number($event ?? 1)"
                />
              </div>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-xs text-[rgb(var(--muted))] mb-1">Estoque</label>
                <UInput
                  type="number"
                  :model-value="createForm.stock"
                  @update:model-value="createForm.stock = Number($event ?? 0)"
                />
              </div>
              <div>
                <label class="block text-xs text-[rgb(var(--muted))] mb-1">Tipo</label>
                <UInput
                  :model-value="createForm.tipo"
                  @update:model-value="createForm.tipo = String($event ?? '')"
                />
              </div>
            </div>
          </div>
          <template #footer>
            <div class="flex justify-end gap-2">
              <UButton
                label="Cancelar"
                color="neutral"
                variant="ghost"
                @click="createDialogOpen = false"
              />
              <UButton label="Criar" color="primary" :loading="creating" @click="submitCreate" />
            </div>
          </template>
        </UCard>
      </template>
    </UModal>

    <WebhookSourcesDrawer v-model:open="sourcesDrawerOpen" default-entity="products" />
  </section>
</template>
