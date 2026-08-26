import { describe, expect, it } from 'vitest'
import { normalizeTaskSourceBoardIds, normalizeTaskSourceMode } from './task-source'

const CURRENT = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
const SOURCE = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'

describe('tasks page sources', () => {
  it('defaults new pages to their own tasks only', () => {
    expect(normalizeTaskSourceMode(undefined)).toBe('own')
    expect(normalizeTaskSourceMode('invalid')).toBe('own')
    expect(normalizeTaskSourceMode(' ALL ')).toBe('all')
    expect(normalizeTaskSourceMode('selected')).toBe('selected')
  })

  it('normalizes sources and removes the current page', () => {
    expect(
      normalizeTaskSourceBoardIds([SOURCE.toUpperCase(), SOURCE, CURRENT, 'invalid'], CURRENT),
    ).toEqual([SOURCE])
  })
})
