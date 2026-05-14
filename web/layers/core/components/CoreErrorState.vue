<script setup lang="ts">
withDefaults(
  defineProps<{
    title?: string;
    message?: string;
    retryLabel?: string;
    showRetry?: boolean;
    compact?: boolean;
  }>(),
  {
    title: "Algo deu errado",
    message: "Nao foi possivel carregar os dados. Tente novamente em instantes.",
    retryLabel: "Tentar de novo",
    showRetry: true,
    compact: false
  }
);

const emit = defineEmits<{ (e: "retry"): void }>();
</script>

<template>
  <div class="core-error-state" :class="{ 'is-compact': compact }" role="alert">
    <span class="material-icons-round core-error-state__icon" aria-hidden="true">error_outline</span>
    <h3 class="core-error-state__title">{{ title }}</h3>
    <p v-if="message" class="core-error-state__msg">{{ message }}</p>
    <button
      v-if="showRetry"
      type="button"
      class="core-error-state__retry"
      @click="emit('retry')"
    >
      <span class="material-icons-round" aria-hidden="true">refresh</span>
      {{ retryLabel }}
    </button>
  </div>
</template>

<style scoped>
.core-error-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  padding: 2.5rem 1.5rem;
  text-align: center;
  color: rgba(226, 232, 240, 0.85);
}

.core-error-state.is-compact {
  padding: 1.4rem 1rem;
}

.core-error-state__icon {
  font-size: 2.4rem;
  color: rgba(248, 113, 113, 0.85);
}

.core-error-state__title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 700;
  color: #fecaca;
}

.core-error-state__msg {
  margin: 0;
  max-width: 30rem;
  font-size: 0.85rem;
  color: rgba(226, 232, 240, 0.72);
  line-height: 1.45;
}

.core-error-state__retry {
  margin-top: 0.6rem;
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.5rem 0.9rem;
  border-radius: 8px;
  border: 1px solid rgba(248, 113, 113, 0.4);
  background: rgba(248, 113, 113, 0.12);
  color: #fecaca;
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
}

.core-error-state__retry:hover,
.core-error-state__retry:focus {
  background: rgba(248, 113, 113, 0.2);
  border-color: rgba(248, 113, 113, 0.7);
  outline: none;
}

.core-error-state__retry .material-icons-round {
  font-size: 1rem;
}
</style>
