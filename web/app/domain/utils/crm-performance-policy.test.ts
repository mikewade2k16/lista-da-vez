import { describe, expect, it } from 'vitest'

import {
  DEFAULT_CRM_CONSULTANT_RULES,
  DEFAULT_CRM_GOAL_PAYOUT_POLICY,
  classifyCrmListUsageRate,
  normalizeCrmGoalPayoutPolicy,
} from './crm-performance-policy'

describe('crm performance policy utils', () => {
  it('classifies list usage by configured thresholds', () => {
    expect(classifyCrmListUsageRate(8, null).label).toBe('Pessimo')
    expect(classifyCrmListUsageRate(50, null).label).toBe('Normal')
    expect(classifyCrmListUsageRate(82, null).label).toBe('Otimo')
    expect(classifyCrmListUsageRate(100, null).label).toBe('Perfeito')
  })

  it('preserves an explicit empty group but falls back to defaults when absent', () => {
    const policy = normalizeCrmGoalPayoutPolicy({ consultant: [] })
    expect(policy.consultant).toEqual([])
    expect(policy.managerShopping).toEqual(DEFAULT_CRM_GOAL_PAYOUT_POLICY.managerShopping)
    expect(policy.managerBairro).toEqual(DEFAULT_CRM_GOAL_PAYOUT_POLICY.managerBairro)
    expect(policy.support).toEqual(DEFAULT_CRM_GOAL_PAYOUT_POLICY.support)
  })

  it('seeds both manager groups from the legacy manager array', () => {
    const legacy = [{ threshold: 80, value: 0.8, mode: 'percent' as const }]
    const policy = normalizeCrmGoalPayoutPolicy({ manager: legacy })
    expect(policy.managerShopping).toEqual(legacy)
    expect(policy.managerBairro).toEqual(legacy)
  })

  it('keeps explicit manager groups over the legacy fallback', () => {
    const policy = normalizeCrmGoalPayoutPolicy({
      manager: [{ threshold: 80, value: 0.8, mode: 'percent' }],
      managerShopping: [{ threshold: 100, value: 1, mode: 'percent' }],
      managerBairro: [],
    })
    expect(policy.managerShopping).toEqual([{ threshold: 100, value: 1, mode: 'percent' }])
    expect(policy.managerBairro).toEqual([])
  })

  it('normalizes consultant rules with defaults and overrides', () => {
    const fallback = normalizeCrmGoalPayoutPolicy({})
    expect(fallback.consultantRules).toEqual(DEFAULT_CRM_CONSULTANT_RULES)

    const overridden = normalizeCrmGoalPayoutPolicy({
      consultantRules: {
        base: 'store',
        qualityPenaltyPercent: 0.2,
        storeFloorPercent: 45,
        storeFullPercent: 75,
        reducedRate: 1.2,
        reducedRequiresOwnPercent: 95,
      },
    })
    expect(overridden.consultantRules).toEqual({
      base: 'store',
      qualityPenaltyPercent: 0.2,
      storeFloorPercent: 45,
      storeFullPercent: 75,
      reducedRate: 1.2,
      reducedRequiresOwnPercent: 95,
    })
  })
})
