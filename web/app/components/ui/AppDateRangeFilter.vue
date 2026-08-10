<script setup lang="ts">
import { CalendarDays } from 'lucide-vue-next'

withDefaults(
  defineProps<{
    modelValue: string
    endDate?: string
    placeholder?: string
    ariaLabel?: string
    disabled?: boolean
  }>(),
  {
    endDate: '',
    placeholder: 'Selecionar período',
    ariaLabel: 'Período',
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
  'update:endDate': [value: string]
}>()
</script>

<template>
  <AppDatePicker
    :model-value="modelValue"
    :end-date="endDate"
    @update:model-value="emit('update:modelValue', $event)"
    @update:end-date="emit('update:endDate', $event)"
  >
    <template #default="{ label }">
      <button
        type="button"
        class="app-date-range-filter"
        :aria-label="ariaLabel"
        :disabled="disabled"
      >
        <CalendarDays :size="13" />
        <span>{{ label || placeholder }}</span>
      </button>
    </template>
  </AppDatePicker>
</template>

<style scoped>
.app-date-range-filter {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  width: 100%;
  min-width: 0;
  min-height: 2rem;
  padding: 0 0.65rem;
  overflow: hidden;
  border: 1px solid rgb(var(--ring) / 0.16);
  border-radius: 0.55rem;
  background: rgb(var(--surface-2) / 0.88);
  color: var(--text-main);
  font-size: 0.72rem;
  font-weight: 600;
  cursor: pointer;
}

.app-date-range-filter:hover:not(:disabled) {
  border-color: rgb(var(--ring) / 0.4);
}

.app-date-range-filter > svg {
  flex: 0 0 auto;
  color: var(--text-muted);
}

.app-date-range-filter > span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-date-range-filter:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}
</style>
