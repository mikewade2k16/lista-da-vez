import { describe, expect, it } from 'vitest'

import { buildReportData, buildReportRowsFromApi, normalizeReportFilters } from './reports'

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

describe('normalizeReportFilters', () => {
  it('returns defaults for an empty filter object', () => {
    expect(normalizeReportFilters({})).toEqual({
      dateFrom: '',
      dateTo: '',
      consultantIds: [],
      outcomes: [],
      sourceIds: [],
      visitReasonIds: [],
      startModes: [],
      existingCustomerModes: [],
      completionLevels: [],
      campaignIds: [],
      minSaleAmount: '',
      maxSaleAmount: '',
      search: '',
    })
  })

  it('promotes legacy singular filters to arrays', () => {
    const normalized = normalizeReportFilters({ consultantId: 'c1', outcome: 'compra' })
    expect(normalized.consultantIds).toEqual(['c1'])
    expect(normalized.outcomes).toEqual(['compra'])
  })

  it('passes plural array filters through', () => {
    expect(normalizeReportFilters({ outcomes: ['reserva'] }).outcomes).toEqual(['reserva'])
  })
})

describe('buildReportData aggregation', () => {
  it('returns empty rows and zeroed metrics for no history', () => {
    const report = buildReportData({})
    expect(report.rows).toEqual([])
    expect(report.metrics.totalAttendances).toBe(0)
    expect(report.metrics.conversions).toBe(0)
    expect(report.metrics.conversionRate).toBe(0)
    expect(report.metrics.soldValue).toBe(0)
  })

  it('sorts rows by finishedAt descending', () => {
    const report = buildReportData({
      history: [
        { serviceId: 'early', finishedAt: Date.parse('2026-05-21T10:00:00') },
        { serviceId: 'late', finishedAt: Date.parse('2026-05-21T15:00:00') },
      ],
    })
    expect(report.rows[0].serviceId).toBe('late')
  })

  it('counts a reserva as a conversion and sums its sale amount', () => {
    const report = buildReportData({
      history: [{ finishOutcome: 'reserva', saleAmount: 100 }],
    })
    expect(report.metrics.conversions).toBe(1)
    expect(report.metrics.soldValue).toBe(100)
  })

  it('does not add sale amount for a non-purchase outcome', () => {
    const report = buildReportData({
      history: [{ finishOutcome: 'nao-compra', saleAmount: 999 }],
    })
    expect(report.metrics.soldValue).toBe(0)
  })

  it('fills row fallbacks for a sparse entry', () => {
    const report = buildReportData({
      history: [{ finishedAt: Date.parse('2026-05-21T10:00:00') }],
    })
    expect(report.rows[0]).toEqual(
      expect.objectContaining({
        outcome: 'nao-compra',
        outcomeLabel: 'Nao compra',
        startModeLabel: 'Na vez',
        saleAmount: 0,
      }),
    )
  })

  it('falls back to the raw id when there is no matching label option', () => {
    const report = buildReportData({
      history: [{ finishedAt: Date.parse('2026-05-21T10:00:00'), visitReasons: ['vr-x'] }],
    })
    expect(report.rows[0].visitReasonsLabel).toBe('vr-x')
  })

  it('excludes rows filtered out by outcome or search', () => {
    const byOutcome = buildReportData({
      history: [{ finishOutcome: 'nao-compra', finishedAt: Date.parse('2026-05-21T10:00:00') }],
      filters: { outcomes: ['compra'] },
    })
    expect(byOutcome.rows).toEqual([])

    const bySearch = buildReportData({
      history: [{ finishedAt: Date.parse('2026-05-21T10:00:00'), customerName: 'Ana' }],
      filters: { search: 'zzz' },
    })
    expect(bySearch.rows).toEqual([])
  })
})

describe('buildReportRowsFromApi', () => {
  it('resolves visit reason labels from the provided options', () => {
    const rows = buildReportRowsFromApi(
      [{ visitReasons: ['vr-1'] }],
      [{ id: 'vr-1', label: 'Noiva' }],
    )
    expect(rows[0].visitReasonsLabel).toBe('Noiva')
  })
})
