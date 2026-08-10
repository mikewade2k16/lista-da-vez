import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useAuthStore } from './auth'
import { useAttendanceRecordingFeatureStore } from './attendanceRecordingFeature'

function fetchMock() {
  return (globalThis as typeof globalThis & { $fetch: ReturnType<typeof vi.fn> }).$fetch
}

function authenticate(role = 'platform_admin') {
  const auth = useAuthStore()
  auth.accessToken = 'test-token'
  auth.user = { id: 'user-1', name: 'Admin' }
  auth.principal = {
    role,
    accountId: 'account-1',
    tenantId: 'account-1',
    permissions: [],
    permissionsResolved: true,
  }
  auth.hydrated = true
  return auth
}

describe('useAttendanceRecordingFeatureStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('fails closed and hydrates the account-scoped GET response', async () => {
    authenticate()
    fetchMock().mockResolvedValue({
      feature: {
        accountId: 'account-1',
        enabled: true,
        updatedAt: '2026-07-27T12:00:00Z',
      },
    })
    const store = useAttendanceRecordingFeatureStore()

    expect(store.enabled).toBe(false)
    await expect(store.load()).resolves.toBe(true)

    expect(store.loaded).toBe(true)
    expect(store.enabled).toBe(true)
    expect(store.accountId).toBe('account-1')
    expect(fetchMock()).toHaveBeenCalledWith(
      '/v1/operations/transcriptions/feature',
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: 'Bearer test-token' }),
      }),
    )
  })

  it('uses the server PUT response and does not update optimistically', async () => {
    authenticate()
    fetchMock().mockResolvedValue({
      feature: { accountId: 'account-1', enabled: false, updatedAt: null },
    })
    const store = useAttendanceRecordingFeatureStore()

    await expect(store.save(true)).resolves.toBe(true)

    expect(store.enabled).toBe(false)
    const [, options] = fetchMock().mock.calls[0]
    expect(options.method).toBe('PUT')
    expect(JSON.parse(options.body)).toEqual({ enabled: true })
  })

  it('blocks non-platform admins before any request', async () => {
    authenticate('owner')
    const store = useAttendanceRecordingFeatureStore()

    await expect(store.save(true)).resolves.toBe(false)
    expect(fetchMock()).not.toHaveBeenCalled()
  })
})
