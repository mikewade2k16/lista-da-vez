<script setup lang="ts">
withDefaults(
  defineProps<{
    modelValue: string
    ariaLabel?: string
    disabled?: boolean
  }>(),
  {
    ariaLabel: 'Mês de referência',
    disabled: false,
  },
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

function updateValue(event: Event): void {
  emit('update:modelValue', (event.target as HTMLInputElement).value)
}
</script>

<template>
  <label class="app-month-input">
    <input
      :value="modelValue"
      type="month"
      :aria-label="ariaLabel"
      :disabled="disabled"
      @change="updateValue"
    />
  </label>
</template>

<style scoped>
.app-month-input {
  display: inline-flex;
  align-items: center;
  min-height: 2rem;
  padding: 0 0.6rem;
  border: 1px solid rgb(var(--ring) / 0.16);
  border-radius: 0.55rem;
  background: rgb(var(--surface-2) / 0.88);
  flex: 0 0 auto;
}

.app-month-input:focus-within {
  border-color: rgb(var(--ring) / 0.42);
  box-shadow: 0 0 0 2px rgb(var(--ring) / 0.1);
}

.app-month-input input {
  min-width: 7.5rem;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--text-main);
  color-scheme: dark light;
  font-size: 0.72rem;
  font-weight: 600;
}

.app-month-input input:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}
</style>
