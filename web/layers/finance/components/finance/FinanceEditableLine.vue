<script setup lang="ts">
import type { FinanceLineItem } from '../../types/finances'
import type { FinanceSheetEditor } from '../../composables/useFinanceSheetEditor'
import {
  formatMoney,
  formatSignedMoney,
  formatAdjustmentInputHint,
  lineTotal,
} from '../../utils/finance-helpers'
defineProps<{
  kind: 'entrada' | 'saida'
  row: FinanceLineItem
  index: number
  editor: FinanceSheetEditor
}>()
</script>
<template>
  <FinanceLineCard
    :kind="kind"
    :row="row"
    :index="index"
    :category-options="editor.categoryOptions(kind)"
    :details-open="editor.isLineDetailsOpen(kind, row, index)"
    :effective-date-modal-open="editor.isEffectiveDateModalOpen(kind, row, index)"
    :adjustment-modal-open="editor.isAdjustmentModalOpen(kind, row, index)"
    :adjustment-history-open="editor.isAdjustmentHistoryOpen(kind, row, index)"
    :adjustment-draft="editor.ensureAdjustmentDraft(kind, row, index)"
    :adjustment-input-hint="formatAdjustmentInputHint()"
    :line-total="lineTotal(row)"
    :fixed-account="editor.resolveFixedById(row.fixedAccountId)"
    :format-money="formatMoney"
    :format-signed-money="formatSignedMoney"
    @line-card-click="editor.onLineCardClick(kind, row, index, $event)"
    @persist="editor.queuePersist()"
    @effective-toggle="editor.onEffectiveToggle(kind, row, index, $event)"
    @effective-date-open="editor.setEffectiveDateModalOpen(kind, row, index, $event)"
    @effective-date-changed="editor.onEffectiveDateChanged(row)"
    @effective-date-submit-shortcut="editor.onEffectiveDateSubmitShortcut(kind, row, index, $event)"
    @effective-date-cancel-shortcut="editor.onEffectiveDateCancelShortcut(kind, row, index)"
    @effective-today="editor.setEffectiveToday(row)"
    @effective-clear="editor.clearEffectiveDate(row)"
    @effective-close="editor.closeEffectiveDateModal(kind, row, index)"
    @line-total-input="editor.onLineTotalInput(row, $event)"
    @adjustment-open="editor.setAdjustmentModalOpen(kind, row, index, $event)"
    @adjustment-submit-shortcut="editor.onAdjustmentSubmitShortcut(kind, row, index, $event)"
    @adjustment-cancel-shortcut="editor.onAdjustmentCancelShortcut(kind, row, index)"
    @adjustment-add="editor.addLineAdjustment(kind, row, index)"
    @adjustment-close="editor.closeAdjustmentModal(kind, row, index)"
    @toggle-details="editor.toggleLineDetails(kind, row, index)"
    @remove-line="editor.removeRow(kind, index)"
    @toggle-adjustment-history="editor.toggleAdjustmentHistory(kind, row, index)"
    @set-adjustment-sign="editor.setAdjustmentSign(row, $event.adjustment, $event.sign)"
    @set-adjustment-absolute="
      editor.setAdjustmentAbsoluteAmount(row, $event.adjustment, $event.value)
    "
    @adjustment-history-changed="editor.onAdjustmentHistoryChanged(row)"
    @remove-adjustment="editor.removeLineAdjustment(row, $event)"
  />
</template>
