<script setup lang="ts">
withDefaults(
  defineProps<{
    icon?: string;
    title?: string;
    description?: string;
    actionLabel?: string;
    compact?: boolean;
  }>(),
  {
    icon: "inbox",
    title: "Nada por aqui ainda",
    description: "",
    actionLabel: "",
    compact: false
  }
);

const emit = defineEmits<{ (e: "action"): void }>();
</script>

<template>
  <div class="core-empty-state" :class="{ 'is-compact': compact }" role="status">
    <span class="material-icons-round core-empty-state__icon" aria-hidden="true">{{ icon }}</span>
    <h3 class="core-empty-state__title">{{ title }}</h3>
    <p v-if="description" class="core-empty-state__desc">{{ description }}</p>
    <button
      v-if="actionLabel"
      type="button"
      class="core-empty-state__action"
      @click="emit('action')"
    >
      {{ actionLabel }}
    </button>
  </div>
</template>

<style scoped>
.core-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  padding: 2.5rem 1.5rem;
  text-align: center;
  color: rgba(226, 232, 240, 0.78);
}

.core-empty-state.is-compact {
  padding: 1.4rem 1rem;
}

.core-empty-state__icon {
  font-size: 2.4rem;
  color: rgba(148, 163, 184, 0.5);
  margin-bottom: 0.2rem;
}

.core-empty-state__title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 700;
  color: #e2e8f0;
}

.core-empty-state__desc {
  margin: 0;
  max-width: 28rem;
  font-size: 0.85rem;
  color: rgba(148, 163, 184, 0.85);
  line-height: 1.45;
}

.core-empty-state__action {
  margin-top: 0.6rem;
  padding: 0.5rem 0.9rem;
  border-radius: 8px;
  border: 1px solid rgba(129, 140, 248, 0.4);
  background: rgba(99, 102, 241, 0.14);
  color: #c7d2fe;
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
}

.core-empty-state__action:hover,
.core-empty-state__action:focus {
  background: rgba(99, 102, 241, 0.22);
  border-color: rgba(129, 140, 248, 0.65);
  outline: none;
}
</style>
