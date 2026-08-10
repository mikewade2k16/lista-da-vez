<script setup lang="ts">
import { computed } from 'vue'

import AppSegmentedFilter from '~/components/ui/AppSegmentedFilter.vue'
import { goalWeekPeriods } from '~/utils/goal-periods'

const props = withDefaults(
  defineProps<{
    modelValue: string
    includeMonth?: boolean
    ariaLabel?: string
    disabled?: boolean
    month?: string
  }>(),
  {
    includeMonth: true,
    ariaLabel: 'Período da meta',
    disabled: false,
    month: '',
  },
)

const emit = defineEmits<{ 'update:modelValue': [string] }>()

const options = computed(() => [
  ...(props.includeMonth ? [{ value: 'month', label: 'Mês' }] : []),
  ...goalWeekPeriods(props.month).map((value, index) => ({ value, label: `S${index + 1}` })),
])
</script>

<template>
  <AppSegmentedFilter
    :model-value="modelValue"
    :options="options"
    :aria-label="ariaLabel"
    :disabled="disabled"
    @update:model-value="emit('update:modelValue', $event)"
  />
</template>
