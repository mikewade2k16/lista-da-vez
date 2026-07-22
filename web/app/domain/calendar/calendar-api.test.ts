import { describe, expect, it, vi } from 'vitest'

import {
  fetchEventsInRange,
  fetchScope,
  normalizeCalendarScope,
  type ApiRequest,
} from './calendar-api'

describe('calendar api scope', () => {
  it('normalizes the authoritative client scope defensively', () => {
    expect(
      normalizeCalendarScope({
        canSelect: false,
        lockedClientId: ' client-1 ',
        clients: [
          { id: ' client-1 ', name: ' Dr Lucas ' },
          { id: 'client-1', name: 'Duplicado' },
          { id: '', name: 'Invalido' },
        ],
      }),
    ).toEqual({
      canSelect: false,
      lockedClientId: 'client-1',
      clients: [{ id: 'client-1', name: 'Dr Lucas' }],
    })
  })

  it('loads scope from the canonical endpoint', async () => {
    const api = vi.fn().mockResolvedValue({ canSelect: true, clients: [] })

    await expect(fetchScope(api as ApiRequest)).resolves.toEqual({
      canSelect: true,
      lockedClientId: '',
      clients: [],
    })
    expect(api).toHaveBeenCalledWith('/v1/calendar/scope')
  })

  it('adds the effective client to the events query only when present', async () => {
    const api = vi.fn().mockResolvedValue({ events: [] })

    await fetchEventsInRange(api as ApiRequest, '2026-07-01', '2026-07-31', 'client-1')
    await fetchEventsInRange(api as ApiRequest, '2026-07-01', '2026-07-31')

    expect(api.mock.calls[0]?.[0]).toBe(
      '/v1/calendar/events?from=2026-07-01&to=2026-07-31&clientId=client-1',
    )
    expect(api.mock.calls[1]?.[0]).toBe('/v1/calendar/events?from=2026-07-01&to=2026-07-31')
  })
})
