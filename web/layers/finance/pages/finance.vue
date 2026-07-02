<script setup lang="ts">
// Pagina /finance — port fiel de web-reference/app/pages/admin/finance.vue.
// O layout (UDashboardGroup + sidebar redimensionavel + painel) e os estilos sao
// reproduzidos identicos; a logica vive nos composables (config + sheet). O
// slideover de configuracao esta em FinanceConfigPanel. FONTE: mock BFF (badge
// MOCK so para admin — LegacyMarker).
import { useCoreAccountStore } from '../../core/stores/account'
import AdminPageHeader from '../../core/components/admin/AdminPageHeader.vue'
import LegacyMarker from '~/components/admin/LegacyMarker.vue'
import { useFinanceConfigEditor, FINANCE_CONFIG_KEY } from '../composables/useFinanceConfigEditor'
import { useFinanceSheetEditor } from '../composables/useFinanceSheetEditor'
import {
  STATUS_OPTIONS,
  formatMoney,
  formatSignedMoney,
  formatAdjustmentInputHint,
  lineTotal,
} from '../utils/finance-helpers'

definePageMeta({
  layout: 'dashboard',
  // workspaceId vazio DE PROPOSITO nesta fase mock: 'finance' ainda nao existe em
  // auth.allowedWorkspaces e o auth.global.ts barra workspaceId fora da lista
  // (redireciona pro /operacao). Vazio = rota nao-gated por workspace.
  workspaceId: '',
  pageLabel: 'Finance',
})

const coreAccount = useCoreAccountStore()
const config = useFinanceConfigEditor()
const editor = useFinanceSheetEditor({
  configDraft: config.configDraft,
  clientRecurringEntries: config.clientRecurringEntries,
})

provide(FINANCE_CONFIG_KEY, config)

const {
  draft,
  selectedSheetId,
  detailLoading,
  creating,
  deletingId,
  errorMessage,
  filteredSheets,
  queuePersist,
  addLine,
  onCreateSheet,
  onDeleteSheet,
  selectSheet,
  entradaDisplayItems,
  categoryOptions,
  resolveFixedById,
  isLineDetailsOpen,
  isEffectiveDateModalOpen,
  isAdjustmentModalOpen,
  isAdjustmentHistoryOpen,
  ensureAdjustmentDraft,
  onLineCardClick,
  onEffectiveToggle,
  setEffectiveDateModalOpen,
  onEffectiveDateChanged,
  onEffectiveDateSubmitShortcut,
  onEffectiveDateCancelShortcut,
  setEffectiveToday,
  clearEffectiveDate,
  closeEffectiveDateModal,
  onLineTotalInput,
  setAdjustmentModalOpen,
  onAdjustmentSubmitShortcut,
  onAdjustmentCancelShortcut,
  addLineAdjustment,
  closeAdjustmentModal,
  toggleLineDetails,
  removeRow,
  toggleAdjustmentHistory,
  setAdjustmentSign,
  setAdjustmentAbsoluteAmount,
  onAdjustmentHistoryChanged,
  removeLineAdjustment,
  onRecurringGroupEffectiveToggle,
  onRecurringGroupEffectiveDateChange,
  onRecurringStoreEffectiveToggle,
  onRecurringStoreEffectiveDateChange,
  entriesExpected,
  entriesEffective,
  exitsExpected,
  exitsEffective,
  balanceExpected,
  balanceEffective,
  activeSheetSaving,
} = editor

const { openConfigPanel, loading: configLoading } = config

// Config carregada -> re-hidrata rascunho e sincroniza linhas fixas/recorrentes.
watch(
  () => config.config.value,
  () => {
    config.syncConfigDraft()
    editor.syncAllFixedRows(false)
  },
  { deep: true },
)
watch(
  () => config.configDraft,
  () => editor.syncAllFixedRows(true),
  { deep: true },
)
watch(config.clientRecurringEntries, () => editor.syncAllFixedRows(true), { deep: true })
watch(
  () => coreAccount.activeAccountId,
  () => {
    void config.loadConfig()
  },
)

onMounted(async () => {
  await Promise.all([editor.fetchSheets(), config.loadConfig()])
})
</script>

<template>
  <section class="finances-page space-y-4">
    <AdminPageHeader
      eyebrow="Finance"
      title="Finance v2"
      description="Planilhas mensais com entradas e saidas."
    />

    <LegacyMarker
      kind="mock"
      label="Finance roda sobre BFF temporario em memoria (nao persiste no banco real)."
      detail="Back Go pendente — ver docs/finance/PLANO_MODULO_FINANCE.md e docs/LEGADO.md #6"
    />

    <UAlert
      v-if="errorMessage"
      class="finances-page__alert finances-page__alert--error"
      color="error"
      variant="soft"
      icon="i-lucide-alert-triangle"
      title="Erro"
      :description="errorMessage"
    />

    <UDashboardGroup class="finances-page__group !static !inset-auto !h-auto !w-full">
      <UDashboardSidebar
        class="finances-page__sidebar"
        resizable
        collapsible
        :min-size="12"
        :default-size="12"
        :max-size="20"
        :collapsed-size="12"
      >
        <template #header="{ collapsed }">
          <div class="finances-page__sidebar-header flex items-center justify-between gap-2">
            <h2
              class="finances-page__sidebar-title font-semibold text-[rgb(var(--text))]"
              :class="collapsed ? 'text-[11px]' : 'text-sm'"
            >
              Planilhas
            </h2>
            <div class="finances-page__sidebar-actions flex items-center gap-1">
              <UButton
                class="finances-page__sidebar-action finances-page__sidebar-action--create"
                icon="i-lucide-plus"
                size="sm"
                color="neutral"
                variant="soft"
                :square="collapsed"
                :loading="creating"
                @click="onCreateSheet"
              />
              <UDashboardSidebarCollapse
                class="finances-page__sidebar-action finances-page__sidebar-action--collapse"
                color="neutral"
                variant="ghost"
                size="sm"
              />
            </div>
          </div>
        </template>

        <div class="finances-page__sidebar-list space-y-2">
          <button
            v-for="sheet in filteredSheets"
            :key="sheet.id"
            type="button"
            class="finances-page__sheet-card w-full rounded-[var(--radius-sm)] border p-3 text-left"
            :class="
              sheet.id === selectedSheetId
                ? 'border-primary bg-[rgb(var(--surface))]'
                : 'border-[rgb(var(--border))] bg-transparent'
            "
            @click="selectSheet(sheet.id)"
          >
            <div
              class="finances-page__sheet-card-header mb-1 flex items-start justify-between gap-2"
            >
              <p
                class="finances-page__sheet-card-title line-clamp-1 text-sm font-semibold text-[rgb(var(--text))]"
              >
                {{ sheet.title || 'Sem titulo' }}
              </p>
              <UButton
                class="finances-page__sheet-card-delete"
                icon="i-lucide-trash-2"
                color="error"
                variant="ghost"
                size="xs"
                :loading="deletingId === sheet.id"
                @click.stop="onDeleteSheet(sheet.id)"
              />
            </div>
            <p class="finances-page__sheet-card-meta text-xs text-[rgb(var(--muted))]">
              {{ sheet.period }} | {{ sheet.clientName }}
            </p>
            <p class="finances-page__sheet-card-balance text-xs text-[rgb(var(--muted))]">
              Saldo: {{ formatMoney(sheet.summary.effectiveBalance) }}
            </p>
          </button>
        </div>
      </UDashboardSidebar>

      <UDashboardPanel class="finances-page__panel">
        <section
          class="rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-4"
        >
          <div v-if="!selectedSheetId" class="py-14 text-center text-sm text-[rgb(var(--muted))]">
            Selecione uma planilha ou crie uma nova.
          </div>

          <div v-else-if="detailLoading" class="py-14 text-center text-sm text-[rgb(var(--muted))]">
            Carregando planilha...
          </div>

          <template v-else>
            <div
              class="finances-page__editor-header mb-4 flex flex-wrap items-start justify-between gap-3"
            >
              <input
                v-model="draft.title"
                class="finances-page__title finances-page__title-input w-full max-w-[620px]"
                placeholder="Titulo da planilha"
                @input="queuePersist()"
              />
              <div class="finances-page__editor-actions flex items-center gap-2">
                <UInput
                  v-model="draft.period"
                  class="finances-page__period-input"
                  type="month"
                  @update:model-value="queuePersist()"
                />
                <OmniSelectMenuInput
                  v-model="draft.status"
                  class="finances-page__status-select"
                  :items="STATUS_OPTIONS"
                  item-display-mode="text"
                  :searchable="false"
                  :creatable="false"
                  :clear="false"
                  :badge-mode="true"
                  option-edit-mode="none"
                  color="neutral"
                  variant="none"
                  :highlight="false"
                  @update:model-value="queuePersist()"
                />
                <UButton
                  class="finances-page__editor-action finances-page__editor-action--config"
                  icon="i-lucide-settings-2"
                  color="neutral"
                  variant="soft"
                  :loading="configLoading"
                  @click="openConfigPanel"
                >
                  Configuracao
                </UButton>
                <UButton
                  class="finances-page__editor-action finances-page__editor-action--delete"
                  icon="i-lucide-trash-2"
                  color="error"
                  variant="soft"
                  :loading="deletingId === selectedSheetId"
                  @click="selectedSheetId && onDeleteSheet(selectedSheetId)"
                />
              </div>
            </div>

            <div class="finances-page__tables grid gap-3 lg:grid-cols-2">
              <UCard class="finances-page__table-card finances-page__table-card--entrada">
                <template #header>
                  <div class="finances-page__table-header flex items-center justify-between">
                    <h3 class="finances-page__table-title text-sm font-semibold">Entradas</h3>
                    <UButton
                      class="finances-page__table-add-line finances-page__table-add-line--entrada"
                      icon="i-lucide-plus"
                      size="xs"
                      label="Linha"
                      @click="addLine('entrada')"
                    />
                  </div>
                </template>
                <div class="space-y-2">
                  <template v-for="item in entradaDisplayItems" :key="item.key">
                    <FinanceRecurringGroupCard
                      v-if="item.kind === 'group'"
                      :title="item.group.title"
                      :category="item.group.category"
                      :base-amount="item.group.baseAmount"
                      :adjustment-amount="item.group.adjustmentAmount"
                      :total-amount="item.group.totalAmount"
                      :effective="item.group.effective"
                      :effective-date="item.group.effectiveDate"
                      :stores="
                        item.group.rows.map((row) => ({
                          key: row.key,
                          rowId: row.rowId,
                          name: row.name,
                          amount: row.amount,
                          effective: row.row.effective,
                          effectiveDate: row.row.effectiveDate,
                        }))
                      "
                      :format-money="formatMoney"
                      :format-signed-money="formatSignedMoney"
                      @group-effective-toggle="onRecurringGroupEffectiveToggle(item.group, $event)"
                      @group-effective-date-change="
                        onRecurringGroupEffectiveDateChange(item.group, $event)
                      "
                      @child-effective-toggle="
                        onRecurringStoreEffectiveToggle(item.group, $event.rowId, $event.next)
                      "
                      @child-effective-date-change="
                        onRecurringStoreEffectiveDateChange(item.group, $event.rowId, $event.value)
                      "
                    />

                    <FinanceLineCard
                      v-else
                      kind="entrada"
                      :row="item.row"
                      :index="item.index"
                      :category-options="categoryOptions('entrada')"
                      :details-open="isLineDetailsOpen('entrada', item.row, item.index)"
                      :effective-date-modal-open="
                        isEffectiveDateModalOpen('entrada', item.row, item.index)
                      "
                      :adjustment-modal-open="
                        isAdjustmentModalOpen('entrada', item.row, item.index)
                      "
                      :adjustment-history-open="
                        isAdjustmentHistoryOpen('entrada', item.row, item.index)
                      "
                      :adjustment-draft="ensureAdjustmentDraft('entrada', item.row, item.index)"
                      :adjustment-input-hint="formatAdjustmentInputHint()"
                      :line-total="lineTotal(item.row)"
                      :fixed-account="resolveFixedById(item.row.fixedAccountId)"
                      :format-money="formatMoney"
                      :format-signed-money="formatSignedMoney"
                      @line-card-click="onLineCardClick('entrada', item.row, item.index, $event)"
                      @persist="queuePersist()"
                      @effective-toggle="onEffectiveToggle('entrada', item.row, item.index, $event)"
                      @effective-date-open="
                        setEffectiveDateModalOpen('entrada', item.row, item.index, $event)
                      "
                      @effective-date-changed="onEffectiveDateChanged(item.row)"
                      @effective-date-submit-shortcut="
                        onEffectiveDateSubmitShortcut('entrada', item.row, item.index, $event)
                      "
                      @effective-date-cancel-shortcut="
                        onEffectiveDateCancelShortcut('entrada', item.row, item.index)
                      "
                      @effective-today="setEffectiveToday(item.row)"
                      @effective-clear="clearEffectiveDate(item.row)"
                      @effective-close="closeEffectiveDateModal('entrada', item.row, item.index)"
                      @line-total-input="onLineTotalInput(item.row, $event)"
                      @adjustment-open="
                        setAdjustmentModalOpen('entrada', item.row, item.index, $event)
                      "
                      @adjustment-submit-shortcut="
                        onAdjustmentSubmitShortcut('entrada', item.row, item.index, $event)
                      "
                      @adjustment-cancel-shortcut="
                        onAdjustmentCancelShortcut('entrada', item.row, item.index)
                      "
                      @adjustment-add="addLineAdjustment('entrada', item.row, item.index)"
                      @adjustment-close="closeAdjustmentModal('entrada', item.row, item.index)"
                      @toggle-details="toggleLineDetails('entrada', item.row, item.index)"
                      @remove-line="removeRow('entrada', item.index)"
                      @toggle-adjustment-history="
                        toggleAdjustmentHistory('entrada', item.row, item.index)
                      "
                      @set-adjustment-sign="
                        setAdjustmentSign(item.row, $event.adjustment, $event.sign)
                      "
                      @set-adjustment-absolute="
                        setAdjustmentAbsoluteAmount(item.row, $event.adjustment, $event.value)
                      "
                      @adjustment-history-changed="onAdjustmentHistoryChanged(item.row)"
                      @remove-adjustment="removeLineAdjustment(item.row, $event)"
                    />
                  </template>
                </div>
                <template #footer>
                  <div class="finances-page__table-footer text-xs text-[rgb(var(--muted))]">
                    Esperado: {{ formatMoney(entriesExpected) }} | Efetivado:
                    {{ formatMoney(entriesEffective) }}
                  </div>
                </template>
              </UCard>

              <UCard class="finances-page__table-card finances-page__table-card--saida">
                <template #header>
                  <div class="finances-page__table-header flex items-center justify-between">
                    <h3 class="finances-page__table-title text-sm font-semibold">Saidas</h3>
                    <UButton
                      class="finances-page__table-add-line finances-page__table-add-line--saida"
                      icon="i-lucide-plus"
                      size="xs"
                      label="Linha"
                      @click="addLine('saida')"
                    />
                  </div>
                </template>
                <div class="space-y-2">
                  <FinanceLineCard
                    v-for="(row, index) in draft.saidas"
                    :key="row.id"
                    kind="saida"
                    :row="row"
                    :index="index"
                    :category-options="categoryOptions('saida')"
                    :details-open="isLineDetailsOpen('saida', row, index)"
                    :effective-date-modal-open="isEffectiveDateModalOpen('saida', row, index)"
                    :adjustment-modal-open="isAdjustmentModalOpen('saida', row, index)"
                    :adjustment-history-open="isAdjustmentHistoryOpen('saida', row, index)"
                    :adjustment-draft="ensureAdjustmentDraft('saida', row, index)"
                    :adjustment-input-hint="formatAdjustmentInputHint()"
                    :line-total="lineTotal(row)"
                    :fixed-account="resolveFixedById(row.fixedAccountId)"
                    :format-money="formatMoney"
                    :format-signed-money="formatSignedMoney"
                    @line-card-click="onLineCardClick('saida', row, index, $event)"
                    @persist="queuePersist()"
                    @effective-toggle="onEffectiveToggle('saida', row, index, $event)"
                    @effective-date-open="setEffectiveDateModalOpen('saida', row, index, $event)"
                    @effective-date-changed="onEffectiveDateChanged(row)"
                    @effective-date-submit-shortcut="
                      onEffectiveDateSubmitShortcut('saida', row, index, $event)
                    "
                    @effective-date-cancel-shortcut="
                      onEffectiveDateCancelShortcut('saida', row, index)
                    "
                    @effective-today="setEffectiveToday(row)"
                    @effective-clear="clearEffectiveDate(row)"
                    @effective-close="closeEffectiveDateModal('saida', row, index)"
                    @line-total-input="onLineTotalInput(row, $event)"
                    @adjustment-open="setAdjustmentModalOpen('saida', row, index, $event)"
                    @adjustment-submit-shortcut="
                      onAdjustmentSubmitShortcut('saida', row, index, $event)
                    "
                    @adjustment-cancel-shortcut="onAdjustmentCancelShortcut('saida', row, index)"
                    @adjustment-add="addLineAdjustment('saida', row, index)"
                    @adjustment-close="closeAdjustmentModal('saida', row, index)"
                    @toggle-details="toggleLineDetails('saida', row, index)"
                    @remove-line="removeRow('saida', index)"
                    @toggle-adjustment-history="toggleAdjustmentHistory('saida', row, index)"
                    @set-adjustment-sign="setAdjustmentSign(row, $event.adjustment, $event.sign)"
                    @set-adjustment-absolute="
                      setAdjustmentAbsoluteAmount(row, $event.adjustment, $event.value)
                    "
                    @adjustment-history-changed="onAdjustmentHistoryChanged(row)"
                    @remove-adjustment="removeLineAdjustment(row, $event)"
                  />
                </div>
                <template #footer>
                  <div class="finances-page__table-footer text-xs text-[rgb(var(--muted))]">
                    Esperado: {{ formatMoney(exitsExpected) }} | Efetivado:
                    {{ formatMoney(exitsEffective) }}
                  </div>
                </template>
              </UCard>
            </div>

            <div
              class="finances-page__balance mt-4 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-4"
            >
              <p
                class="finances-page__balance-label text-xs uppercase tracking-wide text-[rgb(var(--muted))]"
              >
                Saldo
              </p>
              <p
                class="finances-page__balance-value text-2xl font-semibold"
                :class="
                  balanceEffective >= 0 ? 'text-[rgb(var(--success))]' : 'text-[rgb(var(--danger))]'
                "
              >
                {{ formatMoney(balanceEffective) }}
              </p>
              <p class="finances-page__balance-expected text-xs text-[rgb(var(--muted))]">
                Esperado: {{ formatMoney(balanceExpected) }}
              </p>
              <UBadge color="neutral" variant="soft" class="finances-page__balance-status mt-2">
                {{ activeSheetSaving ? 'Salvando...' : 'Salvo automaticamente' }}
              </UBadge>
            </div>
          </template>
        </section>
      </UDashboardPanel>
    </UDashboardGroup>

    <FinanceConfigPanel />
  </section>
</template>

<style scoped>
.finances-page :deep(.omni-select-menu-input) {
  display: flex;
}
.finances-page__group {
  display: flex;
  width: 100%;
  min-height: 680px;
  gap: 12px;
}

.finances-page__panel {
  min-width: 0;
  min-height: 50svh;
}

.finances-page__title {
  border: none;
  border-bottom: 1px solid rgb(var(--border));
  border-radius: 0;
  background: transparent;
  color: rgb(var(--text));
  font-size: 1.7rem;
  font-weight: 700;
  padding: 2px 0 8px;
  outline: none;
}

@media (max-width: 1024px) {
  .finances-page__group {
    min-height: 0;
    flex-direction: column;
  }
}
</style>
