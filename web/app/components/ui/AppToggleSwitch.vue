<script setup>
const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  disabled: {
    type: Boolean,
    default: false
  },
  label: {
    type: String,
    default: ""
  },
  compact: {
    type: Boolean,
    default: false
  }
});

const emit = defineEmits(["update:modelValue", "change"]);

function toggle() {
  if (props.disabled) {
    return;
  }

  const nextValue = !props.modelValue;
  emit("update:modelValue", nextValue);
  emit("change", nextValue);
}
</script>

<template>
  <button
    class="app-toggle-switch"
    :class="{
      'is-on': modelValue,
      'is-disabled': disabled,
      'app-toggle-switch--compact': compact
    }"
    type="button"
    role="switch"
    :aria-checked="modelValue ? 'true' : 'false'"
    :aria-label="label || 'Alternar estado'"
    :disabled="disabled"
    @click="toggle"
  >
    <span class="app-toggle-switch__track">
      <span class="app-toggle-switch__thumb" />
    </span>
    <span v-if="label" class="app-toggle-switch__label">{{ label }}</span>
  </button>
</template>

<style scoped>
.app-toggle-switch {
  display: inline-flex;
  align-items: center;
  gap: 0.55rem;
  border: none;
  padding: 0;
  background: transparent;
  color: var(--text-main);
  cursor: pointer;
}

.app-toggle-switch__track {
  width: 2.7rem;
  min-width: 2.7rem;
  height: 1.55rem;
  padding: 0.16rem;
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.2);
  transition: background 0.18s ease;
}

.app-toggle-switch__thumb {
  width: 1.2rem;
  height: 1.2rem;
  border-radius: 999px;
  background: #ffffff;
  box-shadow: 0 6px 14px rgba(15, 23, 42, 0.28);
  transition: transform 0.18s ease;
}

.app-toggle-switch.is-on .app-toggle-switch__track {
  background: rgba(34, 197, 94, 0.34);
}

.app-toggle-switch.is-on .app-toggle-switch__thumb {
  transform: translateX(1.1rem);
}

.app-toggle-switch.is-disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.app-toggle-switch__label {
  font-size: 0.82rem;
  color: var(--text-muted);
}

.app-toggle-switch--compact .app-toggle-switch__track {
  width: 2.35rem;
  min-width: 2.35rem;
  height: 1.35rem;
}

.app-toggle-switch--compact .app-toggle-switch__thumb {
  width: 1rem;
  height: 1rem;
}

.app-toggle-switch--compact.is-on .app-toggle-switch__thumb {
  transform: translateX(0.95rem);
}
</style>