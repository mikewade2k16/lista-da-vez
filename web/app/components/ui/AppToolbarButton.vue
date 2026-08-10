<script setup lang="ts">
type ButtonVariant = 'primary' | 'soft' | 'ghost' | 'danger'

withDefaults(
  defineProps<{
    label?: string
    icon?: string
    variant?: ButtonVariant
    active?: boolean
    disabled?: boolean
    loading?: boolean
    title?: string
  }>(),
  {
    label: '',
    icon: '',
    variant: 'soft',
    active: false,
    disabled: false,
    loading: false,
    title: '',
  },
)

const emit = defineEmits<{ click: [event: MouseEvent] }>()
</script>

<template>
  <button
    type="button"
    class="app-toolbar-button"
    :class="[`app-toolbar-button--${variant}`, { 'is-active': active }]"
    :disabled="disabled || loading"
    :title="title || undefined"
    @click="emit('click', $event)"
  >
    <UIcon
      v-if="loading || icon"
      :name="loading ? 'i-lucide-loader-circle' : icon"
      class="app-toolbar-button__icon"
      :class="{ 'is-spinning': loading }"
    />
    <span v-if="label">{{ label }}</span>
    <slot></slot>
  </button>
</template>

<style scoped>
.app-toolbar-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.35rem;
  min-height: 2rem;
  padding: 0 0.65rem;
  border: 1px solid rgb(var(--border) / 0.72);
  border-radius: 0.55rem;
  background: rgb(var(--surface-2) / 0.72);
  color: var(--text-muted);
  font-size: 0.7rem;
  font-weight: 750;
  white-space: nowrap;
  cursor: pointer;
}

.app-toolbar-button:hover:not(:disabled) {
  border-color: rgb(var(--ring) / 0.4);
  color: var(--text-main);
}

.app-toolbar-button--primary,
.app-toolbar-button.is-active {
  border-color: rgb(var(--primary) / 0.4);
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.app-toolbar-button--ghost {
  border-color: transparent;
  background: transparent;
}

.app-toolbar-button--danger {
  border-color: rgb(var(--danger) / 0.28);
  background: rgb(var(--danger) / 0.08);
  color: rgb(var(--danger));
}

.app-toolbar-button:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.app-toolbar-button__icon {
  width: 0.85rem;
  height: 0.85rem;
  flex: 0 0 auto;
}

.app-toolbar-button__icon.is-spinning {
  animation: app-toolbar-spin 0.8s linear infinite;
}

@keyframes app-toolbar-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
