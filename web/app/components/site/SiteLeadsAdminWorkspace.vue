<script setup lang="ts">
import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniMinimalPopover from '~/components/omni/overlay/OmniMinimalPopover.vue'
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import WebhookSourcesDrawer from '~/components/site/WebhookSourcesDrawer.vue'
import type { LeadCreateInput, LeadFieldKey, LeadItem } from '~/types/leads'
import type {
  OmniFilterDefinition,
  OmniFocusCell,
  OmniTableCellUpdate,
  OmniTableColumn,
} from '~/types/omni/collection'

const {
  leads,
  loading,
  creating,
  deletingId,
  errorMessage,
  savingMap,
  fetchLeads,
  updateField,
  createLead,
  deleteLead,
} = useLeadsManager()

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
})

const filterDefinitions = computed<OmniFilterDefinition[]>(() => [
  {
    key: 'query',
    label: 'Buscar',
    type: 'text',
    placeholder: 'Nome, email, telefone, cupom...',
    mode: 'all',
  },
  {
    key: 'statusFilter',
    label: 'Status',
    type: 'select',
    placeholder: 'Status',
    options: [
      { label: 'Novo', value: 'new' },
      { label: 'Contatado', value: 'contacted' },
      { label: 'Qualificado', value: 'qualified' },
      { label: 'Perdido', value: 'lost' },
    ],
    accessor: (row) => row.status,
  },
])

const allTableColumns = computed<OmniTableColumn[]>(() => [
  {
    key: 'nome',
    label: 'Nome',
    type: 'text',
    editable: true,
    minWidth: 200,
    focusOnCreate: true,
    locked: true,
    defaultOrder: 10,
  },
  { key: 'email', label: 'Email', type: 'text', editable: true, minWidth: 220, defaultOrder: 20 },
  {
    key: 'telefone',
    label: 'Telefone',
    type: 'text',
    editable: true,
    minWidth: 160,
    defaultOrder: 30,
  },
  {
    key: 'status',
    label: 'Status',
    type: 'select',
    editable: true,
    immediate: true,
    minWidth: 140,
    defaultOrder: 40,
    options: [
      { label: 'Novo', value: 'new' },
      { label: 'Contatado', value: 'contacted' },
      { label: 'Qualificado', value: 'qualified' },
      { label: 'Perdido', value: 'lost' },
    ],
  },
  {
    key: 'sourceLabel',
    label: 'Fonte',
    type: 'text',
    editable: false,
    minWidth: 160,
    defaultOrder: 50,
  },
  { key: 'page', label: 'Pagina', type: 'text', editable: false, minWidth: 140, defaultOrder: 60 },
  { key: 'cupom', label: 'Cupom', type: 'text', editable: false, minWidth: 130, defaultOrder: 70 },
  {
    key: 'createdAt',
    label: 'Captado em',
    type: 'text',
    editable: false,
    minWidth: 160,
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
    preferenceKey: 'admin.site.leads',
    allColumns: allTableColumns,
    columnExcludeKeys,
  })

const filteredRows = computed(() => {
  const rows = leads.value as unknown as Array<Record<string, unknown>>
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

const updatableFields = new Set<LeadFieldKey>(['nome', 'email', 'telefone', 'status', 'notes'])

function toLead(row: Record<string, unknown>) {
  return row as unknown as LeadItem
}

function rowId(row: Record<string, unknown>) {
  return String(row.id ?? '').trim()
}

function onCellUpdate(payload: OmniTableCellUpdate) {
  const id = String(payload.rowId).trim()
  if (!id) return
  const field = String(payload.key) as LeadFieldKey
  if (!updatableFields.has(field)) return
  updateField(id, field, payload.value, { immediate: payload.immediate })
}

const createDialogOpen = ref(false)
const createForm = reactive<LeadCreateInput>({
  nome: '',
  email: '',
  telefone: '',
  page: '',
  cupom: '',
  consent: false,
  consentLabel: '',
  sourceLabel: 'manual',
  notes: '',
})

function openCreate() {
  createForm.nome = ''
  createForm.email = ''
  createForm.telefone = ''
  createForm.page = ''
  createForm.cupom = ''
  createForm.consent = false
  createForm.consentLabel = ''
  createForm.sourceLabel = 'manual'
  createForm.notes = ''
  createDialogOpen.value = true
}

async function submitCreate() {
  if (!canCreate.value) return
  const createdId = await createLead({ ...createForm })
  if (!createdId) return
  createDialogOpen.value = false
  focusCell.value = { rowId: createdId, columnKey: 'nome', token: Date.now() }
}

function onResetFilters() {
  filtersState.value = { query: '', statusFilter: '' }
}

async function onDeleteLead(id: string) {
  if (!canDelete.value) return
  if (import.meta.client && !window.confirm('Excluir este lead?')) return
  await deleteLead(id)
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
  void fetchLeads()
})
</script>

<template>
  <section class="site-leads-workspace flex h-full min-h-0 flex-col gap-4 overflow-hidden">
    <AdminPageHeader
      eyebrow="Site"
      title="Leads"
      description="Leads captados pelo site (via webhook ou criacao manual). Ligado a API real /v1/admin/leads."
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
          label="Novo lead"
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

    <div class="site-leads-workspace__scroll flex-1 min-h-0 overflow-y-auto">
      <OmniDataTable
        v-model="selectedIds"
        :rows="tableRows"
        :columns="tableColumns"
        row-key="id"
        :loading="loading"
        :focus-cell="focusCell"
        empty-text="Nenhum lead encontrado."
        @update:cell="onCellUpdate"
      >
        <template #cell-actions="{ row }">
          <div class="flex items-center justify-end gap-1">
            <OmniMinimalPopover
              :open="infoOpen(rowId(row))"
              title="Detalhes do lead"
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
                  {{ toLead(row).nome || '-' }}
                </p>
                <p>
                  <strong>Email:</strong>
                  {{ toLead(row).email || '-' }}
                </p>
                <p>
                  <strong>Telefone:</strong>
                  {{ toLead(row).telefone || '-' }}
                </p>
                <p>
                  <strong>Status:</strong>
                  {{ toLead(row).status }}
                </p>
                <p>
                  <strong>Fonte:</strong>
                  {{ toLead(row).sourceLabel || '-' }}
                </p>
                <p>
                  <strong>Pagina:</strong>
                  {{ toLead(row).page || '-' }}
                </p>
                <p>
                  <strong>Cupom:</strong>
                  {{ toLead(row).cupom || '-' }}
                </p>
                <p>
                  <strong>Consent:</strong>
                  {{ toLead(row).consent ? 'sim' : 'nao' }}
                </p>
                <p>
                  <strong>Captado:</strong>
                  {{ toLead(row).createdAt }}
                </p>
                <p v-if="toLead(row).notes">
                  <strong>Notas:</strong>
                  {{ toLead(row).notes }}
                </p>
              </div>
            </OmniMinimalPopover>

            <UButton
              v-if="canDelete"
              icon="i-lucide-trash-2"
              color="error"
              variant="ghost"
              size="sm"
              title="Excluir lead"
              aria-label="Excluir"
              :loading="deletingId === rowId(row)"
              @click="onDeleteLead(rowId(row))"
            />
          </div>
        </template>
      </OmniDataTable>
    </div>

    <UModal v-model:open="createDialogOpen">
      <template #content>
        <UCard>
          <template #header>
            <h3 class="text-base font-semibold">Novo lead</h3>
          </template>
          <div class="space-y-3">
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Nome</label>
              <UInput
                :model-value="createForm.nome"
                @update:model-value="createForm.nome = String($event ?? '')"
              />
            </div>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Email</label>
              <UInput
                :model-value="createForm.email"
                type="email"
                @update:model-value="createForm.email = String($event ?? '')"
              />
            </div>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Telefone</label>
              <UInput
                :model-value="createForm.telefone"
                @update:model-value="createForm.telefone = String($event ?? '')"
              />
            </div>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Pagina de origem</label>
              <UInput
                :model-value="createForm.page"
                @update:model-value="createForm.page = String($event ?? '')"
              />
            </div>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Cupom</label>
              <UInput
                :model-value="createForm.cupom"
                @update:model-value="createForm.cupom = String($event ?? '')"
              />
            </div>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Notas</label>
              <UInput
                :model-value="createForm.notes"
                @update:model-value="createForm.notes = String($event ?? '')"
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

    <WebhookSourcesDrawer v-model:open="sourcesDrawerOpen" default-entity="leads" />
  </section>
</template>
