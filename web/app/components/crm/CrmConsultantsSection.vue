<script setup lang="ts">
import { ref } from 'vue'

import { formatCurrencyBRL } from '~/domain/utils/admin-metrics'
import type { CRMConsultantMetric } from '~/stores/crm'
import type { ErpConsultantLinkOption } from '~/stores/erp'
import type { MergedCrmConsultant } from '~/composables/useCrmConsultantMetrics'

const props = defineProps<{
  mergedConsultants: MergedCrmConsultant[]
  managementConsultantRows: CRMConsultantMetric[]
  canManageConsultantLinks: boolean
  loadingConsultantLinks: boolean
  savingConsultantLink: boolean
  consultantLinkOptions: ErpConsultantLinkOption[]
  consultantLinkDrafts: Record<string, string>
  queueStatsAvailable: boolean
  consultantLinkKey: (storeCode: unknown, employeeId: unknown) => string
  linkStatusLabel: (status?: string | null) => string
  linkStatusClass: (status?: string | null) => string
}>()

const emit = defineEmits<{
  (event: 'auto-link' | 'refresh-links'): void
  (event: 'save-link' | 'remove-link', row: MergedCrmConsultant): void
  (event: 'update-draft', payload: { key: string; value: string }): void
}>()

const expandedRowKey = ref('')

function formatCurrencyFromCents(value?: number | null) {
  return formatCurrencyBRL((Number(value || 0) || 0) / 100)
}

function formatNumber(value?: number | null) {
  return Number(value || 0).toLocaleString('pt-BR')
}

function formatPA(value?: number | null) {
  return Number(value || 0).toFixed(2)
}

function formatPct(value?: number | null) {
  const n = Number(value || 0)
  return n ? `${n.toFixed(1)}%` : '-'
}

function tableRowKey(row: MergedCrmConsultant | CRMConsultantMetric) {
  return `${row.consultantId}-${row.storeSlug}-${row.storeCnpj || ''}`
}

function draftKeyForRow(row: MergedCrmConsultant) {
  return props.consultantLinkKey(row.storeCnpj, row.erpEmployeeId || row.consultantId)
}

function isExpanded(row: MergedCrmConsultant) {
  return expandedRowKey.value === tableRowKey(row)
}

function toggleEditor(row: MergedCrmConsultant) {
  const nextKey = tableRowKey(row)
  expandedRowKey.value = expandedRowKey.value === nextKey ? '' : nextKey
}

function updateDraft(row: MergedCrmConsultant, event: Event) {
  const target = event.target as HTMLSelectElement | null
  emit('update-draft', {
    key: draftKeyForRow(row),
    value: target?.value || '',
  })
}

function currentLinkedLabel(row: MergedCrmConsultant) {
  return (
    row.linkEmployee?.linkedConsultantName || row.profileConsultantName || 'Sem consultor vinculado'
  )
}

function queueStatusLabel(row: MergedCrmConsultant) {
  if (!props.queueStatsAvailable) return 'sem dados fila'
  if (row.queue) return 'identificado'
  if (String(row.profileConsultantId || row.profileConsultantName || '').trim()) {
    return 'sem atend. no periodo'
  }
  return 'nao identificado'
}

function queueStatusClass(row: MergedCrmConsultant) {
  if (!props.queueStatsAvailable) return 'crm-badge--neutral'
  if (row.queue) return 'crm-badge--ok'
  if (String(row.profileConsultantId || row.profileConsultantName || '').trim()) {
    return 'crm-badge--info'
  }
  return 'crm-badge--warn'
}

function handleSave(row: MergedCrmConsultant) {
  emit('save-link', row)
  expandedRowKey.value = ''
}

function handleRemove(row: MergedCrmConsultant) {
  emit('remove-link', row)
  expandedRowKey.value = ''
}
</script>

<template>
  <article class="insight-card insight-card--wide">
    <header class="crm-section__header">
      <div>
        <h3 class="insight-card__title">Indicadores por consultor</h3>
        <p class="insight-card__text">
          ERP + fila de atendimento. Colunas de fila marcadas com (F) cruzadas pelo vinculo, codigo
          ou nome.
        </p>
      </div>
      <div class="crm-section__side">
        <span class="crm-section__meta">{{ mergedConsultants.length }} consultor(es)</span>
        <div v-if="canManageConsultantLinks" class="crm-inline-link-toolbar">
          <button
            type="button"
            class="crm-btn crm-btn--ghost crm-btn--sm"
            :disabled="loadingConsultantLinks || savingConsultantLink"
            @click="emit('auto-link')"
          >
            Vincular automaticamente
          </button>
          <button
            type="button"
            class="crm-btn crm-btn--ghost crm-btn--sm"
            :disabled="loadingConsultantLinks || savingConsultantLink"
            @click="emit('refresh-links')"
          >
            Atualizar vinculos
          </button>
        </div>
      </div>
    </header>

    <div class="insight-table-wrap">
      <table class="insight-table crm-table crm-table--consultants">
        <thead>
          <tr>
            <th>Consultor</th>
            <th>Vinculo</th>
            <th>Loja</th>
            <th>Vendido</th>
            <th>Ticket medio</th>
            <th>P.A.</th>
            <th>Pedidos</th>
            <th>Atend. (F)</th>
            <th>Conversao (F)</th>
            <th>Canc. fila (F)</th>
            <th>Status fila</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in mergedConsultants"
            :key="tableRowKey(row)"
            :class="{ 'crm-tr--unmatched': !row.queue && queueStatsAvailable }"
          >
            <td>
              <div class="crm-row-heading">
                <strong>{{ row.consultantName }}</strong>
                <small class="crm-muted">ERP {{ row.erpEmployeeId || row.consultantId }}</small>
              </div>
            </td>
            <td>
              <div class="crm-link-cell">
                <div class="crm-link-summary">
                  <span class="crm-badge" :class="linkStatusClass(row.linkStatus)">
                    {{ linkStatusLabel(row.linkStatus) }}
                  </span>
                  <button
                    v-if="canManageConsultantLinks"
                    type="button"
                    class="crm-link-toggle"
                    :disabled="loadingConsultantLinks || savingConsultantLink"
                    @click="toggleEditor(row)"
                  >
                    {{ isExpanded(row) ? 'Fechar' : 'Editar' }}
                  </button>
                </div>

                <small class="crm-link-caption">{{ currentLinkedLabel(row) }}</small>

                <template v-if="canManageConsultantLinks && isExpanded(row)">
                  <select
                    class="crm-link-select"
                    :value="consultantLinkDrafts[draftKeyForRow(row)] || ''"
                    :disabled="loadingConsultantLinks || savingConsultantLink"
                    @change="updateDraft(row, $event)"
                  >
                    <option value="">Selecionar consultor</option>
                    <option
                      v-for="consultant in consultantLinkOptions"
                      :key="consultant.consultantId"
                      :value="consultant.consultantId"
                    >
                      {{ consultant.consultantName }} - {{ consultant.storeName }}
                    </option>
                  </select>

                  <div class="crm-link-actions">
                    <button
                      type="button"
                      class="crm-btn crm-btn--ghost crm-btn--xs"
                      :disabled="savingConsultantLink"
                      @click="handleSave(row)"
                    >
                      Salvar
                    </button>
                    <button
                      v-if="row.linkEmployee?.linkId"
                      type="button"
                      class="crm-btn crm-btn--ghost crm-btn--xs"
                      :disabled="savingConsultantLink"
                      @click="handleRemove(row)"
                    >
                      Remover
                    </button>
                  </div>
                </template>
              </div>
            </td>
            <td>{{ row.storeLabel }}</td>
            <td>{{ formatCurrencyFromCents(row.salesCents) }}</td>
            <td>{{ formatCurrencyFromCents(row.ticketAverageCents) }}</td>
            <td>{{ formatPA(row.paScore) }}</td>
            <td>{{ formatNumber(row.orders) }}</td>
            <td :class="{ 'crm-td--queue': row.queue }">
              {{ row.queue ? formatNumber(row.queue.attendances) : '-' }}
            </td>
            <td>
              <span v-if="row.queue" :class="{ 'crm-rate--good': row.queue.conversionRate >= 30 }">
                {{ formatPct(row.queue.conversionRate) }}
              </span>
              <span v-else class="crm-muted">-</span>
            </td>
            <td>
              <span
                v-if="row.queue"
                :class="{ 'crm-rate--bad': row.queue.queueCancellationRate > 10 }"
              >
                {{ formatPct(row.queue.queueCancellationRate) }}
              </span>
              <span v-else class="crm-muted">-</span>
            </td>
            <td>
              <span class="crm-badge" :class="queueStatusClass(row)">
                {{ queueStatusLabel(row) }}
              </span>
            </td>
          </tr>
          <tr v-if="!mergedConsultants.length">
            <td class="crm-empty" colspan="11">
              Nenhum consultor com pedidos ERP no periodo selecionado.
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </article>
</template>

<style scoped>
.crm-section__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
  margin-bottom: 1rem;
}

.crm-section__side {
  display: grid;
  justify-items: flex-end;
  gap: 0.6rem;
}

.crm-section__meta {
  color: rgb(var(--muted));
  font-size: 0.88rem;
}

.crm-inline-link-toolbar {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.55rem;
}

.crm-btn {
  min-height: 42px;
  border: none;
  border-radius: 12px;
  padding: 0.75rem 1rem;
  background: rgb(var(--primary));
  color: rgb(255 255 255);
  font-weight: 700;
  cursor: pointer;
  white-space: nowrap;
}

.crm-btn:disabled,
.crm-link-toggle:disabled {
  cursor: wait;
  opacity: 0.72;
}

.crm-btn--ghost {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.crm-btn--sm {
  min-height: 38px;
  padding: 0.45rem 0.75rem;
  font-size: 0.84rem;
  border-radius: 10px;
}

.crm-btn--xs {
  min-height: 34px;
  padding: 0.42rem 0.68rem;
  border-radius: 10px;
  font-size: 0.76rem;
}

.crm-table {
  min-width: 860px;
}

.crm-table--consultants {
  min-width: 1100px;
}

.crm-row-heading {
  display: grid;
  gap: 0.2rem;
}

.crm-row-heading small {
  color: rgb(var(--muted));
}

.crm-empty {
  padding: 1rem;
  text-align: center;
  color: rgb(var(--muted));
}

.crm-muted {
  color: rgb(var(--muted) / 0.72);
}

.crm-td--queue {
  color: rgb(var(--success));
  font-weight: 600;
}

.crm-tr--unmatched td {
  opacity: 0.82;
}

.crm-link-cell {
  display: grid;
  gap: 0.45rem;
  min-width: 240px;
}

.crm-link-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
}

.crm-link-toggle {
  padding: 0;
  border: none;
  background: none;
  color: rgb(var(--primary));
  font-size: 0.78rem;
  font-weight: 700;
  cursor: pointer;
}

.crm-link-caption {
  color: rgb(var(--muted));
  font-size: 0.74rem;
}

.crm-link-select {
  min-height: 34px;
  min-width: 220px;
  padding: 0 0.7rem;
  border-radius: 10px;
  border: 1px solid rgb(var(--border) / 0.88);
  background: rgb(var(--surface) / 0.95);
  color: rgb(var(--text));
  font-size: 0.82rem;
}

.crm-link-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.crm-badge {
  display: inline-block;
  padding: 0.2rem 0.55rem;
  border-radius: 6px;
  font-size: 0.72rem;
  font-weight: 700;
  white-space: nowrap;
}

.crm-badge--ok {
  background: rgb(var(--success) / 0.12);
  color: rgb(var(--success));
}

.crm-badge--warn {
  background: rgb(var(--primary) / 0.14);
  color: rgb(var(--primary));
}

.crm-badge--info {
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
}

.crm-badge--neutral {
  background: rgb(var(--border) / 0.56);
  color: rgb(var(--muted));
}

.crm-rate--good {
  color: rgb(var(--success));
  font-weight: 700;
}

.crm-rate--bad {
  color: rgb(var(--danger));
  font-weight: 700;
}

@media (max-width: 860px) {
  .crm-section__header {
    display: grid;
    grid-template-columns: 1fr;
  }

  .crm-section__side,
  .crm-inline-link-toolbar {
    justify-items: flex-start;
    justify-content: flex-start;
  }
}
</style>
