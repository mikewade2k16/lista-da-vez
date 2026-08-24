import { describe, expect, it, vi } from 'vitest'
import type { ApiRequest } from '~/domain/calendar/calendar-api'
import {
  cancelMetaActionProposal,
  confirmMetaActionProposal,
  listMetaActionProposals,
  metaActionCancellationKey,
  metaActionConfirmationKey,
  normalizeMetaAdsActionProposal,
  reconcileMetaActionProposal,
} from './meta-ads-actions-api'

describe('meta ads action proposals API', () => {
  it('usa chave de confirmacao deterministica e acknowledgement somente quando explicito', async () => {
    const api = vi.fn().mockResolvedValue({
      id: 'action-1',
      action: 'update_campaign',
      source: 'assistant',
      status: 'succeeded',
    })
    const key = metaActionConfirmationKey('message-1', 'action-1')

    await confirmMetaActionProposal(api as ApiRequest, 'action-1', key, true)

    expect(key).toBe('assistant-confirm:message-1:action-1')
    expect(api).toHaveBeenCalledWith('/v1/meta-ads/action-proposals/action-1/confirm', {
      method: 'POST',
      headers: { 'Idempotency-Key': key },
      body: { acknowledgeSpend: true },
    })
  })

  it('nao envia acknowledgement sem confirmacao reforcada do usuario', async () => {
    const api = vi.fn().mockResolvedValue({
      id: 'action-2',
      action: 'pause_campaign',
      status: 'succeeded',
    })

    await confirmMetaActionProposal(api as ApiRequest, 'action-2', 'assistant-confirm:key', false)

    expect(api).toHaveBeenCalledWith('/v1/meta-ads/action-proposals/action-2/confirm', {
      method: 'POST',
      headers: { 'Idempotency-Key': 'assistant-confirm:key' },
      body: {},
    })
  })

  it('normaliza status desconhecido como unknown e preserva a flag financeira autoritativa', () => {
    expect(
      normalizeMetaAdsActionProposal({
        id: 'action-3',
        action: 'update_campaign',
        source: 'assistant',
        status: 'invented',
        payload: { budget: { type: 'daily', amount: 50 } },
        result: null,
        requiresSpendAcknowledgement: true,
      }),
    ).toMatchObject({
      id: 'action-3',
      action: 'update_campaign',
      source: 'assistant',
      status: 'unknown',
      requiresSpendAcknowledgement: true,
      payload: { budget: { type: 'daily', amount: 50 } },
      result: {},
    })
  })

  it('preserva a nova ação durável de promoção de post', () => {
    expect(
      normalizeMetaAdsActionProposal({
        id: 'action-post',
        action: 'promote_instagram_post',
        status: 'pending',
        result: { campaignId: '11', adSetId: '22', creativeId: '33', adId: '44' },
      }),
    ).toMatchObject({
      action: 'promote_instagram_post',
      status: 'pending',
      result: { campaignId: '11', adSetId: '22', creativeId: '33', adId: '44' },
    })
  })

  it('preserva estados terminais de cancelamento/expiracao e o vencimento', () => {
    expect(
      normalizeMetaAdsActionProposal({
        id: 'action-3b',
        action: 'pause_campaign',
        status: 'expired',
        expiresAt: '2026-08-18T12:30:00Z',
      }),
    ).toMatchObject({
      status: 'expired',
      expiresAt: '2026-08-18T12:30:00Z',
    })
    expect(
      normalizeMetaAdsActionProposal({ action: 'pause_campaign', status: 'cancelled' }).status,
    ).toBe('cancelled')
  })

  it('cancela com chave deterministica antes de recusar o card', async () => {
    const api = vi.fn().mockResolvedValue({
      id: 'action-5',
      action: 'pause_campaign',
      status: 'cancelled',
    })
    const key = metaActionCancellationKey('message-5', 'action-5')

    await cancelMetaActionProposal(api as ApiRequest, 'action-5', key)

    expect(key).toBe('assistant-cancel:message-5:action-5')
    expect(api).toHaveBeenCalledWith('/v1/meta-ads/action-proposals/action-5/cancel', {
      method: 'POST',
      headers: { 'Idempotency-Key': key },
    })
  })

  it('reconcilia pelo endpoint duravel sem chave de criacao', async () => {
    const api = vi.fn().mockResolvedValue({
      id: 'action-4',
      action: 'pause_campaign',
      status: 'unknown',
    })

    await reconcileMetaActionProposal(api as ApiRequest, 'action-4')

    expect(api).toHaveBeenCalledWith('/v1/meta-ads/action-proposals/action-4/reconcile', {
      method: 'POST',
    })
  })

  it('lista e normaliza a trilha de acoes com limite bounded', async () => {
    const api = vi.fn().mockResolvedValue({
      proposals: [
        { id: 'action-6', action: 'create_campaign', status: 'succeeded' },
        { id: '', action: 'pause_campaign', status: 'failed' },
      ],
    })

    const result = await listMetaActionProposals(api as ApiRequest, 999)

    expect(api).toHaveBeenCalledWith('/v1/meta-ads/action-proposals', {
      query: { limit: 100 },
    })
    expect(result).toHaveLength(1)
    expect(result[0]).toMatchObject({ id: 'action-6', status: 'succeeded' })
  })
})
