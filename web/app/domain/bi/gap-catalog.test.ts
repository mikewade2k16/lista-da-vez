import { describe, expect, it } from 'vitest'

import { BI_ERP_API_GAPS, filterBiGaps } from './gap-catalog'

describe('ERP x API gap catalog', () => {
  it('keeps every supplier request uniquely identified and prioritized', () => {
    expect(BI_ERP_API_GAPS).toHaveLength(18)
    expect(new Set(BI_ERP_API_GAPS.map((gap) => gap.id)).size).toBe(BI_ERP_API_GAPS.length)
    expect(BI_ERP_API_GAPS.filter((gap) => gap.priority === 'P0')).toHaveLength(8)
  })

  it('filters by priority, domain and normalized text', () => {
    expect(filterBiGaps(BI_ERP_API_GAPS, { priority: 'P0' })).toHaveLength(8)
    expect(filterBiGaps(BI_ERP_API_GAPS, { domain: 'customer' })).toHaveLength(3)
    expect(filterBiGaps(BI_ERP_API_GAPS, { search: 'PAGAMENTO' })[0]?.id).toBe('payment-method')
  })
})
