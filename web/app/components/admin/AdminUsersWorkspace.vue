<script setup lang="ts">
import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import AdminUserCreateDialog from './users/AdminUserCreateDialog.vue'
import AdminUserPasswordDialog from './users/AdminUserPasswordDialog.vue'
import AdminUsersActionsCell from './users/AdminUsersActionsCell.vue'
import { provideAdminUsersContext } from '~/composables/useAdminUsersManager'
import type { AdminUserFieldKey, AdminUserItem } from '~/types/admin-users'
import type {
  OmniFilterDefinition,
  OmniFocusCell,
  OmniTableCellUpdate,
  OmniTableColumn,
} from '~/types/omni/collection'

// Host do manager: cria UMA instancia e a provê (provide/inject) para o drawer e os
// panels descendentes compartilharem o mesmo estado/acoes (antes cada um instanciava
// o seu, com estado desconectado). Os consumidores seguem chamando useAdminUsersManager().
// createUser/setPassword nao sao desestruturados aqui: vivem nos modais filhos
// (AdminUserCreateDialog/AdminUserPasswordDialog), que usam o mesmo manager via inject.
const {
  users,
  filters,
  page,
  perPage,
  total,
  loading,
  creating,
  deletingId,
  errorMessage,
  fetchUsers,
  updateField,
  deleteUser,
  moveUserAccount,
} = provideAdminUsersContext()

const auth = useAuthStore()
const isPlatformAdmin = computed(() => auth.role === 'platform_admin')
// Visualizar/editar usuarios e DELEGADO: platform_admin OU quem tem core.users.manage.
// A listagem e a editabilidade fina sao escopadas no backend (404/403 fora do escopo);
// o front so precisa nao esconder a pagina do delegado (padrao isPlatformAdmin || has).
const canViewUsers = computed(
  () => isPlatformAdmin.value || auth.permissionKeys.includes('core.users.manage'),
)
// Criar/excluir usuario sao identity-global → so platform_admin (o backend restringe
// POST/DELETE/PUT account a platform_admin; abrir esses botoes a delegado so daria 403).
const canCreateUser = computed(() => isPlatformAdmin.value)
const canDeleteUser = computed(() => isPlatformAdmin.value)

// Opcoes de vinculo no modal de criacao: cliente (account) + agencia (organization).
// Diferente de /operacao/usuarios (que ja esta dentro de um cliente), aqui no admin
// global o cliente/agencia precisam ser escolhidos.
const clientsManager = useClientsManager()
const orgsManager = useAdminOrganizationsManager()
const accountOptions = computed(() => [
  { value: '', label: 'Sem cliente' },
  ...clientsManager.clients.value.map((c) => ({ value: c.id, label: c.name })),
])
const organizationOptions = computed(() => [
  { value: '', label: 'Sem agencia' },
  ...orgsManager.organizations.value.map((o) => ({ value: o.id, label: o.name })),
])
// So clientes reais (sem a opcao vazia) — usado no filtro server-side por cliente
// e no <select> inline da coluna "Cliente". Agencias entram em organizationOptions,
// nunca aqui (mover para uma conta-agencia e' 400 no backend).
const clientOptions = computed(() =>
  clientsManager.clients.value
    .filter((c) => !c.isAgency)
    .map((c) => ({ value: c.id, label: c.name })),
)
const clientNameById = computed(() => {
  const map = new Map<string, string>()
  for (const c of clientsManager.clients.value) map.set(c.id, c.name)
  return map
})

const selectedIds = ref<Array<string | number>>([])
const focusCell = ref<OmniFocusCell | null>(null)

const filtersState = ref<Record<string, unknown>>({
  query: '',
  statusFilter: '',
  platformAdminFilter: '',
  clientFilter: '',
})

const filterDefinitions = computed<OmniFilterDefinition[]>(() => [
  {
    key: 'query',
    label: 'Buscar',
    type: 'text',
    placeholder: 'Email, nome ou nick...',
    mode: 'all',
  },
  {
    key: 'statusFilter',
    label: 'Status',
    type: 'select',
    placeholder: 'Status',
    options: [
      { label: 'Ativo', value: true },
      { label: 'Inativo', value: false },
    ],
    accessor: (row) => row.isActive,
  },
  {
    key: 'platformAdminFilter',
    label: 'Tipo',
    type: 'select',
    placeholder: 'Tipo',
    options: [
      { label: 'Platform admin', value: true },
      { label: 'Membro de conta', value: false },
    ],
    accessor: (row) => row.isPlatformAdmin,
  },
  {
    key: 'clientFilter',
    label: 'Cliente',
    type: 'select',
    placeholder: 'Cliente',
    // Filtro aplicado NO SERVIDOR (accountId) — traduzido em syncFiltersToBackend.
    options: clientOptions.value,
  },
])

const allTableColumns = computed<OmniTableColumn[]>(() => [
  {
    key: 'email',
    label: 'Email',
    type: 'text',
    editable: true,
    minWidth: 240,
    focusOnCreate: true,
    defaultOrder: 10,
  },
  {
    key: 'displayName',
    label: 'Nome',
    type: 'text',
    editable: true,
    minWidth: 200,
    locked: true,
    defaultOrder: 20,
  },
  { key: 'nick', label: 'Nick', type: 'text', editable: true, minWidth: 140, defaultOrder: 30 },
  {
    key: 'isActive',
    label: 'Ativo',
    type: 'switch',
    editable: true,
    immediate: true,
    minWidth: 110,
    defaultOrder: 40,
  },
  {
    key: 'isPlatformAdmin',
    label: 'Platform admin',
    type: 'switch',
    editable: true,
    immediate: true,
    minWidth: 150,
    defaultOrder: 50,
  },
  {
    key: 'accountCount',
    label: 'Qtd clientes',
    type: 'number',
    editable: false,
    minWidth: 120,
    defaultOrder: 60,
  },
  {
    // Coluna "Cliente": custom para permitir edicao inline (mover de cliente) via
    // <select> quando o usuario tem exatamente 1 cliente e nao e platform_admin.
    // Caso contrario, renderiza os nomes read-only (slot #cell-accountNames).
    key: 'accountNames',
    label: 'Cliente',
    type: 'custom',
    editable: false,
    minWidth: 260,
    defaultOrder: 70,
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
    preferenceKey: 'admin.manage.users',
    allColumns: allTableColumns,
    columnExcludeKeys,
  })

// Os filtros sao aplicados NO SERVIDOR (q/status/platformAdmin). Aqui so
// deduplicamos por id por seguranca; nao ha filtragem client-side sobre o
// conjunto inteiro (que antes exigia baixar todos os usuarios).
const tableRows = computed(() => {
  const seen = new Set<string>()
  return (users.value as unknown as Array<Record<string, unknown>>).filter((row) => {
    const id = String((row as Record<string, unknown>).id ?? '').trim()
    if (!id || seen.has(id)) return false
    seen.add(id)
    return true
  })
})

// Traduz o estado dos filtros da UI -> parametros do backend e dispara o fetch
// (debounced). statusFilter/platformAdminFilter sao boolean | '' na UI.
function syncFiltersToBackend() {
  filters.q = String(filtersState.value.query ?? '')
  const status = filtersState.value.statusFilter
  filters.status = status === true ? 'active' : status === false ? 'inactive' : ''
  const admin = filtersState.value.platformAdminFilter
  filters.platformAdmin = admin === true ? 'true' : admin === false ? 'false' : ''
  filters.accountId = String(filtersState.value.clientFilter ?? '')
}

let filterTimer: ReturnType<typeof setTimeout> | null = null
watch(
  filtersState,
  () => {
    if (!canViewUsers.value) return
    if (filterTimer) clearTimeout(filterTimer)
    filterTimer = setTimeout(() => {
      syncFiltersToBackend()
      void fetchUsers({ page: 1 })
    }, 300)
  },
  { deep: true },
)

onBeforeUnmount(() => {
  if (filterTimer) clearTimeout(filterTimer)
})

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / perPage.value)))
const rangeLabel = computed(() => {
  if (total.value === 0) return 'Nenhum usuario'
  const start = (page.value - 1) * perPage.value + 1
  const end = Math.min(page.value * perPage.value, total.value)
  return `${start}-${end} de ${total.value}`
})

function onPageChange(next: number) {
  if (!canViewUsers.value) return
  void fetchUsers({ page: next })
}

const updatableFields = new Set<AdminUserFieldKey>([
  'email',
  'displayName',
  'nick',
  'isActive',
  'isPlatformAdmin',
])

function toUser(row: Record<string, unknown>) {
  return row as unknown as AdminUserItem
}

function rowId(row: Record<string, unknown>) {
  return String(row.id ?? '').trim()
}

function onCellUpdate(payload: OmniTableCellUpdate) {
  const id = String(payload.rowId).trim()
  if (!id) return
  const field = String(payload.key) as AdminUserFieldKey
  if (!updatableFields.has(field)) return
  updateField(id, field, payload.value, { immediate: payload.immediate })
}

// Atribuir/mover cliente inline (coluna "Cliente"). Habilitado para usuarios que
// nao sao platform_admin nem membros de agencia — inclui quem ainda NAO tem cliente
// (clientAccountId == ''), que antes ficava travado num "-" morto. Reusa
// moveUserAccount: para 0 vinculos ele apenas matricula (vincula); para 1 vinculo
// ele move (substitui). Confirma antes com a mensagem certa para cada caso.
const movingId = ref<string | null>(null)
async function onAssignClient(row: Record<string, unknown>, accountId: string) {
  const user = toUser(row)
  const id = rowId(row)
  const target = String(accountId ?? '').trim()
  if (!id || !target || target === user.clientAccountId) return
  const targetName = clientNameById.value.get(target) ?? 'este cliente'
  const message = user.clientAccountId
    ? `Mover este usuario para o cliente ${targetName}? Ele perde acesso ao cliente atual.`
    : `Vincular este usuario ao cliente ${targetName} como owner?`
  if (import.meta.client && !window.confirm(message)) return
  movingId.value = id
  await moveUserAccount(id, target)
  movingId.value = null
}

// Drawer de edicao por usuario (dados + senha + vinculos/nivel). O modulo/pagina
// por usuario (Fase 1B) entra dentro do proprio drawer.
const editDrawerOpen = ref(false)
const editUser = ref<AdminUserItem | null>(null)
function openEdit(row: Record<string, unknown>) {
  editUser.value = toUser(row)
  editDrawerOpen.value = true
}
function onUserUpdated() {
  void fetchUsers()
}

// Modal de criacao (AdminUserCreateDialog): o host so controla a abertura e popula
// as opcoes; a logica de form/validacao/submit vive no proprio dialog. Ao abrir,
// carrega clientes/agencias para os selects (mesmo comportamento do openCreate antigo).
const createDialogOpen = ref(false)
function openCreate() {
  createDialogOpen.value = true
  void clientsManager.fetchClients()
  void orgsManager.fetchOrganizations()
}
function onUserCreated(createdId: string) {
  focusCell.value = { rowId: createdId, columnKey: 'email', token: Date.now() }
}

// Definir/Resetar senha de um usuario ja criado (acao explicita, so platform_admin).
// O backend so toca no password_hash quando recebe `password` nao-vazio. O modal
// (AdminUserPasswordDialog) cuida do form/validacao/submit; o host so abre e alveja.
const passwordDialogOpen = ref(false)
const passwordTarget = ref<{ id: string; email: string } | null>(null)
function openPassword(row: Record<string, unknown>) {
  if (!canCreateUser.value) return
  passwordTarget.value = { id: rowId(row), email: String(toUser(row).email ?? '') }
  passwordDialogOpen.value = true
}

function onResetFilters() {
  filtersState.value = { query: '', statusFilter: '', platformAdminFilter: '', clientFilter: '' }
  syncFiltersToBackend()
  void fetchUsers({ page: 1 })
}

async function onDeleteUser(id: string) {
  if (!canDeleteUser.value) return
  if (
    import.meta.client &&
    !window.confirm('Desativar este usuario? Ele perde acesso imediato em todas as contas.')
  )
    return
  await deleteUser(id)
}

// Memberships/detalhes por linha + estado dos popovers vivem na celula de acoes
// (AdminUsersActionsCell), que busca via o manager compartilhado sob demanda.

onMounted(() => {
  // Gate por permissao ANTES de disparar o fetch (espelha o back: rota exige
  // platform_admin). Evita 403 de ruido no bootstrap.
  if (!canViewUsers.value) return
  void fetchUsers()
  // Carrega os clientes para popular o filtro "Cliente" e o <select> inline de
  // mover usuario de cliente. So admin (a tela ja e gateada por canViewUsers).
  void clientsManager.fetchClients()
})
</script>

<template>
  <section class="admin-users-workspace flex h-full min-h-0 flex-col gap-4 overflow-hidden">
    <AdminPageHeader
      eyebrow="Admin"
      title="Usuarios"
      description="Visao cross-account de todos os usuarios da plataforma. Inclui memberships e flag de platform admin. Backed pela API real /v1/admin/users."
    />

    <OmniCollectionFilters
      v-model="filtersState"
      v-model:visible-columns="visibleColumnKeys"
      v-model:locked-columns="lockedColumnKeys"
      v-model:column-order="columnOrder"
      :viewer-user-type="canCreateUser ? 'admin' : 'client'"
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
          icon="i-lucide-plus"
          label="Novo usuario"
          color="primary"
          :loading="creating"
          :disabled="creating || !canCreateUser"
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

    <div class="admin-users-workspace__table-scroll flex-1 min-h-0 overflow-y-auto">
      <OmniDataTable
        v-model="selectedIds"
        :rows="tableRows"
        :columns="tableColumns"
        row-key="id"
        :loading="loading"
        :focus-cell="focusCell"
        empty-text="Nenhum usuario encontrado com os filtros atuais."
        @update:cell="onCellUpdate"
      >
        <template #cell-accountNames="{ row }">
          <select
            v-if="!toUser(row).isPlatformAdmin && !toUser(row).isAgencyMember"
            class="w-full rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-3 py-2 text-sm"
            :class="{ 'text-[rgb(var(--muted))]': !toUser(row).clientAccountId }"
            :value="toUser(row).clientAccountId"
            :disabled="movingId === rowId(row) || loading"
            :title="toUser(row).accountNames || 'Sem cliente — selecione para vincular'"
            @change="onAssignClient(row, ($event.target as HTMLSelectElement).value)"
          >
            <option v-if="!toUser(row).clientAccountId" value="" disabled>
              Sem cliente — vincular...
            </option>
            <option v-for="opt in clientOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
          <span v-else class="flex items-center gap-1.5 text-sm" :title="toUser(row).accountNames">
            <UBadge v-if="toUser(row).isAgencyMember" color="primary" variant="soft" size="xs">
              Agencia
            </UBadge>
            <span class="line-clamp-1">{{ toUser(row).accountNames || '-' }}</span>
          </span>
        </template>

        <template #cell-actions="{ row }">
          <AdminUsersActionsCell
            :user="toUser(row)"
            :can-view="canViewUsers"
            :can-manage="canCreateUser"
            :can-delete="canDeleteUser"
            :deleting="deletingId === rowId(row)"
            @edit="openEdit(row)"
            @password="openPassword(row)"
            @delete="onDeleteUser(rowId(row))"
          />
        </template>
      </OmniDataTable>
    </div>

    <div
      v-if="totalPages > 1 || total > 0"
      class="admin-users-workspace__pagination flex items-center justify-between gap-3 px-1 py-2"
    >
      <span class="text-xs text-[rgb(var(--muted))]">{{ rangeLabel }}</span>
      <UPagination
        v-if="totalPages > 1"
        :page="page"
        :total="total"
        :items-per-page="perPage"
        :sibling-count="1"
        show-edges
        size="sm"
        @update:page="onPageChange"
      />
    </div>

    <AdminUserCreateDialog
      v-model:open="createDialogOpen"
      :can-create="canCreateUser"
      :account-options="accountOptions"
      :organization-options="organizationOptions"
      @created="onUserCreated"
    />

    <AdminUserPasswordDialog v-model:open="passwordDialogOpen" :target="passwordTarget" />

    <AdminUserEditDrawer v-model:open="editDrawerOpen" :user="editUser" @updated="onUserUpdated" />
  </section>
</template>
