<script setup lang="ts">
import { ref, watch } from 'vue'

// Secao colapsavel (accordion) reutilizavel, generalizando o BioCollapsibleItem
// para qualquer editor/workspace (nao acoplada ao bio). Cabecalho clicavel com
// titulo + resumo curto opcional (ex.: contagem) e chevron que alterna o corpo.
// O estado aberto/fechado e local; `defaultOpen` so define o valor inicial
// (default: aberto). Acessivel: o header e um <button> com aria-expanded.
// Conteudo via slot `default`; slot opcional `actions` (botoes a direita do
// titulo, ex.: adicionar/remover). Usa v-show para nao remover o corpo do DOM.

const props = withDefaults(
  defineProps<{
    title: string
    summary?: string
    defaultOpen?: boolean
  }>(),
  {
    summary: undefined,
    defaultOpen: true,
  },
)

const open = ref(props.defaultOpen)

// Reage a mudancas externas de defaultOpen (ex.: o pai recolhe todos os blocos
// menos o ativo) sem prender o estado se o usuario ja interagiu manualmente.
watch(
  () => props.defaultOpen,
  (next) => {
    open.value = next
  },
)

function toggle() {
  open.value = !open.value
}
</script>

<template>
  <section class="omni-collapse" :class="{ 'omni-collapse--open': open }">
    <div class="omni-collapse__head">
      <button type="button" class="omni-collapse__toggle" :aria-expanded="open" @click="toggle">
        <UIcon
          class="omni-collapse__chevron"
          :class="{ 'omni-collapse__chevron--open': open }"
          name="i-lucide-chevron-right"
        />
        <span class="omni-collapse__title">{{ title }}</span>
        <span v-if="summary" class="omni-collapse__summary">{{ summary }}</span>
      </button>
      <div v-if="$slots.actions" class="omni-collapse__actions">
        <slot name="actions"></slot>
      </div>
    </div>

    <div v-show="open" class="omni-collapse__body">
      <slot></slot>
    </div>
  </section>
</template>

<style scoped>
.omni-collapse {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface) / 0.7);
}

.omni-collapse--open {
  border-color: rgb(var(--primary) / 0.35);
}

.omni-collapse__head {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.45rem 0.55rem;
}

.omni-collapse__toggle {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex: 1;
  min-width: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--text-main);
  cursor: pointer;
  text-align: left;
}

.omni-collapse__chevron {
  flex-shrink: 0;
  color: var(--text-muted);
  transition:
    color 0.15s ease,
    transform 0.15s ease;
}

.omni-collapse__chevron--open {
  transform: rotate(90deg);
}

.omni-collapse__toggle:hover .omni-collapse__chevron {
  color: rgb(var(--primary));
}

.omni-collapse__title {
  font-size: 0.82rem;
  font-weight: 700;
  color: var(--text-main);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.omni-collapse__summary {
  font-size: 0.74rem;
  font-weight: 500;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}

.omni-collapse__actions {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  flex-shrink: 0;
}

.omni-collapse__body {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  padding: 0.6rem 0.6rem 0.65rem;
  border-top: 1px solid var(--line-soft);
}
</style>
