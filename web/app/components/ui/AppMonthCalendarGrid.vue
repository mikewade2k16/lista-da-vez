<script setup lang="ts">
import { computed } from 'vue'
import AppCalendarSurface from './AppCalendarSurface.vue'
import { buildMonthMatrix, type DayCellModel, type WeekStart } from '~/utils/calendar'

const props = withDefaults(
  defineProps<{
    monthKey: string
    title: string
    weekdays: string[]
    weekStartsOn?: WeekStart
    current?: boolean
    focus?: boolean
    compact?: boolean
    visibleDays?: DayCellModel[]
  }>(),
  { weekStartsOn: 'sunday', current: false, focus: false, compact: false, visibleDays: undefined },
)
const days = computed(
  () => props.visibleDays || buildMonthMatrix(props.monthKey, props.weekStartsOn).flat(),
)
defineSlots<{
  default(): unknown
  day(props: { day: DayCellModel; index: number }): unknown
  overlay(): unknown
  'header-actions'(): unknown
  'before-grid'(): unknown
  'after-grid'(): unknown
}>()
</script>
<template>
  <AppCalendarSurface
    :title="title"
    :tag="current ? 'Mês atual' : ''"
    :focus="focus"
    :compact="compact"
  >
    <template #header-actions><slot name="header-actions"></slot></template>
    <slot></slot>
    <slot name="before-grid"></slot>
    <div class="app-month-calendar__weekdays">
      <span v-for="label in weekdays" :key="label">{{ label }}</span>
    </div>
    <div class="app-month-calendar__days" role="grid" :aria-label="title">
      <slot
        v-for="(day, index) in days"
        :key="day.dateKey"
        name="day"
        :day="day"
        :index="index"
      ></slot>
      <slot name="overlay"></slot>
    </div>
    <slot name="after-grid"></slot>
  </AppCalendarSurface>
</template>
<style scoped>
.app-month-calendar__weekdays,
.app-month-calendar__days {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
}
.app-month-calendar__weekdays {
  padding: 0.55rem 0.7rem 0.25rem;
  color: var(--text-muted);
  font-size: 0.64rem;
  font-weight: 800;
  text-transform: uppercase;
}
.app-month-calendar__weekdays span {
  padding-inline: 0.35rem;
}
.app-month-calendar__days {
  position: relative;
  gap: 0.35rem;
  padding: 0.35rem 0.7rem 0.7rem;
}
@media (max-width: 800px) {
  .app-month-calendar__weekdays,
  .app-month-calendar__days {
    grid-template-columns: repeat(7, 130px);
    min-width: 910px;
  }
}
</style>
