<script setup lang="ts">
import ClientsContactPopover from '~/components/manager/clients/ClientsContactPopover.vue'
import ClientsStoresPopover from '~/components/manager/clients/ClientsStoresPopover.vue'
import ClientsWebhookPopover from '~/components/manager/clients/ClientsWebhookPopover.vue'
import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniMinimalPopover from '~/components/omni/overlay/OmniMinimalPopover.vue'
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import AccountBoardCard from './AccountBoardCard.vue'
import AccountDetailModal from './AccountDetailModal.vue'
import AccountCreateModal from './AccountCreateModal.vue'
import type { AccountFieldKey, AccountItem, AccountModuleAccess } from '~/types/accounts'
import type { ClientStoreCharge } from '~/types/clients'
import type {
  OmniFilterDefinition,
  OmniFocusCell,
  OmniTableCellUpdate,
  OmniTableColumn,
} from '~/types/omni/collection'

const {
  clients,
  loading,
  creating,
  deletingId,
  errorMessage,
  savingMap,
  fetchClients,
  updateField,
  saveContactAndLogo,
  saveWebhookEnabled,
  rotateWebhookKey,
  saveStores,
  createClient,
  deleteClient,
} = useClientsManager()

const auth = useAuthStore()
const canCreateClient = computed(() => auth.role === 'platform_admin')
const canDeleteClient = computed(() => auth.role === 'platform_admin')

// Organizations carregadas sob demanda para a coluna select.
const orgsManager = useAdminOrganizationsManager()
const organizationOptions = computed(() => [
  { label: 'Sem organization', value: '' },
  ...orgsManager.organizations.value.map((o) => ({ label: o.name, value: o.id })),
])

const viewMode = ref<'table' | 'board'>('table')
const selectedIds = ref<Array<string | number>>([])
const focusCell = ref<OmniFocusCell | null>(null)

// Modal de criar conta (C10) — substitui a criacao inline em branco.
const createModalOpen = ref(false)

// Modal de detalhe espelhado com o board card (mesma fonte: account-fields.ts).
// detailAccount resolve o objeto vivo em `clients` para refletir patches inline.
const detailAccountId = ref<string | null>(null)
const detailOpen = ref(false)
const detailAccount = computed<AccountItem | null>(
  () => clients.value.find((a) => a.id === detailAccountId.value) ?? null,
)

function openDetail(account: AccountItem) {
  detailAccountId.value = account.id
  detailOpen.value = true
}

function onDetailUpdate(payload: { field: AccountFieldKey; value: unknown; immediate?: boolean }) {
  const id = detailAccountId.value
  if (!id || !canCreateClient.value) return
  updateField(id, payload.field, payload.value, { immediate: payload.immediate })
}
const permissionDenied = computed(() =>
  errorMessage.value.toLowerCase().includes('nao tem permissao'),
)

const filtersState = ref<Record<string, unknown>>({
  query: '',
  statusFilter: '',
  billingModeFilter: '',
})

function clientModules(account: AccountItem): AccountModuleAccess[] {
  return Array.isArray(account.modules) ? account.modules : []
}

function modulesSummary(account: AccountItem) {
  const modules = clientModules(account)
  if (modules.length === 0) return 'Sem modulos'
  return modules.map((m) => m.name || m.code).join(', ')
}

// Opcoes do multiselect = catalogo REAL de modulos vindo do backend. O endpoint
// /v1/admin/accounts ja embute TODOS os modulos do catalogo (queue, crm, site,
// tasks, notifications, roadmap, core) com `enabled` por account. NAO usar lista
// hardcoded: codes fake (core_panel, atendimento, indicators, finance, kanban)
// nao existem no backend e clicar neles nao habilita/desabilita nada.
const moduleSelectOptions = computed(() => {
  const byCode = new Map<string, string>()
  for (const account of clients.value) {
    for (const module of clientModules(account)) {
      const code = String(module.code ?? '').trim()
      // `core` e a plataforma base, nao um modulo contratavel pela account.
      if (!code || code === 'core') continue
      byCode.set(code, String(module.name ?? '').trim() || code)
    }
  }
  return [...byCode.entries()].map(([value, label]) => ({ value, label }))
})

const filterDefinitions = computed<OmniFilterDefinition[]>(() => [
  {
    key: 'query',
    label: 'Buscar',
    type: 'text',
    placeholder: 'Pesquisar por texto...',
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
  {
    key: 'billingModeFilter',
    label: 'Modo cobranca',
    type: 'select',
    placeholder: 'Modo cobranca',
    options: [
      { label: 'Unico', value: 'single' },
      { label: 'Por loja', value: 'per_store' },
    ],
    accessor: (row) => row.billingMode,
  },
])

function toAccount(row: Record<string, unknown>) {
  return row as unknown as AccountItem
}

function canEditMonthlyPaymentAmount(row: Record<string, unknown>) {
  return toAccount(row).billingMode !== 'per_store'
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
    key: 'organizationId',
    label: 'Organization',
    type: 'select',
    editable: true,
    immediate: true,
    minWidth: 200,
    defaultOrder: 15,
    options: organizationOptions.value,
  },
  {
    key: 'status',
    label: 'Ativo',
    type: 'switch',
    editable: true,
    switchOnValue: 'active',
    switchOffValue: 'inactive',
    minWidth: 110,
    defaultOrder: 20,
  },
  {
    key: 'userCount',
    label: 'Qtd usuarios',
    type: 'number',
    editable: false,
    minWidth: 140,
    defaultOrder: 30,
  },
  {
    key: 'userNicks',
    label: 'Nicks usuarios',
    type: 'text',
    editable: false,
    minWidth: 220,
    defaultOrder: 40,
  },
  {
    key: 'projectCount',
    label: 'Qtd projetos',
    type: 'number',
    editable: false,
    minWidth: 140,
    defaultOrder: 50,
  },
  {
    key: 'projectSegments',
    label: 'Segmentos',
    type: 'text',
    editable: false,
    minWidth: 220,
    defaultOrder: 60,
  },
  {
    key: 'billingMode',
    label: 'Modo cobranca',
    type: 'select',
    editable: true,
    minWidth: 180,
    immediate: true,
    defaultOrder: 70,
    options: [
      { label: 'Unico', value: 'single' },
      { label: 'Por loja', value: 'per_store' },
    ],
  },
  {
    key: 'monthlyPaymentAmount',
    label: 'Valor mensal',
    type: 'money',
    editable: true,
    editableWhen: (row) => canEditMonthlyPaymentAmount(row),
    minWidth: 170,
    defaultOrder: 80,
  },
  {
    key: 'paymentDueDay',
    label: 'Dia pagamento',
    type: 'number',
    editable: true,
    minWidth: 130,
    defaultOrder: 90,
  },
  {
    key: 'requireUserStoreLink',
    label: 'Obriga loja',
    type: 'switch',
    editable: true,
    immediate: true,
    minWidth: 140,
    defaultOrder: 100,
  },
  {
    key: 'requireUserRegistration',
    label: 'Obriga matricula',
    type: 'switch',
    editable: true,
    immediate: true,
    minWidth: 160,
    defaultOrder: 110,
  },
  {
    key: 'moduleCodes',
    label: 'Modulos',
    type: 'multiselect',
    editable: true,
    immediate: true,
    minWidth: 260,
    options: moduleSelectOptions.value,
    defaultOrder: 120,
  },
  {
    key: 'actions',
    label: 'Opcoes',
    type: 'custom',
    minWidth: 220,
    align: 'center',
    defaultOrder: 1000,
  },
])

const columnExcludeKeys = ['actions']
const { visibleColumnKeys, lockedColumnKeys, columnOrder, tableColumns, resetToDefaults } =
  useOmniVisibleColumns({
    preferenceKey: 'admin.manage.clients',
    allColumns: allTableColumns,
    columnExcludeKeys,
  })

const filteredRows = computed(() => {
  const rows = clients.value as unknown as Array<Record<string, unknown>>
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

const updatableFields = new Set<AccountFieldKey>([
  'name',
  'slug',
  'status',
  'organizationId',
  'billingMode',
  'monthlyPaymentAmount',
  'paymentDueDay',
  'logo',
  'webhookEnabled',
  'contactPhone',
  'contactSite',
  'contactAddress',
  'requireUserStoreLink',
  'requireUserRegistration',
  'modules',
])

function rowId(row: Record<string, unknown>) {
  return String(row.id ?? '').trim()
}

function onCellUpdate(payload: OmniTableCellUpdate) {
  const id = String(payload.rowId).trim()
  if (!id) return

  if (String(payload.key) === 'moduleCodes') {
    updateField(id, 'modules', payload.value, { immediate: true })
    return
  }

  const field = String(payload.key) as AccountFieldKey
  if (!updatableFields.has(field)) return
  updateField(id, field, payload.value, { immediate: payload.immediate })
}

function onCreateClient() {
  if (!canCreateClient.value) return
  createModalOpen.value = true
}

async function submitCreate(payload: {
  name: string
  slug: string
  planCode: string
  adminEmail: string
}) {
  const createdId = await createClient(payload)
  if (!createdId) return
  createModalOpen.value = false
  if (viewMode.value === 'table') {
    focusCell.value = { rowId: createdId, columnKey: 'name', token: Date.now() }
  }
}

function onResetFilters() {
  filtersState.value = { query: '', statusFilter: '', billingModeFilter: '' }
}

async function onDeleteClient(id: string) {
  if (!canDeleteClient.value) return
  if (import.meta.client && !window.confirm('Excluir esta conta? Esta acao nao pode ser desfeita.'))
    return
  await deleteClient(id)
}

onMounted(() => {
  void fetchClients()
  void orgsManager.fetchOrganizations()
})

// Controle de open state do info popover (OmniMinimalPopover é controlled).
const openInfoFor = ref<string | null>(null)
function infoOpen(id: string) {
  return openInfoFor.value === id
}
function setInfoOpen(id: string, value: boolean) {
  openInfoFor.value = value ? id : null
}
</script>

<template>
  <section class="clients-admin-workspace flex h-full min-h-0 flex-col gap-4 overflow-hidden">
    <AdminPageHeader
      eyebrow="Manager"
      title="Clientes"
      description="Tabela generica reutilizavel com filtros desacoplados, selecao em massa e CRUD ligado a API real /v1/admin/accounts."
    />

    <OmniCollectionFilters
      v-model="filtersState"
      v-model:visible-columns="visibleColumnKeys"
      v-model:locked-columns="lockedColumnKeys"
      v-model:column-order="columnOrder"
      :viewer-user-type="canCreateClient ? 'admin' : 'client'"
      :filters="filterDefinitions"
      :table-columns="allTableColumns"
      :column-exclude-keys="columnExcludeKeys"
      :loading="loading"
      @reset="onResetFilters"
      @reset-columns="resetToDefaults"
    >
      <template #actions>
        <UButton
          icon="i-lucide-table"
          :color="viewMode === 'table' ? 'primary' : 'neutral'"
          variant="soft"
          size="sm"
          title="Visao em tabela"
          aria-label="Visao em tabela"
          @click="viewMode = 'table'"
        />
        <UButton
          icon="i-lucide-layout-grid"
          :color="viewMode === 'board' ? 'primary' : 'neutral'"
          variant="soft"
          size="sm"
          title="Visao em cards"
          aria-label="Visao em cards"
          @click="viewMode = 'board'"
        />
        <UBadge color="neutral" variant="soft">Selecionados: {{ selectedIds.length }}</UBadge>
        <UButton
          icon="i-lucide-plus"
          label="Novo cliente"
          color="primary"
          :loading="creating"
          :disabled="creating || !canCreateClient"
          @click="onCreateClient"
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

    <div class="clients-admin-workspace__table-scroll flex-1 min-h-0 overflow-y-auto">
      <OmniDataTable
        v-if="!permissionDenied && viewMode === 'table'"
        v-model="selectedIds"
        :rows="tableRows"
        :columns="tableColumns"
        row-key="id"
        :loading="loading"
        :focus-cell="focusCell"
        empty-text="Nenhum cliente encontrado com os filtros atuais."
        @update:cell="onCellUpdate"
      >
        <template #cell-actions="{ row }">
          <div class="flex items-center justify-end gap-1">
            <UButton
              icon="i-lucide-maximize-2"
              color="neutral"
              variant="ghost"
              size="sm"
              title="Abrir detalhes"
              aria-label="Abrir detalhes"
              @click="openDetail(toAccount(row))"
            />

            <ClientsContactPopover
              :account="toAccount(row)"
              :busy="
                Boolean(savingMap[`${rowId(row)}:logo`]) ||
                Boolean(savingMap[`${rowId(row)}:contactPhone`]) ||
                Boolean(savingMap[`${rowId(row)}:contactSite`]) ||
                Boolean(savingMap[`${rowId(row)}:contactAddress`])
              "
              @save="saveContactAndLogo(rowId(row), $event)"
            />

            <ClientsWebhookPopover
              :account="toAccount(row)"
              :busy="Boolean(savingMap[`${rowId(row)}:webhookEnabled`])"
              @toggle-enabled="saveWebhookEnabled(rowId(row), $event)"
              @rotate-key="rotateWebhookKey(rowId(row))"
            />

            <ClientsStoresPopover
              v-if="toAccount(row).billingMode === 'per_store'"
              :stores="toAccount(row).stores"
              :busy="Boolean(savingMap[`${rowId(row)}:stores`])"
              @save="saveStores(rowId(row), $event as unknown as ClientStoreCharge[])"
            />

            <OmniMinimalPopover
              :open="infoOpen(rowId(row))"
              title="Informacoes do cliente"
              width-class="w-[280px] max-w-[90vw]"
              @update:open="setInfoOpen(rowId(row), $event)"
            >
              <template #trigger>
                <UButton
                  icon="i-lucide-info"
                  color="neutral"
                  variant="ghost"
                  size="sm"
                  title="Detalhes do cliente"
                  aria-label="Info"
                />
              </template>

              <div class="manage-clients__info-popover space-y-1 text-xs">
                <p>
                  <strong>ID:</strong>
                  {{ toAccount(row).id }}
                </p>
                <p>
                  <strong>Nome:</strong>
                  {{ toAccount(row).name }}
                </p>
                <p>
                  <strong>Status:</strong>
                  {{ toAccount(row).status }}
                </p>
                <p>
                  <strong>Webhook:</strong>
                  {{ toAccount(row).webhookEnabled ? 'Ligado' : 'Desligado' }}
                </p>
                <p>
                  <strong>Chave:</strong>
                  {{ toAccount(row).webhookKey || '-' }}
                </p>
                <p>
                  <strong>Telefone:</strong>
                  {{ toAccount(row).contactPhone || '-' }}
                </p>
                <p>
                  <strong>Site:</strong>
                  {{ toAccount(row).contactSite || '-' }}
                </p>
                <p>
                  <strong>Endereco:</strong>
                  {{ toAccount(row).contactAddress || '-' }}
                </p>
                <p>
                  <strong>Obriga loja:</strong>
                  {{ toAccount(row).requireUserStoreLink ? 'sim' : 'nao' }}
                </p>
                <p>
                  <strong>Obriga matricula:</strong>
                  {{ toAccount(row).requireUserRegistration ? 'sim' : 'nao' }}
                </p>
                <p>
                  <strong>Modulos:</strong>
                  {{ modulesSummary(toAccount(row)) }}
                </p>
              </div>
            </OmniMinimalPopover>

            <UButton
              v-if="canDeleteClient"
              icon="i-lucide-trash-2"
              color="error"
              variant="ghost"
              size="sm"
              title="Excluir cliente"
              aria-label="Excluir"
              :loading="deletingId === rowId(row)"
              @click="onDeleteClient(rowId(row))"
            />
          </div>
        </template>
      </OmniDataTable>

      <div
        v-else-if="!permissionDenied && viewMode === 'board'"
        class="clients-admin-workspace__board"
      >
        <AccountBoardCard
          v-for="row in tableRows"
          :key="String(row.id)"
          :account="toAccount(row)"
          @open="openDetail"
        />
        <p v-if="tableRows.length === 0" class="clients-admin-workspace__board-empty">
          Nenhum cliente encontrado com os filtros atuais.
        </p>
      </div>
    </div>

    <AccountCreateModal
      v-model:open="createModalOpen"
      :creating="creating"
      @submit="submitCreate"
    />

    <AccountDetailModal
      v-model:open="detailOpen"
      :account="detailAccount"
      :can-edit="canCreateClient"
      @update-field="onDetailUpdate"
    />
  </section>
</template>

<style scoped>
.clients-admin-workspace__board {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(20rem, 1fr));
  gap: 0.85rem;
  padding-bottom: 0.5rem;
}

.clients-admin-workspace__board-empty {
  grid-column: 1 / -1;
  margin: 0;
  padding: 2rem 0;
  text-align: center;
  color: var(--text-muted);
  font-size: 0.85rem;
}
</style>
