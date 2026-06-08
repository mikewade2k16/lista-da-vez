<script setup lang="ts">
import OmniCollectionFilters from '~/components/omni/filters/OmniCollectionFilters.vue'
import AppDetailDialog from '~/components/ui/AppDetailDialog.vue'
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import WebhookSourcesDrawer from '~/components/site/WebhookSourcesDrawer.vue'
import SiteTrackingDashboard from '~/components/site/SiteTrackingDashboard.vue'
import type { SiteTrackingEventItem } from '~/types/tracking'
import type { OmniFilterDefinition, OmniTableColumn } from '~/types/omni/collection'

const {
  events,
  filters,
  page,
  perPage,
  total,
  loading,
  errorMessage,
  fetchEvents,
  setPage,
  setPerPage,
  resetFilters,
} = useSiteTrackingManager()

const {
  analytics,
  days: analyticsDays,
  loading: analyticsLoading,
  errorMessage: analyticsError,
  fetchAnalytics,
  setDays,
} = useSiteTrackingAnalytics()

const activeTab = ref<'resumo' | 'eventos'>('resumo')
const tabs = [
  { key: 'resumo' as const, label: 'Resumo', icon: 'i-lucide-bar-chart-3' },
  { key: 'eventos' as const, label: 'Eventos', icon: 'i-lucide-list' },
]

function onChangeDays(value: number) {
  setDays(value)
  void fetchAnalytics()
}

const auth = useAuthStore()
const managerRoles = new Set(['platform_admin', 'owner', 'director', 'manager', 'admin'])
const canManageSources = computed(() => managerRoles.has(String(auth.role ?? '')))

const selectedIds = ref<Array<string | number>>([])
const sourcesDrawerOpen = ref(false)

const filtersState = ref<Record<string, unknown>>({
  query: '',
  sourceFilter: '',
  eventTypeFilter: '',
  pagePathFilter: '',
})

const pageSizeOptions = [
  { label: '25 por pagina', value: 25 },
  { label: '50 por pagina', value: 50 },
  { label: '100 por pagina', value: 100 },
  { label: '200 por pagina', value: 200 },
]

const sourceOptions = computed(() => {
  const map = new Map<string, { label: string; value: string }>()
  for (const item of events.value) {
    const value = (item.source || item.sourceLabel).trim()
    if (!value || map.has(value)) continue
    map.set(value, {
      label: item.sourceLabel || item.source || value,
      value,
    })
  }
  return [...map.values()].sort((left, right) => left.label.localeCompare(right.label, 'pt-BR'))
})

const eventTypeOptions = computed(() => {
  const map = new Map<string, { label: string; value: string }>()
  for (const item of events.value) {
    const value = item.eventType.trim()
    if (!value || map.has(value)) continue
    map.set(value, { label: value, value })
  }
  return [...map.values()].sort((left, right) => left.label.localeCompare(right.label, 'pt-BR'))
})

const filterDefinitions = computed<OmniFilterDefinition[]>(() => [
  {
    key: 'query',
    label: 'Buscar',
    type: 'text',
    placeholder: 'Fonte, evento, visitante, sessao, produto...',
    mode: 'all',
  },
  {
    key: 'sourceFilter',
    label: 'Fonte',
    type: 'select',
    placeholder: 'Filtrar por fonte',
    options: sourceOptions.value,
    accessor: (row) => row.source || row.sourceLabel,
  },
  {
    key: 'eventTypeFilter',
    label: 'Tipo de evento',
    type: 'select',
    placeholder: 'Filtrar por tipo',
    options: eventTypeOptions.value,
    accessor: (row) => row.eventType,
  },
  {
    key: 'pagePathFilter',
    label: 'Pagina',
    type: 'text',
    placeholder: '/produto/perola',
    mode: 'all',
  },
])

function formatDateTime(value: unknown) {
  const text = String(value ?? '').trim()
  if (!text) return '-'
  const parsed = new Date(text)
  if (Number.isNaN(parsed.getTime())) return text
  return parsed.toLocaleString('pt-BR')
}

const allTableColumns = computed<OmniTableColumn[]>(() => [
  {
    key: 'receivedAt',
    label: 'Recebido em',
    type: 'text',
    editable: false,
    minWidth: 170,
    locked: true,
    defaultOrder: 10,
    formatter: (value) => formatDateTime(value),
  },
  {
    key: 'sourceLabel',
    label: 'Fonte',
    type: 'text',
    editable: false,
    minWidth: 180,
    defaultOrder: 20,
  },
  {
    key: 'eventType',
    label: 'Tipo',
    type: 'text',
    editable: false,
    minWidth: 150,
    defaultOrder: 30,
  },
  {
    key: 'eventName',
    label: 'Evento',
    type: 'text',
    editable: false,
    minWidth: 190,
    defaultOrder: 40,
  },
  {
    key: 'pagePath',
    label: 'Pagina',
    type: 'text',
    editable: false,
    minWidth: 220,
    defaultOrder: 50,
  },
  {
    key: 'pageTitle',
    label: 'Titulo',
    type: 'text',
    editable: false,
    minWidth: 220,
    defaultOrder: 60,
  },
  {
    key: 'sessionId',
    label: 'Sessao',
    type: 'text',
    editable: false,
    minWidth: 180,
    defaultOrder: 70,
  },
  {
    key: 'visitorId',
    label: 'Visitante',
    type: 'text',
    editable: false,
    minWidth: 180,
    defaultOrder: 80,
  },
  {
    key: 'productCode',
    label: 'Produto',
    type: 'text',
    editable: false,
    minWidth: 140,
    defaultOrder: 90,
  },
  {
    key: 'deviceType',
    label: 'Device',
    type: 'text',
    editable: false,
    minWidth: 130,
    defaultOrder: 100,
  },
  {
    key: 'utmCampaign',
    label: 'UTM campanha',
    type: 'text',
    editable: false,
    minWidth: 180,
    defaultOrder: 110,
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
    preferenceKey: 'admin.site.tracking',
    allColumns: allTableColumns,
    columnExcludeKeys,
  })

const tableRows = computed<Array<Record<string, unknown>>>(() => {
  const seen = new Set<string>()
  return events.value
    .filter((item) => {
      const id = item.id.trim()
      if (!id || seen.has(id)) return false
      seen.add(id)
      return true
    })
    .map((item) => item as unknown as Record<string, unknown>)
})

function toEvent(row: Record<string, unknown>) {
  return row as unknown as SiteTrackingEventItem
}

function syncManagerFilters() {
  filters.q = String(filtersState.value.query ?? '').trim()
  filters.source = String(filtersState.value.sourceFilter ?? '').trim()
  filters.eventType = String(filtersState.value.eventTypeFilter ?? '').trim()
  filters.pagePath = String(filtersState.value.pagePathFilter ?? '').trim()
}

let filterSyncTimer: ReturnType<typeof setTimeout> | null = null

watch(
  filtersState,
  () => {
    if (filterSyncTimer) clearTimeout(filterSyncTimer)
    filterSyncTimer = setTimeout(() => {
      syncManagerFilters()
      setPage(1)
      selectedIds.value = []
      void fetchEvents()
    }, 260)
  },
  { deep: true },
)

function onResetFilters() {
  if (filterSyncTimer) clearTimeout(filterSyncTimer)
  filtersState.value = {
    query: '',
    sourceFilter: '',
    eventTypeFilter: '',
    pagePathFilter: '',
  }
  resetFilters()
  setPage(1)
  selectedIds.value = []
  void fetchEvents()
}

async function onRefresh() {
  syncManagerFilters()
  selectedIds.value = []
  await fetchEvents()
}

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / perPage.value)))
const pageStart = computed(() => (total.value === 0 ? 0 : (page.value - 1) * perPage.value + 1))
const pageEnd = computed(() => Math.min(total.value, page.value * perPage.value))

async function goToPage(nextPage: number) {
  if (nextPage < 1 || nextPage > totalPages.value || nextPage === page.value) return
  setPage(nextPage)
  selectedIds.value = []
  await fetchEvents()
}

async function onPerPageChange(value: string | number) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return
  setPerPage(parsed)
  setPage(1)
  selectedIds.value = []
  await fetchEvents()
}

const detailOpen = ref(false)
const selectedEventId = ref<string | null>(null)
const selectedEvent = computed(
  () => events.value.find((item) => item.id === selectedEventId.value) ?? null,
)

function openDetails(id: string) {
  selectedEventId.value = id
  detailOpen.value = true
}

const detailSubtitle = computed(() => {
  const item = selectedEvent.value
  if (!item) return ''
  return [
    item.sourceLabel || item.source,
    item.pagePath || item.pageUrl,
    formatDateTime(item.receivedAt),
  ]
    .filter(Boolean)
    .join(' • ')
})

const detailSections = computed(() => {
  const item = selectedEvent.value
  if (!item) return []
  return [
    {
      id: 'event',
      title: 'Evento',
      fields: [
        { label: 'Tipo', value: item.eventType },
        { label: 'Nome', value: item.eventName },
        { label: 'Fonte', value: item.sourceLabel || item.source },
        { label: 'Batch', value: item.batchId },
        { label: 'Evento origem', value: item.sourceEventId },
        { label: 'Produto', value: item.productCode },
      ],
    },
    {
      id: 'page',
      title: 'Pagina',
      fields: [
        { label: 'URL', value: item.pageUrl },
        { label: 'Path', value: item.pagePath },
        { label: 'Titulo', value: item.pageTitle },
        { label: 'Grupo', value: item.pageGroup },
        { label: 'Nome', value: item.pageName },
        { label: 'Referrer', value: item.referrer },
      ],
    },
    {
      id: 'element',
      title: 'Elemento',
      fields: [
        { label: 'Tag', value: item.elementTag },
        { label: 'Texto', value: item.elementText },
        { label: 'Href', value: item.elementHref },
        { label: 'ID', value: item.elementId },
        { label: 'Classes', value: item.elementClasses },
        { label: 'Role', value: item.elementRole },
      ],
    },
    {
      id: 'session',
      title: 'Sessao e device',
      fields: [
        { label: 'Sessao', value: item.sessionId },
        { label: 'Visitante', value: item.visitorId },
        { label: 'Device', value: item.deviceType },
        { label: 'Idioma', value: item.browserLang },
        { label: 'Timezone', value: item.timezone },
        { label: 'Ativo (s)', value: item.activeSeconds },
        { label: 'Scroll', value: item.scrollDepth },
        {
          label: 'Tela',
          value: [item.screenWidth, item.screenHeight].every((value) => value)
            ? `${item.screenWidth}x${item.screenHeight}`
            : '-',
        },
        {
          label: 'Viewport',
          value: [item.viewportWidth, item.viewportHeight].every((value) => value)
            ? `${item.viewportWidth}x${item.viewportHeight}`
            : '-',
        },
      ],
    },
    {
      id: 'utm',
      title: 'UTM',
      fields: [
        { label: 'Source', value: item.utmSource },
        { label: 'Medium', value: item.utmMedium },
        { label: 'Campaign', value: item.utmCampaign },
        { label: 'Term', value: item.utmTerm },
        { label: 'Content', value: item.utmContent },
      ],
    },
    {
      id: 'transport',
      title: 'Entrega',
      fields: [
        { label: 'IP', value: item.ip },
        { label: 'User agent', value: item.userAgent },
        { label: 'Enviado em', value: formatDateTime(item.sentAt) },
        { label: 'Recebido em', value: formatDateTime(item.receivedAt) },
      ],
    },
  ]
})

function stringifyPayload(payload: string) {
  const text = String(payload ?? '').trim()
  if (!text) return ''
  try {
    return JSON.stringify(JSON.parse(text), null, 2)
  } catch {
    return text
  }
}

onMounted(() => {
  syncManagerFilters()
  void fetchAnalytics()
  void fetchEvents()
})

onBeforeUnmount(() => {
  if (filterSyncTimer) clearTimeout(filterSyncTimer)
})
</script>

<template>
  <section class="site-tracking-workspace flex h-full min-h-0 flex-col gap-4 overflow-hidden">
    <AdminPageHeader
      eyebrow="Site"
      title="Tracking"
      description="Dashboard de analytics e eventos brutos recebidos pelo webhook de tracking do site."
    />

    <div class="flex flex-wrap items-center justify-between gap-3">
      <div
        class="flex items-center gap-1 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-1"
      >
        <UButton
          v-for="tab in tabs"
          :key="tab.key"
          :icon="tab.icon"
          :label="tab.label"
          size="sm"
          :color="activeTab === tab.key ? 'primary' : 'neutral'"
          :variant="activeTab === tab.key ? 'soft' : 'ghost'"
          @click="activeTab = tab.key"
        />
      </div>
      <UButton
        v-if="canManageSources"
        icon="i-lucide-webhook"
        label="Fontes"
        color="primary"
        variant="soft"
        @click="sourcesDrawerOpen = true"
      />
    </div>

    <template v-if="activeTab === 'resumo'">
      <UAlert
        v-if="analyticsError"
        color="error"
        variant="soft"
        icon="i-lucide-alert-triangle"
        title="Erro"
        :description="analyticsError"
      />
      <div class="flex-1 min-h-0 overflow-y-auto">
        <SiteTrackingDashboard
          :analytics="analytics"
          :loading="analyticsLoading"
          :days="analyticsDays"
          @update:days="onChangeDays"
          @refresh="fetchAnalytics"
        />
      </div>
    </template>

    <template v-else>
      <OmniCollectionFilters
        v-model="filtersState"
        v-model:visible-columns="visibleColumnKeys"
        v-model:locked-columns="lockedColumnKeys"
        v-model:column-order="columnOrder"
        viewer-user-type="admin"
        :filters="filterDefinitions"
        :table-columns="allTableColumns"
        :column-exclude-keys="columnExcludeKeys"
        :loading="loading"
        @reset="onResetFilters"
        @reset-columns="resetToDefaults"
      >
        <template #actions>
          <UBadge color="neutral" variant="soft">
            Mostrando {{ tableRows.length }} de {{ total }}
          </UBadge>
          <UBadge color="neutral" variant="soft">Selecionados: {{ selectedIds.length }}</UBadge>
          <UButton
            icon="i-lucide-refresh-cw"
            label="Atualizar"
            color="neutral"
            variant="soft"
            :loading="loading"
            @click="onRefresh"
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

      <div class="flex-1 min-h-0 overflow-y-auto">
        <OmniDataTable
          v-model="selectedIds"
          :rows="tableRows"
          :columns="tableColumns"
          row-key="id"
          :loading="loading"
          empty-text="Nenhum evento de tracking encontrado."
        >
          <template #cell-actions="{ row }">
            <div class="flex items-center justify-end gap-1">
              <UButton
                icon="i-lucide-info"
                color="neutral"
                variant="ghost"
                size="sm"
                title="Detalhes"
                aria-label="Detalhes do evento"
                @click="openDetails(toEvent(row).id)"
              />
            </div>
          </template>
        </OmniDataTable>
      </div>

      <div
        class="flex flex-wrap items-center justify-between gap-3 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-4 py-3"
      >
        <p class="text-sm text-[rgb(var(--muted))]">
          {{ pageStart }}-{{ pageEnd }} de {{ total }} eventos
        </p>
        <div class="flex flex-wrap items-center gap-2">
          <USelect
            :model-value="perPage"
            :items="pageSizeOptions"
            @update:model-value="onPerPageChange($event as string | number)"
          />
          <UButton
            icon="i-lucide-chevron-left"
            color="neutral"
            variant="soft"
            :disabled="page <= 1 || loading"
            @click="goToPage(page - 1)"
          />
          <UBadge color="neutral" variant="soft">Pagina {{ page }} de {{ totalPages }}</UBadge>
          <UButton
            icon="i-lucide-chevron-right"
            color="neutral"
            variant="soft"
            :disabled="page >= totalPages || loading"
            @click="goToPage(page + 1)"
          />
        </div>
      </div>
    </template>

    <AppDetailDialog
      v-model="detailOpen"
      :title="selectedEvent?.eventName || selectedEvent?.eventType || 'Detalhes do tracking'"
      :subtitle="detailSubtitle"
      :sections="detailSections"
      width="min(72rem, calc(100vw - 2rem))"
    >
      <section
        v-if="selectedEvent?.eventData"
        class="grid gap-2 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-4"
      >
        <header class="grid gap-1">
          <h4 class="text-sm font-semibold text-[rgb(var(--text))]">eventData</h4>
          <p class="text-xs text-[rgb(var(--muted))]">Payload normalizado do evento.</p>
        </header>
        <pre
          class="max-h-72 overflow-auto rounded-[var(--radius-sm)] bg-[rgb(var(--surface))] p-3 text-xs"
          >{{ stringifyPayload(selectedEvent.eventData) }}</pre
        >
      </section>

      <section
        v-if="selectedEvent?.rawPayload"
        class="grid gap-2 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-4"
      >
        <header class="grid gap-1">
          <h4 class="text-sm font-semibold text-[rgb(var(--text))]">rawPayload</h4>
          <p class="text-xs text-[rgb(var(--muted))]">Corpo original recebido pelo webhook.</p>
        </header>
        <pre
          class="max-h-72 overflow-auto rounded-[var(--radius-sm)] bg-[rgb(var(--surface))] p-3 text-xs"
          >{{ stringifyPayload(selectedEvent.rawPayload) }}</pre
        >
      </section>
    </AppDetailDialog>

    <WebhookSourcesDrawer v-model:open="sourcesDrawerOpen" default-entity="tracking" />
  </section>
</template>
