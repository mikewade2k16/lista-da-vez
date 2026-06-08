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

const {
  users,
  loading,
  creating,
  deletingId,
  errorMessage,
  savingMap,
  fetchUsers,
  updateField,
  createUser,
  deleteUser,
  fetchMemberships,
} = useAdminUsersManager()

const auth = useAuthStore()
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

const selectedIds = ref<Array<string | number>>([])
const focusCell = ref<OmniFocusCell | null>(null)

const filtersState = ref<Record<string, unknown>>({
  query: '',
  statusFilter: '',
  platformAdminFilter: '',
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
    label: 'Qtd contas',
    type: 'number',
    editable: false,
    minWidth: 120,
    defaultOrder: 60,
  },
  {
    key: 'accountNames',
    label: 'Contas',
    type: 'text',
    editable: false,
    minWidth: 240,
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

const filteredRows = computed(() => {
  const rows = users.value as unknown as Array<Record<string, unknown>>
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
  createDialogOpen.value = true
  void clientsManager.fetchClients()
  void orgsManager.fetchOrganizations()
}

async function submitCreate() {
  if (!canCreateUser.value) return
  const createdId = await createUser({ ...createForm })
  if (!createdId) return
  createDialogOpen.value = false
  focusCell.value = { rowId: createdId, columnKey: 'email', token: Date.now() }
}

function onResetFilters() {
  filtersState.value = { query: '', statusFilter: '', platformAdminFilter: '' }
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
  void fetchUsers()
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
        <template #cell-actions="{ row }">
          <div class="flex items-center justify-end gap-1">
            <OmniMinimalPopover
              :open="popoverIsOpen(rowId(row), 'memberships')"
              title="Contas (memberships)"
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
                  <strong>Qtd contas:</strong>
                  {{ toUser(row).accountCount }}
                </p>
                <p>
                  <strong>Contas:</strong>
                  {{ toUser(row).accountNames || '-' }}
                </p>
              </div>
            </OmniMinimalPopover>

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
              <UButton label="Criar" color="primary" :loading="creating" @click="submitCreate" />
            </div>
          </template>
        </UCard>
      </template>
    </UModal>
  </section>
</template>
