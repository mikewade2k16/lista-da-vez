<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { formatCurrencyBRL, formatDurationMinutes } from '~/domain/utils/admin-metrics'
import AppDetailDialog from '~/components/ui/AppDetailDialog.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'

type RangeKey = 'today' | '7d' | '30d' | 'month'

interface OptionItem {
  id?: string
  label?: string
}

interface HistoryEntry {
  serviceId?: string
  storeId?: string
  storeName?: string
  personId?: string
  personName?: string
  finishedAt?: number
  finishOutcome?: string
  saleAmount?: number
  durationMs?: number
  queueWaitMs?: number
  startMode?: string
  customerName?: string
  customerPhone?: string
  customerEmail?: string
  customerProfession?: string
  isExistingCustomer?: boolean
  isWindowService?: boolean
  isGift?: boolean
  productSeen?: string
  productClosed?: string
  productDetails?: string
  productsSeen?: unknown[]
  productsClosed?: unknown[]
  visitReasons?: string[]
  visitReasonDetails?: Record<string, string>
  customerSources?: string[]
  customerSourceDetails?: Record<string, string>
  lossReasons?: string[]
  lossReason?: string
  lossReasonDetails?: Record<string, string>
  queueJumpReason?: string
  notes?: string
  campaignMatches?: Array<Record<string, unknown>>
  campaignBonusTotal?: number
}

interface AttendanceRow {
  key: string
  serviceId: string
  storeName: string
  consultantName: string
  finishedAt: number
  finishedAtLabel: string
  outcome: string
  outcomeLabel: string
  saleAmount: number
  saleAmountLabel: string
  durationLabel: string
  queueWaitLabel: string
  startModeLabel: string
  customerName: string
  customerPhone: string
  customerEmail: string
  customerProfession: string
  existingCustomerLabel: string
  windowServiceLabel: string
  giftLabel: string
  productSeenLabel: string
  productClosedLabel: string
  visitReasonsLabel: string
  customerSourcesLabel: string
  lossReasonsLabel: string
  queueJumpReason: string
  notes: string
  campaignNamesLabel: string
  campaignBonusTotalLabel: string
}

const DAY_IN_MS = 24 * 60 * 60 * 1000
const PAGE_SIZE = 5

const RANGE_OPTIONS: Array<{ key: RangeKey; label: string }> = [
  { key: 'today', label: 'Hoje' },
  { key: '7d', label: '7 dias' },
  { key: '30d', label: '30 dias' },
  { key: 'month', label: 'Mes' },
]

const OUTCOME_LABELS: Record<string, string> = {
  compra: 'Compra',
  reserva: 'Reserva',
  'nao-compra': 'Nao compra',
}

const START_MODE_LABELS: Record<string, string> = {
  queue: 'Na vez',
  'queue-jump': 'Fora da vez',
}

const props = defineProps<{
  consultantId: string
  consultantName?: string
  storeId?: string
  storeName?: string
  entries?: HistoryEntry[]
  visitReasonOptions?: OptionItem[]
  customerSourceOptions?: OptionItem[]
}>()

const activeRange = ref<RangeKey>('7d')
const searchTerm = ref('')
const outcomeFilter = ref('all')
const currentPage = ref(1)
const selectedRow = ref<AttendanceRow | null>(null)

function startOfDay(timestamp: number) {
  const date = new Date(timestamp)
  date.setHours(0, 0, 0, 0)
  return date.getTime()
}

function toComparableText(value: unknown) {
  return String(value || '')
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
}

function formatDateTime(timestamp: number) {
  if (!timestamp) {
    return '-'
  }

  return new Intl.DateTimeFormat('pt-BR', {
    dateStyle: 'short',
    timeStyle: 'short',
  }).format(timestamp)
}

function formatDurationValue(value: number) {
  return value > 0 ? formatDurationMinutes(value) : '-'
}

function normalizeList(values: unknown) {
  return Array.isArray(values)
    ? values.map((item) => String(item || '').trim()).filter(Boolean)
    : []
}

function normalizeProductLabel(item: unknown) {
  if (typeof item === 'string') {
    return item.trim()
  }

  if (item && typeof item === 'object') {
    const record = item as Record<string, unknown>

    return String(
      record.name || record.label || record.title || record.sku || record.id || '',
    ).trim()
  }

  return ''
}

function formatProductList(values: unknown, fallback = '') {
  const normalized = Array.isArray(values)
    ? values.map((item) => normalizeProductLabel(item)).filter(Boolean)
    : []

  if (normalized.length) {
    return normalized.join(', ')
  }

  return String(fallback || '').trim() || '-'
}

function buildLabelMap(options: OptionItem[] | undefined) {
  return new Map(
    (options || []).map((item) => [
      String(item?.id || '').trim(),
      String(item?.label || '').trim(),
    ]),
  )
}

function formatListWithDetails(
  values: unknown,
  details: Record<string, string> | undefined,
  labelMap: Map<string, string>,
) {
  const normalizedValues = normalizeList(values)
  const items = normalizedValues.map((value) => {
    const label = labelMap.get(value) || value
    const detail = String(details?.[value] || '').trim()
    return detail ? `${label}: ${detail}` : label
  })

  Object.entries(details || {}).forEach(([key, value]) => {
    const trimmedValue = String(value || '').trim()
    if (!trimmedValue || normalizedValues.includes(key)) {
      return
    }

    const label = labelMap.get(key) || key
    items.push(`${label}: ${trimmedValue}`)
  })

  return items.filter(Boolean).join(', ') || '-'
}

function formatLossReasons(entry: HistoryEntry) {
  const normalizedValues = normalizeList(entry.lossReasons)
  if (String(entry.lossReason || '').trim()) {
    normalizedValues.unshift(String(entry.lossReason || '').trim())
  }

  return formatListWithDetails(
    [...new Set(normalizedValues)],
    entry.lossReasonDetails,
    new Map<string, string>(),
  )
}

function formatCampaignNames(matches: unknown) {
  const items = Array.isArray(matches)
    ? matches
        .map((item) => {
          if (!item || typeof item !== 'object') {
            return ''
          }

          const record = item as Record<string, unknown>
          return String(record.campaignName || record.campaignId || '').trim()
        })
        .filter(Boolean)
    : []

  return items.join(', ') || '-'
}

const outcomeOptions = [
  { value: 'all', label: 'Todos os desfechos' },
  { value: 'compra', label: 'Compra' },
  { value: 'reserva', label: 'Reserva' },
  { value: 'nao-compra', label: 'Nao compra' },
]

const visitReasonMap = computed(() => buildLabelMap(props.visitReasonOptions))
const customerSourceMap = computed(() => buildLabelMap(props.customerSourceOptions))

const rangeStart = computed(() => {
  const now = Date.now()
  const today = startOfDay(now)

  switch (activeRange.value) {
    case 'today':
      return today
    case '30d':
      return today - 29 * DAY_IN_MS
    case 'month': {
      const date = new Date(now)
      date.setDate(1)
      date.setHours(0, 0, 0, 0)
      return date.getTime()
    }
    case '7d':
    default:
      return today - 6 * DAY_IN_MS
  }
})

const consultantEntries = computed(() => {
  const targetConsultantId = String(props.consultantId || '').trim()
  const targetStoreId = String(props.storeId || '').trim()

  return (props.entries || [])
    .filter((entry) => String(entry.personId || '').trim() === targetConsultantId)
    .filter((entry) => !targetStoreId || String(entry.storeId || '').trim() === targetStoreId)
    .sort((left, right) => Number(right.finishedAt || 0) - Number(left.finishedAt || 0))
})

const rangedEntries = computed(() => {
  const start = rangeStart.value
  const end = Date.now()

  return consultantEntries.value.filter((entry) => {
    const finishedAt = Number(entry.finishedAt || 0)
    return finishedAt >= start && finishedAt <= end
  })
})

const rows = computed<AttendanceRow[]>(() =>
  rangedEntries.value.map((entry, index) => ({
    key: `${String(entry.serviceId || 'attendance').trim()}-${Number(entry.finishedAt || 0)}-${index}`,
    serviceId: String(entry.serviceId || '').trim() || '-',
    storeName: String(entry.storeName || props.storeName || '').trim() || '-',
    consultantName: String(entry.personName || props.consultantName || '').trim() || '-',
    finishedAt: Number(entry.finishedAt || 0),
    finishedAtLabel: formatDateTime(Number(entry.finishedAt || 0)),
    outcome: String(entry.finishOutcome || 'nao-compra').trim() || 'nao-compra',
    outcomeLabel:
      OUTCOME_LABELS[String(entry.finishOutcome || 'nao-compra').trim()] || 'Nao compra',
    saleAmount: Number(entry.saleAmount || 0),
    saleAmountLabel: formatCurrencyBRL(Number(entry.saleAmount || 0)),
    durationLabel: formatDurationValue(Number(entry.durationMs || 0)),
    queueWaitLabel: formatDurationValue(Number(entry.queueWaitMs || 0)),
    startModeLabel: START_MODE_LABELS[String(entry.startMode || 'queue').trim()] || 'Na vez',
    customerName: String(entry.customerName || '').trim() || '-',
    customerPhone: String(entry.customerPhone || '').trim() || '-',
    customerEmail: String(entry.customerEmail || '').trim() || '-',
    customerProfession: String(entry.customerProfession || '').trim() || '-',
    existingCustomerLabel: entry.isExistingCustomer ? 'Recorrente' : 'Novo cliente',
    windowServiceLabel: entry.isWindowService ? 'Sim' : 'Nao',
    giftLabel: entry.isGift ? 'Sim' : 'Nao',
    productSeenLabel: formatProductList(
      entry.productsSeen,
      entry.productSeen || entry.productDetails || '',
    ),
    productClosedLabel: formatProductList(entry.productsClosed, entry.productClosed || ''),
    visitReasonsLabel: formatListWithDetails(
      entry.visitReasons,
      entry.visitReasonDetails,
      visitReasonMap.value,
    ),
    customerSourcesLabel: formatListWithDetails(
      entry.customerSources,
      entry.customerSourceDetails,
      customerSourceMap.value,
    ),
    lossReasonsLabel: formatLossReasons(entry),
    queueJumpReason: String(entry.queueJumpReason || '').trim() || '-',
    notes: String(entry.notes || '').trim() || '-',
    campaignNamesLabel: formatCampaignNames(entry.campaignMatches),
    campaignBonusTotalLabel: formatCurrencyBRL(Number(entry.campaignBonusTotal || 0)),
  })),
)

const filteredRows = computed(() => {
  const normalizedSearch = toComparableText(searchTerm.value)

  return rows.value.filter((row) => {
    if (outcomeFilter.value !== 'all' && row.outcome !== outcomeFilter.value) {
      return false
    }

    if (!normalizedSearch) {
      return true
    }

    return [
      row.serviceId,
      row.customerName,
      row.customerPhone,
      row.customerEmail,
      row.productClosedLabel,
      row.productSeenLabel,
      row.visitReasonsLabel,
      row.customerSourcesLabel,
      row.notes,
    ].some((value) => toComparableText(value).includes(normalizedSearch))
  })
})

const totalPages = computed(() => Math.max(1, Math.ceil(filteredRows.value.length / PAGE_SIZE)))

const pagedRows = computed(() => {
  const start = (currentPage.value - 1) * PAGE_SIZE
  return filteredRows.value.slice(start, start + PAGE_SIZE)
})

const detailSections = computed(() => {
  if (!selectedRow.value) {
    return []
  }

  const row = selectedRow.value

  return [
    {
      id: 'service',
      title: 'Atendimento',
      fields: [
        { label: 'Data e hora', value: row.finishedAtLabel },
        { label: 'ID atendimento', value: row.serviceId },
        { label: 'Loja', value: row.storeName },
        { label: 'Consultor', value: row.consultantName },
        { label: 'Desfecho', value: row.outcomeLabel },
        { label: 'Venda', value: row.saleAmountLabel },
        { label: 'Duracao', value: row.durationLabel },
        { label: 'Espera em fila', value: row.queueWaitLabel },
        { label: 'Tipo', value: row.startModeLabel },
        { label: 'Janela', value: row.windowServiceLabel },
        { label: 'Presente', value: row.giftLabel },
      ],
    },
    {
      id: 'client',
      title: 'Cliente',
      fields: [
        { label: 'Nome', value: row.customerName },
        { label: 'Telefone', value: row.customerPhone },
        { label: 'Email', value: row.customerEmail },
        { label: 'Profissao', value: row.customerProfession },
        { label: 'Relacionamento', value: row.existingCustomerLabel },
      ],
    },
    {
      id: 'commercial',
      title: 'Dados comerciais',
      fields: [
        { label: 'Produto visto', value: row.productSeenLabel },
        { label: 'Produto fechado', value: row.productClosedLabel },
        { label: 'Motivos de visita', value: row.visitReasonsLabel },
        { label: 'Origens do cliente', value: row.customerSourcesLabel },
        { label: 'Motivos de perda', value: row.lossReasonsLabel },
        { label: 'Motivo fora da vez', value: row.queueJumpReason },
        { label: 'Campanhas', value: row.campaignNamesLabel },
        { label: 'Bonus de campanha', value: row.campaignBonusTotalLabel },
      ],
    },
    {
      id: 'notes',
      title: 'Observacoes',
      fields: [{ label: 'Notas', value: row.notes }],
    },
  ]
})

const detailOpen = computed(() => Boolean(selectedRow.value))

const detailTitle = computed(() => {
  if (!selectedRow.value) {
    return 'Detalhes do atendimento'
  }

  return selectedRow.value.customerName !== '-'
    ? selectedRow.value.customerName
    : `Atendimento ${selectedRow.value.serviceId}`
})

const detailSubtitle = computed(() => {
  if (!selectedRow.value) {
    return ''
  }

  return [selectedRow.value.finishedAtLabel, selectedRow.value.consultantName]
    .filter(Boolean)
    .join(' | ')
})

watch([activeRange, searchTerm, outcomeFilter], () => {
  currentPage.value = 1
})

watch(totalPages, (value) => {
  if (currentPage.value > value) {
    currentPage.value = value
  }
})

function openDetails(row: AttendanceRow) {
  selectedRow.value = row
}

function handleDetailDialogChange(isOpen: boolean) {
  if (!isOpen) {
    selectedRow.value = null
  }
}
</script>

<template>
  <section
    class="consultant-attendances insight-card insight-card--wide"
    data-testid="consultant-attendances-table"
  >
    <header class="consultant-attendances__header intel-card__header">
      <div>
        <h3 class="insight-card__title">Ultimos atendimentos</h3>
        <p class="consultant-attendances__text">
          Recorte recente do consultor com filtros locais e detalhe completo por atendimento.
        </p>
      </div>
      <span class="insight-tag">{{ filteredRows.length }} no recorte</span>
    </header>

    <div class="consultant-attendances__toolbar">
      <div class="consultant-attendances__ranges" role="tablist" aria-label="Filtros de periodo">
        <button
          v-for="option in RANGE_OPTIONS"
          :key="option.key"
          type="button"
          class="consultant-attendances__range"
          :class="{ 'consultant-attendances__range--active': activeRange === option.key }"
          @click="activeRange = option.key"
        >
          {{ option.label }}
        </button>
      </div>

      <div class="consultant-attendances__filters">
        <label class="settings-field consultant-attendances__search">
          <span>Buscar</span>
          <input v-model="searchTerm" type="text" placeholder="Cliente, produto ou anotacao" />
        </label>

        <label class="settings-field consultant-attendances__outcome">
          <span>Desfecho</span>
          <AppSelectField
            :model-value="outcomeFilter"
            :options="outcomeOptions"
            placeholder="Todos os desfechos"
            @update:model-value="outcomeFilter = String($event || 'all')"
          />
        </label>
      </div>
    </div>

    <div class="insight-table-wrap">
      <table class="insight-table">
        <thead>
          <tr>
            <th>Data/Hora</th>
            <th>Cliente</th>
            <th>Desfecho</th>
            <th>Venda</th>
            <th>Tipo</th>
            <th>Produto</th>
            <th>Acoes</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!pagedRows.length">
            <td colspan="7">Nenhum atendimento encontrado para os filtros selecionados.</td>
          </tr>
          <tr v-for="row in pagedRows" :key="row.key">
            <td>{{ row.finishedAtLabel }}</td>
            <td>{{ row.customerName }}</td>
            <td>
              <span
                class="consultant-attendances__badge"
                :class="`consultant-attendances__badge--${row.outcome}`"
              >
                {{ row.outcomeLabel }}
              </span>
            </td>
            <td>{{ row.saleAmountLabel }}</td>
            <td>{{ row.startModeLabel }}</td>
            <td>{{ row.productClosedLabel }}</td>
            <td>
              <button
                type="button"
                class="consultant-attendances__action"
                @click="openDetails(row)"
              >
                Ver detalhes
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <footer class="consultant-attendances__footer">
      <span class="consultant-attendances__meta">
        {{ filteredRows.length }} resultado(s) | Pagina {{ currentPage }} de {{ totalPages }}
      </span>

      <div class="consultant-attendances__pagination">
        <button
          type="button"
          class="consultant-attendances__page-btn"
          :disabled="currentPage <= 1"
          @click="currentPage -= 1"
        >
          Anterior
        </button>
        <button
          type="button"
          class="consultant-attendances__page-btn"
          :disabled="currentPage >= totalPages"
          @click="currentPage += 1"
        >
          Proxima
        </button>
      </div>
    </footer>

    <AppDetailDialog
      :model-value="detailOpen"
      :title="detailTitle"
      :subtitle="detailSubtitle"
      :sections="detailSections"
      width="min(58rem, calc(100vw - 2rem))"
      @update:model-value="handleDetailDialogChange"
    />
  </section>
</template>

<style scoped>
.consultant-attendances {
  display: grid;
  gap: 1rem;
}

.consultant-attendances__header {
  align-items: flex-start;
}

.consultant-attendances__text {
  margin: 0.28rem 0 0;
  color: rgb(var(--muted) / 0.92);
  font-size: 0.8rem;
  line-height: 1.45;
}

.consultant-attendances__toolbar {
  display: grid;
  gap: 0.85rem;
}

.consultant-attendances__ranges {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.consultant-attendances__range {
  padding: 0.42rem 0.72rem;
  border-radius: 999px;
  border: 1px solid rgb(var(--border) / 0.82);
  background: rgb(var(--surface-2) / 0.74);
  color: rgb(var(--muted) / 0.96);
  font-size: 0.74rem;
  font-weight: 600;
  cursor: pointer;
}

.consultant-attendances__range--active {
  border-color: rgb(var(--ring) / 0.42);
  background: rgb(var(--primary) / 0.18);
  color: rgb(var(--primary));
}

.consultant-attendances__filters {
  display: grid;
  grid-template-columns: minmax(0, 1.5fr) minmax(0, 1fr);
  gap: 0.85rem;
}

.consultant-attendances__search,
.consultant-attendances__outcome {
  min-width: 0;
}

.consultant-attendances__search input {
  width: 100%;
}

.consultant-attendances__badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 5.9rem;
  padding: 0.3rem 0.55rem;
  border-radius: 999px;
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.consultant-attendances__badge--compra {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.consultant-attendances__badge--reserva {
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.consultant-attendances__badge--nao-compra {
  background: rgb(var(--danger) / 0.14);
  color: rgb(var(--danger));
}

.consultant-attendances__action,
.consultant-attendances__page-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.5rem 0.8rem;
  border-radius: 0.8rem;
  border: 1px solid rgb(var(--ring) / 0.22);
  background: rgb(var(--surface-2) / 0.82);
  color: rgb(var(--text));
  font-size: 0.76rem;
  font-weight: 700;
  cursor: pointer;
  transition:
    border-color 0.18s ease,
    background 0.18s ease,
    color 0.18s ease;
}

.consultant-attendances__action:hover,
.consultant-attendances__page-btn:hover:not(:disabled) {
  border-color: rgb(var(--ring) / 0.42);
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.consultant-attendances__page-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.consultant-attendances__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.85rem;
}

.consultant-attendances__meta {
  color: rgb(var(--muted) / 0.9);
  font-size: 0.78rem;
}

.consultant-attendances__pagination {
  display: flex;
  align-items: center;
  gap: 0.55rem;
}

.consultant-attendances :deep(.insight-table td),
.consultant-attendances :deep(.insight-table th) {
  white-space: nowrap;
}

.consultant-attendances :deep(.insight-table td:nth-child(2)),
.consultant-attendances :deep(.insight-table td:nth-child(6)) {
  max-width: 14rem;
  overflow: hidden;
  text-overflow: ellipsis;
}

@media (max-width: 900px) {
  .consultant-attendances__filters {
    grid-template-columns: minmax(0, 1fr);
  }

  .consultant-attendances__footer {
    flex-direction: column;
    align-items: stretch;
  }

  .consultant-attendances__pagination {
    justify-content: flex-end;
  }
}

@media (max-width: 720px) {
  .consultant-attendances__pagination {
    justify-content: stretch;
  }

  .consultant-attendances__page-btn {
    flex: 1;
  }
}
</style>
