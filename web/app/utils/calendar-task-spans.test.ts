import { describe, expect, it } from 'vitest'

import { hasTaskSpanInMonth, type CalendarTaskSpan } from './calendar-task-spans'

const span = (startKey: string, endKey: string): CalendarTaskSpan => ({
  id: `${startKey}-${endKey}`,
  title: 'Tarefa',
  clientId: 'client-1',
  startKey,
  endKey,
})

describe('hasTaskSpanInMonth', () => {
  it('detects tasks inside or crossing the focused month', () => {
    expect(hasTaskSpanInMonth([span('2026-07-03', '2026-07-08')], '2026-07')).toBe(true)
    expect(hasTaskSpanInMonth([span('2026-06-28', '2026-07-02')], '2026-07')).toBe(true)
    expect(hasTaskSpanInMonth([span('2026-07-30', '2026-08-04')], '2026-07')).toBe(true)
  })

  it('ignores tasks outside the focused month and invalid month keys', () => {
    expect(hasTaskSpanInMonth([span('2026-08-01', '2026-08-04')], '2026-07')).toBe(false)
    expect(hasTaskSpanInMonth([span('2026-07-03', '2026-07-08')], '')).toBe(false)
  })
})
