<script setup lang="ts">
import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import type { LeadItem } from '~/types/leads'
import type { OmniFilterDefinition, OmniTableColumn } from '~/types/omni/collection'

const {
  leads,
  clientOptions,
  loading,
  deletingId,
  errorMessage,
  savingMap,
  fetchLeads,
  deleteLead,
} = useLeadsManager()
const sessionSimulation = useSessionSimulationStore()
const isAdminViewer = computed(() => sessionSimulation.userType === 'admin')

const selectedIds = ref<Array<string | number>>([])

const filtersState = ref<Record<string, unknown>>({
  query: '',
  clientIdFilter: '',
})

const clientSelectOptions = computed(() =>
  clientOptions.value.map((item) => ({
    label: String(item.name || `Cliente #${item.id}`),
    value: Number(item.id),
  })),
)

const filterDefinitions = computed<OmniFilterDefinition[]>(() => {
  const definitions: OmniFilterDefinition[] = [
    {
      key: 'query',
      label: 'Buscar',
      type: 'text',
      placeholder: 'Pesquisar por nome, email, telefone, cupom...',
      mode: 'columns',
      columns: [
        'clientName',
        'nome',
        'email',
        'telefone',
        'source',
        'page',
        'cupom',
        'consentLabel',
        'formattedDate',
        'payloadJson',
        'trackingData',
      ],
    },
  ]

  if (clientSelectOptions.value.length > 0) {
    definitions.push({
      key: 'clientIdFilter',
      label: 'Cliente',
      adminOnly: true,
      type: 'select',
      placeholder: 'Cliente',
      options: clientSelectOptions.value,
      accessor: (row) => row.clientId,
    })
  }

  return definitions
})

const allTableColumns = computed<OmniTableColumn[]>(() => [
  {
    key: 'clientName',
    label: 'Cliente',
    adminOnly: true,
    type: 'text',
    editable: false,
    minWidth: 170,
  },
  { key: 'nome', label: 'Nome', type: 'text', editable: false, minWidth: 190 },
  { key: 'email', label: 'Email', type: 'text', editable: false, minWidth: 220 },
  { key: 'telefone', label: 'Telefone', type: 'text', editable: false, minWidth: 150 },
  { key: 'source', label: 'Origem', type: 'text', editable: false, minWidth: 130 },
  { key: 'page', label: 'Pagina', type: 'text', editable: false, minWidth: 140 },
  { key: 'cupom', label: 'Cupom', type: 'text', editable: false, minWidth: 130 },
  { key: 'consentLabel', label: 'Consentimento', type: 'text', editable: false, minWidth: 140 },
  { key: 'formattedDate', label: 'Data cadastro', type: 'text', editable: false, minWidth: 160 },
  { key: 'actions', label: 'Opcoes', type: 'custom', minWidth: 170, align: 'center' },
])

const columnExcludeKeys = ['actions']
const alwaysVisibleColumnKeys = new Set(['actions'])
const { visibleColumnKeys, tableColumns } = useOmniVisibleColumns({
  preferenceKey: 'admin.site.leads',
  allColumns: allTableColumns,
  columnExcludeKeys,
  alwaysVisibleColumnKeys,
})

const filteredRows = computed(() => {
  const rows = leads.value as unknown as Array<Record<string, unknown>>
  return applyOmniFilters(rows, filtersState.value, filterDefinitions.value)
})

function toLead(row: Record<string, unknown>) {
  return row as unknown as LeadItem
}

function rowIdValue(row: Record<string, unknown>) {
  const parsed = Number(row.id)
  return Number.isFinite(parsed) ? parsed : 0
}

function parseBlock(value: unknown) {
  const raw = String(value ?? '').trim()
  if (!raw) {
    return '-'
  }

  try {
    const parsed = JSON.parse(raw)
    return JSON.stringify(parsed, null, 2)
  } catch {
    return raw
  }
}

function normalizePhoneForWhats(raw: unknown) {
  let digits = String(raw ?? '').replace(/\D+/g, '')
  if (!digits) {
    return ''
  }

  if (digits.length <= 11) {
    digits = `55${digits}`
  }

  return digits
}

function openWhatsapp(phone: unknown) {
  if (!import.meta.client) {
    return
  }

  const normalized = normalizePhoneForWhats(phone)
  if (!normalized) {
    return
  }

  window.open(`https://wa.me/${normalized}`, '_blank', 'noopener,noreferrer')
}

function openEmail(email: unknown) {
  if (!import.meta.client) {
    return
  }

  const normalized = String(email ?? '').trim()
  if (!normalized) {
    return
  }

  window.open(`mailto:${encodeURIComponent(normalized)}`, '_blank')
}

function onResetFilters() {
  filtersState.value = {
    query: '',
    clientIdFilter: '',
  }
}

async function onDeleteLead(id: number) {
  if (import.meta.client) {
    const confirmed = window.confirm('Excluir este lead? Esta acao nao pode ser desfeita.')
    if (!confirmed) {
      return
    }
  }

  await deleteLead(id)
}

onMounted(() => {
  sessionSimulation.initialize()
  void fetchLeads()
})
</script>

<template>
  <section class="space-y-4">
    <AdminPageHeader
      eyebrow="Site"
      title="Leads"
      description="Tabela de leads em modo teste usando os componentes genericos para validar o layout antes da API final."
    />

    <OmniCollectionFilters
      v-model="filtersState"
      v-model:visible-columns="visibleColumnKeys"
      :filters="filterDefinitions"
      :viewer-user-type="isAdminViewer ? 'admin' : 'client'"
      :table-columns="allTableColumns"
      :column-exclude-keys="columnExcludeKeys"
      :loading="loading"
      @reset="onResetFilters"
    >
      <template #actions>
        <UBadge color="neutral" variant="soft">Selecionados: {{ selectedIds.length }}</UBadge>
        <UButton
          icon="i-lucide-refresh-cw"
          label="Atualizar"
          color="neutral"
          variant="soft"
          :loading="loading"
          @click="fetchLeads"
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
      v-model="selectedIds"
      :rows="filteredRows"
      :columns="tableColumns"
      :viewer-user-type="isAdminViewer ? 'admin' : 'client'"
      row-key="id"
      :loading="loading"
      empty-text="Nenhum lead encontrado com os filtros atuais."
    >
      <template #cell-actions="{ row }">
        <div class="flex items-center justify-end gap-1">
          <UButton
            v-if="toLead(row).telefone"
            icon="i-lucide-message-circle"
            color="success"
            variant="ghost"
            size="sm"
            aria-label="Abrir WhatsApp"
            @click="openWhatsapp(toLead(row).telefone)"
          />
          <UButton
            v-if="toLead(row).email"
            icon="i-lucide-mail"
            color="neutral"
            variant="ghost"
            size="sm"
            aria-label="Enviar email"
            @click="openEmail(toLead(row).email)"
          />

          <UPopover :content="{ align: 'end', side: 'bottom' }">
            <UButton
              icon="i-lucide-info"
              color="neutral"
              variant="ghost"
              size="sm"
              aria-label="Info"
            />
            <template #content>
              <div class="w-[360px] space-y-2 p-3 text-xs">
                <p>
                  <strong>ID:</strong>
                  {{ toLead(row).id }}
                </p>
                <p>
                  <strong>Cliente:</strong>
                  {{ toLead(row).clientName }}
                </p>
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
                  <strong>Origem:</strong>
                  {{ toLead(row).source || '-' }}
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
                  <strong>Consentimento:</strong>
                  {{ toLead(row).consentLabel }}
                </p>
                <p>
                  <strong>Data cadastro:</strong>
                  {{ toLead(row).formattedDate || '-' }}
                </p>

                <div class="space-y-1">
                  <p class="font-semibold">Tracking data</p>
                  <pre
                    class="max-h-[140px] overflow-auto rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-2 text-[11px]"
                    >{{ parseBlock(toLead(row).trackingData) }}</pre
                  >
                </div>

                <div class="space-y-1">
                  <p class="font-semibold">Payload</p>
                  <pre
                    class="max-h-[190px] overflow-auto rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-2 text-[11px]"
                    >{{ parseBlock(toLead(row).payloadJson) }}</pre
                  >
                </div>
              </div>
            </template>
          </UPopover>

          <UButton
            icon="i-lucide-trash-2"
            color="error"
            variant="ghost"
            size="sm"
            aria-label="Excluir"
            :loading="
              deletingId === rowIdValue(row) || Boolean(savingMap[`${rowIdValue(row)}:delete`])
            "
            @click="onDeleteLead(rowIdValue(row))"
          />
        </div>
      </template>
    </OmniDataTable>
  </section>
</template>
