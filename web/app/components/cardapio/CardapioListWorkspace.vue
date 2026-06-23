<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

import CardapioCreateModal from '~/components/cardapio/CardapioCreateModal.vue'
import CardapioDuplicateModal from '~/components/cardapio/CardapioDuplicateModal.vue'
import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import { useCardapioStore } from '~/stores/cardapio'
import { useTenantsStore } from '~/stores/tenants'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import type {
  OmniFilterDefinition,
  OmniFocusCell,
  OmniTableCellUpdate,
  OmniTableColumn,
} from '~/types/omni/collection'
import type { RestaurantListItem } from '~/domain/cardapio/types'

const store = useCardapioStore()
const tenantsStore = useTenantsStore()
const auth = useAuthStore()
const ui = useUiStore()

const isAdmin = computed(() => String(auth.role || '').trim() === 'platform_admin')
const viewerUserType = computed<'admin' | 'client'>(() => (isAdmin.value ? 'admin' : 'client'))

const createOpen = ref(false)
const creating = ref(false)
const duplicateOpen = ref(false)
const duplicating = ref(false)
const duplicateSource = ref<RestaurantListItem | null>(null)
const focusCell = ref<OmniFocusCell | null>(null)
const selectedIds = ref<Array<string | number>>([])

// Filtros server-side (busca + cliente): a lista re-le do back ao mudar, igual
// ao comportamento anterior. accountFilter so existe para platform_admin.
const filtersState = ref<Record<string, unknown>>({ query: '', accountFilter: '' })

let searchTimer: ReturnType<typeof setTimeout> | null = null

const tenantOptions = computed(() =>
  (tenantsStore.tenants || []).map((tenant) => ({ id: tenant.id, name: tenant.name })),
)

const filterDefinitions = computed<OmniFilterDefinition[]>(() => [
  {
    key: 'query',
    label: 'Buscar',
    type: 'text',
    placeholder: 'Buscar por nome ou slug',
    mode: 'all',
  },
  {
    key: 'accountFilter',
    label: 'Cliente',
    type: 'select',
    adminOnly: true,
    placeholder: 'Todos os clientes',
    options: tenantOptions.value.map((tenant) => ({ label: tenant.name, value: tenant.id })),
  },
])

function dateLabel(value: unknown): string {
  const date = new Date(String(value ?? ''))
  if (Number.isNaN(date.getTime())) {
    return '—'
  }
  return date.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit', year: 'numeric' })
}

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
  {
    key: 'slug',
    label: 'Slug',
    type: 'custom',
    minWidth: 180,
    defaultOrder: 20,
  },
  {
    key: 'accountId',
    label: 'Cliente',
    type: 'select',
    adminOnly: true,
    editable: true,
    minWidth: 200,
    defaultOrder: 30,
    placeholder: 'Selecione o cliente',
    options: tenantOptions.value.map((tenant) => ({ label: tenant.name, value: tenant.id })),
  },
  {
    key: 'primaryDomain',
    label: 'Dominio',
    type: 'text',
    editable: true,
    minWidth: 200,
    defaultOrder: 40,
    placeholder: 'exemplo.com.br',
  },
  {
    key: 'isActive',
    label: 'Status',
    type: 'switch',
    editable: true,
    immediate: true,
    switchOnValue: true,
    switchOffValue: false,
    minWidth: 110,
    defaultOrder: 50,
  },
  {
    key: 'updatedAt',
    label: 'Atualizado',
    type: 'text',
    editable: false,
    minWidth: 140,
    defaultOrder: 60,
    formatter: (value) => dateLabel(value),
  },
  {
    key: 'actions',
    label: 'Opcoes',
    type: 'custom',
    minWidth: 90,
    align: 'center',
    defaultOrder: 1000,
  },
])

const columnExcludeKeys = ['actions']
// Para admin, a coluna 'accountId' (Cliente) deve sempre aparecer, mesmo que a
// preferencia salva no localStorage seja anterior a existencia dela. Reativo:
// so injeta quando o papel admin resolve. O OmniDataTable ainda esconde a coluna
// adminOnly para nao-admin (entao a lista vazia para client nao a forca).
const forceVisibleColumnKeys = computed(() => (isAdmin.value ? ['accountId'] : []))
const { visibleColumnKeys, lockedColumnKeys, columnOrder, tableColumns, resetToDefaults } =
  useOmniVisibleColumns({
    preferenceKey: 'cardapio.list.restaurants',
    allColumns: allTableColumns,
    columnExcludeKeys,
    forceVisibleColumnKeys,
  })

const tableRows = computed(() => store.restaurants as unknown as Array<Record<string, unknown>>)

function toRestaurant(row: Record<string, unknown>): RestaurantListItem {
  return row as unknown as RestaurantListItem
}

async function refresh() {
  await store.loadRestaurants({
    accountId: isAdmin.value ? String(filtersState.value.accountFilter || '').trim() : '',
    q: String(filtersState.value.query || '').trim(),
  })
}

watch(
  () => filtersState.value.query,
  () => {
    if (searchTimer) {
      clearTimeout(searchTimer)
    }
    searchTimer = setTimeout(() => {
      void refresh()
    }, 300)
  },
)

watch(
  () => filtersState.value.accountFilter,
  () => {
    void refresh()
  },
)

function onResetFilters() {
  filtersState.value = { query: '', accountFilter: '' }
}

function openEditor(id: string, accountId = '') {
  const account = String(accountId || '').trim()
  void navigateTo(`/cardapio/${id}${account ? `?account=${encodeURIComponent(account)}` : ''}`)
}

function onCellUpdate(payload: OmniTableCellUpdate) {
  const id = String(payload.rowId).trim()
  if (!id) return

  const row = store.restaurants.find((item) => item.id === id)
  if (!row) return

  // Multi-tenant: na lista o scopeAccountId esta vazio; o accountId da linha
  // garante que a edicao caia na account certa (senao 404 fora do escopo).
  const accountId = String(row.accountId || '').trim()
  const key = String(payload.key)

  if (key === 'name') {
    const name = String(payload.value ?? '').trim()
    if (!name) return
    void savePatch(id, accountId, { name })
    return
  }

  if (key === 'isActive') {
    void savePatch(id, accountId, { isActive: payload.value === true })
    return
  }

  if (key === 'accountId') {
    const target = String(payload.value ?? '').trim()
    if (!target || target === accountId) return
    void moveAccount(id, accountId, target)
    return
  }

  if (key === 'primaryDomain') {
    void saveDomain(id, accountId, String(payload.value ?? ''))
  }
}

async function savePatch(id: string, accountId: string, body: Record<string, unknown>) {
  try {
    await store.patchRestaurantScoped(id, accountId, body)
  } catch {
    ui.error('Nao foi possivel salvar a alteracao.')
    void refresh()
  }
}

// Move o restaurante para outra conta (so platform_admin). O PATCH nao devolve o
// nome da nova account, entao recarregamos a lista para refletir Cliente/dominio.
async function moveAccount(id: string, accountId: string, target: string) {
  try {
    await store.patchRestaurantScoped(id, accountId, { accountId: target })
    await refresh()
    ui.success('Estabelecimento movido para o novo cliente.')
  } catch {
    ui.error('Nao foi possivel mover o estabelecimento.')
    void refresh()
  }
}

// Edicao inline do dominio primario: host vazio e NO-OP (remover so na aba
// Dominios). Em sucesso a store ja atualiza a linha lean.
async function saveDomain(id: string, accountId: string, host: string) {
  if (!String(host || '').trim()) return
  try {
    await store.setPrimaryDomain(id, accountId, host)
  } catch {
    ui.error('Nao foi possivel salvar o dominio.')
    void refresh()
  }
}

async function onDelete(row: Record<string, unknown>) {
  const restaurant = toRestaurant(row)
  const { confirmed } = (await ui.confirm({
    title: 'Excluir estabelecimento',
    message: `Excluir o estabelecimento "${restaurant.name}"? Esta acao nao pode ser desfeita.`,
    confirmLabel: 'Excluir',
  })) as { confirmed: boolean }
  if (!confirmed) return

  try {
    await store.deleteRestaurantScoped(restaurant.id, String(restaurant.accountId || '').trim())
    ui.success('Estabelecimento excluido.')
  } catch {
    ui.error('Nao foi possivel excluir o estabelecimento.')
  }
}

// Duplicar (so platform_admin): abre o modal com o restaurante de origem. O back
// nega demais papeis; o botao tambem so aparece para admin.
function openDuplicate(row: Record<string, unknown>) {
  duplicateSource.value = toRestaurant(row)
  duplicateOpen.value = true
}

async function onDuplicate(payload: { name: string; slug: string }) {
  const source = duplicateSource.value
  if (!source) return

  duplicating.value = true
  try {
    // accountId da PROPRIA linha: na lista o scopeAccountId esta vazio, entao a
    // duplicacao de um restaurante de outra account precisa do escopo da linha.
    const created = await store.duplicateRestaurant(
      source.id,
      { name: payload.name, slug: payload.slug },
      String(source.accountId || '').trim(),
    )
    duplicateOpen.value = false
    duplicateSource.value = null
    ui.success('Estabelecimento duplicado.')
    openEditor(created.id, source.accountId)
  } catch {
    ui.error('Nao foi possivel duplicar o estabelecimento.')
  } finally {
    duplicating.value = false
  }
}

function openCreate() {
  createOpen.value = true
}

async function onCreate(payload: { name: string; slug: string; accountId: string }) {
  creating.value = true
  const result = await store.createRestaurant({
    name: payload.name,
    slug: payload.slug,
    accountId: isAdmin.value ? payload.accountId : undefined,
  })
  creating.value = false

  if (!result.ok) {
    ui.error(result.message)
    return
  }

  createOpen.value = false
  ui.success('Estabelecimento criado.')
  // Admin pode criar sob a account de um cliente: leva o accountId na query pro
  // editor escopar o GET corretamente (senao 404 quando != account ativa).
  const account = isAdmin.value ? String(payload.accountId || '').trim() : ''
  openEditor(result.restaurant.id, account)
}

onMounted(async () => {
  await auth.ensureSession()
  if (isAdmin.value) {
    await tenantsStore.ensureLoaded()
  }
  await refresh()
})
</script>

<template>
  <section class="cardapio-list flex h-full min-h-0 flex-col gap-4 overflow-hidden">
    <AdminPageHeader
      eyebrow="Presence"
      title="Estabelecimentos"
      description="Gerencie o site de cada estabelecimento: visual, cardapio, pedidos e dominios."
    />

    <OmniCollectionFilters
      v-model="filtersState"
      v-model:visible-columns="visibleColumnKeys"
      v-model:locked-columns="lockedColumnKeys"
      v-model:column-order="columnOrder"
      :viewer-user-type="viewerUserType"
      :filters="filterDefinitions"
      :table-columns="allTableColumns"
      :column-exclude-keys="columnExcludeKeys"
      :loading="store.listPending"
      @reset="onResetFilters"
      @reset-columns="resetToDefaults"
    >
      <template #actions>
        <UButton
          icon="i-lucide-plus"
          label="Novo estabelecimento"
          color="primary"
          :loading="creating"
          :disabled="creating"
          @click="openCreate"
        />
      </template>
    </OmniCollectionFilters>

    <UAlert
      v-if="store.listError"
      color="error"
      variant="soft"
      icon="i-lucide-alert-triangle"
      title="Erro"
      :description="store.listError"
    />

    <div class="cardapio-list__table-scroll min-h-0 flex-1 overflow-y-auto">
      <OmniDataTable
        v-model="selectedIds"
        :rows="tableRows"
        :columns="tableColumns"
        :viewer-user-type="viewerUserType"
        row-key="id"
        :loading="store.listPending"
        :focus-cell="focusCell"
        empty-text="Nenhum estabelecimento encontrado com os filtros atuais."
        @update:cell="onCellUpdate"
      >
        <template #cell-slug="{ row }">
          <button
            type="button"
            class="cardapio-list__link cardapio-list__link--mono"
            @click="openEditor(toRestaurant(row).id, toRestaurant(row).accountId)"
          >
            {{ toRestaurant(row).slug || '—' }}
          </button>
        </template>

        <template #cell-actions="{ row }">
          <div class="flex items-center justify-end gap-1">
            <UButton
              icon="i-lucide-pencil"
              color="neutral"
              variant="ghost"
              size="sm"
              title="Abrir editor"
              aria-label="Abrir editor"
              @click="openEditor(toRestaurant(row).id, toRestaurant(row).accountId)"
            />
            <UButton
              v-if="isAdmin"
              icon="i-lucide-copy"
              color="neutral"
              variant="ghost"
              size="sm"
              title="Duplicar estabelecimento"
              aria-label="Duplicar estabelecimento"
              @click="openDuplicate(row)"
            />
            <UButton
              icon="i-lucide-trash-2"
              color="error"
              variant="ghost"
              size="sm"
              title="Excluir estabelecimento"
              aria-label="Excluir estabelecimento"
              @click="onDelete(row)"
            />
          </div>
        </template>
      </OmniDataTable>
    </div>

    <CardapioCreateModal
      :open="createOpen"
      :saving="creating"
      :is-admin="isAdmin"
      :tenants="tenantOptions"
      @close="createOpen = false"
      @submit="onCreate"
    />

    <CardapioDuplicateModal
      :open="duplicateOpen"
      :saving="duplicating"
      :source-name="duplicateSource?.name || ''"
      :source-slug="duplicateSource?.slug || ''"
      @close="duplicateOpen = false"
      @submit="onDuplicate"
    />
  </section>
</template>

<style scoped>
.cardapio-list__link {
  color: rgb(var(--primary));
  cursor: pointer;
  text-align: left;
}

.cardapio-list__link:hover {
  text-decoration: underline;
}

.cardapio-list__link--mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.84rem;
}
</style>
