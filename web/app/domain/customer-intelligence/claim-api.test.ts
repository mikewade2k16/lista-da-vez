import { describe, expect, it, vi } from 'vitest'
import { fetchCustomerClaims, reviewCustomerClaim } from './claim-api'
import { validCustomerClaimReasonCode } from './claim-types'

const BACKEND_CLAIM = {
  id: 'claim-1',
  accountId: 'account-private',
  clientAccountId: 'client-1',
  subjectId: 'subject-private',
  relationshipId: 'relationship-1',
  factKey: 'preferred_product',
  valueType: 'string',
  value: 'Produto A',
  extractionMethod: 'runtime',
  extractorKey: 'customer_profile',
  extractorVersion: 'v1',
  promptBindingId: 'prompt-binding-1',
  runtimeRunId: 'runtime-run-1',
  confidence: 0.91,
  verificationState: 'unverified',
  sensitivity: 'internal',
  status: 'candidate',
  sourceOutcomeEventId: 'outcome-1',
  sourceClaimOrdinal: 0,
  revision: 3,
  evidence: [
    {
      observationId: 'observation-1',
      sourceKey: 'whatsapp',
      locator: 'message',
      rawContent: 'nao pode atravessar a normalizacao',
    },
  ],
  createdAt: '2026-07-23T12:00:00Z',
  updatedAt: '2026-07-23T12:00:00Z',
}

describe('customer intelligence claim HTTP contracts', () => {
  it('lists each real status with explicit client scope and minimized evidence', async () => {
    const api = vi.fn().mockResolvedValue([BACKEND_CLAIM])

    const claims = await fetchCustomerClaims(api, 'relationship-1', 'client-1', 'candidate')
    await fetchCustomerClaims(api, 'relationship-1', 'client-1', 'accepted')
    await fetchCustomerClaims(api, 'relationship-1', 'client-1', 'rejected')

    expect(api.mock.calls.map((call) => call[0])).toEqual([
      '/v1/customer-intelligence/relationships/relationship-1/claims?clientAccountId=client-1&status=candidate&limit=50',
      '/v1/customer-intelligence/relationships/relationship-1/claims?clientAccountId=client-1&status=accepted&limit=50',
      '/v1/customer-intelligence/relationships/relationship-1/claims?clientAccountId=client-1&status=rejected&limit=50',
    ])
    expect(claims[0]).not.toHaveProperty('accountId')
    expect(claims[0]).not.toHaveProperty('clientAccountId')
    expect(claims[0]).not.toHaveProperty('subjectId')
    expect(claims[0]?.evidence).toEqual([
      {
        observationId: 'observation-1',
        sourceKey: 'whatsapp',
        locator: 'message',
      },
    ])
  })

  it('reviews through the registered route with reasonCode and optimistic revision', async () => {
    const api = vi.fn().mockResolvedValue({
      ...BACKEND_CLAIM,
      status: 'accepted',
      revision: 4,
      reviewedAt: '2026-07-23T13:00:00Z',
      reviewReasonCode: 'quality.confirmed_by_operator',
    })

    const reviewed = await reviewCustomerClaim(api, 'claim-1', 'client-1', {
      status: 'accepted',
      reasonCode: 'quality.confirmed_by_operator',
      expectedRevision: 3,
    })

    expect(api).toHaveBeenCalledWith(
      '/v1/customer-intelligence/claims/claim-1/review?clientAccountId=client-1',
      {
        method: 'POST',
        body: {
          status: 'accepted',
          reasonCode: 'quality.confirmed_by_operator',
          expectedRevision: 3,
        },
      },
    )
    expect(reviewed.status).toBe('accepted')
    expect(reviewed.revision).toBe(4)
  })

  it('accepts only the backend safe-key format for review reason codes', () => {
    expect(validCustomerClaimReasonCode('quality.confirmed_by_operator')).toBe(true)
    expect(validCustomerClaimReasonCode('Quality confirmed manually')).toBe(false)
    expect(validCustomerClaimReasonCode(`a${'b'.repeat(160)}`)).toBe(false)
  })
})
