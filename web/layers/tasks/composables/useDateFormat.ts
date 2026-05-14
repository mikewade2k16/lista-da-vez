import { ref, watch } from 'vue'

// Module-level singletons — every AppDatePicker instance shares the same format settings.
// Changing format in one picker updates the label on ALL date triggers on the page.
// Settings persist to localStorage so they survive page reloads.

function getStored(key: string, fallback: string): string {
  if (typeof localStorage === 'undefined') return fallback
  return localStorage.getItem(key) || fallback
}

const _dateFormat = ref(getStored('tasks-date-format', 'dmy'))
const _timeFormat = ref<'24h' | '12h'>(getStored('tasks-time-format', '24h') as '24h' | '12h')

if (typeof window !== 'undefined') {
  watch(_dateFormat, v => localStorage.setItem('tasks-date-format', v))
  watch(_timeFormat, v => localStorage.setItem('tasks-time-format', v))
}

export function useDateFormat() {
  return { dateFormat: _dateFormat, timeFormat: _timeFormat }
}
