import { describe, expect, it } from 'vitest'

import { buildCrmListUsageSummary, crmListUsageStatus } from './crm-list-usage'

describe('crm list usage utils', () => {
  it('measures list usage as consultant coverage instead of attendance/order volume', () => {
    const rows = [
      ...Array.from({ length: 10 }, () => ({
        orders: 4,
        queue: { attendances: 4 },
        storeSlug: 'riomar',
        storeLabel: 'Riomar',
      })),
      ...Array.from({ length: 90 }, () => ({
        orders: 3,
        queue: null,
        storeSlug: 'jardins',
        storeLabel: 'Jardins',
      })),
    ]

    const summary = buildCrmListUsageSummary(rows)

    expect(summary.totalConsultants).toBe(100)
    expect(summary.coveredConsultants).toBe(10)
    expect(summary.usageRate).toBe(10)
    expect(summary.bestStore?.storeSlug).toBe('riomar')
    expect(summary.hasPositiveStoreHighlight).toBe(true)
    expect(summary.worstStore?.storeSlug).toBe('jardins')
  })

  it('does not mark a best store or consultant as positive when every rate is bad', () => {
    const summary = buildCrmListUsageSummary(
      [
        {
          consultantName: 'A',
          orders: 10,
          queue: { attendances: 4 },
          storeSlug: 'riomar',
          storeLabel: 'Riomar',
        },
        {
          consultantName: 'B',
          orders: 10,
          queue: { attendances: 0 },
          storeSlug: 'jardins',
          storeLabel: 'Jardins',
        },
      ],
      { minOrdersForHighlight: 5 },
    )

    expect(summary.bestStore?.storeSlug).toBe('riomar')
    expect(summary.bestStore?.usageRate).toBe(40)
    expect(summary.hasPositiveStoreHighlight).toBe(false)
    expect(summary.bestConsultant?.consultantName).toBe('A')
    expect(summary.bestConsultant?.usageRate).toBe(40)
    expect(summary.hasPositiveConsultantHighlight).toBe(false)
  })

  it('caps consultant coverage percentage at 100 percent', () => {
    const summary = buildCrmListUsageSummary([
      {
        consultantName: 'A',
        orders: 4,
        queue: { attendances: 9 },
        storeSlug: 'riomar',
        storeLabel: 'Riomar',
      },
    ])

    expect(summary.bestConsultant?.usageRate).toBe(100)
  })

  it('classifies consultant rows by coverage status', () => {
    expect(crmListUsageStatus({ orders: 2, queue: { attendances: 3 } })).toBe('covered')
    expect(crmListUsageStatus({ orders: 4, queue: { attendances: 2 } })).toBe('partial')
    expect(crmListUsageStatus({ orders: 4, queue: null })).toBe('unused')
    expect(crmListUsageStatus({ orders: 0, queue: { attendances: 2 } })).toBe('no-sales')
  })
})
