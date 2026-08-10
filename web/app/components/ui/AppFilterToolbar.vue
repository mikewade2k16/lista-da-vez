<script setup lang="ts">
withDefaults(
  defineProps<{
    ariaLabel?: string
    surface?: boolean
    nowrap?: boolean
  }>(),
  {
    ariaLabel: 'Filtros',
    surface: true,
    nowrap: false,
  },
)
</script>

<template>
  <div
    class="app-filter-toolbar"
    :class="{
      'app-filter-toolbar--surface': surface,
      'app-filter-toolbar--nowrap': nowrap,
    }"
    role="search"
    :aria-label="ariaLabel"
  >
    <div class="app-filter-toolbar__main">
      <slot></slot>
    </div>
    <div v-if="$slots.actions" class="app-filter-toolbar__actions">
      <slot name="actions"></slot>
    </div>
  </div>
</template>

<style scoped>
.app-filter-toolbar {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
}

.app-filter-toolbar--surface {
  padding: 0.6rem 0.7rem;
  border: 1px solid rgb(var(--border) / 0.72);
  border-radius: var(--radius-md);
  background: rgb(var(--surface) / 0.72);
}

.app-filter-toolbar__main,
.app-filter-toolbar__actions {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
}

.app-filter-toolbar__main {
  flex: 1 1 auto;
  flex-wrap: wrap;
}

.app-filter-toolbar__actions {
  flex: 0 0 auto;
  justify-content: flex-end;
  flex-wrap: wrap;
}

.app-filter-toolbar--nowrap,
.app-filter-toolbar--nowrap .app-filter-toolbar__main,
.app-filter-toolbar--nowrap .app-filter-toolbar__actions {
  flex-wrap: nowrap;
}

.app-filter-toolbar--nowrap {
  overflow-x: auto;
  scrollbar-width: thin;
}

@media (max-width: 900px) {
  .app-filter-toolbar:not(.app-filter-toolbar--nowrap) {
    align-items: stretch;
    flex-direction: column;
  }

  .app-filter-toolbar:not(.app-filter-toolbar--nowrap) .app-filter-toolbar__actions {
    justify-content: flex-start;
  }
}
</style>
