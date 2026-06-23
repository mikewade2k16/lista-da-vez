<script setup lang="ts">
import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniMinimalPopover from '~/components/omni/overlay/OmniMinimalPopover.vue'
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import type { AccountMembershipItem, AdminUserFieldKey, AdminUserItem } from '~/types/admin-users'
import type {
  OmniFilterDefinition,
  OmniFocusCell,
  OmniTableCellUpdate,
  OmniTableColumn,
} from '~/types/omni/collection'

// Espelha o minimo do backend (admin_users_service.go: "must be at least 8 chars").
const PASSWORD_MIN_LENGTH = 8

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
  createUser,
  deleteUser,
  setPassword,
  moveUserAccount,
  fetchMemberships,
} = useAdminUsersManager()

const auth = useAuthStore()
const canViewUsers = computed(() => auth.role === 'platform_admin')
const canCreateUser = computed(() => auth.role === 'platform_admin')
const canDeleteUser = computed(() => auth.role === 'platform_admin')

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

// Move inline de cliente (coluna "Cliente"). So habilitado quando o usuario tem
// exatamente 1 cliente (clientAccountId != '') e nao e platform_admin. Confirma
// antes; em sucesso o composable ja aplica o user retornado na linha.
const movingId = ref<string | null>(null)
async function onMoveClient(row: Record<string, unknown>, accountId: string) {
  const user = toUser(row)
  const id = rowId(row)
  const target = String(accountId ?? '').trim()
  if (!id || !target || target === user.clientAccountId) return
  const targetName = clientNameById.value.get(target) ?? 'este cliente'
  if (
    import.meta.client &&
    !window.confirm(
      `Mover este usuario para o cliente ${targetName}? Ele perde acesso ao cliente atual.`,
    )
  )
    return
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

const createDialogOpen = ref(false)
const createForm = reactive({
  email: '',
  displayName: '',
  nick: '',
  isPlatformAdmin: false,
  temporaryPassword: '',
  accountId: '',
  organizationId: '',
  role: 'owner',
  orgRole: 'agency_member',
})

function openCreate() {
  createForm.email = ''
  createForm.displayName = ''
  createForm.nick = ''
  createForm.isPlatformAdmin = false
  createForm.temporaryPassword = ''
  createForm.accountId = ''
  createForm.organizationId = ''
  createForm.role = 'owner'
  createForm.orgRole = 'agency_member'
  createAgencyConfirmed.value = false
  createDialogOpen.value = true
  void clientsManager.fetchClients()
  void orgsManager.fetchOrganizations()
}

// Senha na criacao: opcional (vazia = fluxo de convite), mas se preenchida tem
// que respeitar o minimo do backend. Bloqueia o submit com hint inline.
const createPasswordError = computed(() => {
  const pw = createForm.temporaryPassword.trim()
  if (!pw) return ''
  return pw.length < PASSWORD_MIN_LENGTH ? `Minimo de ${PASSWORD_MIN_LENGTH} caracteres.` : ''
})

// Um usuario sem cliente/agencia e sem ser platform_admin nao consegue logar (sem
// papel resolvido o login falha). Evita criar um usuario "inutil": exige cliente
// (com papel), agencia (com cargo) OU a flag de platform admin.
const createNeedsClient = computed(
  () => !createForm.isPlatformAdmin && !createForm.accountId && !createForm.organizationId,
)

// Vincular Cliente + Agencia juntos torna o usuario MEMBRO DA AGENCIA (ve todos os
// clientes/modulos da agencia) — perigoso para um usuario que deveria ser so deste
// cliente. Quando os dois selects estao preenchidos, exigimos confirmacao explicita.
const createBindsClientAndAgency = computed(
  () => Boolean(createForm.accountId) && Boolean(createForm.organizationId),
)
const createAgencyConfirmed = ref(false)

async function submitCreate() {
  if (!canCreateUser.value || createPasswordError.value || createNeedsClient.value) return
  if (createBindsClientAndAgency.value && !createAgencyConfirmed.value) return
  const createdId = await createUser({ ...createForm })
  if (!createdId) return
  createDialogOpen.value = false
  focusCell.value = { rowId: createdId, columnKey: 'email', token: Date.now() }
}

// Definir/Resetar senha de um usuario ja criado (acao explicita, so platform_admin).
// O backend so toca no password_hash quando recebe `password` nao-vazio.
const passwordDialogOpen = ref(false)
const passwordTarget = ref<{ id: string; email: string } | null>(null)
const passwordValue = ref('')
const passwordSaving = ref(false)
const passwordError = computed(() => {
  const pw = passwordValue.value.trim()
  if (!pw) return ''
  return pw.length < PASSWORD_MIN_LENGTH ? `Minimo de ${PASSWORD_MIN_LENGTH} caracteres.` : ''
})

function openPassword(row: Record<string, unknown>) {
  if (!canCreateUser.value) return
  passwordTarget.value = { id: rowId(row), email: String(toUser(row).email ?? '') }
  passwordValue.value = ''
  passwordDialogOpen.value = true
}

async function submitPassword() {
  const target = passwordTarget.value
  const pw = passwordValue.value.trim()
  if (!target || pw.length < PASSWORD_MIN_LENGTH) return
  passwordSaving.value = true
  const ok = await setPassword(target.id, pw)
  passwordSaving.value = false
  if (ok) passwordDialogOpen.value = false
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

const membershipsOpenFor = ref<string | null>(null)
const memberships = ref<AccountMembershipItem[]>([])

async function openMemberships(id: string) {
  membershipsOpenFor.value = id
  memberships.value = await fetchMemberships(id)
}

// Controle de open state dos popovers — OmniMinimalPopover é controlled.
// Sem isto, click no trigger não abre o painel.
const openPopovers = reactive<Record<string, boolean>>({})

function popoverIsOpen(rowId: string, type: 'memberships' | 'info') {
  return Boolean(openPopovers[`${rowId}:${type}`])
}

function setPopoverOpen(rowId: string, type: 'memberships' | 'info', value: boolean) {
  const key = `${rowId}:${type}`
  if (value) openPopovers[key] = true
  else delete openPopovers[key]
}

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
            v-if="toUser(row).clientAccountId && !toUser(row).isPlatformAdmin"
            class="w-full rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-3 py-2 text-sm"
            :value="toUser(row).clientAccountId"
            :disabled="movingId === rowId(row) || loading"
            :title="toUser(row).accountNames"
            @change="onMoveClient(row, ($event.target as HTMLSelectElement).value)"
          >
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
          <div class="flex items-center justify-end gap-1">
            <OmniMinimalPopover
              :open="popoverIsOpen(rowId(row), 'memberships')"
              title="Clientes (memberships)"
              width-class="w-[300px] max-w-[90vw]"
              @update:open="setPopoverOpen(rowId(row), 'memberships', $event)"
              @opened="openMemberships(rowId(row))"
            >
              <template #trigger>
                <UButton
                  icon="i-lucide-building-2"
                  color="neutral"
                  variant="ghost"
                  size="sm"
                  title="Contas que este usuario participa"
                  aria-label="Memberships"
                />
              </template>
              <div v-if="membershipsOpenFor === rowId(row)" class="space-y-2 text-xs">
                <p v-if="memberships.length === 0" class="text-[rgb(var(--muted))]">
                  Este usuario nao e membro de nenhuma conta.
                </p>
                <ul v-else class="space-y-1">
                  <li
                    v-for="m in memberships"
                    :key="m.accountId"
                    class="flex items-center justify-between gap-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-2 py-1"
                  >
                    <span class="font-medium">{{ m.accountName }}</span>
                    <span class="text-[rgb(var(--muted))]">{{ m.accountSlug }}</span>
                    <UBadge :color="m.isActive ? 'success' : 'neutral'" variant="soft" size="xs">
                      {{ m.isActive ? 'ativo' : 'inativo' }}
                    </UBadge>
                  </li>
                </ul>
              </div>
            </OmniMinimalPopover>

            <OmniMinimalPopover
              :open="popoverIsOpen(rowId(row), 'info')"
              title="Detalhes"
              width-class="w-[280px] max-w-[90vw]"
              @update:open="setPopoverOpen(rowId(row), 'info', $event)"
            >
              <template #trigger>
                <UButton
                  icon="i-lucide-info"
                  color="neutral"
                  variant="ghost"
                  size="sm"
                  title="Detalhes do usuario"
                  aria-label="Info"
                />
              </template>
              <div class="space-y-1 text-xs">
                <p>
                  <strong>ID:</strong>
                  {{ toUser(row).id }}
                </p>
                <p>
                  <strong>Email:</strong>
                  {{ toUser(row).email }}
                </p>
                <p>
                  <strong>Nome:</strong>
                  {{ toUser(row).displayName }}
                </p>
                <p>
                  <strong>Nick:</strong>
                  {{ toUser(row).nick || '-' }}
                </p>
                <p>
                  <strong>Platform admin:</strong>
                  {{ toUser(row).isPlatformAdmin ? 'sim' : 'nao' }}
                </p>
                <p>
                  <strong>Trocar senha:</strong>
                  {{ toUser(row).mustChangePassword ? 'sim' : 'nao' }}
                </p>
                <p>
                  <strong>Qtd clientes:</strong>
                  {{ toUser(row).accountCount }}
                </p>
                <p>
                  <strong>Cliente:</strong>
                  {{ toUser(row).accountNames || '-' }}
                </p>
                <p>
                  <strong>Membro de agencia:</strong>
                  {{ toUser(row).isAgencyMember ? 'sim' : 'nao' }}
                </p>
              </div>
            </OmniMinimalPopover>

            <UButton
              v-if="canCreateUser"
              icon="i-lucide-pencil"
              color="neutral"
              variant="ghost"
              size="sm"
              title="Editar usuario (dados, nivel, senha)"
              aria-label="Editar"
              @click="openEdit(row)"
            />

            <UButton
              v-if="canCreateUser"
              icon="i-lucide-key-round"
              color="neutral"
              variant="ghost"
              size="sm"
              title="Definir/Resetar senha"
              aria-label="Definir senha"
              @click="openPassword(row)"
            />

            <UButton
              v-if="canDeleteUser"
              icon="i-lucide-trash-2"
              color="error"
              variant="ghost"
              size="sm"
              title="Desativar usuario"
              aria-label="Excluir"
              :loading="deletingId === rowId(row)"
              @click="onDeleteUser(rowId(row))"
            />
          </div>
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

    <UModal v-model:open="createDialogOpen">
      <template #content>
        <UCard>
          <template #header>
            <h3 class="text-base font-semibold">Novo usuario</h3>
          </template>

          <div class="space-y-3">
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Email</label>
              <UInput
                :model-value="createForm.email"
                placeholder="usuario@exemplo.com"
                @update:model-value="createForm.email = String($event ?? '')"
              />
            </div>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Nome</label>
              <UInput
                :model-value="createForm.displayName"
                placeholder="Nome completo"
                @update:model-value="createForm.displayName = String($event ?? '')"
              />
            </div>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Nick (opcional)</label>
              <UInput
                :model-value="createForm.nick"
                placeholder="apelido curto"
                @update:model-value="createForm.nick = String($event ?? '')"
              />
            </div>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">
                Senha temporaria (opcional — se vazia, user precisa convite)
              </label>
              <UInput
                :model-value="createForm.temporaryPassword"
                type="password"
                placeholder="minimo 8 chars"
                @update:model-value="createForm.temporaryPassword = String($event ?? '')"
              />
              <p v-if="createPasswordError" class="text-xs text-[rgb(var(--danger))] mt-1">
                {{ createPasswordError }}
              </p>
            </div>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Cliente (opcional)</label>
              <select
                class="w-full rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-3 py-2 text-sm"
                :value="createForm.accountId"
                @change="createForm.accountId = ($event.target as HTMLSelectElement).value"
              >
                <option v-for="opt in accountOptions" :key="opt.value" :value="opt.value">
                  {{ opt.label }}
                </option>
              </select>
              <p v-if="createNeedsClient" class="text-xs text-[rgb(var(--danger))] mt-1">
                Selecione um cliente, uma agencia (abaixo) ou marque platform admin — senao o
                usuario nao consegue logar.
              </p>
            </div>
            <div v-if="createForm.accountId">
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Papel no cliente</label>
              <select
                class="w-full rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-3 py-2 text-sm"
                :value="createForm.role"
                @change="createForm.role = ($event.target as HTMLSelectElement).value"
              >
                <option value="owner">Owner (acesso total do cliente)</option>
                <option value="director">Director</option>
                <option value="marketing">Marketing</option>
              </select>
              <p class="text-xs text-[rgb(var(--muted))] mt-1">
                Cria o papel legado (login + operacao). Sem isso o usuario nao consegue entrar.
              </p>
            </div>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Agencia (opcional)</label>
              <select
                class="w-full rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-3 py-2 text-sm"
                :value="createForm.organizationId"
                @change="createForm.organizationId = ($event.target as HTMLSelectElement).value"
              >
                <option v-for="opt in organizationOptions" :key="opt.value" :value="opt.value">
                  {{ opt.label }}
                </option>
              </select>
            </div>
            <div v-if="createForm.organizationId">
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Cargo na agencia</label>
              <select
                class="w-full rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-3 py-2 text-sm"
                :value="createForm.orgRole"
                @change="createForm.orgRole = ($event.target as HTMLSelectElement).value"
              >
                <option value="agency_owner">Dono da agencia (acesso total)</option>
                <option value="agency_member">Membro (acesso limitado)</option>
              </select>
              <p class="text-xs text-[rgb(var(--muted))] mt-1">
                O cargo define o acesso: dono ve tudo da agencia; membro tem acesso limitado. Ele
                entra como membro da conta-agencia e navega pelos clientes da agencia.
              </p>
            </div>
            <div
              v-if="createBindsClientAndAgency"
              class="rounded-[var(--radius-md)] border border-[rgb(var(--danger))] bg-[rgb(var(--surface-2))] px-3 py-2"
            >
              <p class="text-xs text-[rgb(var(--danger))]">
                Atencao: vincular uma agencia torna o usuario MEMBRO DA AGENCIA — ele passa a ver
                todos os clientes e modulos da agencia. Para um usuario so deste cliente, deixe
                Agencia vazio.
              </p>
              <label class="mt-2 flex items-center gap-2 text-xs">
                <input v-model="createAgencyConfirmed" type="checkbox" />
                <span>Entendo, e um membro de agencia</span>
              </label>
            </div>
            <div class="flex items-center gap-2">
              <USwitch v-model="createForm.isPlatformAdmin" />
              <span class="text-sm">Platform admin (acesso global)</span>
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
              <UButton
                label="Criar"
                color="primary"
                :loading="creating"
                :disabled="
                  creating ||
                  Boolean(createPasswordError) ||
                  createNeedsClient ||
                  (createBindsClientAndAgency && !createAgencyConfirmed)
                "
                @click="submitCreate"
              />
            </div>
          </template>
        </UCard>
      </template>
    </UModal>

    <UModal v-model:open="passwordDialogOpen">
      <template #content>
        <UCard>
          <template #header>
            <h3 class="text-base font-semibold">Definir senha</h3>
          </template>

          <div class="space-y-3">
            <p class="text-xs text-[rgb(var(--muted))]">
              Define uma nova senha para
              <strong>{{ passwordTarget?.email || 'este usuario' }}</strong>
              . O usuario passa a logar com ela imediatamente.
            </p>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Nova senha</label>
              <UInput
                :model-value="passwordValue"
                type="password"
                placeholder="minimo 8 chars"
                @update:model-value="passwordValue = String($event ?? '')"
                @keyup.enter="submitPassword"
              />
              <p v-if="passwordError" class="text-xs text-[rgb(var(--danger))] mt-1">
                {{ passwordError }}
              </p>
            </div>
          </div>

          <template #footer>
            <div class="flex justify-end gap-2">
              <UButton
                label="Cancelar"
                color="neutral"
                variant="ghost"
                @click="passwordDialogOpen = false"
              />
              <UButton
                label="Salvar senha"
                color="primary"
                :loading="passwordSaving"
                :disabled="passwordSaving || Boolean(passwordError) || !passwordValue.trim()"
                @click="submitPassword"
              />
            </div>
          </template>
        </UCard>
      </template>
    </UModal>

    <AdminUserEditDrawer v-model:open="editDrawerOpen" :user="editUser" @updated="onUserUpdated" />
  </section>
</template>
