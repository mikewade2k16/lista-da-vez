import type { PerformanceFeedbackMetrics } from '~/types/performance-feedback'

export interface PerformanceFeedbackCardMetrics {
  soldValue: number
  attendances: number
  conversions: number
  nonConversions: number
  conversionRate: number
  ticketAverage: number
  paScore: number
  qualityScore: number
  avgDurationMs: number
  nonClientConversions: number
  queueJumpServices: number
  salesGoal: number
  ticketGoal: number
  conversionGoal: number
  paGoal: number
  cancellationRate?: number
  erpOrders?: number
  soldValueSource?: string
  ticketAverageSource?: string
  paScoreSource?: string
}

export function performanceFeedbackMetricsFromCard(
  source: PerformanceFeedbackCardMetrics,
): PerformanceFeedbackMetrics {
  return {
    ...source,
    transcriptionSamples: 0,
  }
}
