<script setup lang="ts">
import type { ToolsViewMode } from '~/composables/useToolsViewMode'

// Alterna entre cards (padrao) e tabela.
const mode = defineModel<ToolsViewMode>({ required: true })
</script>

<template>
  <div class="tools-view" role="group" aria-label="Visualizacao">
    <button
      type="button"
      :class="{ 'is-active': mode === 'cards' }"
      :aria-pressed="mode === 'cards'"
      aria-label="Ver em cards"
      title="Cards"
      @click="mode = 'cards'"
    >
      <UIcon name="i-lucide-layout-grid" />
    </button>
    <button
      type="button"
      :class="{ 'is-active': mode === 'table' }"
      :aria-pressed="mode === 'table'"
      aria-label="Ver em tabela"
      title="Tabela"
      @click="mode = 'table'"
    >
      <UIcon name="i-lucide-table-2" />
    </button>
  </div>
</template>

<style scoped>
.tools-view {
  display: inline-flex;
  padding: 0.15rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
}

.tools-view button {
  display: grid;
  place-items: center;
  width: 1.9rem;
  height: 1.7rem;
  border: none;
  border-radius: calc(var(--radius-sm) - 2px);
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
}

.tools-view button.is-active {
  background: rgb(var(--primary) / 0.16);
  color: var(--text-main);
}
</style>
