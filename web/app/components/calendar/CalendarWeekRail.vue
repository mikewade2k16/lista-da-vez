<script setup lang="ts">
import { computed } from 'vue'
import AppCalendarPeriodRail from '~/components/ui/AppCalendarPeriodRail.vue'
import type { CalendarView } from '~/utils/calendar'

const props = defineProps<{
  weeks: { index: number; label: string; startKey: string }[]
  view: CalendarView
  focusedWeekIndex: number
  currentWeekIndex: number
}>()

const emit = defineEmits<{ select: [index: number]; month: [] }>()
const options = computed(() =>
  props.weeks.map((week) => ({
    value: String(week.index),
    label: week.label,
    title: `Semana ${week.index + 1}`,
    current: week.index === props.currentWeekIndex,
  })),
)
const selected = computed(() => (props.view === 'month' ? 'month' : String(props.focusedWeekIndex)))
function update(value: string): void {
  if (value === 'month') emit('month')
  else emit('select', Number(value))
}
</script>

<template>
  <AppCalendarPeriodRail
    :model-value="selected"
    :options="options"
    aria-label="Navegação do calendário"
    @update:model-value="update"
  />
</template>
