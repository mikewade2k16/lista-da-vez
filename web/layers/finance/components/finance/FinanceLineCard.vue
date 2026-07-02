<script setup lang="ts">
// Portado de web-reference/app/components/admin/finance/FinanceLineCard.vue.
// Componentes Omni (OmniSelectMenuInput/OmniMoneyInput/OmniMinimalPopover/
// OmniSwitchInput) sao auto-importados entre layers pelo Nuxt — sem import explicito.
import type {
  FinanceFixedAccountConfig,
  FinanceLineAdjustment,
  FinanceLineItem,
} from '../../types/finances'

type FinanceLineKind = 'entrada' | 'saida'

interface FinanceLineCardAdjustmentDraft {
  amountInput: string
  note: string
  date: string
}

interface FinanceLineCardCategoryOption {
  label: string
  value: string
}

const props = defineProps<{
  kind: FinanceLineKind
  row: FinanceLineItem
  index: number
  categoryOptions: FinanceLineCardCategoryOption[]
  detailsOpen: boolean
  effectiveDateModalOpen: boolean
  adjustmentModalOpen: boolean
  adjustmentHistoryOpen: boolean
  adjustmentDraft: FinanceLineCardAdjustmentDraft
  adjustmentInputHint: string
  lineTotal: number
  fixedAccount: FinanceFixedAccountConfig | null
  formatMoney: (value: unknown) => string
  formatSignedMoney: (value: unknown) => string
}>()

// row/adjustmentDraft sao objetos reativos vindos do pai (draft.entradas/saidas e
// o rascunho de ajuste). O pai depende da mutacao POR REFERENCIA (edita o campo e
// o handler @persist le o mesmo objeto ja atualizado). toRef expoe refs gravaveis
// para os v-model sem mutar a prop pelo nome (vue/no-mutating-props).
const rowModel = toRef(props, 'row')
const draftModel = toRef(props, 'adjustmentDraft')

const emit = defineEmits<{
  'line-card-click': [event: MouseEvent]
  persist: []
  'effective-toggle': [next: boolean]
  'effective-date-open': [open: boolean]
  'effective-date-changed': []
  'effective-date-submit-shortcut': [event: KeyboardEvent]
  'effective-date-cancel-shortcut': []
  'effective-today': []
  'effective-clear': []
  'effective-close': []
  'line-total-input': [value: unknown]
  'adjustment-open': [open: boolean]
  'adjustment-submit-shortcut': [event: KeyboardEvent]
  'adjustment-cancel-shortcut': []
  'adjustment-add': []
  'adjustment-close': []
  'toggle-details': []
  'remove-line': []
  'toggle-adjustment-history': []
  'set-adjustment-sign': [payload: { adjustment: FinanceLineAdjustment; sign: '+' | '-' }]
  'set-adjustment-absolute': [payload: { adjustment: FinanceLineAdjustment; value: number }]
  'adjustment-history-changed': []
  'remove-adjustment': [adjustmentId: string]
}>()

const adjustmentSignOptions = [
  { label: '+', value: '+' },
  { label: '-', value: '-' },
]

const fixedBadgeColor = computed(() => (props.kind === 'entrada' ? 'info' : 'warning'))

const adjustmentColor = computed(() => {
  if (props.row.adjustmentAmount === 0) return 'neutral'
  return props.row.adjustmentAmount > 0 ? 'success' : 'error'
})

function adjustmentSignValue(adjustment: FinanceLineAdjustment) {
  return Number(adjustment.amount || 0) < 0 ? '-' : '+'
}
</script>

<template>
  <div class="finances-page__line-card" @click="emit('line-card-click', $event)">
    <div class="grid gap-2 md:grid-cols-6">
      <div class="finances-page__line-col finances-page__line-col--description md:col-span-2">
        <UInput
          v-model="rowModel.description"
          class="finances-page__line-description-input"
          placeholder="Descricao"
          :disabled="Boolean(row.fixedAccountId)"
          @update:model-value="emit('persist')"
        />
        <div v-if="row.fixedAccountId" class="finances-page__line-fixed-meta">
          <UBadge
            class="finances-page__line-fixed-badge"
            :color="fixedBadgeColor"
            variant="soft"
            size="xs"
          >
            Fixo
          </UBadge>
          <span class="text-xs text-[rgb(var(--muted))]">
            {{ row.category || 'Sem categoria' }}
          </span>
        </div>
      </div>

      <OmniSelectMenuInput
        v-model="rowModel.category"
        class="finances-page__line-category-input"
        :items="categoryOptions"
        placeholder="Categoria"
        searchable
        creatable
        clear
        :disabled="Boolean(row.fixedAccountId)"
        item-display-mode="text"
        :badge-mode="true"
        option-edit-mode="color"
        color="neutral"
        variant="none"
        :highlight="false"
        @update:model-value="emit('persist')"
      />

      <div
        class="finances-page__line-col finances-page__line-col--effective finances-page__effective-cell"
        @click.stop
      >
        <OmniSwitchInput
          v-model="rowModel.effective"
          class="finances-page__line-effective-switch"
          aria-label="Efetivado"
          @update:model-value="emit('effective-toggle', Boolean($event))"
        />
        <OmniMinimalPopover
          v-if="row.effective"
          :open="effectiveDateModalOpen"
          title="Data de efetivacao"
          width-class="w-[320px] max-w-[92vw]"
          focus-selector="input[type=date]"
          :close-on-submit-shortcut="false"
          @update:open="emit('effective-date-open', $event)"
          @submit-shortcut="emit('effective-date-submit-shortcut', $event)"
          @cancel-shortcut="emit('effective-date-cancel-shortcut')"
        >
          <template #trigger>
            <UButton
              icon="i-lucide-calendar-days"
              color="neutral"
              variant="ghost"
              size="xs"
              class="finances-page__popover-trigger finances-page__effective-date-trigger"
            />
          </template>

          <div class="finances-page__inline-modal-row">
            <UInput
              v-model="rowModel.effectiveDate"
              type="date"
              class="finances-page__effective-date-input"
              @update:model-value="emit('effective-date-changed')"
            />
            <UButton
              size="xs"
              color="neutral"
              variant="ghost"
              class="finances-page__effective-date-action finances-page__effective-date-action--today"
              @click="emit('effective-today')"
            >
              Hoje
            </UButton>
            <UButton
              size="xs"
              color="neutral"
              variant="ghost"
              class="finances-page__effective-date-action finances-page__effective-date-action--clear"
              @click="emit('effective-clear')"
            >
              Limpar
            </UButton>
            <UButton
              size="xs"
              color="neutral"
              variant="ghost"
              class="finances-page__effective-date-action finances-page__effective-date-action--close"
              @click="emit('effective-close')"
            >
              Fechar
            </UButton>
          </div>
        </OmniMinimalPopover>
      </div>

      <OmniMoneyInput
        :model-value="lineTotal"
        class="finances-page__line-total-input"
        :disabled="Boolean(row.fixedAccountId)"
        @update:model-value="emit('line-total-input', $event)"
      />

      <div
        class="finances-page__line-col finances-page__line-col--actions finances-page__line-actions"
      >
        <OmniMinimalPopover
          :open="adjustmentModalOpen"
          title="Adicionar ajuste (+/-)"
          width-class="w-[390px] max-w-[96vw]"
          focus-selector="input"
          :close-on-submit-shortcut="false"
          @update:open="emit('adjustment-open', $event)"
          @submit-shortcut="emit('adjustment-submit-shortcut', $event)"
          @cancel-shortcut="emit('adjustment-cancel-shortcut')"
        >
          <template #trigger>
            <UButton
              icon="i-lucide-plus"
              color="neutral"
              variant="ghost"
              size="xs"
              class="finances-page__popover-trigger finances-page__adjustment-trigger"
            />
          </template>

          <div class="grid gap-2 md:grid-cols-[minmax(0,1fr)_150px_minmax(0,1fr)]">
            <UInput
              v-model="draftModel.amountInput"
              class="finances-page__adjustment-input finances-page__adjustment-input--amount"
              :placeholder="adjustmentInputHint"
            />
            <UInput
              v-model="draftModel.date"
              type="date"
              class="finances-page__adjustment-input finances-page__adjustment-input--date"
            />
            <UInput
              v-model="draftModel.note"
              class="finances-page__adjustment-input finances-page__adjustment-input--note"
              placeholder="Observacao do ajuste"
            />
          </div>

          <template #footer>
            <UButton
              size="xs"
              color="neutral"
              variant="soft"
              class="finances-page__adjustment-footer-action finances-page__adjustment-footer-action--add"
              @click="emit('adjustment-add')"
            >
              Adicionar
            </UButton>
            <UButton
              size="xs"
              color="neutral"
              variant="ghost"
              class="finances-page__adjustment-footer-action finances-page__adjustment-footer-action--close"
              @click="emit('adjustment-close')"
            >
              Fechar
            </UButton>
          </template>
        </OmniMinimalPopover>

        <UButton
          :icon="detailsOpen ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'"
          color="neutral"
          variant="ghost"
          size="xs"
          class="finances-page__line-action-button finances-page__line-action-button--details"
          @click="emit('toggle-details')"
        />
        <UButton
          icon="i-lucide-x"
          color="error"
          variant="ghost"
          size="xs"
          class="finances-page__line-action-button finances-page__line-action-button--remove"
          @click="emit('remove-line')"
        />
      </div>
    </div>

    <div v-if="detailsOpen" class="finances-page__line-details">
      <div class="finances-page__line-values">
        <div class="finances-page__line-value-card">
          <p class="finances-page__line-value-label">Valor original</p>
          <UBadge class="finances-page__line-value-badge" color="neutral" variant="soft">
            {{ formatMoney(row.amount) }}
          </UBadge>
        </div>
        <div class="finances-page__line-value-card">
          <p class="finances-page__line-value-label">Ajustado (+/-)</p>
          <UBadge class="finances-page__line-value-badge" :color="adjustmentColor" variant="soft">
            {{ formatSignedMoney(row.adjustmentAmount) }}
          </UBadge>
        </div>
        <div class="finances-page__line-value-card">
          <p class="finances-page__line-value-label">Total da linha</p>
          <UBadge class="finances-page__line-value-badge" color="primary" variant="soft">
            {{ formatMoney(lineTotal) }}
          </UBadge>
        </div>
      </div>

      <div v-if="row.fixedAccountId && fixedAccount" class="finances-page__line-fixed-details">
        <p class="text-xs font-semibold text-[rgb(var(--muted))]">
          Conta fixa vinculada: {{ fixedAccount.name }}
        </p>
        <div v-if="(fixedAccount.members || []).length > 0" class="space-y-1">
          <p class="text-[11px] uppercase tracking-wide text-[rgb(var(--muted))]">Composicao</p>
          <div
            v-for="member in fixedAccount.members || []"
            :key="member.id"
            class="finances-page__line-fixed-member"
          >
            <span class="truncate text-xs">{{ member.name }}</span>
            <span class="text-xs font-medium">{{ formatMoney(member.amount) }}</span>
          </div>
        </div>
      </div>

      <div class="finances-page__line-adjustments">
        <button
          type="button"
          class="finances-page__line-adjustments-toggle"
          @click="emit('toggle-adjustment-history')"
        >
          <span class="finances-page__line-adjustments-title">Historico de ajustes</span>
          <span class="finances-page__line-adjustments-meta">
            {{ (row.adjustments || []).length }} itens
            <UIcon
              :name="adjustmentHistoryOpen ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'"
              class="size-4"
            />
          </span>
        </button>

        <div v-if="adjustmentHistoryOpen" class="finances-page__line-adjustments-content">
          <p v-if="(row.adjustments || []).length === 0" class="text-xs text-[rgb(var(--muted))]">
            Sem ajustes lancados.
          </p>
          <div
            v-for="adjustment in row.adjustments || []"
            :key="adjustment.id"
            class="finances-page__line-adjustment-item"
          >
            <OmniSelectMenuInput
              :model-value="adjustmentSignValue(adjustment)"
              class="finances-page__history-input finances-page__history-input--sign"
              :items="adjustmentSignOptions"
              item-display-mode="text"
              :searchable="false"
              :creatable="false"
              :clear="false"
              :badge-mode="true"
              option-edit-mode="none"
              color="neutral"
              variant="none"
              :highlight="false"
              @update:model-value="
                emit('set-adjustment-sign', { adjustment, sign: $event === '-' ? '-' : '+' })
              "
            />
            <OmniMoneyInput
              :model-value="Math.abs(Number(adjustment.amount || 0))"
              class="finances-page__history-input finances-page__history-input--amount"
              @update:model-value="
                emit('set-adjustment-absolute', { adjustment, value: Number($event || 0) })
              "
            />
            <UInput
              v-model="adjustment.date"
              type="date"
              class="finances-page__history-input finances-page__history-input--date"
              @update:model-value="emit('adjustment-history-changed')"
            />
            <UInput
              v-model="adjustment.note"
              class="finances-page__history-input finances-page__history-input--note"
              placeholder="Observacao"
              @update:model-value="emit('adjustment-history-changed')"
            />
            <UButton
              icon="i-lucide-trash-2"
              color="error"
              variant="ghost"
              size="xs"
              class="finances-page__history-input finances-page__history-input--remove"
              @click="emit('remove-adjustment', adjustment.id)"
            />
          </div>
        </div>
      </div>

      <UTextarea
        v-model="rowModel.details"
        :rows="2"
        placeholder="Detalhes da categoria/conta fixa deste mes..."
        @update:model-value="emit('persist')"
      />
    </div>
  </div>
</template>

<style scoped>
.finances-page__line-card {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  padding: 10px;
  cursor: pointer;
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease;
}

.finances-page__line-card:hover {
  border-color: rgb(var(--primary));
  box-shadow: inset 0 0 0 1px rgba(var(--primary), 0.24);
}

.finances-page__effective-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finances-page__line-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 4px;
}

.finances-page__popover-trigger {
  flex-shrink: 0;
}

.finances-page__inline-modal-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.finances-page__line-details {
  margin-top: 8px;
  display: grid;
  gap: 8px;
  border-top: 1px dashed rgb(var(--border));
  padding-top: 8px;
}

.finances-page__line-values {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.finances-page__line-value-card {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2));
  padding: 8px;
  display: grid;
  gap: 6px;
}

.finances-page__line-value-label {
  font-size: 11px;
  letter-spacing: 0.03em;
  text-transform: uppercase;
  color: rgb(var(--muted));
  font-weight: 600;
}

.finances-page__line-value-badge {
  width: fit-content;
}

.finances-page__line-fixed-meta {
  margin-top: 6px;
  display: flex;
  align-items: center;
  gap: 6px;
}

.finances-page__line-fixed-details {
  border: 1px dashed rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2));
  padding: 8px;
  display: grid;
  gap: 6px;
}

.finances-page__line-fixed-member {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  padding: 6px 8px;
}

.finances-page__line-adjustments {
  border: 1px dashed rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2));
  padding: 8px;
  display: grid;
  gap: 8px;
}

.finances-page__line-adjustments-title {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.03em;
  text-transform: uppercase;
  color: rgb(var(--muted));
}

.finances-page__line-adjustments-toggle {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.finances-page__line-adjustments-meta {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: rgb(var(--muted));
}

.finances-page__line-adjustments-content {
  display: grid;
  gap: 8px;
}

.finances-page__line-adjustment-item {
  display: grid;
  gap: 6px;
  grid-template-columns: 72px minmax(120px, 150px) minmax(130px, 150px) minmax(0, 1fr) auto;
  align-items: center;
}

@media (max-width: 1024px) {
  .finances-page__line-values {
    grid-template-columns: 1fr;
  }

  .finances-page__line-adjustment-item {
    grid-template-columns: 1fr;
  }
}
</style>
