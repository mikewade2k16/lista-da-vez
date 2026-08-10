<script setup lang="ts">
withDefaults(
  defineProps<{
    label: string
    showLabel?: boolean
  }>(),
  {
    showLabel: false,
  },
)
</script>

<template>
  <div class="app-filter-field">
    <span class="app-filter-field__label" :class="{ 'sr-only': !showLabel }">{{ label }}</span>
    <slot></slot>
  </div>
</template>

<style scoped>
.app-filter-field {
  display: grid;
  gap: 0.25rem;
  min-width: 0;
}

.app-filter-field__label {
  color: var(--text-muted);
  font-size: 0.66rem;
  font-weight: 700;
  line-height: 1;
}
</style>
