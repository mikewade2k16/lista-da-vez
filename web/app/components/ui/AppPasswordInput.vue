<script setup lang="ts">
import { ref } from 'vue'

// Campo de senha reutilizavel com toggle de visibilidade (olhinho). Antes o
// padrao estava duplicado inline no login e no esqueceu-senha (e o perfil/criar-
// usuario nem tinham). Centraliza o SVG + o comportamento aqui. Usa os tokens do
// design system; a classe do input e injetavel (inputClass) para herdar o estilo
// do contexto onde for usado (painel: product-add__input; auth: admin-auth-input).
// v-model via modelValue + update:modelValue (padrao do projeto; sem defineModel).
withDefaults(
  defineProps<{
    modelValue?: string
    placeholder?: string
    autocomplete?: string
    inputClass?: string
    disabled?: boolean
    required?: boolean
  }>(),
  {
    modelValue: '',
    placeholder: 'Senha',
    autocomplete: 'current-password',
    inputClass: 'product-add__input',
    disabled: false,
    required: false,
  },
)

const emit = defineEmits<{ (event: 'update:modelValue', value: string): void }>()
const visible = ref(false)

function onInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLInputElement).value)
}

function toggle() {
  visible.value = !visible.value
}
</script>

<template>
  <div class="app-password-input">
    <input
      :value="modelValue"
      :class="[inputClass, 'app-password-input__field']"
      :type="visible ? 'text' : 'password'"
      :placeholder="placeholder"
      :autocomplete="autocomplete"
      :disabled="disabled"
      :required="required"
      @input="onInput"
    />
    <button
      type="button"
      class="app-password-input__toggle"
      :aria-label="visible ? 'Ocultar senha' : 'Mostrar senha'"
      :title="visible ? 'Ocultar senha' : 'Mostrar senha'"
      tabindex="-1"
      @click="toggle"
    >
      <svg
        v-if="!visible"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
        <circle cx="12" cy="12" r="3" />
      </svg>
      <svg
        v-else
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94" />
        <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" />
        <line x1="1" y1="1" x2="23" y2="23" />
      </svg>
    </button>
  </div>
</template>

<style scoped>
.app-password-input {
  position: relative;
  width: 100%;
}

.app-password-input__field {
  /* espaco a direita para o botao nao cobrir o texto digitado */
  padding-right: 2.6rem;
}

.app-password-input__toggle {
  position: absolute;
  top: 50%;
  right: 0.55rem;
  transform: translateY(-50%);
  display: grid;
  place-items: center;
  width: 1.9rem;
  height: 1.9rem;
  padding: 0;
  border: 0;
  border-radius: var(--radius-soft);
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  transition:
    color 0.15s ease,
    background 0.15s ease;
}

.app-password-input__toggle:hover {
  color: var(--text-main);
  background: var(--bg-muted);
}

.app-password-input__toggle:focus-visible {
  outline: 2px solid var(--accent-focus);
  outline-offset: 1px;
  color: var(--text-main);
}

.app-password-input__toggle svg {
  width: 1.05rem;
  height: 1.05rem;
}
</style>
