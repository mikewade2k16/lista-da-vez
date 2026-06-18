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
  // No proxy do Pinia, o ref `integratedStoreId` chega desempacotado como string.
  integratedStoreId: '',
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
    operationsStore.refreshOperationSnapshot.mockReset().mockResolvedValue(null)
    operationsStore.refreshOverview.mockReset().mockResolvedValue(null)
    operationsStore.integratedStoreId = ''
    authStore.accessibleStoreIds = ['store-1']
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

  it('colapsa rajadas de eventos no modo "all" num unico refreshOverview (debounce trailing)', async () => {
    vi.useFakeTimers()
    try {
      authStore.accessibleStoreIds = ['store-1', 'store-2', 'store-3']

      const { useOperationsRealtime } = await import('./useOperationsRealtime')

      useOperationsRealtime({ scopeMode: 'all' })
      await flushRealtimeTicket()

      const socket = MockWebSocket.instances[0]
      socket?.open()

      // Rajada de eventos de lojas diferentes dentro da janela de debounce.
      socket?.message({ type: 'operation.updated', storeId: 'store-1', action: 'service-started' })
      socket?.message({ type: 'operation.updated', storeId: 'store-2', action: 'service-started' })
      socket?.message({ type: 'operation.updated', storeId: 'store-3', action: 'service-started' })
      await Promise.resolve()

      // Antes do silencio terminar, nenhum overview foi disparado ainda.
      expect(operationsStore.refreshOverview).not.toHaveBeenCalled()

      // Passada a janela de debounce, dispara UM unico refreshOverview (trailing).
      await vi.advanceTimersByTimeAsync(300)
      expect(operationsStore.refreshOverview).toHaveBeenCalledTimes(1)

      // Nenhuma loja aberta no detalhe (integratedStoreId vazio): sem refetch de snapshot.
      expect(operationsStore.refreshOperationSnapshot).not.toHaveBeenCalled()
    } finally {
      vi.useRealTimers()
    }
  })

  it('garante o refresh trailing apos o ULTIMO evento e revalida snapshot so da loja ativa', async () => {
    vi.useFakeTimers()
    try {
      authStore.accessibleStoreIds = ['store-1', 'store-2']
      // Loja aberta no detalhe operavel do modo "Todas as lojas".
      operationsStore.integratedStoreId = 'store-1'

      const { useOperationsRealtime } = await import('./useOperationsRealtime')

      useOperationsRealtime({ scopeMode: 'all' })
      await flushRealtimeTicket()

      const socket = MockWebSocket.instances[0]
      socket?.open()

      // Primeiro evento agenda o trailing.
      socket?.message({ type: 'operation.updated', storeId: 'store-2', action: 'service-started' })
      await Promise.resolve()

      // Quase no fim da janela chega um novo evento, que reinicia o debounce.
      await vi.advanceTimersByTimeAsync(200)
      expect(operationsStore.refreshOverview).not.toHaveBeenCalled()
      socket?.message({ type: 'operation.updated', storeId: 'store-1', action: 'service-finished' })
      await Promise.resolve()

      // Apos os 200ms anteriores ainda nao basta: a janela reiniciou no ultimo evento.
      await vi.advanceTimersByTimeAsync(200)
      expect(operationsStore.refreshOverview).not.toHaveBeenCalled()

      // Concluida a janela contada a partir do ultimo evento: um unico refresh final.
      await vi.advanceTimersByTimeAsync(100)
      expect(operationsStore.refreshOverview).toHaveBeenCalledTimes(1)

      // Snapshot revalidado apenas para a loja ativa (store-1), nunca para store-2.
      expect(operationsStore.refreshOperationSnapshot).toHaveBeenCalledTimes(1)
      expect(operationsStore.refreshOperationSnapshot).toHaveBeenCalledWith('store-1')
    } finally {
      vi.useRealTimers()
    }
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
