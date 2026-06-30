<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'

// Campo de busca reutilizavel com debounce e botao de limpar. Mantem o valor
// digitado localmente (resposta imediata ao teclado) e so propaga `update:modelValue`
// apos o debounce, para o consumidor filtrar sem refazer trabalho a cada tecla.
// O clear zera na hora (sem esperar o debounce). Acessivel: input com aria-label.
//
// Uso: filtro client-side de listas longas (permissoes, paginas). NAO faz fetch —
// o consumidor decide o que fazer com o termo (aqui sempre filtro local).

const props = withDefaults(
  defineProps<{
    // Termo atual (v-model). Fonte de verdade fica no pai.
    modelValue: string
    placeholder?: string
    // Atraso do debounce em ms. 0 = sem debounce (propaga na hora).
    debounceMs?: number
    ariaLabel?: string
  }>(),
  {
    placeholder: 'Buscar...',
    debounceMs: 220,
    ariaLabel: 'Buscar',
  },
)

const emit = defineEmits<{ 'update:modelValue': [string] }>()

// Espelho local do texto: digita aqui, propaga depois do debounce.
const local = ref(props.modelValue)
let timer: ReturnType<typeof setTimeout> | null = null

// Re-sincroniza quando o pai limpa/reseta o termo de fora (ex.: troca de escopo).
watch(
  () => props.modelValue,
  (next) => {
    if (next !== local.value) local.value = next
  },
)

function flush(value: string) {
  if (value !== props.modelValue) emit('update:modelValue', value)
}

function onInput(raw: unknown) {
  const value = String(raw ?? '')
  local.value = value
  if (timer) clearTimeout(timer)
  if (props.debounceMs <= 0) {
    flush(value)
    return
  }
  timer = setTimeout(() => {
    flush(value)
    timer = null
  }, props.debounceMs)
}

function clear() {
  if (timer) {
    clearTimeout(timer)
    timer = null
  }
  local.value = ''
  flush('')
}

onBeforeUnmount(() => {
  if (timer) clearTimeout(timer)
})
</script>

<template>
  <div class="app-search">
    <UIcon name="i-lucide-search" class="app-search__icon" />
    <input
      :value="local"
      type="search"
      class="app-search__input"
      :placeholder="placeholder"
      :aria-label="ariaLabel"
      @input="onInput(($event.target as HTMLInputElement).value)"
      @keydown.esc.stop="clear"
    />
    <button
      v-if="local"
      type="button"
      class="app-search__clear"
      aria-label="Limpar busca"
      title="Limpar"
      @click="clear"
    >
      <UIcon name="i-lucide-x" class="app-search__clear-icon" />
    </button>
  </div>
</template>

<style scoped>
.app-search {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  width: 100%;
  padding: 0.4rem 0.55rem;
  border-radius: var(--radius-md);
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface));
}

.app-search__icon {
  flex-shrink: 0;
  width: 1rem;
  height: 1rem;
  color: rgb(var(--muted));
}

.app-search__input {
  flex: 1;
  min-width: 0;
  border: 0;
  background: transparent;
  color: rgb(var(--text));
  font-size: 0.85rem;
  outline: none;
}

.app-search__input::placeholder {
  color: rgb(var(--muted));
}

/* Remove o X nativo do type=search (Chrome/Edge) — usamos o nosso. */
.app-search__input::-webkit-search-cancel-button {
  appearance: none;
}

.app-search__clear {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 1.25rem;
  height: 1.25rem;
  padding: 0;
  border: 0;
  border-radius: 999px;
  background: rgb(var(--surface-2));
  color: rgb(var(--muted));
  cursor: pointer;
}

.app-search__clear:hover {
  color: rgb(var(--text));
}

.app-search__clear-icon {
  width: 0.85rem;
  height: 0.85rem;
}
</style>
