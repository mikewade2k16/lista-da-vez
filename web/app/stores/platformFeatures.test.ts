import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useAuthStore } from './auth'
import {
  defaultExperimentalFeatures,
  normalizeExperimentalFeatures,
  usePlatformFeaturesStore,
} from './platformFeatures'

function fetchMock() {
  return (globalThis as typeof globalThis & { $fetch: ReturnType<typeof vi.fn> }).$fetch
}

function authenticate(role = 'platform_admin') {
  const auth = useAuthStore()
  auth.accessToken = 'test-token'
  auth.user = { id: 'user-1', name: 'Admin' }
  auth.principal = {
    role,
    permissions: [],
    permissionsResolved: true,
  }
  auth.hydrated = true
  return auth
}

describe('usePlatformFeaturesStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('defaults audio recording to disabled and fails closed before load', () => {
    expect(defaultExperimentalFeatures()).toEqual({
      version: 1,
      attendanceAudioRecording: false,
    })
    authenticate()
    const store = usePlatformFeaturesStore()
    expect(store.attendanceAudioRecordingEnabled).toBe(false)
  })

  it('normalizes only an explicit true as enabled', () => {
    expect(
      normalizeExperimentalFeatures({
        version: 0,
        attendanceAudioRecording: 'true',
      }),
    ).toEqual({
      version: 1,
      attendanceAudioRecording: false,
    })
  })

  it('hydrates the authoritative GET response', async () => {
    authenticate()
    fetchMock().mockResolvedValue({
      features: { version: 1, attendanceAudioRecording: true },
      updatedAt: '2026-07-24T12:00:00Z',
      updatedBy: 'user-1',
    })
    const store = usePlatformFeaturesStore()

    await expect(store.load()).resolves.toBe(true)

    expect(store.loaded).toBe(true)
    expect(store.attendanceAudioRecordingEnabled).toBe(true)
    expect(fetchMock()).toHaveBeenCalledWith(
      '/v1/platform/experimental-features',
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: 'Bearer test-token' }),
      }),
    )
  })

  it('rehydrates from PUT response without applying an optimistic value', async () => {
    authenticate()
    fetchMock().mockResolvedValue({
      features: { version: 1, attendanceAudioRecording: false },
      updatedAt: '2026-07-24T12:00:00Z',
      updatedBy: 'user-1',
    })
    const store = usePlatformFeaturesStore()

    await expect(store.save({ version: 1, attendanceAudioRecording: true })).resolves.toBe(true)

    expect(store.features.attendanceAudioRecording).toBe(false)
    const [, options] = fetchMock().mock.calls[0]
    expect(options.method).toBe('PUT')
    expect(JSON.parse(options.body)).toEqual({
      features: { version: 1, attendanceAudioRecording: true },
    })
  })

  it('blocks non-platform admins before any request', async () => {
    authenticate('owner')
    const store = usePlatformFeaturesStore()

    await expect(store.save({ version: 1, attendanceAudioRecording: true })).resolves.toBe(false)

    expect(fetchMock()).not.toHaveBeenCalled()
  })
})
