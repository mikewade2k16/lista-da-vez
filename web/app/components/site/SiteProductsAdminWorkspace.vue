<script setup lang="ts">
import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniMinimalPopover from '~/components/omni/overlay/OmniMinimalPopover.vue'
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import WebhookSourcesDrawer from '~/components/site/WebhookSourcesDrawer.vue'
import SiteProductCreateDialog from '~/components/site/SiteProductCreateDialog.vue'
import SiteProductInfoCard from '~/components/site/SiteProductInfoCard.vue'
import SiteProductsTableFooter from '~/components/site/SiteProductsTableFooter.vue'
import SiteErpUnmatchedPanel from '~/components/site/SiteErpUnmatchedPanel.vue'
import type {
  ProductCreateInput,
  ProductFieldKey,
  ProductItem,
  ProductStatus,
} from '~/types/products'
import type {
  OmniFocusCell,
  OmniSelectOption,
  OmniTableCellUpdate,
  OmniTableImageUpload,
} from '~/types/omni/collection'

const {
  products,
  total,
  mode,
  page,
  perPage,
  facets,
  loading,
  creating,
  syncing,
  sourceMode,
  sourceLoading,
  deletingId,
  errorMessage,
  selectedAccountId,
  fetchProducts,
  loadFacets,
  setPage,
  setMode,
  applyServerFilters,
  updateField,
  uploadProductImage,
  createProduct,
  deleteProduct,
  syncProducts,
  loadSource,
  setSourceMode,
  erpMatching,
  erpUnmatched,
  erpUnmatchedTotal,
  erpUnmatchedPage,
  erpUnmatchedPerPage,
  erpUnmatchedQuery,
  erpUnmatchedLoading,
  erpCreatingSku,
  erpMatch,
  loadErpUnmatched,
  createFromErp,
} = useProductsManager()

const auth = useAuthStore()
const ui = useUiStore()
const tenants = useTenantsStore()
const canCreate = computed(
  () =>
    auth.role === 'platform_admin' ||
    auth.role === 'owner' ||
    auth.role === 'director' ||
    auth.role === 'manager',
)
const canDelete = computed(() => canCreate.value)
const canSync = computed(() => auth.role === 'platform_admin')
const isAdmin = computed(() => auth.role === 'platform_admin')

// Filtro por cliente (admin). Cada cliente pode ter integracao/produtos
// diferentes; o admin escolhe qual ver e sincronizar. Sentinela 'active' p/ o
// USelect (Reka proibe item com value vazio); convertido p/ '' (account do
// contexto) no handler.
const CLIENT_SENTINEL = 'active'
const clientItems = computed(() => [
  { label: 'Cliente ativo', value: CLIENT_SENTINEL },
  ...tenants.tenants.map((t: { id: string; name: string }) => ({ label: t.name, value: t.id })),
])
const clientValue = computed(() => selectedAccountId.value || CLIENT_SENTINEL)

function onChangeClient(next: string) {
  selectedAccountId.value = next === CLIENT_SENTINEL ? '' : String(next ?? '')
  void fetchProducts()
  // Facets sao por account: recarrega para refletir o cliente selecionado.
  void loadFacets(true)
  // Fonte tambem e por account: recarrega o toggle para o cliente selecionado.
  if (canSync.value) void loadSource()
}

async function onSyncProducts() {
  if (!canSync.value) return
  const result = await syncProducts()
  if (!result) {
    ui.error(errorMessage.value || 'Falha ao sincronizar produtos.')
    return
  }
  ui.success(
    `Sincronizacao concluida: ${result.inserted} novos, ${result.updated} atualizados, ${result.skipped} ignorados.`,
  )
}

// Fonte de produtos (Local XAMPP / Online). So 'local'/'online' no USelect (Reka
// proibe value vazio); 'custom' so aparece como leitura quando vem do backend, e
// nao e oferecido como opcao. Ao trocar: PATCH + toast + re-sync para a bio
// refletir (a bio le site.products, que e o que o sync grava).
const sourceItems = [
  { label: 'Local (XAMPP)', value: 'local' },
  { label: 'Online', value: 'online' },
]
const sourceLabel = computed(() =>
  sourceMode.value === 'local'
    ? 'Local (XAMPP)'
    : sourceMode.value === 'custom'
      ? 'Custom'
      : 'Online',
)
// O USelect so tem 'local'/'online'; 'custom' (base_url fora das 2 conhecidas)
// cai em 'online' para nao passar um value fora dos items (Reka).
const sourceSelectValue = computed(() => (sourceMode.value === 'local' ? 'local' : 'online'))

async function onChangeSource(next: string) {
  if (!canSync.value) return
  if (next !== 'local' && next !== 'online') return
  if (next === sourceMode.value) return
  const applied = await setSourceMode(next)
  if (!applied) {
    ui.error(errorMessage.value || 'Falha ao trocar a fonte.')
    return
  }
  ui.success(`Fonte trocada para ${sourceLabel.value} — sincronize para atualizar os produtos.`)
  // Re-sincroniza automaticamente para a bio refletir a nova fonte.
  void onSyncProducts()
}

async function onErpMatch() {
  if (!canSync.value) return
  const result = await erpMatch()
  if (!result) {
    ui.error(errorMessage.value || 'Falha ao cruzar produtos com o ERP.')
    return
  }
  ui.success(`Cruzamento: ${result.products} produtos vinculados ao ERP (${result.matched} links).`)
}

// Abas: produtos do site x itens do ERP fora do site. A aba do ERP carrega sob
// demanda (so na 1a vez que e aberta).
const activeTab = ref<'products' | 'erp'>('products')
const erpLoaded = ref(false)

const tabItems = computed(() => [
  { label: 'Produtos do site', value: 'products', icon: 'i-lucide-package' },
  { label: 'Produtos do ERP (fora do site)', value: 'erp', icon: 'i-lucide-database' },
])

function onChangeTab(value: string | number) {
  const next = value === 'erp' ? 'erp' : 'products'
  activeTab.value = next
  if (next === 'erp' && !erpLoaded.value) {
    erpLoaded.value = true
    void loadErpUnmatched({ page: 1 })
  }
}

function onErpPage(page: number) {
  void loadErpUnmatched({ page })
}

function onErpSearch(q: string) {
  void loadErpUnmatched({ page: 1, q })
}

async function onErpPull(sku: string) {
  const ok = await createFromErp(sku)
  if (ok) ui.success('Produto criado a partir do ERP.')
  else ui.error(errorMessage.value || 'Falha ao puxar o item do ERP.')
}

const selectedIds = ref<Array<string | number>>([])
const focusCell = ref<OmniFocusCell | null>(null)

const filtersState = ref<Record<string, unknown>>({
  query: '',
  statusFilter: '',
  categoryFilter: '',
  campaignFilter: '',
})

// Opcoes de categoria/campanha vem dos facets (lista COMPLETA da account,
// independe da paginacao) — assim os selects mostram todas as tags mesmo no
// modo paginado, onde so uma pagina esta carregada.
function facetOptions(values: string[]): OmniSelectOption[] {
  return [...values]
    .map((value) => String(value ?? '').trim())
    .filter(Boolean)
    .sort((a, b) => a.localeCompare(b, 'pt-BR'))
    .map((tag) => ({ label: tag, value: tag }))
}

const categoryOptions = computed(() => facetOptions(facets.value.categories))
const campaignOptions = computed(() => facetOptions(facets.value.campaigns))

// Colunas + definicoes de filtro vem do composable (categoria/campanha pelas
// options dos facets — lista completa mesmo no modo paginado).
const { allTableColumns, filterDefinitions } = useSiteProductColumns(
  categoryOptions,
  campaignOptions,
)

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
  'categories',
  'campaigns',
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
  const key = String(payload.key)

  // Switch "Tem estoque" e' derivado: nao existe campo hasStock no backend.
  // Traduz para stock (on => 1, off => 0) e PATCH o estoque numerico.
  if (key === 'hasStock') {
    updateField(id, 'stock', payload.value === true ? 1 : 0, { immediate: true })
    return
  }

  const field = key as ProductFieldKey
  if (!updatableFields.has(field)) return
  updateField(id, field, payload.value, { immediate: payload.immediate })
}

async function onUploadImage(payload: OmniTableImageUpload) {
  const id = String(payload.rowId).trim()
  if (!id) return
  const ok = await uploadProductImage(id, payload.file)
  if (ok) ui.success('Imagem atualizada.')
  else ui.error(errorMessage.value || 'Falha ao enviar a imagem.')
}

const createDialogOpen = ref(false)

function openCreate() {
  createDialogOpen.value = true
}

async function submitCreate(payload: ProductCreateInput) {
  if (!canCreate.value) return
  const createdId = await createProduct(payload)
  if (!createdId) return
  createDialogOpen.value = false
  focusCell.value = { rowId: createdId, columnKey: 'name', token: Date.now() }
}

function onResetFilters() {
  filtersState.value = { query: '', statusFilter: '', categoryFilter: '', campaignFilter: '' }
}

// No modo paginado os filtros sao server-side: empurra os valores do
// OmniCollectionFilters para o composable (reseta para a pagina 1 e refaz o
// fetch). No modo 'all' a filtragem e client-side (applyOmniFilters), entao o
// watcher nao dispara request.
function toStatusFilter(value: unknown): '' | ProductStatus {
  return value === 'active' || value === 'inactive' ? value : ''
}

watch(
  filtersState,
  (state) => {
    if (mode.value !== 'paged') return
    applyServerFilters({
      q: String(state.query ?? '').trim(),
      status: toStatusFilter(state.statusFilter),
      category: String(state.categoryFilter ?? '').trim(),
      campaign: String(state.campaignFilter ?? '').trim(),
    })
  },
  { deep: true },
)

function onChangeMode(next: 'all' | 'paged') {
  // Ao entrar no modo paginado, leva os filtros atuais para o servidor; ao sair,
  // o composable apenas recarrega tudo (filtros voltam a ser client-side).
  setMode(next)
  if (next === 'paged') {
    applyServerFilters({
      q: String(filtersState.value.query ?? '').trim(),
      status: toStatusFilter(filtersState.value.statusFilter),
      category: String(filtersState.value.categoryFilter ?? '').trim(),
      campaign: String(filtersState.value.campaignFilter ?? '').trim(),
    })
  }
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
  // Facets (categorias/campanhas) para os selects das colunas e dos filtros.
  void loadFacets()
  // Fonte atual (Local/Online) para o toggle — so quem pode sincronizar ve.
  if (canSync.value) void loadSource()
  if (isAdmin.value) {
    void tenants.ensureLoaded()
  }
})
</script>

<template>
  <section class="site-products-workspace flex h-full min-h-0 flex-col gap-4 overflow-hidden">
    <AdminPageHeader
      eyebrow="Site"
      title="Produtos"
      description="Catalogo de produtos do site (manual ou ingerido por webhook). Ligado a API real /v1/admin/products."
    />

    <UTabs
      :items="tabItems"
      :model-value="activeTab"
      :content="false"
      color="primary"
      size="sm"
      @update:model-value="onChangeTab"
    />

    <div
      v-show="activeTab === 'products'"
      class="site-products-workspace__pane flex min-h-0 flex-1 flex-col gap-4"
    >
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
          <USelect
            v-if="isAdmin"
            :model-value="clientValue"
            :items="clientItems"
            value-key="value"
            size="sm"
            icon="i-lucide-building-2"
            title="Filtrar por cliente (account)"
            @update:model-value="onChangeClient(String($event ?? ''))"
          />
          <UBadge
            color="neutral"
            variant="soft"
            :title="`${total} produtos no catalogo desta conta`"
          >
            {{ tableRows.length }} / {{ total }} produtos
          </UBadge>
          <UBadge color="neutral" variant="soft">Selecionados: {{ selectedIds.length }}</UBadge>
          <USelect
            v-if="canSync"
            :model-value="sourceSelectValue"
            :items="sourceItems"
            value-key="value"
            size="sm"
            icon="i-lucide-database"
            title="Fonte dos produtos (Local XAMPP / Online)"
            :loading="sourceLoading"
            :disabled="sourceLoading || syncing"
            @update:model-value="onChangeSource(String($event ?? ''))"
          />
          <UButton
            v-if="canSync"
            icon="i-lucide-refresh-cw"
            label="Sincronizar produtos"
            color="neutral"
            variant="soft"
            :loading="syncing"
            :disabled="syncing"
            @click="onSyncProducts"
          />
          <UButton
            v-if="canSync"
            icon="i-lucide-link"
            label="Cruzar com ERP"
            color="neutral"
            variant="soft"
            :loading="erpMatching"
            :disabled="erpMatching"
            @click="onErpMatch"
          />
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
          @upload:image="onUploadImage"
        >
          <template #cell-erpSynced="{ row }">
            <UBadge
              :color="toProduct(row).erpSynced ? 'success' : 'neutral'"
              variant="soft"
              size="sm"
              :title="
                toProduct(row).erpName ||
                (toProduct(row).erpSynced ? 'Cruzado com ERP' : 'Sem vinculo com ERP')
              "
            >
              {{ toProduct(row).erpSynced ? 'ERP' : '—' }}
            </UBadge>
          </template>

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
                <SiteProductInfoCard :product="toProduct(row)" />
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

      <SiteProductsTableFooter
        :mode="mode"
        :page="page"
        :per-page="perPage"
        :total="total"
        :loading="loading"
        @update:mode="onChangeMode"
        @update:page="setPage"
      />
    </div>

    <div
      v-show="activeTab === 'erp'"
      class="site-products-workspace__pane flex min-h-0 flex-1 flex-col"
    >
      <SiteErpUnmatchedPanel
        :items="erpUnmatched"
        :total="erpUnmatchedTotal"
        :page="erpUnmatchedPage"
        :per-page="erpUnmatchedPerPage"
        :query="erpUnmatchedQuery"
        :loading="erpUnmatchedLoading"
        :creating-sku="erpCreatingSku"
        @update:page="onErpPage"
        @search="onErpSearch"
        @pull="onErpPull"
      />
    </div>

    <SiteProductCreateDialog
      v-model:open="createDialogOpen"
      :creating="creating"
      @submit="submitCreate"
    />

    <WebhookSourcesDrawer v-model:open="sourcesDrawerOpen" default-entity="products" />
  </section>
</template>
