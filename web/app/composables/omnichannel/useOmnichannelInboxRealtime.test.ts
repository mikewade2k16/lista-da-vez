import { describe, expect, it, vi } from 'vitest'
import { rememberInvalidationEvent } from './useOmnichannelScopeInvalidation'
import { createOmnichannelRealtimeCoordinator } from './useOmnichannelInboxRealtime'

function buildEnvelope(eventId = 'event-1', accountId = 'account-a') {
  return {
    type: 'omnichannel.invalidate',
    accountId,
    payload: {
      eventId,
      reason: 'history_reset',
      occurredAt: '2026-08-27T12:00:00.000Z',
    },
  }
}

describe('omnichannel realtime coordinator', () => {
  it('lets two independent tabs refresh once for the same event and deduplicates per tab', () => {
    function createTab() {
      let seen: string[] = []
      const bootstrapRest = vi.fn()
      const coordinator = createOmnichannelRealtimeCoordinator({
        currentAccountId: () => 'account-a',
        publish: (event) => {
          const key = `${event.accountId}:${event.eventId}`
          const remembered = rememberInvalidationEvent(seen, key)
          seen = remembered.nextSeenEventKeys
          if (remembered.duplicate) return false
          bootstrapRest(event.reason)
          return true
        },
        bootstrap: vi.fn(),
      })
      return { bootstrapRest, coordinator }
    }

    const firstTab = createTab()
    const secondTab = createTab()
    const envelope = buildEnvelope('shared-event')

    expect(firstTab.coordinator.handleEnvelope(envelope)).toBe(true)
    expect(secondTab.coordinator.handleEnvelope(envelope)).toBe(true)
    expect(firstTab.bootstrapRest).toHaveBeenCalledTimes(1)
    expect(secondTab.bootstrapRest).toHaveBeenCalledTimes(1)

    expect(firstTab.coordinator.handleEnvelope(envelope)).toBe(false)
    expect(firstTab.bootstrapRest).toHaveBeenCalledTimes(1)
    expect(secondTab.bootstrapRest).toHaveBeenCalledTimes(1)
  })

  it('deduplicates eventId and never accepts a rich payload', () => {
    let seen: string[] = []
    const publish = vi.fn((event: { accountId: string; eventId: string }) => {
      const key = `${event.accountId}:${event.eventId}`
      const remembered = rememberInvalidationEvent(seen, key)
      seen = remembered.nextSeenEventKeys
      return !remembered.duplicate
    })
    const coordinator = createOmnichannelRealtimeCoordinator({
      currentAccountId: () => 'account-a',
      publish,
      bootstrap: vi.fn(),
    })

    expect(coordinator.handleEnvelope(buildEnvelope())).toBe(true)
    expect(coordinator.handleEnvelope(buildEnvelope())).toBe(false)
    expect(
      coordinator.handleEnvelope({
        ...buildEnvelope('event-2'),
        payload: { ...buildEnvelope('event-2').payload, conversationId: 'conversation-old' },
      }),
    ).toBe(false)
    expect(coordinator.handleEnvelope(buildEnvelope('event-3', 'account-b'))).toBe(false)
    expect(publish).toHaveBeenCalledTimes(2)
  })

  it('does a full REST bootstrap on reconnect and starts clean after a manual reset', async () => {
    const bootstrap = vi.fn().mockResolvedValue(undefined)
    const coordinator = createOmnichannelRealtimeCoordinator({
      currentAccountId: () => 'account-a',
      publish: vi.fn(),
      bootstrap,
    })

    await expect(coordinator.handleOpen()).resolves.toBe(false)
    expect(bootstrap).not.toHaveBeenCalled()

    await expect(coordinator.handleOpen()).resolves.toBe(true)
    expect(bootstrap).toHaveBeenCalledWith({
      invalidateInFlight: true,
      clearAll: true,
    })

    coordinator.resetLifecycle()
    await expect(coordinator.handleOpen()).resolves.toBe(false)
    expect(bootstrap).toHaveBeenCalledTimes(1)
  })
})
