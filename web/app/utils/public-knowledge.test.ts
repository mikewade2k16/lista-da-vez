import { describe, expect, it } from 'vitest'
import {
  attemptLabel,
  canPublish,
  catalogStatusLabel,
  reviewSummaryLabel,
} from './public-knowledge'
import type { PublicCatalogStatus, PublicReviewSummary } from '~/types/public-knowledge'

const review = (owner: PublicReviewSummary['owner']): PublicReviewSummary => ({
  owner,
  aiApproved: 2,
  aiConcerns: 1,
  accountant: 'pending',
})

describe('public knowledge labels', () => {
  it('explains failed attempts without hiding the reason', () => {
    expect(attemptLabel(undefined)).toBe('Nunca coletada')
    expect(
      attemptLabel({
        outcome: 'failed',
        errorCode: 'source_http_rejected',
        createdAt: '2026-11-09T12:00:00Z',
      }),
    ).toContain('não contornamos')
    expect(attemptLabel({ outcome: 'unchanged', createdAt: '2026-11-09T12:00:00Z' })).toContain(
      'sem mudança',
    )
  })

  it('summarizes the three review kinds and gates publication on the owner', () => {
    expect(reviewSummaryLabel(review('approved'))).toBe(
      'Você: aprovado · IA: 2 aprovaram, 1 com ressalvas · Contador: pendente',
    )
    expect(canPublish(review('approved'))).toBe(true)
    expect(canPublish(review('changes_requested'))).toBe(false)
  })

  it('prioritizes a version waiting for review over the published one', () => {
    const base = { entry: { key: 'k' } } as PublicCatalogStatus
    const collection = {
      version: 2,
      status: 'collected',
      review: review('pending'),
    } as PublicCatalogStatus['latest']
    expect(
      catalogStatusLabel({
        ...base,
        latest: collection,
        published: { ...collection!, version: 1, status: 'published' },
      }),
    ).toBe('Versão 2 aguardando revisão')
    expect(catalogStatusLabel({ ...base, lastAttempt: { outcome: 'failed', createdAt: '' } })).toBe(
      'Falha na coleta',
    )
  })
})
