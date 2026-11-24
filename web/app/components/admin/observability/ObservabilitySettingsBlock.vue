<script setup lang="ts">
// Bloco colapsavel das configuracoes: fechado por padrao, com titulo e resumo sempre
// visiveis. O rodape (salvar) fica dentro do corpo.
defineProps<{ title: string; summary: string }>()
const open = ref(false)
</script>

<template>
  <section class="obs-settings-block">
    <button
      type="button"
      class="obs-settings-block__head"
      :aria-expanded="open"
      @click="open = !open"
    >
      <UIcon
        :name="open ? 'i-lucide-chevron-down' : 'i-lucide-chevron-right'"
        class="obs-settings-block__chevron"
      />
      <strong class="obs-settings-block__title">{{ title }}</strong>
      <span class="obs-muted obs-settings-block__summary">{{ summary }}</span>
    </button>
    <div v-show="open" class="obs-settings-block__body">
      <slot />
    </div>
  </section>
</template>

<style scoped>
.obs-settings-block {
  min-width: 0;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-md);
  background: rgb(var(--surface));
  box-shadow: var(--shadow-card);
}

.obs-settings-block__head {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  width: 100%;
  padding: 0.75rem 1rem;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.obs-settings-block__title {
  font-size: 0.9rem;
}

.obs-settings-block__summary {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.obs-settings-block__body {
  display: grid;
  gap: 0.9rem;
  padding: 0 1rem 1rem;
}
</style>
