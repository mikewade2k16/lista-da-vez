<script setup lang="ts">
import ClientsContactPopover from '~/components/manager/clients/ClientsContactPopover.vue'
import ClientsStoresPopover from '~/components/manager/clients/ClientsStoresPopover.vue'
import ClientsWebhookPopover from '~/components/manager/clients/ClientsWebhookPopover.vue'
import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniMinimalPopover from '~/components/omni/overlay/OmniMinimalPopover.vue'
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import type { ClientFieldKey, ClientItem, ClientStoreCharge } from '~/types/clients'
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
const sessionSimulation = useSessionSimulationStore()

const selectedIds = ref<Array<string | number>>([])
const focusCell = ref<OmniFocusCell | null>(null)
const canCreateClient = computed(() => sessionSimulation.isAdmin)
const canDeleteClient = computed(() => sessionSimulation.isAdmin)
const permissionDenied = computed(() =>
  errorMessage.value.toLowerCase().includes('nao tem permissao'),
)

const filtersState = ref<Record<string, unknown>>({
  query: '',
  statusFilter: '',
  billingModeFilter: '',
})

const moduleSelectOptions = computed(() => {
  const known = new Map<string, string>([
    ['core_panel', 'Core Panel'],
    ['atendimento', 'Atendimento'],
    ['fila-atendimento', 'Fila de Atendimento'],
    ['indicators', 'Indicadores'],
    ['finance', 'Finance'],
    ['kanban', 'Kanban'],
  ])

  for (const client of clients.value) {
    for (const module of clientModules(client)) {
      const code = String(module.code ?? '')
        .trim()
        .toLowerCase()
      if (!code || known.has(code)) {
        continue
      }

      known.set(code, String(module.name ?? '').trim() || code)
    }
  }

  return [...known.entries()].map(([value, label]) => ({ value, label }))
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

const allTableColumns = computed<OmniTableColumn[]>(() => [
  { key: 'name', label: 'Nome', type: 'text', editable: true, minWidth: 220, focusOnCreate: true },
  {
    key: 'status',
    label: 'Ativo',
    type: 'switch',
    editable: true,
    switchOnValue: 'active',
    switchOffValue: 'inactive',
    minWidth: 110,
  },
  { key: 'userCount', label: 'Qtd usuarios', type: 'number', editable: true, minWidth: 140 },
  { key: 'userNicks', label: 'Nicks usuarios', type: 'text', editable: true, minWidth: 220 },
  { key: 'projectCount', label: 'Qtd projetos', type: 'number', editable: true, minWidth: 140 },
  { key: 'projectSegments', label: 'Segmentos', type: 'text', editable: true, minWidth: 220 },
  {
    key: 'billingMode',
    label: 'Modo cobranca',
    type: 'select',
    editable: true,
    minWidth: 180,
    immediate: true,
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
  },
  { key: 'paymentDueDay', label: 'Dia pagamento', type: 'number', editable: true, minWidth: 130 },
  {
    key: 'requireUserStoreLink',
    label: 'Obriga loja',
    type: 'switch',
    editable: true,
    immediate: true,
    minWidth: 140,
  },
  {
    key: 'requireUserRegistration',
    label: 'Obriga matricula',
    type: 'switch',
    editable: true,
    immediate: true,
    minWidth: 160,
  },
  {
    key: 'moduleCodes',
    label: 'Modulos',
    type: 'multiselect',
    editable: true,
    immediate: true,
    minWidth: 260,
    options: moduleSelectOptions.value,
  },
  { key: 'actions', label: 'Opcoes', type: 'custom', minWidth: 220, align: 'center' },
])

const columnExcludeKeys = ['actions']
const alwaysVisibleColumnKeys = new Set(['actions'])
const { visibleColumnKeys, tableColumns } = useOmniVisibleColumns({
  preferenceKey: 'admin.manage.clients',
  allColumns: allTableColumns,
  columnExcludeKeys,
  alwaysVisibleColumnKeys,
})

const filteredRows = computed(() => {
  const rows = clients.value as unknown as Array<Record<string, unknown>>
  return applyOmniFilters(rows, filtersState.value, filterDefinitions.value)
})

const tableRows = computed(() => {
  const seen = new Set<number>()
  return filteredRows.value.filter((row) => {
    const parsedId = Number((row as Record<string, unknown>).id)
    if (!Number.isFinite(parsedId) || parsedId <= 0 || seen.has(parsedId)) {
      return false
    }

    seen.add(parsedId)
    return true
  })
})

const updatableFields = new Set<ClientFieldKey>([
  'name',
  'status',
  'userCount',
  'userNicks',
  'projectCount',
  'projectSegments',
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

function toClient(row: Record<string, unknown>) {
  return row as unknown as ClientItem
}

function rowIdValue(row: Record<string, unknown>) {
  const parsed = Number(row.id)
  return Number.isFinite(parsed) ? parsed : 0
}

function clientModules(client: ClientItem) {
  return Array.isArray(client.modules) ? client.modules : []
}

function modulesSummary(client: ClientItem) {
  const modules = clientModules(client)
  if (modules.length === 0) {
    return 'Sem modulos'
  }

  return modules.map((module) => module.name || module.code).join(', ')
}

function canEditMonthlyPaymentAmount(row: Record<string, unknown>) {
  return toClient(row).billingMode !== 'per_store'
}

function onCellUpdate(payload: OmniTableCellUpdate) {
  if (String(payload.key) === 'moduleCodes') {
    const id = Number(payload.rowId)
    if (!Number.isFinite(id) || id <= 0) {
      return
    }

    updateField(id, 'modules', payload.value, { immediate: true })
    return
  }

  const field = String(payload.key) as ClientFieldKey
  if (!updatableFields.has(field)) {
    return
  }

  const id = Number(payload.rowId)
  if (!Number.isFinite(id) || id <= 0) {
    return
  }

  updateField(id, field, payload.value, { immediate: payload.immediate })
}

async function onCreateClient() {
  if (!canCreateClient.value) {
    return
  }

  const createdId = await createClient()
  if (!createdId) {
    return
  }

  focusCell.value = { rowId: createdId, columnKey: 'name', token: Date.now() }
}

function onResetFilters() {
  filtersState.value = {
    query: '',
    statusFilter: '',
    billingModeFilter: '',
  }
}

async function onDeleteClient(id: number) {
  if (!canDeleteClient.value) {
    return
  }

  if (import.meta.client) {
    const confirmed = window.confirm('Excluir este cliente? Esta acao nao pode ser desfeita.')
    if (!confirmed) {
      return
    }
  }

  await deleteClient(id)
}

onMounted(() => {
  void fetchClients()
})
</script>

<template>
  <section class="space-y-4">
    <AdminPageHeader
      eyebrow="Manager"
      title="Clientes"
      description="Tabela generica reutilizavel com filtros desacoplados, selecao em massa e CRUD em modo teste."
    />

    <OmniCollectionFilters
      v-model="filtersState"
      v-model:visible-columns="visibleColumnKeys"
      :filters="filterDefinitions"
      :table-columns="allTableColumns"
      :column-exclude-keys="columnExcludeKeys"
      :loading="loading"
      @reset="onResetFilters"
    >
      <template #actions>
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

    <OmniDataTable
      v-if="!permissionDenied"
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
          <ClientsContactPopover
            :client="toClient(row)"
            :busy="
              Boolean(savingMap[`${rowIdValue(row)}:logo`]) ||
              Boolean(savingMap[`${rowIdValue(row)}:contactPhone`]) ||
              Boolean(savingMap[`${rowIdValue(row)}:contactSite`]) ||
              Boolean(savingMap[`${rowIdValue(row)}:contactAddress`])
            "
            @save="saveContactAndLogo(rowIdValue(row), $event)"
          />

          <ClientsWebhookPopover
            :client="toClient(row)"
            :busy="Boolean(savingMap[`${rowIdValue(row)}:webhookEnabled`])"
            @toggle-enabled="saveWebhookEnabled(rowIdValue(row), $event)"
            @rotate-key="rotateWebhookKey(rowIdValue(row))"
          />

          <ClientsStoresPopover
            v-if="toClient(row).billingMode === 'per_store'"
            :stores="toClient(row).stores"
            :busy="Boolean(savingMap[`${rowIdValue(row)}:stores`])"
            @save="saveStores(rowIdValue(row), $event as ClientStoreCharge[])"
          />

          <OmniMinimalPopover title="Informacoes do cliente" width-class="w-[280px] max-w-[90vw]">
            <template #trigger>
              <UButton
                icon="i-lucide-info"
                color="neutral"
                variant="ghost"
                size="sm"
                aria-label="Info"
              />
            </template>

            <div class="manage-clients__info-popover space-y-1 text-xs">
              <p>
                <strong>ID:</strong>
                {{ toClient(row).id }}
              </p>
              <p>
                <strong>Nome:</strong>
                {{ toClient(row).name }}
              </p>
              <p>
                <strong>Status:</strong>
                {{ toClient(row).status }}
              </p>
              <p>
                <strong>Webhook:</strong>
                {{ toClient(row).webhookEnabled ? 'Ligado' : 'Desligado' }}
              </p>
              <p>
                <strong>Chave:</strong>
                {{ toClient(row).webhookKey || '-' }}
              </p>
              <p>
                <strong>Telefone:</strong>
                {{ toClient(row).contactPhone || '-' }}
              </p>
              <p>
                <strong>Site:</strong>
                {{ toClient(row).contactSite || '-' }}
              </p>
              <p>
                <strong>Endereco:</strong>
                {{ toClient(row).contactAddress || '-' }}
              </p>
              <p>
                <strong>Obriga loja:</strong>
                {{ toClient(row).requireUserStoreLink ? 'sim' : 'nao' }}
              </p>
              <p>
                <strong>Obriga matricula:</strong>
                {{ toClient(row).requireUserRegistration ? 'sim' : 'nao' }}
              </p>
              <p>
                <strong>Modulos:</strong>
                {{ modulesSummary(toClient(row)) }}
              </p>
            </div>
          </OmniMinimalPopover>

          <UButton
            v-if="canDeleteClient"
            icon="i-lucide-trash-2"
            color="error"
            variant="ghost"
            size="sm"
            aria-label="Excluir"
            :loading="deletingId === rowIdValue(row)"
            @click="onDeleteClient(rowIdValue(row))"
          />
        </div>
      </template>
    </OmniDataTable>
  </section>
</template>
