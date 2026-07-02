<script setup lang="ts">
import type { CalendarView } from '~/utils/calendar'

defineProps<{
  weeks: { index: number; label: string; startKey: string }[]
  view: CalendarView
  focusedWeekIndex: number
  currentWeekIndex: number
}>()

const emit = defineEmits<{ select: [index: number]; month: [] }>()
</script>

<template>
  <nav class="calendar-weekrail" aria-label="Navegação do calendário">
    <button
      type="button"
      class="calendar-weekrail__item calendar-weekrail__item--month"
      :class="{ 'calendar-weekrail__item--active': view === 'month' }"
      :aria-current="view === 'month' ? 'true' : undefined"
      title="Visão Mês"
      @click="emit('month')"
    >
      M
    </button>

    <button
      v-for="week in weeks"
      :key="week.index"
      type="button"
      class="calendar-weekrail__item"
      :class="{
        'calendar-weekrail__item--active': view === 'week' && week.index === focusedWeekIndex,
        'calendar-weekrail__item--today': week.index === currentWeekIndex,
      }"
      :aria-current="view === 'week' && week.index === focusedWeekIndex ? 'true' : undefined"
      :title="`Semana ${week.index + 1}`"
      @click="emit('select', week.index)"
    >
      {{ week.label }}
    </button>
  </nav>
</template>
