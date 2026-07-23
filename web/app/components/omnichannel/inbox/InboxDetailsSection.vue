<script setup lang="ts">
defineProps<{
  title: string
  defaultOpen?: boolean
}>()
</script>

<template>
  <details class="details-section" :open="defaultOpen">
    <summary class="details-section__summary">
      <h3 class="details-section__title">{{ title }}</h3>
      <UIcon class="details-section__chevron" name="i-lucide-chevron-down" aria-hidden="true" />
    </summary>
    <div class="details-section__body">
      <slot></slot>
    </div>
  </details>
</template>

<style scoped>
.details-section {
  flex: 0 0 auto;
  overflow: clip;
  border: 0;
  border-bottom: 1px solid rgb(var(--border) / 0.55);
  background: transparent;
}

.details-section__summary {
  display: flex;
  min-height: 2.65rem;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.55rem 0.75rem;
  cursor: pointer;
  list-style: none;
  user-select: none;
}

.details-section__summary::-webkit-details-marker {
  display: none;
}

.details-section__summary:hover {
  background: rgb(var(--surface-muted) / 0.28);
}

.details-section__title {
  margin: 0;
  font-size: 0.8rem;
  font-weight: 600;
}

.details-section__chevron {
  flex: 0 0 auto;
  color: rgb(var(--muted));
  transition: transform 160ms ease;
}

.details-section[open] .details-section__chevron {
  transform: rotate(180deg);
}

.details-section[open] .details-section__summary {
  color: rgb(var(--text));
}

.details-section__body {
  display: grid;
  min-width: 0;
  gap: 0.65rem;
  padding: 0.25rem 0.75rem 0.85rem;
}
</style>
