<script setup>
const props = defineProps({
  as: {
    type: String,
    default: 'button',
  },
  type: {
    type: String,
    default: 'button',
  },
  variant: {
    type: String,
    default: 'primary',
  },
  block: {
    type: Boolean,
    default: false,
  },
  disabled: {
    type: Boolean,
    default: false,
  },
})
</script>

<template>
  <component
    :is="props.as"
    class="app-panel-button"
    :class="[
      `app-panel-button--${props.variant}`,
      { 'app-panel-button--block': props.block, 'is-disabled': props.disabled },
    ]"
    :type="props.as === 'button' ? props.type : undefined"
    :disabled="props.as === 'button' ? props.disabled : undefined"
    :aria-disabled="props.disabled ? 'true' : undefined"
  >
    <slot></slot>
  </component>
</template>

<style scoped>
.app-panel-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 36px;
  padding: 0 18px;
  border: 0;
  border-radius: 14px;
  font-size: 0.8rem;
  font-weight: 700;
  line-height: 1;
  text-decoration: none;
  cursor: pointer;
  transition:
    opacity 0.15s ease,
    transform 0.15s ease;
}

.app-panel-button:hover {
  opacity: 0.94;
}

.app-panel-button:active {
  transform: translateY(1px);
}

.app-panel-button--block {
  width: 100%;
}

.app-panel-button--primary {
  background: linear-gradient(135deg, rgb(var(--primary)) 0%, rgb(var(--primary-600)) 100%);
  color: rgb(255 255 255);
}

.app-panel-button--secondary {
  background: rgb(var(--surface-2));
  color: var(--text-main);
}

.app-panel-button--danger {
  background: rgb(var(--danger));
  color: rgb(255 255 255);
}

.app-panel-button--ghost {
  background: transparent;
  color: var(--text-muted);
  border: 1px solid var(--line-soft);
}

.app-panel-button.is-disabled {
  opacity: 0.5;
  cursor: not-allowed;
  pointer-events: none;
}
</style>
