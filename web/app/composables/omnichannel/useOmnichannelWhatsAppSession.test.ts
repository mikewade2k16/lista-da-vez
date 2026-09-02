import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { OmniInstance } from '~/domain/omnichannel/config-types'
import { useOmnichannelWhatsAppSession } from './useOmnichannelWhatsAppSession'

const mocks = vi.hoisted(() => ({
  apiFetch: vi.fn(),
  requestInstanceHistoryReset: vi.fn(),
  permissionKeys: [] as string[],
}))

vi.mock('~/stores/auth', () => ({
  useAuthStore: () => ({ effectivePermissionKeys: mocks.permissionKeys }),
}))

vi.mock('~/composables/useApi', () => ({
  useApi: () => ({ apiFetch: mocks.apiFetch }),
}))

vi.mock('~/composables/omnichannel/useOmnichannelScopeInvalidation', () => ({
  canResetInstanceHistory: (instance: OmniInstance | null | undefined) =>
    instance?.myCapabilities?.resetHistory === true,
  useOmnichannelScopeInvalidation: () => ({
    isResettingInstance: vi.fn(() => false),
    requestInstanceHistoryReset: mocks.requestInstanceHistoryReset,
  }),
}))

function instance(
  id: string,
  options: { isDefault?: boolean; resetHistory?: boolean } = {},
): OmniInstance {
  return {
    id,
    tenantId: 'account-a',
    instanceName: `instance-${id}`,
    provider: 'evolution',
    accessPolicy: 'RESTRICTED',
    accessRevision: 1,
    displayName: `Número ${id}`,
    phoneNumber: `55799999999${id}`,
    queueLabel: null,
    userScopePolicy: 'MULTI_INSTANCE',
    responsibleUserId: null,
    responsibleUserName: null,
    responsibleUserEmail: null,
    isDefault: options.isDefault === true,
    isActive: true,
    hasEvolutionApiKey: true,
    assignedUserIds: [],
    historyVisibleFrom: null,
    historyResetRevision: 1,
    myCapabilities: {
      view: true,
      reply: true,
      manage: false,
      resetHistory: options.resetHistory === true,
    },
    createdAt: '2026-08-27T10:00:00.000Z',
    updatedAt: '2026-08-27T10:00:00.000Z',
  }
}

describe('non-admin WhatsApp session access', () => {
  beforeEach(() => {
    mocks.apiFetch.mockReset()
    mocks.requestInstanceHistoryReset.mockReset()
    mocks.permissionKeys.splice(0)
  })

  it('loads the access contract and exposes resetHistory without legacy admin role', async () => {
    mocks.apiFetch.mockResolvedValue({
      hasMultipleActiveInstances: false,
      instances: [instance('a', { isDefault: true, resetHistory: true })],
    })
    const session = useOmnichannelWhatsAppSession()

    await session.activate()

    expect(mocks.apiFetch).toHaveBeenCalledWith('/tenant/whatsapp/instances/access')
    expect(session.canManageChannel.value).toBe(false)
    expect(session.selectedInstance.value?.id).toBe('a')
    expect(session.canResetHistory.value).toBe(true)
    expect(session.instanceItems.value).toEqual([{ label: 'Número a', value: 'a' }])
    session.deactivate()
  })

  it('preserves the selected accessible instance across an authoritative reload', async () => {
    const instanceA = instance('a', { isDefault: true })
    const instanceB = instance('b', { resetHistory: true })
    mocks.apiFetch
      .mockResolvedValueOnce({
        hasMultipleActiveInstances: true,
        instances: [instanceA, instanceB],
      })
      .mockResolvedValueOnce({
        hasMultipleActiveInstances: true,
        instances: [instanceB, instanceA],
      })
    const session = useOmnichannelWhatsAppSession()

    await session.activate()
    await session.selectInstance('b')
    await session.loadInstances({ silent: true })

    expect(session.selectedInstance.value?.id).toBe('b')
    expect(session.canResetHistory.value).toBe(true)
    expect(mocks.apiFetch).toHaveBeenNthCalledWith(2, '/tenant/whatsapp/instances/access')
    session.deactivate()
  })

  it('uses the management contract when the effective permission allows instance changes', async () => {
    mocks.permissionKeys.push('omnichannel.instances.manage')
    mocks.apiFetch.mockResolvedValue({ instances: [instance('a', { isDefault: true })] })
    const session = useOmnichannelWhatsAppSession()

    await session.activate()

    expect(session.canManageChannel.value).toBe(true)
    expect(mocks.apiFetch).toHaveBeenCalledWith('/tenant/whatsapp/instances')
    expect(session.instanceItems.value.at(-1)).toEqual({
      label: 'Nova conexao WhatsApp',
      value: '__new__',
    })
    session.deactivate()
  })

  it('propagates an authoritative reload failure back to the reset action', async () => {
    const selected = instance('a', { isDefault: true, resetHistory: true })
    mocks.apiFetch
      .mockResolvedValueOnce({
        hasMultipleActiveInstances: false,
        instances: [selected],
      })
      .mockRejectedValueOnce(new Error('GET unavailable'))
    mocks.requestInstanceHistoryReset.mockImplementation(async (_instance, options) => {
      await expect(options.rehydrate()).rejects.toThrow('GET unavailable')
      return { status: 'conflict' }
    })
    const session = useOmnichannelWhatsAppSession()

    await session.activate()
    await session.clearConversationHistory()

    expect(mocks.requestInstanceHistoryReset).toHaveBeenCalledTimes(1)
    expect(mocks.apiFetch).toHaveBeenNthCalledWith(2, '/tenant/whatsapp/instances/access')
    session.deactivate()
  })
})
