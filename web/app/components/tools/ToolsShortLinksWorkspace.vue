<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import ToolsListMobileToolbar from '~/components/tools/ToolsListMobileToolbar.vue'
import ToolsShortLinkCardGrid from '~/components/tools/ToolsShortLinkCardGrid.vue'
import ToolsShortLinkCreateModal from '~/components/tools/ToolsShortLinkCreateModal.vue'
import ToolsShortLinkMobileList from '~/components/tools/ToolsShortLinkMobileList.vue'
import ToolsShortLinkRenameModal from '~/components/tools/ToolsShortLinkRenameModal.vue'
import ToolsStatusChips from '~/components/tools/ToolsStatusChips.vue'
import ToolsViewToggle from '~/components/tools/ToolsViewToggle.vue'
import { applyOmniFilters } from '~/composables/useOmniCollectionFiltering'
import { useMediaMatch } from '~/composables/useMediaMatch'
import { useToolsViewMode } from '~/composables/useToolsViewMode'
import { useUiStore } from '~/stores/ui'
import type { DropdownMenuItem } from '@nuxt/ui'

import type { ShortLinkItem } from '~/types/tools'
import type {
  OmniFilterDefinition,
  OmniTableCellUpdate,
  OmniTableColumn,
} from '~/types/omni/collection'
import { copyToClipboard } from '~/utils/qr-image'

// Encurtador no modelo da aba Produtos do cardapio: faixa de filtros sem
// moldura; no desktop tabela (padrao, destino editavel na celula) ou cards;
// cards no mobile. O link curto e link de verdade, com copiar e abrir ao lado.
// O slug nao e editavel na celula: trocar derruba o link antigo, entao passa
// pelo modal "Renomear slug" (com confirmacao).
const ui = useUiStore()
const {
  shortLinks,
  loading,
  creating,
  errorMessage,
  savingMap,
  canManage,
  fetchShortLinks,
  createShortLink,
  updateShortLink,
  setActive,
  renameSlug,
  deleteShortLink,
} = useShortLinksManager()

const {
  canChooseClient,
  viewerUserType,
  clientOptions,
  activeClientLabel,
  activeClientId,
  ensureClientOptions,
} = useToolsClientOptions()

const viewMode = useToolsViewMode('tools.short-links', 'table')
const createOpen = ref(false)
const renameOpen = ref(false)
const renameItem = ref<ShortLinkItem | null>(null)

const filtersState = ref<Record<string, unknown>>({ query: '', statusFilter: '' })

const filterDefinitions = computed<OmniFilterDefinition[]>(() => [
  {
    key: 'query',
    label: 'Buscar',
    type: 'text',
    placeholder: 'Buscar por slug ou destino...',
    mode: 'columns',
    columns: ['clientName', 'slug', 'targetUrl', 'shortUrl'],
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
const toolbarFilters = computed(() => filterDefinitions.value.filter((f) => f.key === 'query'))

// Campos que o card do mobile sempre mostra (fora do menu "Campos do card").
const MOBILE_FIXED_KEYS = ['actions', 'shortUrl', 'slug', 'targetUrl']

const allTableColumns = computed<OmniTableColumn[]>(() => [
  { key: 'shortUrl', label: 'Link curto', type: 'custom', editable: false, minWidth: 220 },
  { key: 'slug', label: 'Slug', type: 'text', editable: false, minWidth: 130, maxWidth: 200 },
  {
    key: 'targetUrl',
    label: 'Destino',
    type: 'text',
    editable: canManage.value,
    placeholder: 'https://...',
    minWidth: 180,
    maxWidth: 300,
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
  { key: 'hits', label: 'Cliques', type: 'number', editable: false, minWidth: 80 },
  {
    key: 'lastHitAt',
    label: 'Ultimo clique',
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
  { key: 'actions', label: 'Acoes', type: 'custom', minWidth: 120, align: 'center' },
])

const columnExcludeKeys = ['actions']
const { visibleColumnKeys, lockedColumnKeys, columnOrder, tableColumns, resetToDefaults } =
  useOmniVisibleColumns({
    // v2: layout enxuto. Slug, cliente e datas seguem disponiveis em "Colunas".
    preferenceKey: 'tools.short-links.v2',
    allColumns: allTableColumns,
    columnExcludeKeys,
    defaultVisibleColumnKeys: ['shortUrl', 'targetUrl', 'isActive', 'hits', 'actions'],
  })

const isMobile = useMediaMatch('(max-width: 720px)')
const { visibleColumnKeys: mobileFieldKeys } = useOmniVisibleColumns({
  preferenceKey: 'tools.short-links.mobile',
  allColumns: allTableColumns,
  columnExcludeKeys: MOBILE_FIXED_KEYS,
  defaultVisibleColumnKeys: ['isActive', 'hits'],
})

const filteredRows = computed(() => {
  const rows = shortLinks.value as unknown as Array<Record<string, unknown>>
  return applyOmniFilters(rows, filtersState.value, filterDefinitions.value)
})
const filteredItems = computed(() => filteredRows.value as unknown as ShortLinkItem[])
const activeCount = computed(() => shortLinks.value.filter((item) => item.isActive).length)

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
  shortLinks.value.length
    ? 'Nenhum link com os filtros atuais.'
    : 'Nenhum link curto ainda. Crie o primeiro em "Novo link".',
)

function toLink(row: Record<string, unknown>) {
  return row as unknown as ShortLinkItem
}

function shortLabel(url: string) {
  return String(url ?? '').replace(/^https?:\/\//i, '')
}

function formatDate(value: string) {
  const raw = String(value ?? '').trim()
  if (!raw) return '-'
  const date = new Date(raw)
  return Number.isNaN(date.getTime()) ? raw : date.toLocaleString('pt-BR')
}

function openExternal(url: string) {
  const target = String(url ?? '').trim()
  if (import.meta.client && target) window.open(target, '_blank', 'noopener,noreferrer')
}

async function copyLink(item: ShortLinkItem) {
  if (await copyToClipboard(item.shortUrl)) ui.success('Link curto copiado.')
  else ui.error('Nao foi possivel copiar o link.')
}

async function toggle(id: string, isActive: boolean) {
  const updated = await setActive(id, isActive)
  if (!updated) {
    ui.error(errorMessage.value || 'Nao foi possivel alterar o link.')
    return
  }
  ui.success(isActive ? 'Link ativado.' : 'Link desativado: quem abrir ve pagina nao encontrada.')
}

function onCellUpdate(payload: OmniTableCellUpdate) {
  const id = String(payload.rowId ?? '').trim()
  const field = String(payload.key)
  if (!id || !canManage.value) return
  if (field === 'isActive') {
    void toggle(id, Boolean(payload.value))
    return
  }
  if (field === 'targetUrl') {
    updateShortLink(id, 'targetUrl', payload.value, { immediate: payload.immediate })
  }
}

// Mobile: o destino nao e editavel no card; o menu abre este prompt.
async function editTarget(item: ShortLinkItem) {
  const { confirmed, value } = (await ui.prompt({
    title: 'Editar destino',
    message: `Para onde ${shortLabel(item.shortUrl)} leva. O link curto continua o mesmo.`,
    inputLabel: 'URL de destino',
    inputPlaceholder: 'https://...',
    initialValue: item.targetUrl,
    confirmLabel: 'Salvar',
    required: true,
  })) as { confirmed: boolean; value: string }
  if (!confirmed || !String(value ?? '').trim()) return
  updateShortLink(item.id, 'targetUrl', String(value).trim(), { immediate: true })
}

function openRename(item: ShortLinkItem) {
  renameItem.value = item
  renameOpen.value = true
}

async function remove(item: ShortLinkItem) {
  const { confirmed } = (await ui.confirm({
    title: 'Excluir link curto',
    message: `Excluir ${shortLabel(item.shortUrl)}? Quem tiver o link cai em pagina nao encontrada. Para pausar, use o Ativo.`,
    confirmLabel: 'Excluir',
  })) as { confirmed: boolean }
  if (!confirmed) return
  if (await deleteShortLink(item.id)) ui.success('Link excluido.')
  else ui.error(errorMessage.value || 'Nao foi possivel excluir o link.')
}

function menuItems(item: ShortLinkItem): DropdownMenuItem[][] {
  return [
    [
      {
        label: 'Abrir destino',
        icon: 'i-lucide-link',
        onSelect: () => openExternal(item.targetUrl),
      },
      { label: 'Renomear slug', icon: 'i-lucide-pencil-line', onSelect: () => openRename(item) },
      {
        label: 'Excluir',
        icon: 'i-lucide-trash-2',
        color: 'error',
        onSelect: () => void remove(item),
      },
    ],
  ]
}

function onResetFilters() {
  filtersState.value = { query: '', statusFilter: '' }
}

onMounted(() => {
  void Promise.all([fetchShortLinks(), ensureClientOptions()])
})
</script>

<template>
  <section class="tools-list">
    <AdminPageHeader
      eyebrow="Tools"
      title="Encurtador de Link"
      description="Links curtos rastreados: troque o destino, pause e conte cliques."
    />
    <template v-if="isMobile">
      <ToolsListMobileToolbar
        v-model:search="search"
        v-model:status="status"
        v-model:visible-columns="mobileFieldKeys"
        placeholder="Buscar link"
        create-label="Novo link curto"
        :can-create="canManage"
        :total="shortLinks.length"
        :active="activeCount"
        :columns="allTableColumns"
        :exclude-keys="MOBILE_FIXED_KEYS"
        @create="createOpen = true"
      />
      <UAlert
        v-if="errorMessage"
        color="error"
        variant="soft"
        icon="i-lucide-alert-triangle"
        :description="errorMessage"
      />
      <ToolsShortLinkMobileList
        :rows="filteredItems"
        :visible-keys="mobileFieldKeys"
        :can-manage="canManage"
        :saving-map="savingMap"
        :empty-text="emptyText"
        @copy="copyLink"
        @open="openExternal"
        @edit-target="editTarget"
        @rename="openRename"
        @remove="remove"
        @toggle="toggle"
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
          <ToolsStatusChips v-model="status" :total="shortLinks.length" :active="activeCount" />
          <div class="tools-list__primary">
            <ToolsViewToggle v-model="viewMode" />
            <UButton
              v-if="canManage"
              icon="i-lucide-plus"
              label="Novo link"
              color="primary"
              :loading="creating"
              @click="createOpen = true"
            />
          </div>
        </template>
      </OmniCollectionFilters>

      <UAlert
        v-if="errorMessage"
        color="error"
        variant="soft"
        icon="i-lucide-alert-triangle"
        :description="errorMessage"
      />

      <ToolsShortLinkCardGrid
        v-if="viewMode === 'cards'"
        :rows="filteredItems"
        :can-manage="canManage"
        :saving-map="savingMap"
        :empty-text="loading ? 'Carregando...' : emptyText"
        @copy="copyLink"
        @open="openExternal"
        @edit-target="editTarget"
        @rename="openRename"
        @remove="remove"
        @toggle="toggle"
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
        <template #cell-shortUrl="{ row }">
          <div class="tools-link" :class="{ 'is-off': !toLink(row).isActive }">
            <a
              class="tools-link__anchor"
              :href="toLink(row).shortUrl"
              target="_blank"
              rel="noopener noreferrer"
              :title="`Abrir ${toLink(row).shortUrl}`"
            >
              {{ shortLabel(toLink(row).shortUrl) }}
            </a>
            <UButton
              icon="i-lucide-copy"
              color="neutral"
              variant="ghost"
              size="xs"
              title="Copiar link curto"
              :aria-label="`Copiar ${toLink(row).shortUrl}`"
              @click="copyLink(toLink(row))"
            />
          </div>
        </template>
        <template #cell-actions="{ row }">
          <div class="tools-list__actions">
            <UButton
              icon="i-lucide-external-link"
              color="neutral"
              variant="ghost"
              size="sm"
              title="Abrir link curto"
              aria-label="Abrir link curto"
              @click="openExternal(toLink(row).shortUrl)"
            />
            <UDropdownMenu
              v-if="canManage"
              :items="menuItems(toLink(row))"
              :content="{ align: 'end' }"
            >
              <UButton
                icon="i-lucide-ellipsis"
                color="neutral"
                variant="ghost"
                size="sm"
                title="Mais opcoes"
                aria-label="Mais opcoes"
                :loading="
                  Boolean(
                    savingMap[`${toLink(row).id}:delete`] || savingMap[`${toLink(row).id}:slug`],
                  )
                "
              />
            </UDropdownMenu>
            <UButton
              v-else
              icon="i-lucide-link"
              color="neutral"
              variant="ghost"
              size="sm"
              title="Abrir destino"
              aria-label="Abrir destino"
              @click="openExternal(toLink(row).targetUrl)"
            />
          </div>
        </template>
      </OmniDataTable>
    </template>

    <ToolsShortLinkCreateModal
      v-model:open="createOpen"
      :can-choose-client="canChooseClient"
      :client-options="clientOptions"
      :active-client-label="activeClientLabel"
      :active-client-id="activeClientId"
      :saving="creating"
      :create="createShortLink"
      :error-text="() => errorMessage"
      @created="ui.success(`Link criado: ${$event.shortUrl}`)"
    />

    <ToolsShortLinkRenameModal
      v-model:open="renameOpen"
      :item="renameItem"
      :saving="Boolean(renameItem && savingMap[`${renameItem.id}:slug`])"
      :rename="renameSlug"
      @renamed="ui.success(`Slug alterado. Novo link: ${$event.shortUrl}`)"
    />
  </section>
</template>

<style scoped src="./tools-list.css"></style>
<style scoped src="./tools-link.css"></style>
