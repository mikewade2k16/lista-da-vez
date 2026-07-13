<script setup lang="ts">
import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import { applyOmniFilters } from '~/composables/useOmniCollectionFiltering'
import type { ShortLinkItem } from '~/types/tools'
import type {
  OmniFilterDefinition,
  OmniTableCellUpdate,
  OmniTableColumn,
} from '~/types/omni/collection'

const {
  shortLinks,
  loading,
  creating,
  deletingId,
  errorMessage,
  savingMap,
  fetchShortLinks,
  createShortLink,
  updateShortLink,
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

const selectedIds = ref<Array<string | number>>([])
const modalOpen = ref(false)
const createSuccessMessage = ref('')

const createForm = reactive({
  targetUrl: '',
  slug: '',
  clientId: '',
})

const filtersState = ref<Record<string, unknown>>({ query: '' })

const filterDefinitions = computed<OmniFilterDefinition[]>(() => [
  {
    key: 'query',
    label: 'Buscar',
    type: 'text',
    placeholder: 'Pesquisar por slug ou destino...',
    mode: 'columns',
    columns: ['clientName', 'slug', 'targetUrl', 'shortUrl'],
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
  {
    key: 'slug',
    label: 'Slug',
    type: 'text',
    editable: true,
    placeholder: 'meu-slug',
    minWidth: 150,
    maxWidth: 200,
  },
  {
    key: 'targetUrl',
    label: 'Destino',
    type: 'text',
    editable: true,
    placeholder: 'https://...',
    minWidth: 200,
    maxWidth: 300,
  },
  { key: 'hits', label: 'Cliques', type: 'number', editable: false, minWidth: 100 },
  {
    key: 'createdAt',
    label: 'Criado em',
    type: 'text',
    editable: false,
    minWidth: 170,
    formatter: (value) => formatDate(String(value ?? '')),
  },
  {
    key: 'shortUrl',
    label: 'Link Curto',
    type: 'text',
    editable: false,
    minWidth: 200,
    maxWidth: 300,
  },
  { key: 'actions', label: 'Opcoes', type: 'custom', minWidth: 160, align: 'center' },
])

const columnExcludeKeys = ['actions']
const { visibleColumnKeys, tableColumns } = useOmniVisibleColumns({
  preferenceKey: 'tools.short-links',
  allColumns: allTableColumns,
  columnExcludeKeys,
})

const filteredRows = computed(() => {
  const rows = shortLinks.value as unknown as Array<Record<string, unknown>>
  return applyOmniFilters(rows, filtersState.value, filterDefinitions.value)
})

function toShortLink(row: Record<string, unknown>) {
  return row as unknown as ShortLinkItem
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

function openExternal(url: string) {
  if (!import.meta.client) return
  const target = String(url ?? '').trim()
  if (!target) return
  window.open(target, '_blank', 'noopener,noreferrer')
}

async function copyText(value: string) {
  if (!import.meta.client) return
  const text = String(value ?? '').trim()
  if (!text || !navigator.clipboard?.writeText) return
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    // clipboard indisponivel — silencioso
  }
}

function openCreateModal() {
  createForm.targetUrl = ''
  createForm.slug = ''
  createForm.clientId = canChooseClient.value ? activeClientId.value : ''
  createSuccessMessage.value = ''
  modalOpen.value = true
}

async function submitCreate() {
  createSuccessMessage.value = ''
  const created = await createShortLink({
    targetUrl: createForm.targetUrl,
    slug: createForm.slug,
    accountId: canChooseClient.value ? createForm.clientId : undefined,
  })
  if (!created) return
  createSuccessMessage.value = `Link curto criado: ${created.shortUrl}`
  modalOpen.value = false
}

async function onDeleteLink(id: string) {
  if (import.meta.client) {
    const confirmed = window.confirm('Excluir este link curto? Esta acao nao pode ser desfeita.')
    if (!confirmed) return
  }
  await deleteShortLink(id)
}

const SHORT_LINK_EDITABLE = new Set(['slug', 'targetUrl'])

function onCellUpdate(payload: OmniTableCellUpdate) {
  const id = String(payload.rowId ?? '').trim()
  const field = String(payload.key)
  if (!id || !SHORT_LINK_EDITABLE.has(field)) return
  updateShortLink(id, field as 'slug' | 'targetUrl', payload.value, {
    immediate: payload.immediate,
  })
}

function onResetFilters() {
  filtersState.value = { query: '' }
}

onMounted(() => {
  void Promise.all([fetchShortLinks(), ensureClientOptions()])
})
</script>

<template>
  <section class="short-links-page space-y-4">
    <AdminPageHeader
      eyebrow="Tools"
      title="Encurtador de Link"
      description="Crie links curtos rastreados com slug personalizado e contagem de cliques."
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
          label="Novo link curto"
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
      empty-text="Nenhum link encontrado com os filtros atuais."
      @update:cell="onCellUpdate"
    >
      <template #cell-actions="{ row }">
        <div class="short-links-page__row-actions flex items-center justify-end gap-1">
          <UButton
            icon="i-lucide-copy"
            color="neutral"
            variant="ghost"
            size="sm"
            title="Copiar link curto"
            aria-label="Copiar link curto"
            @click="copyText(toShortLink(row).shortUrl)"
          />
          <UButton
            icon="i-lucide-external-link"
            color="neutral"
            variant="ghost"
            size="sm"
            title="Abrir link curto"
            aria-label="Abrir link curto"
            @click="openExternal(toShortLink(row).shortUrl)"
          />
          <UButton
            icon="i-lucide-link"
            color="neutral"
            variant="ghost"
            size="sm"
            title="Abrir destino"
            aria-label="Abrir destino"
            @click="openExternal(toShortLink(row).targetUrl)"
          />
          <UButton
            icon="i-lucide-trash-2"
            color="error"
            variant="ghost"
            size="sm"
            title="Excluir"
            aria-label="Excluir"
            :loading="deletingId === rowId(row) || Boolean(savingMap[`${rowId(row)}:delete`])"
            @click="onDeleteLink(rowId(row))"
          />
        </div>
      </template>
    </OmniDataTable>

    <UModal
      v-model:open="modalOpen"
      title="Novo link curto"
      description="Informe a URL original e opcionalmente um slug personalizado."
      :ui="{ content: 'max-w-xl' }"
    >
      <template #body>
        <div class="short-links-page__create-modal-body space-y-3">
          <div class="space-y-1">
            <p class="text-xs text-[rgb(var(--muted))]">URL original</p>
            <UInput v-model="createForm.targetUrl" placeholder="https://..." />
          </div>

          <div class="space-y-1">
            <p class="text-xs text-[rgb(var(--muted))]">Slug personalizado (opcional)</p>
            <UInput v-model="createForm.slug" placeholder="promo2026" />
            <p class="text-[11px] text-[rgb(var(--muted))]">
              Vazio gera um codigo aleatorio. Se o slug ja existir, um sufixo e adicionado.
            </p>
          </div>

          <div v-if="canChooseClient" class="space-y-1">
            <p class="text-xs text-[rgb(var(--muted))]">Conta dona</p>
            <USelect
              v-model="createForm.clientId"
              :items="clientOptions"
              placeholder="Selecionar conta"
            />
          </div>
          <div v-else class="space-y-1">
            <p class="text-xs text-[rgb(var(--muted))]">Conta dona</p>
            <UInput :model-value="activeClientLabel" disabled />
          </div>

          <UAlert
            v-if="createSuccessMessage"
            color="success"
            variant="soft"
            icon="i-lucide-badge-check"
            :description="createSuccessMessage"
          />
        </div>
      </template>

      <template #footer>
        <div class="flex w-full items-center justify-end gap-2">
          <UButton label="Fechar" color="neutral" variant="ghost" @click="modalOpen = false" />
          <UButton
            label="Encurtar"
            icon="i-lucide-scissors"
            color="primary"
            :loading="creating"
            :disabled="creating"
            @click="submitCreate"
          />
        </div>
      </template>
    </UModal>
  </section>
</template>
