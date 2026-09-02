import { describe, expect, it, vi } from 'vitest'
import type { OmniInstance } from '~/domain/omnichannel/config-types'
import {
  canResetInstanceHistory,
  createInstanceHistoryResetAction,
  parseRealtimeInvalidationEnvelope,
  rememberInvalidationEvent,
} from './useOmnichannelScopeInvalidation'

function buildInstance(overrides: Partial<OmniInstance> = {}): OmniInstance {
  return {
    id: 'instance-a',
    tenantId: 'account-a',
    instanceName: 'crow-principal',
    provider: 'evolution',
    accessPolicy: 'RESTRICTED',
    accessRevision: 1,
    displayName: 'Comercial',
    phoneNumber: '5579999999999',
    queueLabel: null,
    userScopePolicy: 'MULTI_INSTANCE',
    responsibleUserId: null,
    responsibleUserName: null,
    responsibleUserEmail: null,
    isDefault: true,
    isActive: true,
    hasEvolutionApiKey: true,
    assignedUserIds: [],
    historyVisibleFrom: null,
    historyResetRevision: 3,
    myCapabilities: {
      view: true,
      reply: true,
      manage: true,
      resetHistory: true,
    },
    createdAt: '2026-08-27T10:00:00.000Z',
    updatedAt: '2026-08-27T10:00:00.000Z',
    ...overrides,
  }
}

function createActionDependencies(
  overrides: Partial<Parameters<typeof createInstanceHistoryResetAction>[0]> = {},
) {
  let busy = false
  return {
    accountId: () => 'account-a',
    accountLabel: () => 'Crow Visuals',
    scopeGeneration: () => 1,
    prompt: vi.fn().mockResolvedValue({ confirmed: true, value: 'crow-principal' }),
    reset: vi.fn().mockResolvedValue({
      instanceId: 'instance-a',
      hiddenBefore: '2026-08-27T12:00:00.000Z',
      resetRevision: 4,
    }),
    tryStart: vi.fn(() => {
      if (busy) return false
      busy = true
      return true
    }),
    finish: vi.fn(() => {
      busy = false
    }),
    isBusy: () => busy,
    publish: vi.fn(),
    success: vi.fn(),
    error: vi.fn(),
    statusCode: vi.fn(() => 0),
    errorMessage: vi.fn((_cause: unknown, fallback: string) => fallback),
    ...overrides,
  }
}

describe('history reset action', () => {
  it('gates the action exclusively by myCapabilities.resetHistory', async () => {
    const instance = buildInstance({
      myCapabilities: { view: true, reply: true, manage: true, resetHistory: false },
    })
    const dependencies = createActionDependencies()
    const action = createInstanceHistoryResetAction(dependencies)

    expect(canResetInstanceHistory(instance)).toBe(false)
    await expect(action(instance)).resolves.toEqual({ status: 'forbidden' })
    expect(dependencies.prompt).not.toHaveBeenCalled()
    expect(dependencies.reset).not.toHaveBeenCalled()
  })

  it('does not call the API when the exact instanceName confirmation is wrong', async () => {
    const dependencies = createActionDependencies({
      prompt: vi.fn().mockResolvedValue({ confirmed: true, value: 'Crow-Principal' }),
    })
    const action = createInstanceHistoryResetAction(dependencies)

    await expect(action(buildInstance())).resolves.toEqual({
      status: 'invalid_confirmation',
    })
    expect(dependencies.reset).not.toHaveBeenCalled()
    expect(dependencies.publish).not.toHaveBeenCalled()
    expect(dependencies.finish).toHaveBeenCalledWith('account-a', 'instance-a')
  })

  it('sends only the selected instance and publishes the authoritative result', async () => {
    const dependencies = createActionDependencies()
    const rehydrate = vi.fn()
    const action = createInstanceHistoryResetAction(dependencies)

    await expect(action(buildInstance(), { rehydrate })).resolves.toMatchObject({
      status: 'success',
      result: { instanceId: 'instance-a', resetRevision: 4 },
    })
    expect(dependencies.reset).toHaveBeenCalledWith('instance-a', {
      confirmation: 'crow-principal',
      expectedRevision: 3,
    })
    expect(dependencies.publish).toHaveBeenCalledWith(
      'account-a',
      'instance-a',
      'crow-principal',
      expect.objectContaining({ instanceId: 'instance-a', resetRevision: 4 }),
    )
    expect(rehydrate).toHaveBeenCalledTimes(1)
  })

  it('blocks a second concurrent mutation for the same instance', async () => {
    let resolvePrompt!: (value: { confirmed: boolean; value: string }) => void
    const promptResult = new Promise<{ confirmed: boolean; value: string }>((resolve) => {
      resolvePrompt = resolve
    })
    const dependencies = createActionDependencies({
      prompt: vi.fn(() => promptResult),
    })
    const action = createInstanceHistoryResetAction(dependencies)
    const first = action(buildInstance())
    expect(dependencies.isBusy()).toBe(true)
    const second = action(buildInstance())

    await expect(second).resolves.toEqual({ status: 'busy' })
    expect(dependencies.prompt).toHaveBeenCalledTimes(1)
    expect(dependencies.tryStart).toHaveBeenCalledTimes(2)
    expect(dependencies.finish).not.toHaveBeenCalled()
    resolvePrompt({ confirmed: true, value: 'crow-principal' })
    await expect(first).resolves.toMatchObject({ status: 'success' })
    expect(dependencies.reset).toHaveBeenCalledTimes(1)
    expect(dependencies.finish).toHaveBeenCalledTimes(1)
    expect(dependencies.isBusy()).toBe(false)
  })

  it('rehydrates a 409 once and never retries the mutation', async () => {
    const conflict = { statusCode: 409 }
    const dependencies = createActionDependencies({
      reset: vi.fn().mockRejectedValue(conflict),
      statusCode: vi.fn(() => 409),
    })
    const rehydrate = vi.fn()
    const action = createInstanceHistoryResetAction(dependencies)

    await expect(action(buildInstance(), { rehydrate })).resolves.toEqual({
      status: 'conflict',
    })
    expect(dependencies.reset).toHaveBeenCalledTimes(1)
    expect(rehydrate).toHaveBeenCalledTimes(1)
    expect(dependencies.publish).not.toHaveBeenCalled()
  })

  it('never claims a 409 was reloaded when the authoritative GET fails', async () => {
    const dependencies = createActionDependencies({
      reset: vi.fn().mockRejectedValue({ statusCode: 409 }),
      statusCode: vi.fn(() => 409),
    })
    const rehydrate = vi.fn().mockRejectedValue(new Error('GET unavailable'))
    const action = createInstanceHistoryResetAction(dependencies)

    await expect(action(buildInstance(), { rehydrate })).resolves.toEqual({
      status: 'conflict',
    })
    expect(dependencies.reset).toHaveBeenCalledTimes(1)
    expect(rehydrate).toHaveBeenCalledTimes(1)
    expect(dependencies.error).toHaveBeenCalledWith(
      'A conexão foi atualizada por outra ação. Atualize os dados e confirme novamente.',
    )
    expect(dependencies.error).not.toHaveBeenCalledWith(
      expect.stringContaining('Os dados foram recarregados'),
    )
  })

  it('does not call the API when the account changes while confirmation is open', async () => {
    let accountId = 'account-a'
    let resolvePrompt!: (value: { confirmed: boolean; value: string }) => void
    const promptResult = new Promise<{ confirmed: boolean; value: string }>((resolve) => {
      resolvePrompt = resolve
    })
    const dependencies = createActionDependencies({
      accountId: () => accountId,
      prompt: vi.fn(() => promptResult),
    })
    const action = createInstanceHistoryResetAction(dependencies)

    const pending = action(buildInstance())
    accountId = 'account-b'
    resolvePrompt({ confirmed: true, value: 'crow-principal' })

    await expect(pending).resolves.toEqual({ status: 'scope_changed' })
    expect(dependencies.reset).not.toHaveBeenCalled()
    expect(dependencies.publish).not.toHaveBeenCalled()
    expect(dependencies.success).not.toHaveBeenCalled()
    expect(dependencies.error).not.toHaveBeenCalled()
    expect(dependencies.finish).toHaveBeenCalledWith('account-a', 'instance-a')
  })

  it('does not call the API when the account generation changes during confirmation', async () => {
    let generation = 1
    const dependencies = createActionDependencies({
      scopeGeneration: () => generation,
      prompt: vi.fn().mockImplementation(async () => {
        generation = 2
        return { confirmed: true, value: 'crow-principal' }
      }),
    })
    const action = createInstanceHistoryResetAction(dependencies)

    await expect(action(buildInstance())).resolves.toEqual({ status: 'scope_changed' })
    expect(dependencies.reset).not.toHaveBeenCalled()
    expect(dependencies.publish).not.toHaveBeenCalled()
    expect(dependencies.success).not.toHaveBeenCalled()
    expect(dependencies.error).not.toHaveBeenCalled()
  })

  it('does not confuse realtime or reconnect data invalidations with an account switch', async () => {
    let dataGeneration = 1
    const dependencies = createActionDependencies({
      prompt: vi.fn().mockImplementation(async () => {
        dataGeneration += 1
        return { confirmed: true, value: 'crow-principal' }
      }),
      reset: vi.fn().mockImplementation(async () => {
        dataGeneration += 1
        return {
          instanceId: 'instance-a',
          hiddenBefore: '2026-08-27T12:00:00.000Z',
          resetRevision: 4,
        }
      }),
    })
    const action = createInstanceHistoryResetAction(dependencies)

    await expect(action(buildInstance())).resolves.toMatchObject({ status: 'success' })
    expect(dataGeneration).toBe(3)
    expect(dependencies.reset).toHaveBeenCalledTimes(1)
    expect(dependencies.publish).toHaveBeenCalledTimes(1)
    expect(dependencies.success).toHaveBeenCalledTimes(1)
  })

  it('uses the immutable instance snapshot captured before confirmation', async () => {
    let resolvePrompt!: (value: { confirmed: boolean; value: string }) => void
    const promptResult = new Promise<{ confirmed: boolean; value: string }>((resolve) => {
      resolvePrompt = resolve
    })
    const dependencies = createActionDependencies({
      prompt: vi.fn(() => promptResult),
    })
    const action = createInstanceHistoryResetAction(dependencies)
    const instance = buildInstance()

    const pending = action(instance)
    instance.instanceName = 'mutated-name'
    instance.historyResetRevision = 99
    resolvePrompt({ confirmed: true, value: 'crow-principal' })

    await expect(pending).resolves.toMatchObject({ status: 'success' })
    expect(dependencies.reset).toHaveBeenCalledWith('instance-a', {
      confirmation: 'crow-principal',
      expectedRevision: 3,
    })
    expect(dependencies.publish).toHaveBeenCalledWith(
      'account-a',
      'instance-a',
      'crow-principal',
      expect.objectContaining({ resetRevision: 4 }),
    )
  })

  it('does not project, rehydrate or toast when the account changes during mutation', async () => {
    let accountId = 'account-a'
    const dependencies = createActionDependencies({
      accountId: () => accountId,
      reset: vi.fn().mockImplementation(async () => {
        accountId = 'account-b'
        return {
          instanceId: 'instance-a',
          hiddenBefore: '2026-08-27T12:00:00.000Z',
          resetRevision: 4,
        }
      }),
    })
    const rehydrate = vi.fn()
    const action = createInstanceHistoryResetAction(dependencies)

    await expect(action(buildInstance(), { rehydrate })).resolves.toEqual({
      status: 'scope_changed',
    })
    expect(dependencies.publish).not.toHaveBeenCalled()
    expect(rehydrate).not.toHaveBeenCalled()
    expect(dependencies.success).not.toHaveBeenCalled()
    expect(dependencies.error).not.toHaveBeenCalled()
    expect(dependencies.finish).toHaveBeenCalledWith('account-a', 'instance-a')
  })

  it('detects an account switch away and back during the mutation', async () => {
    let accountId = 'account-a'
    let accountGeneration = 1
    const dependencies = createActionDependencies({
      accountId: () => accountId,
      scopeGeneration: () => accountGeneration,
      reset: vi.fn().mockImplementation(async () => {
        accountId = 'account-b'
        accountGeneration += 1
        accountId = 'account-a'
        accountGeneration += 1
        return {
          instanceId: 'instance-a',
          hiddenBefore: '2026-08-27T12:00:00.000Z',
          resetRevision: 4,
        }
      }),
    })
    const action = createInstanceHistoryResetAction(dependencies)

    await expect(action(buildInstance())).resolves.toEqual({ status: 'scope_changed' })
    expect(dependencies.publish).not.toHaveBeenCalled()
    expect(dependencies.success).not.toHaveBeenCalled()
    expect(dependencies.error).not.toHaveBeenCalled()
    expect(dependencies.finish).toHaveBeenCalledWith('account-a', 'instance-a')
  })

  it('preserves the projection when the mutation fails', async () => {
    const dependencies = createActionDependencies({
      reset: vi.fn().mockRejectedValue(new Error('offline')),
    })
    const rehydrate = vi.fn()
    const action = createInstanceHistoryResetAction(dependencies)

    await expect(action(buildInstance(), { rehydrate })).resolves.toEqual({ status: 'error' })
    expect(rehydrate).not.toHaveBeenCalled()
    expect(dependencies.publish).not.toHaveBeenCalled()
    expect(dependencies.error).toHaveBeenCalledWith(
      'Não foi possível limpar o histórico desta conexão.',
    )
  })
})

describe('opaque realtime invalidation contract', () => {
  const validEnvelope = {
    type: 'omnichannel.invalidate',
    accountId: 'account-a',
    payload: {
      eventId: 'event-1',
      reason: 'history_reset',
      occurredAt: '2026-08-27T12:00:00.000Z',
    },
  }

  it('accepts only the fixed payload and rejects rich or unknown envelopes', () => {
    expect(parseRealtimeInvalidationEnvelope(validEnvelope)).toEqual({
      accountId: 'account-a',
      eventId: 'event-1',
      reason: 'history_reset',
      occurredAt: '2026-08-27T12:00:00.000Z',
    })
    expect(
      parseRealtimeInvalidationEnvelope({
        ...validEnvelope,
        payload: { ...validEnvelope.payload, instanceId: 'instance-a' },
      }),
    ).toBeNull()
    expect(
      parseRealtimeInvalidationEnvelope({
        ...validEnvelope,
        payload: { ...validEnvelope.payload, reason: 'conversation_updated' },
      }),
    ).toBeNull()
  })

  it('remembers eventId keys and rejects a duplicate without growing state', () => {
    const first = rememberInvalidationEvent([], 'account-a:event-1')
    const duplicate = rememberInvalidationEvent(
      first.nextSeenEventKeys,
      'account-a:event-1',
    )

    expect(first).toEqual({
      duplicate: false,
      nextSeenEventKeys: ['account-a:event-1'],
    })
    expect(duplicate).toEqual({
      duplicate: true,
      nextSeenEventKeys: ['account-a:event-1'],
    })
  })
})
