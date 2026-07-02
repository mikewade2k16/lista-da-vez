<script setup lang="ts">
// Portado de web-reference/app/components/admin/finance/FinanceRecurringGroupCard.vue.
// OmniMoneyInput/OmniMinimalPopover/OmniSwitchInput sao auto-importados entre layers.

interface FinanceRecurringGroupStoreDisplay {
  key: string
  rowId: string
  name: string
  amount: number
  effective: boolean
  effectiveDate: string
}

const props = defineProps<{
  title: string
  category: string
  baseAmount: number
  adjustmentAmount: number
  totalAmount: number
  effective: boolean
  effectiveDate: string
  stores: FinanceRecurringGroupStoreDisplay[]
  formatMoney: (value: unknown) => string
  formatSignedMoney: (value: unknown) => string
}>()

const emit = defineEmits<{
  'group-effective-toggle': [next: boolean]
  'group-effective-date-change': [value: string]
  'child-effective-toggle': [payload: { rowId: string; next: boolean }]
  'child-effective-date-change': [payload: { rowId: string; value: string }]
}>()

const detailsOpen = ref(false)
const effectiveDatePopoverOpen = ref(false)
const lineInteractiveSelector = [
  'button',
  'input',
  'textarea',
  'select',
  'a',
  '[role="button"]',
  '[role="switch"]',
].join(', ')

const adjustmentColor = computed(() => {
  if (props.adjustmentAmount === 0) return 'neutral'
  return props.adjustmentAmount > 0 ? 'success' : 'error'
})

function isInteractiveTarget(target: EventTarget | null) {
  if (!(target instanceof Element)) return false
  return Boolean(target.closest(lineInteractiveSelector))
}

function onCardClick(event: MouseEvent) {
  if (isInteractiveTarget(event.target)) return
  detailsOpen.value = !detailsOpen.value
}

function onParentDateInput(value: unknown) {
  emit('group-effective-date-change', String(value ?? ''))
}

function onChildDateInput(rowId: string, value: unknown) {
  emit('child-effective-date-change', {
    rowId,
    value: String(value ?? ''),
  })
}
</script>

<template>
  <div class="finances-recurring-group" @click="onCardClick">
    <div class="grid gap-2 md:grid-cols-6">
      <div class="md:col-span-2 space-y-2">
        <UInput :model-value="title" disabled class="finances-recurring-group__title" />
        <div class="flex items-center gap-2 text-xs text-[rgb(var(--muted))]">
          <UBadge color="success" variant="soft" size="xs">Por loja</UBadge>
          <span>{{ category || 'Sem categoria' }}</span>
        </div>
      </div>

      <div class="flex items-center md:justify-start">
        <UBadge color="neutral" variant="soft">{{ category || 'Receita mensalidade' }}</UBadge>
      </div>

      <div class="flex items-center gap-2" @click.stop>
        <OmniSwitchInput
          :model-value="effective"
          aria-label="Efetivacao agregada"
          @update:model-value="emit('group-effective-toggle', Boolean($event))"
        />

        <OmniMinimalPopover
          v-if="effective"
          :open="effectiveDatePopoverOpen"
          title="Data geral de efetivacao"
          width-class="w-[320px] max-w-[92vw]"
          focus-selector="input[type=date]"
          @update:open="effectiveDatePopoverOpen = $event"
        >
          <template #trigger>
            <UButton icon="i-lucide-calendar-days" color="neutral" variant="ghost" size="xs" />
          </template>

          <div class="flex flex-wrap items-center gap-2">
            <UInput
              :model-value="effectiveDate"
              type="date"
              class="flex-1 min-w-[180px]"
              @update:model-value="onParentDateInput"
            />
            <UButton
              size="xs"
              color="neutral"
              variant="ghost"
              @click="emit('group-effective-date-change', '')"
            >
              Limpar
            </UButton>
          </div>
        </OmniMinimalPopover>
      </div>

      <div class="flex items-center" @click.stop>
        <OmniMoneyInput
          :model-value="totalAmount"
          disabled
          class="finances-recurring-group__amount"
        />
      </div>

      <div class="flex items-center justify-end gap-2" @click.stop>
        <UButton
          :icon="detailsOpen ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'"
          color="neutral"
          variant="ghost"
          size="xs"
          @click="detailsOpen = !detailsOpen"
        />
      </div>
    </div>

    <div v-if="detailsOpen" class="finances-recurring-group__details">
      <div class="finances-recurring-group__values">
        <div class="finances-recurring-group__value-card">
          <p class="finances-recurring-group__value-label">Valor original</p>
          <UBadge color="neutral" variant="soft">{{ formatMoney(baseAmount) }}</UBadge>
        </div>
        <div class="finances-recurring-group__value-card">
          <p class="finances-recurring-group__value-label">Ajustado (+/-)</p>
          <UBadge :color="adjustmentColor" variant="soft">
            {{ formatSignedMoney(adjustmentAmount) }}
          </UBadge>
        </div>
        <div class="finances-recurring-group__value-card">
          <p class="finances-recurring-group__value-label">Total da linha</p>
          <UBadge color="primary" variant="soft">{{ formatMoney(totalAmount) }}</UBadge>
        </div>
      </div>

      <div class="finances-recurring-group__stores">
        <div class="finances-recurring-group__stores-header">
          <p class="finances-recurring-group__value-label">Pagamentos individuais</p>
          <span class="text-xs text-[rgb(var(--muted))]">{{ stores.length }} lojas</span>
        </div>

        <div class="space-y-2">
          <div
            v-for="store in stores"
            :key="store.key"
            class="finances-recurring-group__store-row"
            @click.stop
          >
            <div class="min-w-0">
              <p class="truncate text-sm font-medium text-[rgb(var(--text))]">{{ store.name }}</p>
              <p class="text-xs text-[rgb(var(--muted))]">{{ formatMoney(store.amount) }}</p>
            </div>

            <OmniSwitchInput
              :model-value="store.effective"
              aria-label="Efetivacao da loja"
              @update:model-value="
                emit('child-effective-toggle', { rowId: store.rowId, next: Boolean($event) })
              "
            />

            <UInput
              :model-value="store.effectiveDate"
              type="date"
              class="finances-recurring-group__store-date"
              :disabled="!store.effective"
              @update:model-value="onChildDateInput(store.rowId, $event)"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.finances-recurring-group {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  padding: 10px;
  cursor: pointer;
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease;
}

.finances-recurring-group:hover {
  border-color: rgb(var(--primary));
  box-shadow: inset 0 0 0 1px rgba(var(--primary), 0.24);
}

.finances-recurring-group__details {
  margin-top: 8px;
  display: grid;
  gap: 10px;
  border-top: 1px dashed rgb(var(--border));
  padding-top: 8px;
}

.finances-recurring-group__values {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.finances-recurring-group__value-card {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2));
  padding: 8px;
  display: grid;
  gap: 6px;
}

.finances-recurring-group__value-label {
  font-size: 11px;
  letter-spacing: 0.03em;
  text-transform: uppercase;
  color: rgb(var(--muted));
  font-weight: 600;
}

.finances-recurring-group__stores {
  border: 1px dashed rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2));
  padding: 10px;
  display: grid;
  gap: 8px;
}

.finances-recurring-group__stores-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.finances-recurring-group__store-row {
  display: grid;
  gap: 8px;
  grid-template-columns: minmax(0, 1fr) auto 170px;
  align-items: center;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  padding: 8px;
}

.finances-recurring-group__store-date {
  min-width: 0;
}

@media (max-width: 960px) {
  .finances-recurring-group__values {
    grid-template-columns: 1fr;
  }

  .finances-recurring-group__store-row {
    grid-template-columns: 1fr;
  }
}
</style>
