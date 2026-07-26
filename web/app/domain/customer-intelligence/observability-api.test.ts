import { describe, expect, it, vi } from 'vitest'
import {
  fetchIntelligenceAuditEvents,
  fetchIntelligenceObservation,
  revealIntelligenceObservation,
} from './audit-api'
import { fetchIntelligenceRuns } from './runs-api'

describe('customer intelligence observability boundaries', () => {
  it('normalizes raw audit rows without exposing metadata', async () => {
    const api = vi.fn().mockResolvedValue({
      items: [
        {
          id: 'audit-1',
          clientAccountId: 'client-1',
          actorUserId: 'user-1',
          eventType: 'prompt_published',
          aggregateType: 'prompt_version',
          aggregateId: 'version-1',
          correlationId: 'correlation-1',
          reasonCode: 'approved',
          metadata: { rawPrompt: 'must-not-leak' },
          occurredAt: '2026-07-23T10:00:00Z',
        },
      ],
      nextCursor: 'next-page',
    })

    const result = await fetchIntelligenceAuditEvents(api, {
      clientAccountId: 'client-1',
      action: 'prompt_published',
      entityType: 'prompt_version',
      occurredFrom: '2026-07-20T09:00',
      occurredTo: '2026-07-23T12:00',
      cursor: 'current-page',
      limit: 25,
    })

    expect(result.items).toEqual([
      expect.objectContaining({
        id: 'audit-1',
        action: 'prompt_published',
        entityType: 'prompt_version',
        entityRef: 'version-1',
        canOpenObservation: false,
        canNavigate: false,
      }),
    ])
    expect(result.items[0]).not.toHaveProperty('metadata')
    expect(result.nextCursor).toBe('next-page')
    const requestedUrl = String(api.mock.calls[0]?.[0])
    expect(requestedUrl).toContain('clientAccountId=client-1')
    expect(requestedUrl).toContain('action=prompt_published')
    expect(requestedUrl).toContain('entityType=prompt_version')
    expect(requestedUrl).toContain('occurredFrom=')
    expect(requestedUrl).toContain('occurredTo=')
    expect(requestedUrl).toContain('cursor=current-page')
    expect(requestedUrl).toContain('limit=25')
  })

  it('opens only audited source observation aggregates', async () => {
    const api = vi.fn().mockResolvedValue([
      {
        id: 'audit-observation-1',
        clientAccountId: 'client-1',
        eventType: 'source.observation_ingested',
        aggregateType: 'source_observation',
        aggregateId: 'observation-1',
        occurredAt: '2026-07-23T10:00:00Z',
      },
    ])

    const result = await fetchIntelligenceAuditEvents(api, {
      clientAccountId: 'client-1',
    })

    expect(result.items[0]).toEqual(
      expect.objectContaining({
        observationRef: 'observation-1',
        canOpenObservation: true,
      }),
    )
  })

  it('does not offer a broken detail link after retention removed the snapshot', async () => {
    const api = vi.fn().mockResolvedValue([
      {
        id: 'audit-retention-1',
        clientAccountId: 'client-1',
        eventType: 'source.observation_retention_applied',
        aggregateType: 'source_observation',
        aggregateId: 'observation-expired-1',
        occurredAt: '2026-07-23T10:00:00Z',
      },
    ])

    const result = await fetchIntelligenceAuditEvents(api, {
      clientAccountId: 'client-1',
    })

    expect(result.items[0]).toEqual(
      expect.objectContaining({
        observationRef: undefined,
        canOpenObservation: false,
      }),
    )
  })

  it('keeps observation access masked until an explicit scoped reveal', async () => {
    const api = vi.fn().mockResolvedValue({
      id: 'observation/1',
      sensitivity: 'personal',
      revealed: true,
      snapshotFields: [],
    })

    await fetchIntelligenceObservation(api, 'observation/1', 'client-1')
    await revealIntelligenceObservation(
      api,
      'observation/1',
      'client-1',
      'customer_support_investigation',
    )

    expect(api).toHaveBeenNthCalledWith(
      1,
      '/v1/customer-intelligence/observations/observation%2F1?clientAccountId=client-1',
      { signal: undefined, dedupe: false },
    )
    expect(api).toHaveBeenNthCalledWith(
      2,
      '/v1/customer-intelligence/observations/observation%2F1/reveal?clientAccountId=client-1',
      {
        method: 'POST',
        body: { reasonCode: 'customer_support_investigation' },
        signal: undefined,
        dedupe: false,
      },
    )
  })

  it('normalizes succeeded runtime rows and usage fields', async () => {
    const api = vi.fn().mockResolvedValue([
      {
        id: 'run-1',
        requestId: 'request-1',
        clientAccountId: 'client-1',
        processKey: 'conversation.reply',
        promptBindingId: 'binding-1',
        agentVersionId: 'agent-version-1',
        modelId: 'model-1',
        outputSchemaVersion: 'v1',
        status: 'succeeded',
        usage: {
          promptTokens: 12,
          completionTokens: 8,
          totalTokens: 20,
          latencyMs: 450,
        },
        createdAt: '2026-07-23T10:00:00Z',
        completedAt: '2026-07-23T10:00:01Z',
      },
    ])

    const result = await fetchIntelligenceRuns(api, {
      clientAccountId: 'client-1',
    })

    expect(result.items[0]).toEqual(
      expect.objectContaining({
        status: 'completed',
        processKey: 'conversation.reply',
        inputUnits: 12,
        outputUnits: 8,
        latencyMs: 450,
        durationMs: 1000,
      }),
    )
  })
})
