<script setup lang="ts">
withDefaults(defineProps<{ title?: string; tag?: string; compact?: boolean; focus?: boolean }>(), {
  title: '',
  tag: '',
  compact: false,
  focus: false,
})
</script>

<template>
  <section class="app-calendar-surface" :class="{ 'is-compact': compact, 'is-focus': focus }">
    <slot name="header">
      <header v-if="title || tag" class="app-calendar-surface__header">
        <h3>{{ title }}</h3>
        <span v-if="tag">{{ tag }}</span>
        <div class="app-calendar-surface__actions"><slot name="header-actions"></slot></div>
      </header>
    </slot>
    <slot></slot>
  </section>
</template>

<style scoped>
.app-calendar-surface {
  min-height: 600px;
  border: 1px solid rgb(var(--primary) / 0.42);
  border-radius: var(--radius-card);
  background: linear-gradient(145deg, rgb(var(--surface-2) / 0.92), rgb(var(--surface) / 0.88));
  box-shadow:
    inset 0 1px 0 rgb(255 255 255/0.025),
    0 10px 35px rgb(0 0 0/0.12);
  overflow: hidden;
}
.app-calendar-surface.is-compact {
  min-height: 0;
}
.app-calendar-surface.is-focus {
  border-color: rgb(var(--primary) / 0.72);
}
.app-calendar-surface__header {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  min-height: 3.2rem;
  padding: 0.55rem 0.7rem;
  overflow-x: auto;
}
.app-calendar-surface__header h3 {
  margin: 0;
  color: var(--text-main);
  font-size: 1rem;
}
.app-calendar-surface__header span {
  border-radius: 999px;
  padding: 0.18rem 0.5rem;
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
  font-size: 0.62rem;
  font-weight: 800;
  text-transform: uppercase;
}
.app-calendar-surface__actions {
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
}
</style>
