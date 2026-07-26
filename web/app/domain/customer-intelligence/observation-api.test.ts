import { describe, expect, it, vi } from 'vitest'
import { fetchRelationshipObservations } from './observation-api'

describe('customer intelligence observation api', () => {
  it('keeps client scope and source filters in the request', async () => {
    const api = vi.fn().mockResolvedValue([])
    await fetchRelationshipObservations(api as never, 'relationship/with spaces', 'client-1', [
      'erp',
      'calendar',
      'erp',
    ])

    expect(api).toHaveBeenCalledTimes(1)
    const [url, options] = api.mock.calls[0]!
    expect(url).toContain(
      '/v1/customer-intelligence/relationships/relationship%2Fwith%20spaces/observations?',
    )
    const query = new URL(String(url), 'https://omni.invalid').searchParams
    expect(query.get('clientAccountId')).toBe('client-1')
    expect(query.getAll('sourceKey')).toEqual(['erp', 'calendar'])
    expect(query.get('limit')).toBe('100')
    expect(options).toEqual({ signal: undefined, dedupe: false })
  })

  it('normalizes a non-array response to an empty collection', async () => {
    const api = vi.fn().mockResolvedValue({ items: [] })
    await expect(
      fetchRelationshipObservations(api as never, 'relationship-1', 'client-1'),
    ).resolves.toEqual([])
  })
})
