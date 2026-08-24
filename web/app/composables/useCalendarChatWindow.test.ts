import { describe, expect, it } from 'vitest'

import { shouldPersistAssistantWindowLayout } from './useCalendarChatWindow'

describe('assistant window layout persistence', () => {
  it('persists only an already hydrated Calendar layout', () => {
    expect(shouldPersistAssistantWindowLayout('calendar', true)).toBe(true)
    expect(shouldPersistAssistantWindowLayout('calendar', false)).toBe(false)
    expect(shouldPersistAssistantWindowLayout('meta_ads', true)).toBe(false)
    expect(shouldPersistAssistantWindowLayout('global', true)).toBe(false)
  })
})
