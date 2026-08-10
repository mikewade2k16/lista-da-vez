import { describe, expect, it } from 'vitest'

import { performanceFeedbackMetricsFromCard } from './performance-feedback'

describe('performanceFeedbackMetricsFromCard', () => {
  it.each([
    {
      name: 'consultora com dados ERP',
      source: {
        soldValue: 14725,
        attendances: 11,
        conversions: 10,
        nonConversions: 1,
        conversionRate: 90.91,
        ticketAverage: 2454.16,
        paScore: 1,
        qualityScore: 0,
        avgDurationMs: 16_376_400,
        nonClientConversions: 10,
        queueJumpServices: 3,
        salesGoal: 33333.33,
        ticketGoal: 0,
        conversionGoal: 0,
        paGoal: 0,
        cancellationRate: 0,
        erpOrders: 6,
        soldValueSource: 'erp',
        ticketAverageSource: 'erp',
        paScoreSource: 'erp',
      },
    },
    {
      name: 'consultora com período sem vendas',
      source: {
        soldValue: 0,
        attendances: 4,
        conversions: 0,
        nonConversions: 4,
        conversionRate: 0,
        ticketAverage: 0,
        paScore: 0,
        qualityScore: 50,
        avgDurationMs: 600_000,
        nonClientConversions: 0,
        queueJumpServices: 0,
        salesGoal: 10000,
        ticketGoal: 500,
        conversionGoal: 30,
        paGoal: 1.2,
      },
    },
  ])('preserva sem recalcular todos os números do card: $name', ({ source }) => {
    const metrics = performanceFeedbackMetricsFromCard(source)

    expect(metrics).toMatchObject(source)
    expect(metrics.transcriptionSamples).toBe(0)
  })
})
