<script setup lang="ts">
export interface AppCalendarPeriodOption {
  value: string
  label: string
  title?: string
  current?: boolean
}

const props = withDefaults(
  defineProps<{
    modelValue: string
    options: AppCalendarPeriodOption[]
    monthValue?: string
    monthLabel?: string
    showMonth?: boolean
    disabled?: boolean
    ariaLabel?: string
  }>(),
  {
    monthValue: 'month',
    monthLabel: 'M',
    showMonth: true,
    disabled: false,
    ariaLabel: 'Período do calendário',
  },
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
function select(value: string): void {
  if (!props.disabled) emit('update:modelValue', value)
}
</script>

<template>
  <nav class="app-calendar-period-rail" :aria-label="ariaLabel">
    <button
      v-if="showMonth"
      type="button"
      class="app-calendar-period-rail__item app-calendar-period-rail__item--month"
      :class="{ 'is-active': modelValue === monthValue }"
      :aria-current="modelValue === monthValue ? 'true' : undefined"
      :disabled="disabled"
      title="Visão mensal"
      @click="select(monthValue)"
    >
      {{ monthLabel }}
    </button>
    <button
      v-for="option in options"
      :key="option.value"
      type="button"
      class="app-calendar-period-rail__item"
      :class="{ 'is-active': modelValue === option.value, 'is-current': option.current }"
      :aria-current="modelValue === option.value ? 'true' : undefined"
      :title="option.title || option.label"
      :disabled="disabled"
      @click="select(option.value)"
    >
      {{ option.label }}
    </button>
  </nav>
</template>

<style scoped>
.app-calendar-period-rail {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.45rem;
  width: 3.25rem;
  padding: 0.55rem 0.4rem;
  border-right: 1px solid rgb(var(--border) / 0.5);
  background: rgb(var(--surface) / 0.52);
}
.app-calendar-period-rail__item {
  position: relative;
  display: grid;
  place-items: center;
  width: 2.15rem;
  height: 2.15rem;
  border: 1px solid rgb(var(--border) / 0.62);
  border-radius: 999px;
  padding: 0;
  background: rgb(var(--surface-2) / 0.64);
  color: var(--text-muted);
  font-size: 0.68rem;
  font-weight: 800;
  cursor: pointer;
  transition: 0.16s ease;
}
.app-calendar-period-rail__item:hover:not(:disabled) {
  border-color: rgb(var(--primary) / 0.48);
  color: rgb(var(--primary));
}
.app-calendar-period-rail__item.is-active {
  border-color: rgb(var(--primary) / 0.72);
  background: rgb(var(--primary));
  color: #fff;
  box-shadow: 0 0 18px rgb(var(--primary) / 0.22);
}
.app-calendar-period-rail__item.is-current:not(.is-active)::after {
  position: absolute;
  top: 0.05rem;
  right: 0.05rem;
  width: 0.35rem;
  height: 0.35rem;
  border-radius: 999px;
  background: rgb(var(--primary));
  content: '';
}
.app-calendar-period-rail__item--month {
  margin-bottom: 0.25rem;
}
.app-calendar-period-rail__item:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
</style>
