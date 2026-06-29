import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { MockWebSocket } from '../../../test/helpers/mock-websocket'

const cleanupFns: Array<() => void> = []
const authStore = {
  isAuthenticated: true,
  activeTenantId: 'tenant-1',
  tenantContext: [{ id: 'tenant-1' }],
  principal: { tenantId: 'tenant-1' },
  accessToken: 'token-123',
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

// Account store mockado vazio: o fallback de resolveAccountId deve cair em auth.activeTenantId
// (tenant-1). Quando activeAccountId estiver populado (switcher v2), ele vence — coberto no app real.
vi.mock('../../core/stores/account', () => ({
  useCoreAccountStore: () => ({ activeAccountId: '' }),
}))

describe('useTasksRealtime', () => {
  beforeEach(() => {
    vi.resetModules()
    MockWebSocket.reset()
    cleanupFns.length = 0
    ;(globalThis as any).WebSocket = MockWebSocket
    authStore.isAuthenticated = true
    authStore.activeTenantId = 'tenant-1'
    authStore.tenantContext = [{ id: 'tenant-1' }]
    authStore.principal = { tenantId: 'tenant-1' }
    authStore.accessToken = 'token-123'
    ;(globalThis as any).$fetch.mockResolvedValue({ ticket: 'ticket-tasks' })
    vi.spyOn(console, 'info').mockImplementation(() => undefined)
    vi.spyOn(console, 'warn').mockImplementation(() => undefined)
    vi.spyOn(console, 'error').mockImplementation(() => undefined)
  })

  afterEach(() => {
    while (cleanupFns.length > 0) {
      cleanupFns.pop()?.()
    }
    vi.restoreAllMocks()
  })

  it('opens a tasks socket and forwards parsed realtime events to the consumer', async () => {
    const onEvent = vi.fn()
    const { useTasksRealtime } = await import('./useTasksRealtime')

    const realtime = useTasksRealtime({
      enabled: true,
      onEvent,
    })
    await flushRealtimeTicket()

    const socket = MockWebSocket.instances[0]

    expect((globalThis as any).$fetch).toHaveBeenCalledWith(
      '/v1/ws/ticket',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({ Authorization: 'Bearer token-123' }),
      }),
    )
    expect(socket?.url).toContain('/v1/realtime/tasks')
    expect(socket?.url).toContain('scope=account')
    expect(socket?.url).toContain('accountId=tenant-1')
    expect(socket?.url).toContain('ticket=ticket-tasks')
    expect(socket?.url).not.toContain('access_token=')

    socket?.open()

    expect(realtime.status.value).toBe('connected')
    expect(realtime.isConnected.value).toBe(true)

    socket?.message({ type: 'task.updated', taskId: 'task-1', version: 4 })

    expect(onEvent).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'task.updated', taskId: 'task-1', version: 4 }),
    )
    expect(realtime.lastEvent.value).toEqual(
      expect.objectContaining({ type: 'task.updated', taskId: 'task-1', version: 4 }),
    )
  })
})
