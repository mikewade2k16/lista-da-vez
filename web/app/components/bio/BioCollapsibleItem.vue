<script setup lang="ts">
import { ref, watch } from 'vue'

// Item de edicao colapsavel (accordion) reutilizavel pelo editor de bio.
// Cabecalho com titulo + resumo curto (ex.: contagem ou nome) e botao
// expandir/recolher (chevron). Clique no header tambem alterna. O estado
// aberto/fechado e local; `defaultOpen` so define o valor inicial. Acessivel
// via aria-expanded no botao. Conteudo via slot padrao; slots opcionais
// `actions` (ao lado do chevron, ex.: remover/reordenar) e `summary`.

const props = defineProps<{
  title: string
  summary?: string
  defaultOpen?: boolean
}>()

const open = ref(props.defaultOpen ?? false)

// Reage a mudancas externas de defaultOpen (ex.: lista passa de 1 para varios
// itens e o pai decide recolher por padrao) sem perder o estado se igual.
watch(
  () => props.defaultOpen,
  (next) => {
    open.value = next ?? false
  },
)

function toggle() {
  open.value = !open.value
}
</script>

<template>
  <div class="bio-collapsible" :class="{ 'bio-collapsible--open': open }">
    <div class="bio-collapsible__head">
      <button type="button" class="bio-collapsible__toggle" :aria-expanded="open" @click="toggle">
        <UIcon
          class="bio-collapsible__chevron"
          :name="open ? 'i-lucide-chevron-down' : 'i-lucide-chevron-right'"
        />
        <span class="bio-collapsible__title">{{ title }}</span>
        <span v-if="summary" class="bio-collapsible__summary">{{ summary }}</span>
      </button>
      <div v-if="$slots.actions" class="bio-collapsible__actions">
        <slot name="actions"></slot>
      </div>
    </div>

    <div v-show="open" class="bio-collapsible__body">
      <slot></slot>
    </div>
  </div>
</template>

<style scoped>
.bio-collapsible {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface) / 0.7);
}

.bio-collapsible--open {
  border-color: rgb(var(--primary) / 0.35);
}

.bio-collapsible__head {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.45rem 0.55rem;
}

.bio-collapsible__toggle {
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

.bio-collapsible__chevron {
  flex-shrink: 0;
  color: var(--text-muted);
  transition: color 0.15s ease;
}

.bio-collapsible__toggle:hover .bio-collapsible__chevron {
  color: rgb(var(--primary));
}

.bio-collapsible__title {
  font-size: 0.82rem;
  font-weight: 700;
  color: var(--text-main);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.bio-collapsible__summary {
  font-size: 0.74rem;
  font-weight: 500;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}

.bio-collapsible__actions {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  flex-shrink: 0;
}

.bio-collapsible__body {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  padding: 0 0.6rem 0.65rem;
  border-top: 1px solid var(--line-soft);
  padding-top: 0.6rem;
}
</style>
