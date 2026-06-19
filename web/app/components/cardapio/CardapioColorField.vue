<script setup lang="ts">
import { computed } from 'vue'

// Campo de cor SEMANTICA do tema (WS-D): <input type="color"> + campo hex editavel,
// sincronizados (mexer em um reflete no outro). O valor e DADO do usuario (hex
// livre), nao token do painel. O swatch usa o proprio valor como fundo. Aceita hex
// parcial digitado mas so propaga quando vira um hex valido (#rgb / #rrggbb).

const props = defineProps<{
  modelValue: string
  label: string
  hint?: string
}>()

const emit = defineEmits<{ (e: 'update:modelValue', value: string): void }>()

const HEX_RE = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/

// O <input type=color> so aceita #rrggbb; se o valor atual nao for valido, mostra
// preto no seletor sem sobrescrever o texto que o usuario digita.
const colorValue = computed(() => (HEX_RE.test(props.modelValue) ? props.modelValue : '#000000'))

function onColor(event: Event) {
  emit('update:modelValue', (event.target as HTMLInputElement).value)
}

function onHex(event: Event) {
  const next = (event.target as HTMLInputElement).value.trim()
  emit('update:modelValue', next)
}
</script>

<template>
  <label class="cardapio-color">
    <span class="cardapio-color__label">{{ label }}</span>
    <span v-if="hint" class="cardapio-color__hint">{{ hint }}</span>
    <span class="cardapio-color__row">
      <input
        type="color"
        class="cardapio-color__swatch"
        :value="colorValue"
        :aria-label="label"
        @input="onColor"
      />
      <input
        type="text"
        class="cardapio-color__hex"
        :value="modelValue"
        placeholder="#000000"
        spellcheck="false"
        @input="onHex"
      />
    </span>
  </label>
</template>

<style scoped>
.cardapio-color {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-width: 0;
}

.cardapio-color__label {
  font-size: 0.78rem;
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-color__hint {
  font-size: 0.7rem;
  color: var(--text-muted);
}

.cardapio-color__row {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  margin-top: 0.1rem;
}

.cardapio-color__swatch {
  width: 2.4rem;
  height: 2.3rem;
  padding: 0;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: transparent;
  cursor: pointer;
  flex-shrink: 0;
}

.cardapio-color__hex {
  width: 100%;
  min-width: 0;
  padding: 0.5rem 0.6rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.85rem;
  font-family: var(--font-mono, ui-monospace, monospace);
}

.cardapio-color__hex:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}
</style>
