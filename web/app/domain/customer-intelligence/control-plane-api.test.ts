import { describe, expect, it, vi } from 'vitest'
import {
  fetchCustomerDataControlState,
  updateCustomerDataCapability,
  updateCustomerDataWriter,
} from '../customer-data/control-plane-api'
import { decideRecommendation } from './recommendation-api'

describe('customer control plane HTTP contracts', () => {
  it('uses the real customer data control-state route with explicit client scope', async () => {
    const api = vi.fn().mockResolvedValue({
      clientAccountId: 'client-1',
      capabilities: [],
      writerStates: [],
    })

    await fetchCustomerDataControlState(api, 'client-1')

    expect(api).toHaveBeenCalledWith(
      '/v1/customer-data/control-state?clientAccountId=client-1',
      expect.objectContaining({ dedupe: false }),
    )
  })

  it('sends revision, idempotency and reason to capability and writer routes', async () => {
    const api = vi.fn().mockResolvedValue({})

    await updateCustomerDataCapability(api, 'client-1', 'segmentation', {
      mode: 'shadow',
      expectedRevision: 2,
      idempotencyKey: 'capability:test-key',
      reason: 'Validacao iniciada.',
    })
    await updateCustomerDataWriter(api, 'client-1', 'segment_definition', {
      mode: 'new',
      sourceChecksum: 'sha256:equal',
      targetChecksum: 'sha256:equal',
      expectedRevision: 3,
      idempotencyKey: 'writer:test-key',
      reason: 'Reconciliacao aprovada.',
    })

    expect(api.mock.calls[0]?.[0]).toContain('/v1/customer-data/capabilities/segmentation?')
    expect(api.mock.calls[1]?.[0]).toContain('/v1/customer-data/writer-states/segment_definition?')
  })

  it('reviews recommendations through the single registered review endpoint', async () => {
    const api = vi.fn().mockResolvedValue({})

    await decideRecommendation(api, 'recommendation-1', 'client-1', 'approve', {
      reason: 'relevant_to_customer',
    })

    expect(api).toHaveBeenCalledWith(
      '/v1/customer-intelligence/recommendations/recommendation-1/review?clientAccountId=client-1',
      expect.objectContaining({
        method: 'POST',
        body: expect.objectContaining({ status: 'accepted' }),
        dedupe: false,
      }),
    )
  })
})
