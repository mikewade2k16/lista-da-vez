import { describe, expect, it, vi } from 'vitest'
import { enqueueRelationshipIntelligenceRefresh } from './refresh-api'

describe('customer intelligence relationship refresh api', () => {
  it('enqueues a client-scoped headless refresh without prompt or provider data', async () => {
    const api = vi.fn().mockResolvedValue({ id: 'job-1', status: 'pending', created: true })

    await enqueueRelationshipIntelligenceRefresh(
      api,
      'relationship/1',
      'client-1',
      'subject-1',
      'panel.request-1',
    )

    expect(api).toHaveBeenCalledWith(
      '/v1/customer-intelligence/relationships/relationship%2F1/refresh',
      {
        method: 'POST',
        body: {
          clientAccountId: 'client-1',
          subjectId: 'subject-1',
          purposeKey: 'customer_profile',
          idempotencyKey: 'panel.request-1',
        },
      },
    )
  })
})
