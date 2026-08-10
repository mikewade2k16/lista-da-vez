<script setup lang="ts">
withDefaults(defineProps<{ title: string; subtitle?: string; ariaLabel?: string }>(), {
  subtitle: '',
  ariaLabel: '',
})
defineEmits<{ close: [] }>()
</script>

<template>
  <aside class="app-day-items-panel" role="dialog" :aria-label="ariaLabel || title">
    <header class="app-day-items-panel__header">
      <span>
        <strong>{{ title }}</strong>
        <small v-if="subtitle">{{ subtitle }}</small>
      </span>
      <button type="button" aria-label="Fechar" @click="$emit('close')">
        <UIcon name="i-lucide-x" />
      </button>
    </header>
    <div class="app-day-items-panel__body"><slot></slot></div>
    <footer v-if="$slots.footer" class="app-day-items-panel__footer">
      <slot name="footer"></slot>
    </footer>
  </aside>
</template>

<style scoped>
.app-day-items-panel {
  display: flex;
  flex: 0 0 min(400px, 92vw);
  flex-direction: column;
  min-width: 0;
  border: 1px solid rgb(var(--border) / 0.72);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.96);
  box-shadow: 0 18px 46px rgb(0 0 0/0.22);
  overflow: hidden;
}
.app-day-items-panel__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.9rem 1rem;
  border-bottom: 1px solid rgb(var(--border) / 0.55);
}
.app-day-items-panel__header > span {
  display: grid;
  gap: 0.16rem;
}
.app-day-items-panel__header strong {
  color: var(--text-main);
  font-size: 0.9rem;
}
.app-day-items-panel__header small {
  color: var(--text-muted);
  font-size: 0.7rem;
}
.app-day-items-panel__header button {
  display: grid;
  place-items: center;
  border: 0;
  padding: 0.2rem;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
}
.app-day-items-panel__body {
  flex: 1;
  min-height: 0;
  padding: 0.8rem 1rem;
  overflow: auto;
}
.app-day-items-panel__footer {
  padding: 0.75rem 1rem;
  border-top: 1px solid rgb(var(--border) / 0.55);
}
</style>
