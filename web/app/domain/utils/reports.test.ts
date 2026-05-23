import { describe, expect, it } from 'vitest'

import { buildReportData } from './reports'

describe('reports utils', () => {
  it('normalizes legacy filters and produces report rows and metrics from filtered history', () => {
    const report = buildReportData({
      history: [
        {
          serviceId: 'svc-1',
          storeId: 'store-1',
          storeName: 'Centro',
          personId: 'consultant-1',
          personName: 'Ana',
          finishedAt: Date.parse('2026-05-21T14:30:00Z'),
          finishOutcome: 'compra',
          saleAmount: 450,
          durationMs: 30 * 60 * 1000,
          queueWaitMs: 5 * 60 * 1000,
          startMode: 'queue',
          isExistingCustomer: true,
          customerName: 'Ana Maria',
          customerPhone: '11999999999',
          customerEmail: 'ana@example.com',
          customerProfession: 'Arquiteta',
          productSeen: 'Colar',
          productClosed: 'Colar Ouro',
          visitReasons: ['vr-1'],
          customerSources: ['src-1'],
          notes: 'Cliente decidiu fechar na hora',
          campaignMatches: [{ campaignId: 'camp-1', campaignName: 'Inverno' }],
          campaignBonusTotal: 25,
        },
      ],
      roster: [{ id: 'consultant-1', name: 'Ana' }],
      visitReasonOptions: [{ id: 'vr-1', label: 'Noiva' }],
      customerSourceOptions: [{ id: 'src-1', label: 'Instagram' }],
      filters: {
        consultantId: 'consultant-1',
        outcome: 'compra',
        search: 'Ana Maria',
      },
    })

    expect(report.filters.consultantIds).toEqual(['consultant-1'])
    expect(report.filters.outcomes).toEqual(['compra'])
    expect(report.rows).toHaveLength(1)
    expect(report.rows[0]).toEqual(
      expect.objectContaining({
        consultantName: 'Ana',
        visitReasonsLabel: 'Noiva',
        customerSourcesLabel: 'Instagram',
      }),
    )
    expect(report.metrics.totalAttendances).toBe(1)
    expect(report.metrics.conversions).toBe(1)
  })
})