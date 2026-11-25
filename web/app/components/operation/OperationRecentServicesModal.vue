<script setup lang="ts">
import { buildNickname } from '~/domain/utils/person-display'
import { formatCurrencyBRL } from '~/domain/utils/admin-metrics'
import { OUTCOME_LABELS, START_MODE_LABELS } from '~/domain/utils/reports'
import { formatDuration } from '~/domain/utils/time'
import { useRecentServices, type RecentServiceItem } from '~/composables/useRecentServices'

// Auditoria "ultimos atendimentos" (platform_admin): detalhe do atendimento
// selecionado (por padrao o mais recente) + lista paginada para navegar.
const props = defineProps<{ open: boolean; storeId: string }>()
const emit = defineEmits<{ close: [] }>()

const { items, total, page, perPage, loading, errorMessage, fetchPage } = useRecentServices(10)
const selectedId = ref('')

const selected = computed<RecentServiceItem | null>(
  () => items.value.find((item) => item.serviceId === selectedId.value) || items.value[0] || null,
)

const dateTimeFormatter = new Intl.DateTimeFormat('pt-BR', {
  dateStyle: 'short',
  timeStyle: 'short',
})

function formatDateTime(ms: number) {
  return ms > 0 ? dateTimeFormatter.format(new Date(ms)) : '-'
}

function personLabel(item: RecentServiceItem) {
  return buildNickname(item.personName || '') || 'Consultor'
}

const outcomeLabels: Record<string, string> = OUTCOME_LABELS
const startModeLabels: Record<string, string> = START_MODE_LABELS

function outcomeLabel(item: RecentServiceItem) {
  if (item.finishOutcome === 'auto') return 'Encerrado automaticamente'
  return outcomeLabels[item.finishOutcome] || item.finishOutcome || '-'
}

function statusLabel(item: RecentServiceItem) {
  if (item.validationStatus === 'pending') return 'Pendente de encerramento'
  if (item.validationStatus === 'cancelled') return 'Cancelado (fora da metrica)'
  return item.closeReason === 'auto' ? 'Encerrado pela gestao' : 'Encerrado pelo consultor'
}

const detailRows = computed(() => {
  const item = selected.value
  if (!item) return []
  return [
    { label: 'Funcionario', value: item.personName || '-' },
    { label: 'Loja', value: item.storeName || '-' },
    { label: 'Encerrado em', value: formatDateTime(item.finishedAt) },
    { label: 'Iniciado em', value: formatDateTime(item.startedAt) },
    { label: 'Duracao', value: formatDuration(Math.max(0, item.durationMs)) },
    { label: 'Desfecho', value: outcomeLabel(item) },
    { label: 'Inicio', value: startModeLabels[item.startMode] || item.startMode || '-' },
    { label: 'Espera na fila', value: formatDuration(Math.max(0, item.queueWaitMs)) },
    { label: 'Valor', value: item.saleAmount > 0 ? formatCurrencyBRL(item.saleAmount) : '-' },
    {
      label: 'Cliente',
      value: item.customerName
        ? `${item.customerName}${item.isExistingCustomer ? ' (ja cliente)' : ''}`
        : '-',
    },
    { label: 'Status', value: statusLabel(item) },
  ]
})

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / perPage.value)))

function loadPage(nextPage: number) {
  selectedId.value = ''
  void fetchPage(props.storeId, nextPage)
}

watch(
  () => [props.open, props.storeId] as const,
  ([open]) => {
    if (open) loadPage(1)
  },
  { immediate: true },
)
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="modal-backdrop"
      data-testid="operation-recent-services-backdrop"
      @click.self="emit('close')"
    >
      <div
        class="finish-modal finish-modal--compact recent-services"
        role="dialog"
        aria-modal="true"
        data-testid="operation-recent-services-modal"
      >
        <div class="finish-modal__header">
          <div>
            <h2 class="finish-modal__title">Ultimos atendimentos</h2>
            <p class="finish-modal__subtitle">
              Quem atendeu, em qual loja e quando. Visivel apenas para platform admin.
            </p>
          </div>
          <div class="finish-modal__header-actions">
            <button
              class="finish-modal__close"
              type="button"
              aria-label="Fechar"
              @click="emit('close')"
            >
              x
            </button>
          </div>
        </div>

        <p v-if="errorMessage" class="recent-services__error">{{ errorMessage }}</p>

        <dl v-if="selected" class="recent-services__detail" data-testid="recent-services-detail">
          <div v-for="row in detailRows" :key="row.label" class="recent-services__detail-row">
            <dt>{{ row.label }}</dt>
            <dd>{{ row.value }}</dd>
          </div>
        </dl>

        <div class="pending-list__body recent-services__list">
          <p v-if="loading && !items.length" class="pending-list__empty">Carregando...</p>
          <p v-else-if="!items.length && !errorMessage" class="pending-list__empty">
            Nenhum atendimento encerrado ainda.
          </p>
          <button
            v-for="item in items"
            :key="item.serviceId"
            type="button"
            class="pending-list__item recent-services__item"
            :class="{
              'recent-services__item--active': selected?.serviceId === item.serviceId,
            }"
            @click="selectedId = item.serviceId"
          >
            <span class="pending-list__info">
              <strong class="recent-services__name">
                {{ personLabel(item) }}
                <span v-if="item.storeName" class="pending-list__store">{{ item.storeName }}</span>
              </strong>
              <span class="pending-list__meta">
                {{ formatDateTime(item.finishedAt) }} · {{ outcomeLabel(item) }}
              </span>
            </span>
            <span class="pending-list__meta">
              {{ formatDuration(Math.max(0, item.durationMs)) }}
            </span>
          </button>
        </div>

        <div v-if="total > 0" class="recent-services__pagination">
          <span class="pending-list__meta">{{ total }} atendimentos</span>
          <UPagination
            v-if="totalPages > 1"
            :page="page"
            :total="total"
            :items-per-page="perPage"
            :sibling-count="1"
            show-edges
            size="sm"
            :disabled="loading"
            @update:page="loadPage"
          />
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.recent-services {
  display: grid;
  gap: 12px;
}

.recent-services__error {
  margin: 0;
  font-size: 0.8rem;
  color: rgb(var(--danger));
}

.recent-services__detail {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 8px 16px;
  margin: 0;
  padding: 12px;
  border-radius: 12px;
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.4);
}

.recent-services__detail-row dt {
  font-size: 0.68rem;
  font-weight: 700;
  text-transform: uppercase;
  color: rgb(var(--muted));
}

.recent-services__detail-row dd {
  margin: 2px 0 0;
  font-size: 0.86rem;
  font-weight: 600;
  color: var(--text-main);
}

.recent-services__list {
  max-height: min(40vh, 360px);
}

.recent-services__item {
  width: 100%;
  text-align: left;
  cursor: pointer;
}

.recent-services__item--active {
  border-color: rgb(var(--primary));
}

.recent-services__name {
  font-size: 0.88rem;
  font-weight: 800;
  color: var(--text-main);
}

.recent-services__pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
</style>
