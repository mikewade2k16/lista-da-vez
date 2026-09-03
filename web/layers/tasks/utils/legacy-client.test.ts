import { describe, expect, it } from 'vitest'

import { legacyTaskClientLabel } from './legacy-client'

describe('legacyTaskClientLabel', () => {
  it('replaces the old numeric IDs with their canonical names', () => {
    expect(legacyTaskClientLabel('106', 'Cliente 106')).toBe('Crow Visuals')
    expect(legacyTaskClientLabel(10, 'Mostarda')).toBe('Mostarda')
    expect(legacyTaskClientLabel('2', 'Cliente #2')).toBe('Dr Antonio Tavares')
  })

  it('never presents an unknown numeric ID as a client name', () => {
    expect(legacyTaskClientLabel('999', 'Cliente 999')).toBe('')
    expect(legacyTaskClientLabel('999', 'Nome humano')).toBe('Nome humano')
  })

  it('keeps the persisted label for real account UUIDs', () => {
    expect(legacyTaskClientLabel('client-account-uuid', 'Katarina')).toBe('Katarina')
  })
})
