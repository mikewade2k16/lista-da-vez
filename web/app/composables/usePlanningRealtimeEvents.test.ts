import { describe, expect, it, vi } from 'vitest'

import {
  isPlanningRealtimeEchoSuppressed,
  suppressPlanningRealtimeEcho,
} from './usePlanningRealtimeEvents'

describe('planning realtime local echo suppression', () => {
  it('suppresses only the edited store during the local mutation window', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1_000)
    suppressPlanningRealtimeEcho('store-a', 2_000)

    expect(isPlanningRealtimeEchoSuppressed('store-a', 2_999)).toBe(true)
    expect(isPlanningRealtimeEchoSuppressed('store-b', 2_999)).toBe(false)
    expect(isPlanningRealtimeEchoSuppressed('store-a', 3_000)).toBe(false)

    vi.restoreAllMocks()
  })
})
