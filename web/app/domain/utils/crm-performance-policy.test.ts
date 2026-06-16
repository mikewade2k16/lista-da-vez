import { describe, expect, it } from 'vitest'

import {
  DEFAULT_CRM_GOAL_PAYOUT_POLICY,
  calculateCrmGoalPayout,
  calculateStoreGoalPayout,
  classifyCrmListUsageRate,
  mapRoleToPayoutGroup,
  normalizeCrmGoalPayoutPolicy,
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

  it('maps store role to the payout group', () => {
    expect(mapRoleToPayoutGroup('queue.manager')).toBe('manager')
    expect(mapRoleToPayoutGroup('Gerente')).toBe('manager')
    expect(mapRoleToPayoutGroup('cashier')).toBe('support')
    expect(mapRoleToPayoutGroup('Caixa e auxiliar')).toBe('support')
    expect(mapRoleToPayoutGroup('queue.consultant')).toBe('consultant')
    expect(mapRoleToPayoutGroup('')).toBe('consultant')
  })

  it('calculates store goal payout over the store total for percent rules', () => {
    const result = calculateStoreGoalPayout({
      storeSold: 250_000,
      storeProgress: 120,
      policy: DEFAULT_CRM_GOAL_PAYOUT_POLICY,
      role: 'consultant',
    })

    expect(result.group).toBe('consultant')
    expect(result.rule?.value).toBe(3.2)
    expect(result.amount).toBeCloseTo(8_000)
  })

  it('returns the fixed amount for support and no payout below the lowest threshold', () => {
    const support = calculateStoreGoalPayout({
      storeSold: 250_000,
      storeProgress: 100,
      policy: DEFAULT_CRM_GOAL_PAYOUT_POLICY,
      role: 'caixa',
    })
    expect(support.rule?.mode).toBe('amount')
    expect(support.amount).toBe(100)

    const belowFloor = calculateStoreGoalPayout({
      storeSold: 250_000,
      storeProgress: 50,
      policy: DEFAULT_CRM_GOAL_PAYOUT_POLICY,
      role: 'consultant',
    })
    expect(belowFloor.rule).toBeNull()
    expect(belowFloor.amount).toBe(0)
  })

  it('preserves an explicit empty group but falls back to defaults when absent', () => {
    const policy = normalizeCrmGoalPayoutPolicy({ consultant: [] })
    expect(policy.consultant).toEqual([])
    expect(policy.manager).toEqual(DEFAULT_CRM_GOAL_PAYOUT_POLICY.manager)
  })
})
