import { describe, expect, it, vi } from 'vitest'

import {
  getMetaAdsActionPolicy,
  normalizeMetaAdsActionPolicy,
  saveMetaAdsActionPolicy,
} from './meta-ads-policy-api'

describe('Meta Ads action policy API', () => {
  it('normalizes the fail-closed policy view', () => {
    expect(
      normalizeMetaAdsActionPolicy({
        configured: true,
        adAccountId: ' ad/one ',
        currency: 'brl-extra',
        maxDailyBudget: '150.50',
        maxLifetimeBudget: -1,
        allowCreate: true,
      }),
    ).toEqual({
      configured: true,
      adAccountId: 'ad/one',
      currency: 'BRL',
      maxDailyBudget: 150.5,
      maxLifetimeBudget: null,
      allowCreate: true,
      allowDuplicate: false,
      allowResume: false,
      updatedAt: '',
    })
  })

  it('uses the account-scoped canonical GET and PUT endpoints', async () => {
    const api = vi.fn().mockResolvedValue({ configured: false })
    await getMetaAdsActionPolicy(api, 'ad/account')
    await saveMetaAdsActionPolicy(api, 'ad/account', {
      maxDailyBudget: 100,
      maxLifetimeBudget: null,
      allowCreate: false,
      allowDuplicate: false,
      allowResume: false,
    })

    expect(api.mock.calls[0]?.[0]).toBe('/v1/meta-ads/ad-accounts/ad%2Faccount/action-policy')
    expect(api.mock.calls[1]).toEqual([
      '/v1/meta-ads/ad-accounts/ad%2Faccount/action-policy',
      {
        method: 'PUT',
        body: {
          maxDailyBudget: 100,
          maxLifetimeBudget: null,
          allowCreate: false,
          allowDuplicate: false,
          allowResume: false,
        },
      },
    ])
  })
})
