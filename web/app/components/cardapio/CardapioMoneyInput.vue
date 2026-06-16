<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import { formatCents, parseCents } from '~/domain/cardapio/types'

// Input de moeda: o modelo (v-model) guarda CENTAVOS inteiros; o campo exibe
// "1.234,56". Reutilizado por todas as secoes (frete, preco, variacao, etc.).
const props = defineProps<{
  modelValue: number
  placeholder?: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: number): void
}>()

const draft = ref(formatCents(props.modelValue))
const focused = ref(false)

const placeholderText = computed(() => props.placeholder || '0,00')

// Quando o valor externo muda (e o campo nao esta em edicao), reflete no campo.
watch(
  () => props.modelValue,
  (value) => {
    if (!focused.value) {
      draft.value = formatCents(value)
    }
  },
)

function onInput(event: Event) {
  draft.value = (event.target as HTMLInputElement).value
  emit('update:modelValue', parseCents(draft.value))
}

function onFocus() {
  focused.value = true
}

function onBlur() {
  focused.value = false
  draft.value = formatCents(props.modelValue)
}
</script>

<template>
  <div class="cardapio-money" :class="{ 'cardapio-money--disabled': disabled }">
    <span class="cardapio-money__prefix">R$</span>
    <input
      :value="draft"
      type="text"
      inputmode="decimal"
      class="cardapio-money__input"
      :placeholder="placeholderText"
      :disabled="disabled"
      @input="onInput"
      @focus="onFocus"
      @blur="onBlur"
    />
  </div>
</template>

<style scoped>
.cardapio-money {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0 0.6rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
}

.cardapio-money:focus-within {
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-money--disabled {
  opacity: 0.55;
}

.cardapio-money__prefix {
  color: var(--text-muted);
  font-size: 0.86rem;
  font-weight: 600;
}

.cardapio-money__input {
  flex: 1;
  min-width: 0;
  padding: 0.55rem 0;
  border: none;
  background: transparent;
  color: var(--text-main);
  font-size: 0.92rem;
}

.cardapio-money__input:focus {
  outline: none;
}
</style>
