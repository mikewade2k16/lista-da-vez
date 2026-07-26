import { describe, expect, it, vi } from 'vitest'
import { fetchSourceSuggestions, reviewSourceSuggestion } from './source-suggestion-api'
import {
  SOURCE_SUGGESTION_REVIEW_REASONS,
  validSourceSuggestionReviewReason,
} from './source-suggestion-types'

const BACKEND_SUGGESTION = {
  id: 'suggestion-1',
  accountId: 'owner-private',
  clientAccountId: 'client-private',
  relationshipId: 'relationship-1',
  sourceKey: 'erp',
  gapCodes: ['purchase_history_missing', 'purchase_history_missing', 'lifetime_value_missing'],
  rationaleCode: 'profile_gap',
  rationale: 'O historico de compras pode completar o perfil.',
  rationaleCiphertext: 'never-expose',
  confidence: 0.84,
  status: 'proposed',
  expiresAt: '2099-08-01T12:00:00Z',
  createdAt: '2026-07-23T12:00:00Z',
}

describe('customer intelligence source suggestion HTTP contracts', () => {
  it('lists with encoded relationship, explicit client scope and bounded limit', async () => {
    const api = vi.fn().mockResolvedValue([BACKEND_SUGGESTION])
    const controller = new AbortController()

    const items = await fetchSourceSuggestions(
      api as never,
      'relationship/with spaces',
      ' client-1 ',
      controller.signal,
    )

    expect(api).toHaveBeenCalledWith(
      '/v1/customer-intelligence/relationships/relationship%2Fwith%20spaces/source-suggestions?clientAccountId=client-1&limit=50',
      { signal: controller.signal, dedupe: false },
    )
    expect(items).toEqual([
      {
        id: 'suggestion-1',
        relationshipId: 'relationship-1',
        sourceKey: 'erp',
        gapCodes: ['purchase_history_missing', 'lifetime_value_missing'],
        rationaleCode: 'profile_gap',
        rationale: 'O historico de compras pode completar o perfil.',
        confidence: 0.84,
        status: 'proposed',
        expiresAt: '2099-08-01T12:00:00Z',
        createdAt: '2026-07-23T12:00:00Z',
        allowedActions: ['accepted', 'rejected'],
      },
    ])
    expect(items[0]).not.toHaveProperty('accountId')
    expect(items[0]).not.toHaveProperty('clientAccountId')
    expect(items[0]).not.toHaveProperty('rationaleCiphertext')
  })

  it('normalizes unknown or non-array responses without granting review actions', async () => {
    const api = vi
      .fn()
      .mockResolvedValueOnce([{ ...BACKEND_SUGGESTION, status: 'unexpected' }])
      .mockResolvedValueOnce({ items: [BACKEND_SUGGESTION] })

    const unknown = await fetchSourceSuggestions(api as never, 'relationship-1', '')
    const empty = await fetchSourceSuggestions(api as never, 'relationship-1', '')

    expect(unknown[0]?.status).toBe('unknown')
    expect(unknown[0]?.allowedActions).toEqual([])
    expect(empty).toEqual([])
  })

  it('reviews in the selected client scope with the exact closed body', async () => {
    const api = vi.fn().mockResolvedValue({
      ...BACKEND_SUGGESTION,
      status: 'accepted',
    })
    const controller = new AbortController()

    const reviewed = await reviewSourceSuggestion(
      api as never,
      'suggestion/1',
      'client-1',
      { status: 'accepted', reason: ' source_relevant ' },
      controller.signal,
    )

    expect(api).toHaveBeenCalledWith(
      '/v1/customer-intelligence/source-suggestions/suggestion%2F1/review?clientAccountId=client-1',
      {
        method: 'POST',
        body: { status: 'accepted', reason: 'source_relevant' },
        signal: controller.signal,
      },
    )
    expect(reviewed.status).toBe('accepted')
    expect(reviewed.allowedActions).toEqual([])
  })

  it('exposes only registered safe-key reasons for each decision', () => {
    expect(SOURCE_SUGGESTION_REVIEW_REASONS.accepted.map((item) => item.value)).toEqual([
      'source_relevant',
      'profile_gap_confirmed',
    ])
    expect(validSourceSuggestionReviewReason('accepted', 'source_relevant')).toBe(true)
    expect(validSourceSuggestionReviewReason('accepted', 'source_not_relevant')).toBe(false)
    expect(validSourceSuggestionReviewReason('rejected', 'source_not_relevant')).toBe(true)
    expect(validSourceSuggestionReviewReason('rejected', 'Motivo livre')).toBe(false)
  })
})
