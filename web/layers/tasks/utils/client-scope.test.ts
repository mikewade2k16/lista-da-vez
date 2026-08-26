import { describe, expect, it } from 'vitest'
import {
  normalizeTaskClientScopeIds,
  normalizeTaskClientScopeMode,
  taskClientVisibleInScope,
} from './client-scope'

const ACTIVE = { value: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', active: true }
const INACTIVE = { value: 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', active: false }

describe('tasks board client scope', () => {
  it('normalizes modes and defaults fail-closed to active', () => {
    expect(normalizeTaskClientScopeMode(' ALL ')).toBe('all')
    expect(normalizeTaskClientScopeMode('selected')).toBe('selected')
    expect(normalizeTaskClientScopeMode('invalid')).toBe('active')
  })

  it('normalizes, validates and deduplicates selected client ids', () => {
    expect(
      normalizeTaskClientScopeIds([` ${ACTIVE.value.toUpperCase()} `, ACTIVE.value, 'invalid']),
    ).toEqual([ACTIVE.value])
  })

  it('applies all, active and selected visibility modes', () => {
    expect(taskClientVisibleInScope(INACTIVE, 'all', [])).toBe(true)
    expect(taskClientVisibleInScope(ACTIVE, 'active', [])).toBe(true)
    expect(taskClientVisibleInScope(INACTIVE, 'active', [])).toBe(false)
    expect(taskClientVisibleInScope(INACTIVE, 'selected', [INACTIVE.value])).toBe(true)
    expect(taskClientVisibleInScope(ACTIVE, 'selected', [INACTIVE.value])).toBe(false)
  })
})
