<script setup lang="ts">
import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniMinimalPopover from '~/components/omni/overlay/OmniMinimalPopover.vue'
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import type { AdminOrganizationFieldKey, AdminOrganizationItem } from '~/types/admin-organizations'
import type {
  OmniFilterDefinition,
  OmniFocusCell,
  OmniTableCellUpdate,
  OmniTableColumn,
} from '~/types/omni/collection'

const {
  organizations,
  loading,
  creating,
  deletingId,
  errorMessage,
  savingMap,
  fetchOrganizations,
  updateField,
  createOrganization,
  deleteOrganization,
} = useAdminOrganizationsManager()

const auth = useAuthStore()
const canCreate = computed(() => auth.role === 'platform_admin')
const canDelete = computed(() => auth.role === 'platform_admin')

const selectedIds = ref<Array<string | number>>([])
const focusCell = ref<OmniFocusCell | null>(null)

const filtersState = ref<Record<string, unknown>>({
  query: '',
  statusFilter: '',
})

const filterDefinitions = computed<OmniFilterDefinition[]>(() => [
  { key: 'query', label: 'Buscar', type: 'text', placeholder: 'Nome ou slug...', mode: 'all' },
  {
    key: 'statusFilter',
    label: 'Status',
    type: 'select',
    placeholder: 'Status',
    options: [
      { label: 'Ativa', value: true },
      { label: 'Inativa', value: false },
    ],
    accessor: (row) => row.isActive,
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
  { key: 'slug', label: 'Slug', type: 'text', editable: true, minWidth: 180, defaultOrder: 20 },
  {
    key: 'isActive',
    label: 'Ativa',
    type: 'switch',
    editable: true,
    immediate: true,
    minWidth: 110,
    defaultOrder: 30,
  },
  {
    key: 'accountCount',
    label: 'Qtd contas',
    type: 'number',
    editable: false,
    minWidth: 120,
    defaultOrder: 40,
  },
  {
    key: 'accountNames',
    label: 'Contas',
    type: 'text',
    editable: false,
    minWidth: 280,
    defaultOrder: 50,
  },
  {
    key: 'actions',
    label: 'Opcoes',
    type: 'custom',
    minWidth: 130,
    align: 'center',
    defaultOrder: 1000,
  },
])

const columnExcludeKeys = ['actions']
const { visibleColumnKeys, lockedColumnKeys, columnOrder, tableColumns, resetToDefaults } =
  useOmniVisibleColumns({
    preferenceKey: 'admin.manage.organizations',
    allColumns: allTableColumns,
    columnExcludeKeys,
  })

const filteredRows = computed(() => {
  const rows = organizations.value as unknown as Array<Record<string, unknown>>
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

const updatableFields = new Set<AdminOrganizationFieldKey>(['name', 'slug', 'isActive'])

function toOrg(row: Record<string, unknown>) {
  return row as unknown as AdminOrganizationItem
}

function rowId(row: Record<string, unknown>) {
  return String(row.id ?? '').trim()
}

function onCellUpdate(payload: OmniTableCellUpdate) {
  const id = String(payload.rowId).trim()
  if (!id) return
  const field = String(payload.key) as AdminOrganizationFieldKey
  if (!updatableFields.has(field)) return
  updateField(id, field, payload.value, { immediate: payload.immediate })
}

const createDialogOpen = ref(false)
const createForm = reactive({ slug: '', name: '' })

function openCreate() {
  createForm.slug = ''
  createForm.name = ''
  createDialogOpen.value = true
}

async function submitCreate() {
  if (!canCreate.value) return
  const createdId = await createOrganization({ ...createForm })
  if (!createdId) return
  createDialogOpen.value = false
  focusCell.value = { rowId: createdId, columnKey: 'name', token: Date.now() }
}

function onResetFilters() {
  filtersState.value = { query: '', statusFilter: '' }
}

async function onDelete(id: string) {
  if (!canDelete.value) return
  if (
    import.meta.client &&
    !window.confirm(
      'Desativar esta organization? Accounts vinculadas continuam funcionando, so perdem o agrupamento.',
    )
  )
    return
  await deleteOrganization(id)
}

onMounted(() => {
  void fetchOrganizations()
})

// Controle de open state do popover de info (OmniMinimalPopover é controlled).
const openInfoPopoverFor = ref<string | null>(null)
function infoOpen(id: string) {
  return openInfoPopoverFor.value === id
}
function setInfoOpen(id: string, value: boolean) {
  openInfoPopoverFor.value = value ? id : null
}
</script>

<template>
  <section class="admin-orgs-workspace flex h-full min-h-0 flex-col gap-4 overflow-hidden">
    <AdminPageHeader
      eyebrow="Admin"
      title="Organizations"
      description="Agencias que agrupam contas. Cada conta pode pertencer a uma organization (opcional). Backed pela API real /v1/admin/organizations."
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
          icon="i-lucide-plus"
          label="Nova organization"
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

    <div class="admin-orgs-workspace__table-scroll flex-1 min-h-0 overflow-y-auto">
      <OmniDataTable
        v-model="selectedIds"
        :rows="tableRows"
        :columns="tableColumns"
        row-key="id"
        :loading="loading"
        :focus-cell="focusCell"
        empty-text="Nenhuma organization encontrada com os filtros atuais."
        @update:cell="onCellUpdate"
      >
        <template #cell-actions="{ row }">
          <div class="flex items-center justify-end gap-1">
            <OmniMinimalPopover
              :open="infoOpen(rowId(row))"
              title="Detalhes"
              width-class="w-[280px] max-w-[90vw]"
              @update:open="setInfoOpen(rowId(row), $event)"
            >
              <template #trigger>
                <UButton
                  icon="i-lucide-info"
                  color="neutral"
                  variant="ghost"
                  size="sm"
                  title="Detalhes da organization"
                  aria-label="Info"
                />
              </template>
              <div class="space-y-1 text-xs">
                <p>
                  <strong>ID:</strong>
                  {{ toOrg(row).id }}
                </p>
                <p>
                  <strong>Nome:</strong>
                  {{ toOrg(row).name }}
                </p>
                <p>
                  <strong>Slug:</strong>
                  {{ toOrg(row).slug }}
                </p>
                <p>
                  <strong>Ativa:</strong>
                  {{ toOrg(row).isActive ? 'sim' : 'nao' }}
                </p>
                <p>
                  <strong>Qtd contas:</strong>
                  {{ toOrg(row).accountCount }}
                </p>
                <p>
                  <strong>Contas:</strong>
                  {{ toOrg(row).accountNames || '-' }}
                </p>
              </div>
            </OmniMinimalPopover>

            <UButton
              v-if="canDelete"
              icon="i-lucide-trash-2"
              color="error"
              variant="ghost"
              size="sm"
              title="Desativar organization"
              aria-label="Desativar"
              :loading="deletingId === rowId(row)"
              @click="onDelete(rowId(row))"
            />
          </div>
        </template>
      </OmniDataTable>
    </div>

    <UModal v-model:open="createDialogOpen">
      <template #content>
        <UCard>
          <template #header>
            <h3 class="text-base font-semibold">Nova organization</h3>
          </template>

          <div class="space-y-3">
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Nome</label>
              <UInput
                :model-value="createForm.name"
                placeholder="Nome da agencia"
                @update:model-value="createForm.name = String($event ?? '')"
              />
            </div>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Slug</label>
              <UInput
                :model-value="createForm.slug"
                placeholder="nome-agencia (lowercase, sem espacos)"
                @update:model-value="createForm.slug = String($event ?? '')"
              />
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
