import { describe, expect, it } from 'vitest'

import {
  DEFAULT_CRM_GOAL_PAYOUT_POLICY,
  calculateCrmGoalPayout,
  classifyCrmListUsageRate,
} from './crm-performance-policy'

describe('crm performance policy utils', () => {
  it('classifies list usage by configured thresholds', () => {
    expect(classifyCrmListUsageRate(8, null).label).toBe('Pessimo')
    expect(classifyCrmListUsageRate(50, null).label).toBe('Normal')
    expect(classifyCrmListUsageRate(82, null).label).toBe('Otimo')
    expect(classifyCrmListUsageRate(100, null).label).toBe('Perfeito')
  })

  it('calculates consultant payout from store goal progress', () => {
    const result = calculateCrmGoalPayout(
      181_336_07,
      120,
      DEFAULT_CRM_GOAL_PAYOUT_POLICY,
      'consultant',
    )

    expect(result.rule?.value).toBe(3.2)
    expect(result.amountCents).toBe(580_275)
  })
})
