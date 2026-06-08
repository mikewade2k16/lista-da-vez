import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { MockWebSocket } from '../../test/helpers/mock-websocket'

const cleanupFns: Array<() => void> = []
const authStore = {
  isAuthenticated: true,
  activeStoreId: 'store-1',
  accessibleStoreIds: ['store-1'],
  accessToken: 'token-123',
  activeTenantId: 'tenant-1',
}
const runtimeStore = {
  state: {
    activeStoreId: 'store-1',
  },
}
const operationsStore = {
  refreshOperationSnapshot: vi.fn().mockResolvedValue(null),
  refreshOverview: vi.fn().mockResolvedValue(null),
}

async function flushRealtimeTicket() {
  await Promise.resolve()
  await Promise.resolve()
  await Promise.resolve()
}

vi.mock('vue', async () => {
  const actual = await vi.importActual<typeof import('vue')>('vue')
  return {
    ...actual,
    onMounted: (handler: () => void) => handler(),
    onBeforeUnmount: (handler: () => void) => {
      cleanupFns.push(handler)
    },
  }
})

vi.mock('~/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('~/stores/app-runtime', () => ({
  useAppRuntimeStore: () => runtimeStore,
}))

vi.mock('~/stores/operations', () => ({
  useOperationsStore: () => operationsStore,
}))

describe('useOperationsRealtime', () => {
  beforeEach(() => {
    vi.resetModules()
    MockWebSocket.reset()
    cleanupFns.length = 0
    operationsStore.refreshOperationSnapshot.mockClear()
    operationsStore.refreshOverview.mockClear()
    ;(globalThis as any).WebSocket = MockWebSocket
    ;(globalThis as any).$fetch.mockResolvedValue({ ticket: 'ticket-operations' })
  })

  afterEach(() => {
    while (cleanupFns.length > 0) {
      cleanupFns.pop()?.()
    }
    vi.restoreAllMocks()
  })

  it('refreshes the affected operation snapshot when a store update arrives', async () => {
    const { useOperationsRealtime } = await import('./useOperationsRealtime')

    const realtime = useOperationsRealtime()
    await flushRealtimeTicket()

    const socket = MockWebSocket.instances[0]

    expect((globalThis as any).$fetch).toHaveBeenCalledWith(
      '/v1/ws/ticket',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({ Authorization: 'Bearer token-123' }),
      }),
    )
    expect(socket?.url).toContain('/v1/realtime/operations')
    expect(socket?.url).toContain('storeId=store-1')
    expect(socket?.url).toContain('ticket=ticket-operations')
    expect(socket?.url).not.toContain('access_token=')

    socket?.open()
    socket?.message({
      type: 'operation.updated',
      storeId: 'store-1',
      action: 'service-started',
    })

    await Promise.resolve()
    await Promise.resolve()

    expect(operationsStore.refreshOperationSnapshot).toHaveBeenCalledWith('store-1')
    expect(realtime.lastEvent.value).toEqual(
      expect.objectContaining({
        type: 'operation.updated',
        storeId: 'store-1',
        action: 'service-started',
      }),
    )
  })

  it('does not open a socket when the ticket request fails', async () => {
    ;(globalThis as any).$fetch.mockRejectedValue(new Error('ticket failed'))
    vi.spyOn(console, 'warn').mockImplementation(() => undefined)

    const { useOperationsRealtime } = await import('./useOperationsRealtime')

    const realtime = useOperationsRealtime()
    await flushRealtimeTicket()

    expect(MockWebSocket.instances).toHaveLength(0)
    expect(realtime.status.value).toBe('error')
  })
})
