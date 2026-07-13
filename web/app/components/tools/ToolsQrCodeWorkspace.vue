<script setup lang="ts">
import QRCode from 'qrcode'
import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import { applyOmniFilters } from '~/composables/useOmniCollectionFiltering'
import type { QrCodeItem } from '~/types/tools'
import type {
  OmniFilterDefinition,
  OmniTableCellUpdate,
  OmniTableColumn,
} from '~/types/omni/collection'

const {
  qrcodes,
  loading,
  creating,
  deletingId,
  errorMessage,
  savingMap,
  fetchQrCodes,
  createQrCode,
  updateQrCode,
  deleteQrCode,
} = useQrcodesManager()

const {
  canChooseClient,
  viewerUserType,
  clientOptions,
  activeClientLabel,
  activeClientId,
  ensureClientOptions,
} = useToolsClientOptions()

const selectedIds = ref<Array<string | number>>([])
const modalOpen = ref(false)
const modalMode = ref<'create' | 'edit'>('create')
const previewImage = ref('')
const previewError = ref('')
const submitSuccessMessage = ref('')
const thumbnails = ref<Record<string, string>>({})

const formState = reactive({
  id: '',
  qrUrl: '',
  slug: '',
  targetUrl: '',
  fillColor: '#000000',
  backColor: '#ffffff',
  size: 220,
  isActive: true,
  clientId: '',
})

const filtersState = ref<Record<string, unknown>>({ query: '', statusFilter: '' })

const filterDefinitions = computed<OmniFilterDefinition[]>(() => [
  {
    key: 'query',
    label: 'Buscar',
    type: 'text',
    placeholder: 'Pesquisar por slug ou destino...',
    mode: 'columns',
    columns: ['clientName', 'slug', 'targetUrl'],
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
    accessor: (row) => (row.isActive ? 'active' : 'inactive'),
  },
])

const allTableColumns = computed<OmniTableColumn[]>(() => [
  {
    key: 'clientName',
    label: 'Cliente',
    adminOnly: true,
    type: 'text',
    editable: false,
    minWidth: 180,
  },
  { key: 'slug', label: 'Slug', type: 'text', editable: false, minWidth: 160 },
  { key: 'targetUrl', label: 'Destino', type: 'text', editable: false, minWidth: 300 },
  {
    key: 'qrImage',
    label: 'QR Code',
    type: 'custom',
    editable: false,
    minWidth: 110,
    align: 'center',
  },
  {
    key: 'isActive',
    label: 'Ativo',
    type: 'switch',
    editable: true,
    immediate: true,
    switchOnValue: true,
    switchOffValue: false,
    minWidth: 110,
    align: 'center',
  },
  { key: 'scanCount', label: 'Scans', type: 'number', editable: false, minWidth: 100 },
  {
    key: 'createdAt',
    label: 'Criado em',
    type: 'text',
    editable: false,
    minWidth: 170,
    formatter: (value) => formatDate(String(value ?? '')),
  },
  { key: 'actions', label: 'Opcoes', type: 'custom', minWidth: 160, align: 'center' },
])

const columnExcludeKeys = ['actions']
const { visibleColumnKeys, tableColumns } = useOmniVisibleColumns({
  preferenceKey: 'tools.qr-codes',
  allColumns: allTableColumns,
  columnExcludeKeys,
})

const filteredRows = computed(() => {
  const rows = qrcodes.value as unknown as Array<Record<string, unknown>>
  return applyOmniFilters(rows, filtersState.value, filterDefinitions.value)
})

const modalTitle = computed(() =>
  modalMode.value === 'create' ? 'Novo QR Code' : 'Editar QR Code',
)
const submitLoading = computed(() =>
  modalMode.value === 'create'
    ? creating.value
    : Boolean(savingMap.value[`${formState.id}:update`]),
)

function toQrCode(row: Record<string, unknown>) {
  return row as unknown as QrCodeItem
}
function rowId(row: Record<string, unknown>) {
  return String(row.id ?? '')
}

function formatDate(value: string) {
  const raw = String(value ?? '').trim()
  if (!raw) return '-'
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toLocaleString('pt-BR')
}

function normalizeUrlish(value: string) {
  const raw = String(value ?? '').trim()
  if (!raw) return ''
  return /^https?:\/\//i.test(raw) ? raw : `https://${raw}`
}

// buildQrDataUrl gera a imagem PNG (data URL) de um QR no cliente.
async function buildQrDataUrl(content: string, fill: string, back: string, size: number) {
  return QRCode.toDataURL(content, {
    width: size,
    margin: 1,
    color: { dark: fill, light: back },
    errorCorrectionLevel: 'H',
  })
}

// buildThumbnails gera as miniaturas da tabela a partir do qrUrl rastreado.
async function buildThumbnails() {
  if (!import.meta.client) return
  const next: Record<string, string> = {}
  for (const item of qrcodes.value) {
    try {
      next[item.id] = await buildQrDataUrl(item.qrUrl, item.fillColor, item.backColor, 96)
    } catch {
      next[item.id] = ''
    }
  }
  thumbnails.value = next
}

watch(qrcodes, () => void buildThumbnails(), { deep: false })

// Preview do modal: em edicao usa o qrUrl real; na criacao usa o destino apenas
// para mostrar a aparencia (o QR salvo aponta para o link rastreado /q/{slug}).
async function refreshPreview() {
  if (!import.meta.client) return
  previewError.value = ''
  const content = formState.qrUrl || normalizeUrlish(formState.targetUrl)
  if (!content) {
    previewImage.value = ''
    return
  }
  try {
    previewImage.value = await buildQrDataUrl(
      content,
      formState.fillColor,
      formState.backColor,
      normalizeSize(formState.size),
    )
  } catch {
    previewImage.value = ''
    previewError.value = 'Falha ao gerar o preview do QR Code.'
  }
}

function normalizeSize(value: number) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return 220
  if (parsed < 120) return 120
  if (parsed > 1000) return 1000
  return Math.floor(parsed)
}

watch(
  () => [
    formState.targetUrl,
    formState.fillColor,
    formState.backColor,
    formState.size,
    formState.qrUrl,
    modalOpen.value,
  ],
  () => {
    if (modalOpen.value) void refreshPreview()
  },
)

function openCreateModal() {
  modalMode.value = 'create'
  Object.assign(formState, {
    id: '',
    qrUrl: '',
    slug: '',
    targetUrl: '',
    fillColor: '#000000',
    backColor: '#ffffff',
    size: 220,
    isActive: true,
    clientId: canChooseClient.value ? activeClientId.value : '',
  })
  previewImage.value = ''
  previewError.value = ''
  submitSuccessMessage.value = ''
  modalOpen.value = true
}

function openEditModal(item: QrCodeItem) {
  modalMode.value = 'edit'
  Object.assign(formState, {
    id: item.id,
    qrUrl: item.qrUrl,
    slug: item.slug,
    targetUrl: item.targetUrl,
    fillColor: item.fillColor,
    backColor: item.backColor,
    size: item.size,
    isActive: item.isActive,
    clientId: item.accountId,
  })
  previewImage.value = ''
  previewError.value = ''
  submitSuccessMessage.value = ''
  modalOpen.value = true
}

async function submitQrCode() {
  submitSuccessMessage.value = ''
  const target = normalizeUrlish(formState.targetUrl)
  if (!target) {
    previewError.value = 'Informe a URL de destino.'
    return
  }
  if (modalMode.value === 'create') {
    const created = await createQrCode({
      targetUrl: formState.targetUrl,
      slug: formState.slug,
      fillColor: formState.fillColor,
      backColor: formState.backColor,
      size: normalizeSize(formState.size),
      isActive: formState.isActive,
      accountId: canChooseClient.value ? formState.clientId : undefined,
    })
    if (!created) return
    submitSuccessMessage.value = `QR criado com slug ${created.slug}.`
    modalOpen.value = false
    return
  }
  const updated = await updateQrCode(
    formState.id,
    {
      targetUrl: formState.targetUrl,
      slug: formState.slug,
      fillColor: formState.fillColor,
      backColor: formState.backColor,
      size: normalizeSize(formState.size),
      isActive: formState.isActive,
    },
    'update',
  )
  if (!updated) return
  submitSuccessMessage.value = `QR atualizado com slug ${updated.slug}.`
  modalOpen.value = false
}

async function onCellUpdate(payload: OmniTableCellUpdate) {
  if (String(payload.key) !== 'isActive') return
  const id = String(payload.rowId ?? '')
  if (!id) return
  await updateQrCode(id, { isActive: Boolean(payload.value) }, 'toggle')
}

async function downloadQr(item: QrCodeItem) {
  if (!import.meta.client) return
  try {
    const dataUrl = await buildQrDataUrl(item.qrUrl, item.fillColor, item.backColor, item.size)
    const anchor = document.createElement('a')
    anchor.href = dataUrl
    anchor.download = `${item.slug || 'qrcode'}.png`
    anchor.rel = 'noopener'
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
  } catch {
    // geracao falhou — sem download
  }
}

async function onDeleteQrCode(id: string) {
  if (import.meta.client) {
    const confirmed = window.confirm('Excluir este QR Code? Esta acao nao pode ser desfeita.')
    if (!confirmed) return
  }
  await deleteQrCode(id)
}

function onResetFilters() {
  filtersState.value = { query: '', statusFilter: '' }
}

onMounted(() => {
  void Promise.all([fetchQrCodes(), ensureClientOptions()])
})
</script>

<template>
  <section class="qr-codes-page space-y-4">
    <AdminPageHeader
      eyebrow="Tools"
      title="QR Code"
      description="Gere QR Codes personalizados e rastreados (scans + status), com preview e download."
    />

    <OmniCollectionFilters
      v-model="filtersState"
      v-model:visible-columns="visibleColumnKeys"
      :filters="filterDefinitions"
      :viewer-user-type="viewerUserType"
      :table-columns="allTableColumns"
      :column-exclude-keys="columnExcludeKeys"
      :loading="loading"
      @reset="onResetFilters"
    >
      <template #actions>
        <UButton
          icon="i-lucide-plus"
          label="Novo QR"
          color="primary"
          :loading="creating"
          :disabled="creating"
          @click="openCreateModal"
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
      :viewer-user-type="viewerUserType"
      row-key="id"
      :loading="loading"
      empty-text="Nenhum QR Code encontrado com os filtros atuais."
      @update:cell="onCellUpdate"
    >
      <template #cell-qrImage="{ row }">
        <div class="flex items-center justify-center">
          <img
            v-if="thumbnails[rowId(row)]"
            :src="thumbnails[rowId(row)]"
            alt="QR"
            class="h-12 w-12 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-white object-contain p-0.5"
          />
          <span v-else class="text-[11px] text-[rgb(var(--muted))]">...</span>
        </div>
      </template>

      <template #cell-actions="{ row }">
        <div class="qr-codes-page__row-actions flex items-center justify-end gap-1">
          <UButton
            icon="i-lucide-pencil"
            color="neutral"
            variant="ghost"
            size="sm"
            title="Editar QR"
            aria-label="Editar QR"
            @click="openEditModal(toQrCode(row))"
          />
          <UButton
            icon="i-lucide-download"
            color="neutral"
            variant="ghost"
            size="sm"
            title="Baixar QR"
            aria-label="Baixar QR"
            @click="downloadQr(toQrCode(row))"
          />
          <UButton
            icon="i-lucide-trash-2"
            color="error"
            variant="ghost"
            size="sm"
            title="Excluir"
            aria-label="Excluir"
            :loading="deletingId === rowId(row) || Boolean(savingMap[`${rowId(row)}:delete`])"
            @click="onDeleteQrCode(rowId(row))"
          />
        </div>
      </template>
    </OmniDataTable>

    <UModal
      v-model:open="modalOpen"
      :title="modalTitle"
      description="Preencha os dados para gerar ou editar o QR Code."
      :ui="{ content: 'max-w-4xl' }"
    >
      <template #body>
        <div class="qr-codes-page__modal-body grid gap-3 md:grid-cols-[minmax(0,1fr)_280px]">
          <div class="space-y-3">
            <div class="grid gap-3 sm:grid-cols-2">
              <div class="space-y-1">
                <p class="text-xs text-[rgb(var(--muted))]">Slug (opcional)</p>
                <UInput
                  v-model="formState.slug"
                  placeholder="meu-qr"
                  @update:model-value="submitSuccessMessage = ''"
                />
              </div>
              <div class="space-y-1">
                <p class="text-xs text-[rgb(var(--muted))]">Tamanho (px)</p>
                <UInput v-model.number="formState.size" type="number" :min="120" :max="1000" />
              </div>
            </div>

            <div class="space-y-1">
              <p class="text-xs text-[rgb(var(--muted))]">Destino (URL)</p>
              <UInput v-model="formState.targetUrl" placeholder="https://..." />
            </div>

            <div v-if="canChooseClient" class="space-y-1">
              <p class="text-xs text-[rgb(var(--muted))]">Conta dona</p>
              <USelect
                v-model="formState.clientId"
                :items="clientOptions"
                placeholder="Selecionar conta"
              />
            </div>
            <div v-else class="space-y-1">
              <p class="text-xs text-[rgb(var(--muted))]">Conta dona</p>
              <UInput :model-value="activeClientLabel" disabled />
            </div>

            <div class="grid gap-3 sm:grid-cols-2">
              <div class="space-y-1">
                <p class="text-xs text-[rgb(var(--muted))]">Cor do QR</p>
                <input
                  v-model="formState.fillColor"
                  type="color"
                  class="h-9 w-full cursor-pointer rounded border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-1"
                />
              </div>
              <div class="space-y-1">
                <p class="text-xs text-[rgb(var(--muted))]">Cor de fundo</p>
                <input
                  v-model="formState.backColor"
                  type="color"
                  class="h-9 w-full cursor-pointer rounded border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-1"
                />
              </div>
            </div>

            <div class="flex items-center gap-2">
              <USwitch v-model="formState.isActive" />
              <span class="text-xs text-[rgb(var(--muted))]">
                QR ativo (desativado, o scan para de funcionar)
              </span>
            </div>

            <p v-if="modalMode === 'create'" class="text-[11px] text-[rgb(var(--muted))]">
              O QR salvo aponta para um link rastreado (/q/...). Este preview usa o destino para
              mostrar a aparencia; a imagem final aparece na tabela apos salvar.
            </p>

            <UAlert
              v-if="previewError"
              color="error"
              variant="soft"
              icon="i-lucide-alert-triangle"
              :description="previewError"
            />
            <UAlert
              v-if="submitSuccessMessage"
              color="success"
              variant="soft"
              icon="i-lucide-badge-check"
              :description="submitSuccessMessage"
            />
          </div>

          <aside
            class="qr-codes-page__preview rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-3"
          >
            <p class="mb-2 text-xs font-medium text-[rgb(var(--muted))]">Pre-visualizacao</p>
            <div
              class="flex min-h-[240px] items-center justify-center rounded-[var(--radius-sm)] border border-dashed border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-2"
            >
              <img
                v-if="previewImage"
                :src="previewImage"
                alt="Preview QR"
                class="max-h-[220px] max-w-full object-contain"
              />
              <p v-else class="text-xs text-[rgb(var(--muted))]">
                Informe a URL para gerar o preview.
              </p>
            </div>
          </aside>
        </div>
      </template>

      <template #footer>
        <div class="flex w-full items-center justify-end gap-2">
          <UButton label="Fechar" color="neutral" variant="ghost" @click="modalOpen = false" />
          <UButton
            :label="modalMode === 'create' ? 'Salvar QR' : 'Atualizar QR'"
            icon="i-lucide-save"
            color="primary"
            :loading="submitLoading"
            :disabled="submitLoading"
            @click="submitQrCode"
          />
        </div>
      </template>
    </UModal>
  </section>
</template>
