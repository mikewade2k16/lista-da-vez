<script setup lang="ts">
import { ref, watch } from 'vue'
import { moneyDraft, moneyValue } from '~/domain/cardapio/money-input'

const props = withDefaults(
  defineProps<{
    modelValue: number
    wholeAmounts?: boolean
    disabled?: boolean
    placeholder?: string
    updateOnInput?: boolean
  }>(),
  { wholeAmounts: false, updateOnInput: false },
)

const emit = defineEmits<{ 'update:modelValue': [value: number] }>()
const draft = ref(moneyDraft(props.modelValue))
const focused = ref(false)
const dirty = ref(false)

watch(
  () => props.modelValue,
  (value) => {
    if (!focused.value) draft.value = moneyDraft(value)
  },
)

function onInput(event: Event) {
  draft.value = (event.target as HTMLInputElement).value
  dirty.value = true
  if (props.updateOnInput) emit('update:modelValue', moneyValue(draft.value, props.wholeAmounts))
}

function onBlur() {
  focused.value = false
  if (!dirty.value) return
  const value = moneyValue(draft.value, props.wholeAmounts)
  dirty.value = false
  draft.value = moneyDraft(value)
  emit('update:modelValue', value)
}
</script>

<template>
  <div class="omni-decimal-money" :class="{ 'omni-decimal-money--disabled': disabled }">
    <span class="omni-decimal-money__prefix">R$</span>
    <input
      :value="draft"
      type="text"
      :inputmode="wholeAmounts ? 'numeric' : 'decimal'"
      :placeholder="placeholder || (wholeAmounts ? '0' : '0,00')"
      :disabled="disabled"
      aria-label="Valor em reais"
      class="omni-decimal-money__input"
      @input="onInput"
      @focus="focused = true"
      @blur="onBlur"
      @keydown.enter="($event.target as HTMLInputElement).blur()"
    />
  </div>
</template>

<style scoped>
.omni-decimal-money {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0 0.6rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
}
.omni-decimal-money:focus-within {
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}
.omni-decimal-money--disabled {
  opacity: 0.55;
}
.omni-decimal-money__prefix {
  color: var(--text-muted);
  font-size: 0.86rem;
  font-weight: 600;
}
.omni-decimal-money__input {
  width: 100%;
  min-width: 0;
  padding: 0.55rem 0;
  border: none;
  background: transparent;
  color: var(--text-main);
  font-size: 0.92rem;
}
.omni-decimal-money__input:focus {
  outline: none;
}
</style>
