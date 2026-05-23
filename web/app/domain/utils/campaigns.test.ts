import { describe, expect, it } from 'vitest'

import { applyCampaignsToHistoryEntry } from './campaigns'

describe('campaign utils', () => {
  it('matches active campaigns and computes the total bonus from fixed plus percentage rules', () => {
    const result = applyCampaignsToHistoryEntry(
      [
        {
          id: 'camp-1',
          name: 'Fechamento premium',
          startsAt: '2026-05-01',
          endsAt: '2026-05-31',
          targetOutcome: 'compra',
          minSaleAmount: 300,
          maxServiceMinutes: 30,
          productCodes: [' ab-1 ', 'AB-1'],
          sourceIds: ['instagram'],
          reasonIds: ['anel'],
          queueJumpOnly: true,
          existingCustomerFilter: 'no',
          bonusFixed: 10,
          bonusRate: 0.1,
        },
      ],
      {
        finishedAt: Date.parse('2026-05-21T12:00:00Z'),
        durationMs: 20 * 60 * 1000,
        saleAmount: 500,
        customerSources: ['instagram'],
        visitReasons: ['anel'],
        productsClosed: [{ code: 'ab-1' }],
        startMode: 'queue-jump',
        finishOutcome: 'compra',
        isExistingCustomer: false,
      },
    )

    expect(result.matches).toEqual([
      {
        campaignId: 'camp-1',
        campaignName: 'Fechamento premium',
        matchedProductCodes: ['AB-1'],
        bonusValue: 60,
      },
    ])
    expect(result.totalBonus).toBe(60)
  })
})