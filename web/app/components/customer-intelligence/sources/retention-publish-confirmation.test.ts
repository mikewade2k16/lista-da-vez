import { describe, expect, it } from 'vitest'
import { isRetentionPublishConfirmed } from './retention-publish-confirmation'

describe('retention publish confirmation', () => {
  it('accepts only the explicit confirmed result', () => {
    expect(isRetentionPublishConfirmed({ confirmed: true, value: '' })).toBe(true)
    expect(isRetentionPublishConfirmed({ confirmed: false, value: '' })).toBe(false)
    expect(isRetentionPublishConfirmed({ confirmed: 'true' })).toBe(false)
    expect(isRetentionPublishConfirmed(null)).toBe(false)
    expect(isRetentionPublishConfirmed('confirmed')).toBe(false)
  })
})
