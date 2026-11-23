<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import ToolsListMobileToolbar from '~/components/tools/ToolsListMobileToolbar.vue'
import ToolsQrCardGrid from '~/components/tools/ToolsQrCardGrid.vue'
import ToolsQrFormModal from '~/components/tools/ToolsQrFormModal.vue'
import ToolsQrMobileList from '~/components/tools/ToolsQrMobileList.vue'
import ToolsQrThumb from '~/components/tools/ToolsQrThumb.vue'
import ToolsQrViewer from '~/components/tools/ToolsQrViewer.vue'
import ToolsStatusChips from '~/components/tools/ToolsStatusChips.vue'
import ToolsViewToggle from '~/components/tools/ToolsViewToggle.vue'
import { applyOmniFilters } from '~/composables/useOmniCollectionFiltering'
import { useMediaMatch } from '~/composables/useMediaMatch'
import { useToolsViewMode } from '~/composables/useToolsViewMode'
import { useUiStore } from '~/stores/ui'
import type { QrCodeItem, UpdateQrCodePayload } from '~/types/tools'
import type {
  OmniFilterDefinition,
  OmniTableCellUpdate,
  OmniTableColumn,
} from '~/types/omni/collection'
import { copyToClipboard, downloadQrImage } from '~/utils/qr-image'

// Lista de QR Codes no modelo da aba Produtos do cardapio: faixa de filtros sem
// moldura; no desktop cards (padrao, QR grande) ou tabela; cards no mobile.
// A imagem do QR abre em tela cheia; o slug e link de teste (/q/{slug}).
const ui = useUiStore()
const {
  qrcodes,
  loading,
  creating,
  errorMessage,
  savingMap,
  canManage,
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

const viewMode = useToolsViewMode('tools.qr-codes')
const formOpen = ref(false)
const formItem = ref<QrCodeItem | null>(null)
const viewerId = ref('')
const viewerItem = computed(() => qrcodes.value.find((item) => item.id === viewerId.value) ?? null)

const filtersState = ref<Record<string, unknown>>({ query: '', statusFilter: '' })

const filterDefinitions = computed<OmniFilterDefinition[]>(() => [
  {
    key: 'query',
    label: 'Buscar',
    type: 'text',
    placeholder: 'Buscar por slug ou destino...',
    mode: 'columns',
    columns: ['clientName', 'slug', 'targetUrl'],
  },
  {
    key: 'statusFilter',
    label: 'Status',
    type: 'select',
    options: [
      { label: 'Ativo', value: 'active' },
      { label: 'Inativo', value: 'inactive' },
    ],
    accessor: (row) => (row.isActive ? 'active' : 'inactive'),
  },
])
// Na barra so a busca: o status vira o seletor segmentado com contagem.
const toolbarFilters = computed(() => filterDefinitions.value.filter((f) => f.key === 'query'))

// Campos que o card do mobile sempre mostra (fora do menu "Campos do card").
const MOBILE_FIXED_KEYS = ['actions', 'qrImage', 'slug', 'targetUrl']

const allTableColumns = computed<OmniTableColumn[]>(() => [
  { key: 'qrImage', label: 'QR', type: 'custom', editable: false, minWidth: 96 },
  { key: 'slug', label: 'Slug', type: 'custom', editable: false, minWidth: 160, maxWidth: 260 },
  // Destino com largura maxima: URLs longas nao podem tomar a tabela.
  {
    key: 'targetUrl',
    label: 'Destino',
    type: 'custom',
    editable: false,
    minWidth: 160,
    maxWidth: 280,
  },
  {
    key: 'isActive',
    label: 'Ativo',
    type: 'switch',
    editable: canManage.value,
    immediate: true,
    switchOnValue: true,
    switchOffValue: false,
    formatter: (value) => (value ? 'Ativo' : 'Inativo'),
    minWidth: 90,
    align: 'center',
  },
  { key: 'scanCount', label: 'Scans', type: 'number', editable: false, minWidth: 80 },
  {
    key: 'lastScannedAt',
    label: 'Ultimo scan',
    type: 'text',
    editable: false,
    minWidth: 150,
    formatter: (value) => formatDate(String(value ?? '')),
  },
  {
    key: 'createdAt',
    label: 'Criado em',
    type: 'text',
    editable: false,
    minWidth: 150,
    formatter: (value) => formatDate(String(value ?? '')),
  },
  {
    key: 'clientName',
    label: 'Cliente',
    adminOnly: true,
    type: 'text',
    editable: false,
    minWidth: 160,
  },
  ...(canManage.value
    ? [
        {
          key: 'actions',
          label: 'Acoes',
          type: 'custom',
          minWidth: 96,
          align: 'center',
        } as OmniTableColumn,
      ]
    : []),
])

const columnExcludeKeys = ['actions']
const { visibleColumnKeys, lockedColumnKeys, columnOrder, tableColumns, resetToDefaults } =
  useOmniVisibleColumns({
    // v2: layout enxuto (QR em destaque). Cliente, datas e ultimo scan em "Colunas".
    preferenceKey: 'tools.qr-codes.v2',
    allColumns: allTableColumns,
    columnExcludeKeys,
    defaultVisibleColumnKeys: ['qrImage', 'slug', 'targetUrl', 'isActive', 'scanCount', 'actions'],
  })

// Mobile: cards no lugar da tabela; campos extras seguem preferencia propria.
const isMobile = useMediaMatch('(max-width: 720px)')
const { visibleColumnKeys: mobileFieldKeys } = useOmniVisibleColumns({
  preferenceKey: 'tools.qr-codes.mobile',
  allColumns: allTableColumns,
  columnExcludeKeys: MOBILE_FIXED_KEYS,
  defaultVisibleColumnKeys: ['isActive', 'scanCount'],
})

const filteredRows = computed(() => {
  const rows = qrcodes.value as unknown as Array<Record<string, unknown>>
  return applyOmniFilters(rows, filtersState.value, filterDefinitions.value)
})
const filteredItems = computed(() => filteredRows.value as unknown as QrCodeItem[])
const activeCount = computed(() => qrcodes.value.filter((item) => item.isActive).length)

const search = computed({
  get: () => String(filtersState.value.query ?? ''),
  set: (value: string) => {
    filtersState.value = { ...filtersState.value, query: value }
  },
})
const status = computed({
  get: () => String(filtersState.value.statusFilter ?? ''),
  set: (value: string) => {
    filtersState.value = { ...filtersState.value, statusFilter: value }
  },
})
const hasActiveFilters = computed(() => Boolean(search.value.trim() || status.value))
const emptyText = computed(() =>
  qrcodes.value.length
    ? 'Nenhum QR Code com os filtros atuais.'
    : 'Nenhum QR Code ainda. Crie o primeiro em "Novo QR".',
)

function toQr(row: Record<string, unknown>) {
  return row as unknown as QrCodeItem
}

function formatDate(value: string) {
  const raw = String(value ?? '').trim()
  if (!raw) return '-'
  const date = new Date(raw)
  return Number.isNaN(date.getTime()) ? raw : date.toLocaleString('pt-BR')
}

function openCreate() {
  formItem.value = null
  formOpen.value = true
}

function openEdit(item: QrCodeItem) {
  viewerId.value = ''
  formItem.value = item
  formOpen.value = true
}

async function setActive(id: string, isActive: boolean) {
  const updated = await updateQrCode(id, { isActive }, 'toggle')
  if (!updated) {
    ui.error(errorMessage.value || 'Nao foi possivel alterar o QR.')
    return
  }
  ui.success(isActive ? 'QR ativado.' : 'QR desativado: quem escanear ve pagina nao encontrada.')
}

function onCellUpdate(payload: OmniTableCellUpdate) {
  if (String(payload.key) !== 'isActive' || !canManage.value) return
  const id = String(payload.rowId ?? '')
  if (id) void setActive(id, Boolean(payload.value))
}

async function download(item: QrCodeItem) {
  if (!(await downloadQrImage(item))) ui.error('Nao foi possivel gerar a imagem do QR.')
}

async function copyLink(item: QrCodeItem) {
  if (await copyToClipboard(item.qrUrl)) ui.success('Link do QR copiado.')
  else ui.error('Nao foi possivel copiar o link.')
}

async function remove(item: QrCodeItem) {
  const { confirmed } = (await ui.confirm({
    title: 'Excluir QR Code',
    message: `Excluir o QR "${item.slug}"? Os QR ja impressos param de funcionar. Para pausar, use o Ativo.`,
    confirmLabel: 'Excluir',
  })) as { confirmed: boolean }
  if (!confirmed) return
  if (await deleteQrCode(item.id)) ui.success('QR excluido.')
  else ui.error(errorMessage.value || 'Nao foi possivel excluir o QR.')
}

async function updateFromModal(id: string, payload: UpdateQrCodePayload) {
  const updated = await updateQrCode(id, payload, 'update')
  if (updated) ui.success('QR atualizado.')
  return updated
}

function onResetFilters() {
  filtersState.value = { query: '', statusFilter: '' }
}

onMounted(() => {
  void Promise.all([fetchQrCodes(), ensureClientOptions()])
})
</script>

<template>
  <section class="tools-list">
    <AdminPageHeader
      eyebrow="Tools"
      title="QR Code"
      description="QR Codes rastreados: troque o destino, pause e conte scans sem reimprimir."
    />
    <template v-if="isMobile">
      <ToolsListMobileToolbar
        v-model:search="search"
        v-model:status="status"
        v-model:visible-columns="mobileFieldKeys"
        placeholder="Buscar QR"
        create-label="Novo QR"
        :can-create="canManage"
        :total="qrcodes.length"
        :active="activeCount"
        :columns="allTableColumns"
        :exclude-keys="MOBILE_FIXED_KEYS"
        @create="openCreate"
      />
      <UAlert
        v-if="errorMessage && !formOpen"
        color="error"
        variant="soft"
        icon="i-lucide-alert-triangle"
        :description="errorMessage"
      />
      <ToolsQrMobileList
        :rows="filteredItems"
        :visible-keys="mobileFieldKeys"
        :can-manage="canManage"
        :saving-map="savingMap"
        :empty-text="emptyText"
        @view="viewerId = $event"
        @download="download"
        @edit="openEdit"
        @remove="remove"
        @toggle="setActive"
      />
    </template>

    <template v-else>
      <OmniCollectionFilters
        v-model="filtersState"
        v-model:visible-columns="visibleColumnKeys"
        v-model:locked-columns="lockedColumnKeys"
        v-model:column-order="columnOrder"
        :filters="toolbarFilters"
        :viewer-user-type="viewerUserType"
        :table-columns="allTableColumns"
        :column-exclude-keys="columnExcludeKeys"
        :show-column-filter="viewMode === 'table'"
        :loading="loading"
        :show-reset="hasActiveFilters"
        @reset="onResetFilters"
        @reset-columns="resetToDefaults"
      >
        <template #actions>
          <ToolsStatusChips v-model="status" :total="qrcodes.length" :active="activeCount" />
          <div class="tools-list__primary">
            <ToolsViewToggle v-model="viewMode" />
            <UButton
              v-if="canManage"
              icon="i-lucide-plus"
              label="Novo QR"
              color="primary"
              :loading="creating"
              @click="openCreate"
            />
          </div>
        </template>
      </OmniCollectionFilters>

      <UAlert
        v-if="errorMessage && !formOpen"
        color="error"
        variant="soft"
        icon="i-lucide-alert-triangle"
        :description="errorMessage"
      />

      <ToolsQrCardGrid
        v-if="viewMode === 'cards'"
        :rows="filteredItems"
        :can-manage="canManage"
        :saving-map="savingMap"
        :empty-text="loading ? 'Carregando...' : emptyText"
        @view="viewerId = $event"
        @download="download"
        @edit="openEdit"
        @remove="remove"
        @toggle="setActive"
      />

      <OmniDataTable
        v-else
        :rows="filteredRows"
        :columns="tableColumns"
        :viewer-user-type="viewerUserType"
        :selectable="false"
        row-key="id"
        :loading="loading"
        :empty-text="emptyText"
        @update:cell="onCellUpdate"
      >
        <template #cell-qrImage="{ row }">
          <ToolsQrThumb
            :slug="toQr(row).slug"
            :qr-url="toQr(row).qrUrl"
            :fill-color="toQr(row).fillColor"
            :back-color="toQr(row).backColor"
            :is-active="toQr(row).isActive"
            @view="viewerId = toQr(row).id"
            @download="download(toQr(row))"
          />
        </template>
        <template #cell-slug="{ row }">
          <a
            class="tools-qr-slug"
            :href="toQr(row).qrUrl"
            target="_blank"
            rel="noopener noreferrer"
            :title="`Testar ${toQr(row).qrUrl} (conta como scan)`"
          >
            <span class="tools-list__ellipsis">{{ toQr(row).slug }}</span>
            <UIcon name="i-lucide-external-link" class="tools-qr-slug__icon" />
          </a>
        </template>
        <template #cell-targetUrl="{ row }">
          <span class="tools-list__ellipsis" :title="toQr(row).targetUrl">
            {{ toQr(row).targetUrl }}
          </span>
        </template>
        <template #cell-actions="{ row }">
          <div class="tools-list__actions">
            <UButton
              icon="i-lucide-pencil"
              color="neutral"
              variant="ghost"
              size="sm"
              title="Editar QR (destino, cores, tamanho)"
              aria-label="Editar"
              @click="openEdit(toQr(row))"
            />
            <UButton
              icon="i-lucide-trash-2"
              color="error"
              variant="ghost"
              size="sm"
              title="Excluir QR"
              aria-label="Excluir"
              :loading="Boolean(savingMap[`${toQr(row).id}:delete`])"
              @click="remove(toQr(row))"
            />
          </div>
        </template>
      </OmniDataTable>
    </template>

    <ToolsQrViewer
      v-if="viewerItem"
      :item="viewerItem"
      :can-manage="canManage"
      :toggling="Boolean(savingMap[`${viewerItem.id}:toggle`])"
      @close="viewerId = ''"
      @copy="copyLink(viewerItem)"
      @download="download(viewerItem)"
      @edit="openEdit(viewerItem)"
      @toggle="setActive(viewerItem.id, $event)"
    />

    <ToolsQrFormModal
      v-model:open="formOpen"
      :item="formItem"
      :can-choose-client="canChooseClient"
      :client-options="clientOptions"
      :active-client-label="activeClientLabel"
      :active-client-id="activeClientId"
      :saving="creating || Boolean(formItem && savingMap[`${formItem.id}:update`])"
      :create="createQrCode"
      :update="updateFromModal"
      :error-text="() => errorMessage"
    />
  </section>
</template>

<style scoped src="./tools-list.css"></style>
<style scoped>
/* Slug = link de teste do QR (abre o /q/{slug} rastreado). */
.tools-qr-slug {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  max-width: 100%;
  color: var(--text-main);
  font-weight: 600;
  text-decoration: none;
}

.tools-qr-slug:hover span {
  color: rgb(var(--primary));
  text-decoration: underline;
}

.tools-qr-slug__icon {
  flex-shrink: 0;
  width: 0.8rem;
  height: 0.8rem;
  color: var(--text-muted);
}
</style>
