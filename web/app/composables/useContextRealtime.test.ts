import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { MockWebSocket } from '../../test/helpers/mock-websocket'

const cleanupFns: Array<() => void> = []
const refreshRuntimeStoreSettings = vi.fn()
const authStore = {
  isAuthenticated: true,
  activeTenantId: 'tenant-1',
  activeStoreId: 'store-1',
  accessToken: 'token-123',
  tenantContext: [{ id: 'tenant-1' }],
  principal: { userId: 'user-1' },
  role: 'consultant',
  fetchContext: vi.fn().mockResolvedValue(undefined),
  applyRuntimeSettingsStatus: vi.fn(),
}
const runtimeStore = {
  state: {
    activeStoreId: 'store-1',
  },
}
const accessControlStore = {
  refreshRealtimeState: vi.fn().mockResolvedValue(undefined),
}
const alertsStore = {
  items: [],
  refreshRealtimeState: vi.fn().mockResolvedValue(undefined),
}
const attendanceRecordingFeatureStore = {
  load: vi.fn().mockResolvedValue(true),
}
const uiStore = {
  notify: vi.fn(),
}
const multiStore = {
  refreshOverview: vi.fn().mockResolvedValue(undefined),
  refreshManagedStores: vi.fn().mockResolvedValue(undefined),
}
const usersStore = {
  refreshUsers: vi.fn().mockResolvedValue(undefined),
}
const operationGoalsStore = {
  ready: false,
  goals: [],
  lastFilters: {},
  shouldSkipRealtimeUpdate: vi.fn().mockReturnValue(false),
  loadGoals: vi.fn().mockResolvedValue(undefined),
}
const crmStore = {
  overview: null as object | null,
  invalidateOverview: vi.fn(),
  refreshOverview: vi.fn().mockResolvedValue(undefined),
}
const coreAccountStore = {
  enabledModules: ['queue'] as string[],
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

vi.mock('~/stores/access-control', () => ({
  useAccessControlStore: () => accessControlStore,
}))

vi.mock('~/stores/alerts', () => ({
  useAlertsStore: () => alertsStore,
}))

vi.mock('~/stores/attendanceRecordingFeature', () => ({
  useAttendanceRecordingFeatureStore: () => attendanceRecordingFeatureStore,
}))

vi.mock('~/stores/ui', () => ({
  useUiStore: () => uiStore,
}))

vi.mock('~/stores/app-runtime', () => ({
  useAppRuntimeStore: () => runtimeStore,
}))

vi.mock('~/stores/multistore', () => ({
  useMultiStoreStore: () => multiStore,
}))

vi.mock('~/stores/users', () => ({
  useUsersStore: () => usersStore,
}))

vi.mock('~/stores/operation-goals', () => ({
  useOperationGoalsStore: () => operationGoalsStore,
}))

vi.mock('~/stores/crm', () => ({
  useCrmStore: () => crmStore,
}))

vi.mock('~/utils/runtime-remote', () => ({
  refreshRuntimeStoreSettings: (...args: unknown[]) => refreshRuntimeStoreSettings(...args),
}))

vi.mock('../../layers/core/stores/account', () => ({
  useCoreAccountStore: () => coreAccountStore,
}))

describe('useContextRealtime', () => {
  beforeEach(() => {
    vi.resetModules()
    MockWebSocket.reset()
    cleanupFns.length = 0
    refreshRuntimeStoreSettings.mockReset()
    refreshRuntimeStoreSettings.mockResolvedValue({ settingsLoadState: 'loaded' })
    authStore.fetchContext.mockClear()
    authStore.applyRuntimeSettingsStatus.mockClear()
    accessControlStore.refreshRealtimeState.mockClear()
    alertsStore.refreshRealtimeState.mockClear()
    attendanceRecordingFeatureStore.load.mockClear()
    uiStore.notify.mockClear()
    multiStore.refreshOverview.mockClear()
    multiStore.refreshManagedStores.mockClear()
    usersStore.refreshUsers.mockClear()
    operationGoalsStore.shouldSkipRealtimeUpdate.mockClear()
    operationGoalsStore.loadGoals.mockClear()
    crmStore.invalidateOverview.mockClear()
    crmStore.refreshOverview.mockClear()
    crmStore.overview = null
    ;(globalThis as any).WebSocket = MockWebSocket
    ;(globalThis as any).$fetch.mockResolvedValue({ ticket: 'ticket-context' })
  })

  afterEach(() => {
    while (cleanupFns.length > 0) {
      cleanupFns.pop()?.()
    }
    vi.restoreAllMocks()
  })

  it('refreshes tenant store settings when a matching settings event arrives', async () => {
    const { useContextRealtime } = await import('./useContextRealtime')

    const realtime = useContextRealtime()
    await flushRealtimeTicket()

    const socket = MockWebSocket.instances[0]

    expect((globalThis as any).$fetch).toHaveBeenCalledWith(
      '/v1/ws/ticket',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({ Authorization: 'Bearer token-123' }),
      }),
    )
    expect(socket?.url).toContain('/v1/realtime/context')
    expect(socket?.url).toContain('tenantId=tenant-1')
    expect(socket?.url).toContain('ticket=ticket-context')
    expect(socket?.url).not.toContain('access_token=')

    socket?.open()
    socket?.message({
      type: 'context.updated',
      tenantId: 'tenant-1',
      resource: 'settings',
      resourceId: 'tenant-1',
    })

    await Promise.resolve()
    await Promise.resolve()

    expect(refreshRuntimeStoreSettings).toHaveBeenCalledTimes(1)
    expect(refreshRuntimeStoreSettings.mock.calls[0]?.[2]).toBe('store-1')
    expect(refreshRuntimeStoreSettings.mock.calls[0]?.[3]).toBe('tenant-1')
    expect(authStore.applyRuntimeSettingsStatus).toHaveBeenCalledWith({
      settingsLoadState: 'loaded',
    })
    expect(realtime.lastEvent.value).toEqual(
      expect.objectContaining({
        type: 'context.updated',
        resource: 'settings',
      }),
    )
  })

  it('refreshes the account recording feature when its realtime event arrives', async () => {
    const { useContextRealtime } = await import('./useContextRealtime')

    useContextRealtime()
    await flushRealtimeTicket()

    const socket = MockWebSocket.instances[0]
    socket?.open()
    socket?.message({
      type: 'context.updated',
      tenantId: 'tenant-1',
      resource: 'attendance_recording',
      resourceId: 'tenant-1',
    })

    await Promise.resolve()
    await Promise.resolve()

    expect(attendanceRecordingFeatureStore.load).toHaveBeenCalledWith(true)
    expect(authStore.fetchContext).not.toHaveBeenCalled()
  })
})
