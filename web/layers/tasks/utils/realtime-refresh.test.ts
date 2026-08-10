import { describe, expect, it } from 'vitest'
import { isTaskUpdatedEventAlreadyApplied } from './realtime-refresh'

describe('isTaskUpdatedEventAlreadyApplied', () => {
  it('descarta somente task.updated cuja versao ja esta no store', () => {
    expect(isTaskUpdatedEventAlreadyApplied({ type: 'task.updated', version: 7 }, 7)).toBe(true)
    expect(isTaskUpdatedEventAlreadyApplied({ type: 'task.updated', version: 7 }, 8)).toBe(true)
    expect(isTaskUpdatedEventAlreadyApplied({ type: 'task.updated', version: 8 }, 7)).toBe(false)
  })

  it('mantem eventos sem versao e outros tipos no fluxo de refresh', () => {
    expect(isTaskUpdatedEventAlreadyApplied({ type: 'task.updated' }, 7)).toBe(false)
    expect(isTaskUpdatedEventAlreadyApplied({ type: 'task.created', version: 7 }, 7)).toBe(false)
  })
})
