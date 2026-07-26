import { describe, expect, it, vi } from 'vitest'
import {
  createRetentionPolicyDraft,
  fetchRetentionPolicyVersions,
  publishRetentionPolicyVersion,
} from './retention-policy-api'
import {
  RETENTION_PUBLICATION_REASON_OPTIONS,
  isRetentionPublicationReasonCode,
  validRetentionDraftCommand,
  validRetentionPublishCommand,
} from './retention-policy-types'

function version(
  overrides: Partial<{
    id: string
    policyKey: string
    version: number
    status: string
    snapshotTtlSeconds: number
    onExpiry: string
    revision: number
  }> = {},
) {
  return {
    id: '11111111-1111-4111-8111-111111111111',
    accountId: '22222222-2222-4222-8222-222222222222',
    policyKey: 'customer_profile.default',
    version: 1,
    status: 'draft',
    snapshotTtlSeconds: 7_776_000,
    onExpiry: 'tombstone',
    legalHoldBehavior: 'preserve',
    blockReingestion: true,
    revision: 1,
    createdAt: '2026-07-23T12:00:00Z',
    ...overrides,
  }
}

describe('retention policy HTTP contracts', () => {
  it('lists only valid versions and preserves the abort signal', async () => {
    const controller = new AbortController()
    const api = vi
      .fn()
      .mockResolvedValue([
        version({ id: 'draft-v1', version: 1 }),
        version({ id: 'published-v2', version: 2, status: 'published', revision: 2 }),
        { id: 'invalid-without-policy-key' },
      ])

    const items = await fetchRetentionPolicyVersions(api as never, controller.signal)

    expect(api).toHaveBeenCalledWith('/v1/customer-intelligence/retention-policies', {
      signal: controller.signal,
      dedupe: false,
    })
    expect(items.map((item) => item.id)).toEqual(['published-v2', 'draft-v1'])
  })

  it('creates only a draft through the policy-key route', async () => {
    const controller = new AbortController()
    const api = vi.fn().mockResolvedValue(
      version({
        id: 'draft-v3',
        policyKey: 'customer_profile.short',
        version: 3,
      }),
    )

    const created = await createRetentionPolicyDraft(
      api as never,
      {
        policyKey: 'customer_profile.short',
        snapshotTtlSeconds: 604_800,
        onExpiry: 'crypto_shred',
      },
      controller.signal,
    )

    expect(created.status).toBe('draft')
    expect(api).toHaveBeenCalledTimes(1)
    expect(api).toHaveBeenCalledWith(
      '/v1/customer-intelligence/retention-policies/customer_profile.short/drafts',
      {
        method: 'POST',
        body: {
          snapshotTtlSeconds: 604_800,
          onExpiry: 'crypto_shred',
        },
        signal: controller.signal,
      },
    )
    expect(String(api.mock.calls[0]?.[0])).not.toContain('/publish')
  })

  it('publishes explicitly with optimistic revision and approval metadata', async () => {
    const controller = new AbortController()
    const api = vi.fn().mockResolvedValue(
      version({
        id: 'draft-v3',
        policyKey: 'customer_profile.short',
        version: 3,
        status: 'published',
        revision: 2,
      }),
    )

    await publishRetentionPolicyVersion(
      api as never,
      'draft-v3',
      {
        expectedRevision: 1,
        reasonCode: 'legal_review_approved',
        approvalReference: 'LEGAL-RETENTION-2026-001',
      },
      controller.signal,
    )

    expect(api).toHaveBeenCalledWith(
      '/v1/customer-intelligence/retention-policy-versions/draft-v3/publish',
      {
        method: 'POST',
        body: {
          expectedRevision: 1,
          reasonCode: 'legal_review_approved',
          approvalReference: 'LEGAL-RETENTION-2026-001',
        },
        signal: controller.signal,
      },
    )
  })
})

describe('retention policy closed frontend validation', () => {
  it('accepts only registered publication reasons and bounded drafts', () => {
    expect(RETENTION_PUBLICATION_REASON_OPTIONS.map((item) => item.value)).toContain(
      'legal_review_approved',
    )
    expect(isRetentionPublicationReasonCode('legal_review_approved')).toBe(true)
    expect(isRetentionPublicationReasonCode('operator_free_text')).toBe(false)
    expect(
      validRetentionDraftCommand({
        policyKey: 'customer_profile.short',
        snapshotTtlSeconds: 86_400,
        onExpiry: 'tombstone',
      }),
    ).toBe(true)
    expect(
      validRetentionDraftCommand({
        policyKey: 'INVALID POLICY',
        snapshotTtlSeconds: 86_399,
        onExpiry: 'tombstone',
      }),
    ).toBe(false)
  })

  it('requires revision, catalog reason and a request-key approval reference', () => {
    expect(
      validRetentionPublishCommand({
        expectedRevision: 1,
        reasonCode: 'legal_review_approved',
        approvalReference: 'LEGAL-RETENTION-2026-001',
      }),
    ).toBe(true)
    expect(
      validRetentionPublishCommand({
        expectedRevision: 0,
        reasonCode: 'legal_review_approved',
        approvalReference: 'invalid approval with spaces',
      }),
    ).toBe(false)
  })
})
