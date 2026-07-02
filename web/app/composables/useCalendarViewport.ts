import { computed, ref } from 'vue'

import {
  addDaysToKey,
  addMonthsToKey,
  addWeeksToKey,
  buildMonthMatrix,
  formatMonthTitle,
  monthKeyOf,
  monthKeyWindow,
  startOfWeekKey,
  todayKey,
  weekStartWindow,
  type CalendarView,
  type WeekStart,
} from '~/utils/calendar'

const RENDER_PAD = 4
const EXTEND_STEP = 3

// useCalendarViewport concentra o "onde estou olhando" do calendario: modo
// (mes/semana), bloco em foco, janela renderizada (scroll infinito), week rail e
// navegacao. Fica separado do store (stores/calendar.ts, que cuida de dados +
// estado de UI) para manter cada arquivo < 450 linhas. Os refs retornados sao os
// mesmos objetos usados internamente, entao a reatividade atravessa o store sem
// perda; o store apenas re-exporta o que os componentes consomem.
export function useCalendarViewport() {
  const weekStartsOn = ref<WeekStart>('sunday')
  const view = ref<CalendarView>('month')

  const today = todayKey()
  const currentMonthKey = computed(() => monthKeyOf(todayKey()))
  const currentWeekKey = computed(() => startOfWeekKey(todayKey(), weekStartsOn.value))

  const focusMonthKey = ref(monthKeyOf(today))
  const focusWeekKey = ref(startOfWeekKey(today, 'sunday'))

  const renderedMonthKeys = ref<string[]>(
    monthKeyWindow(focusMonthKey.value, RENDER_PAD, RENDER_PAD),
  )
  const renderedWeekKeys = ref<string[]>(
    weekStartWindow(focusWeekKey.value, RENDER_PAD, RENDER_PAD),
  )

  const periodTitle = computed(() => formatMonthTitle(focusMonthKey.value))

  const weeksOfFocusedMonth = computed(() =>
    buildMonthMatrix(focusMonthKey.value, weekStartsOn.value)
      .filter((week) => week.filter((cell) => cell.inMonth).length >= 2)
      .map((week, index) => ({
        index,
        label: `S${index + 1}`,
        startKey: week[0]?.dateKey || '',
      })),
  )

  const railActiveIndex = computed(() => {
    if (view.value === 'week') {
      return weeksOfFocusedMonth.value.findIndex((week) => week.startKey === focusWeekKey.value)
    }
    if (focusMonthKey.value === currentMonthKey.value) {
      return weeksOfFocusedMonth.value.findIndex((week) => week.startKey === currentWeekKey.value)
    }
    return -1
  })

  const currentRailIndex = computed(() =>
    focusMonthKey.value === currentMonthKey.value
      ? weeksOfFocusedMonth.value.findIndex((week) => week.startKey === currentWeekKey.value)
      : -1,
  )

  // --- Scroll infinito ----------------------------------------------------------
  function prependMonths(n = EXTEND_STEP): void {
    const first = renderedMonthKeys.value[0]
    if (!first) return
    const added: string[] = []
    for (let i = n; i >= 1; i -= 1) added.push(addMonthsToKey(first, -i))
    renderedMonthKeys.value = [...added, ...renderedMonthKeys.value]
  }

  function appendMonths(n = EXTEND_STEP): void {
    const last = renderedMonthKeys.value[renderedMonthKeys.value.length - 1]
    if (!last) return
    const added: string[] = []
    for (let i = 1; i <= n; i += 1) added.push(addMonthsToKey(last, i))
    renderedMonthKeys.value = [...renderedMonthKeys.value, ...added]
  }

  function prependWeeks(n = EXTEND_STEP): void {
    const first = renderedWeekKeys.value[0]
    if (!first) return
    const added: string[] = []
    for (let i = n; i >= 1; i -= 1) added.push(addWeeksToKey(first, -i))
    renderedWeekKeys.value = [...added, ...renderedWeekKeys.value]
  }

  function appendWeeks(n = EXTEND_STEP): void {
    const last = renderedWeekKeys.value[renderedWeekKeys.value.length - 1]
    if (!last) return
    const added: string[] = []
    for (let i = 1; i <= n; i += 1) added.push(addWeeksToKey(last, i))
    renderedWeekKeys.value = [...renderedWeekKeys.value, ...added]
  }

  function ensureMonthRendered(monthKey: string): void {
    if (renderedMonthKeys.value.includes(monthKey)) return
    renderedMonthKeys.value = monthKeyWindow(monthKey, RENDER_PAD, RENDER_PAD)
  }

  function ensureWeekRendered(weekKey: string): void {
    if (renderedWeekKeys.value.includes(weekKey)) return
    renderedWeekKeys.value = weekStartWindow(weekKey, RENDER_PAD, RENDER_PAD)
  }

  // --- Foco / navegacao ---------------------------------------------------------
  function setFocusMonth(monthKey: string): void {
    if (monthKey) focusMonthKey.value = monthKey
  }

  function setFocusWeek(weekKey: string): void {
    if (!weekKey) return
    focusWeekKey.value = weekKey
    focusMonthKey.value = monthKeyOf(addDaysToKey(weekKey, 3))
  }

  function setView(next: CalendarView): void {
    if (next === view.value) return
    view.value = next
    if (next === 'week') {
      const weeks = weeksOfFocusedMonth.value
      const target =
        weeks.find((week) => week.startKey === currentWeekKey.value)?.startKey ||
        weeks[0]?.startKey ||
        focusWeekKey.value
      focusWeekKey.value = target
      renderedWeekKeys.value = weekStartWindow(target, RENDER_PAD, RENDER_PAD)
    } else {
      focusMonthKey.value = monthKeyOf(addDaysToKey(focusWeekKey.value, 3))
      renderedMonthKeys.value = monthKeyWindow(focusMonthKey.value, RENDER_PAD, RENDER_PAD)
    }
  }

  function goToday(): void {
    if (view.value === 'week') {
      focusWeekKey.value = currentWeekKey.value
      focusMonthKey.value = currentMonthKey.value
      renderedWeekKeys.value = weekStartWindow(currentWeekKey.value, RENDER_PAD, RENDER_PAD)
    } else {
      focusMonthKey.value = currentMonthKey.value
      renderedMonthKeys.value = monthKeyWindow(currentMonthKey.value, RENDER_PAD, RENDER_PAD)
    }
  }

  function goPrev(): void {
    if (view.value === 'week') {
      setFocusWeek(addWeeksToKey(focusWeekKey.value, -1))
      ensureWeekRendered(focusWeekKey.value)
    } else {
      const monthKey = addMonthsToKey(focusMonthKey.value, -1)
      focusMonthKey.value = monthKey
      ensureMonthRendered(monthKey)
    }
  }

  function goNext(): void {
    if (view.value === 'week') {
      setFocusWeek(addWeeksToKey(focusWeekKey.value, 1))
      ensureWeekRendered(focusWeekKey.value)
    } else {
      const monthKey = addMonthsToKey(focusMonthKey.value, 1)
      focusMonthKey.value = monthKey
      ensureMonthRendered(monthKey)
    }
  }

  function selectWeek(weekIndex: number): void {
    const week = weeksOfFocusedMonth.value[weekIndex]
    if (!week) return
    view.value = 'week'
    focusWeekKey.value = week.startKey
    ensureWeekRendered(week.startKey)
  }

  return {
    weekStartsOn,
    view,
    currentMonthKey,
    currentWeekKey,
    focusMonthKey,
    focusWeekKey,
    renderedMonthKeys,
    renderedWeekKeys,
    periodTitle,
    weeksOfFocusedMonth,
    railActiveIndex,
    currentRailIndex,
    prependMonths,
    appendMonths,
    prependWeeks,
    appendWeeks,
    ensureMonthRendered,
    ensureWeekRendered,
    setFocusMonth,
    setFocusWeek,
    setView,
    goToday,
    goPrev,
    goNext,
    selectWeek,
  }
}
