import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useAuthStore } from '~/stores/auth'
import { useTasksClientStore } from './tasks-client'

function getFetchMock() {
  return (globalThis as typeof globalThis & { $fetch: ReturnType<typeof vi.fn> }).$fetch
}

describe('useTasksClientStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    const auth = useAuthStore()
    auth.accessToken = 'test-token'
    auth.user = { id: 'user-1', name: 'Iasmin' }
    auth.principal = { role: 'consultant', permissions: [], permissionsResolved: true }
    auth.hydrated = true
  })

  it('loads the backend-scoped client catalog in an agency workspace regardless of coarse role', async () => {
    getFetchMock().mockResolvedValue({
      tenants: [{ id: 'client-1', name: 'Cliente permitido', isActive: true }],
    })
    const store = useTasksClientStore()

    await store.initialize(true)

    expect(getFetchMock()).toHaveBeenCalledWith(
      '/v1/tenants/clients?includeInactive=true',
      expect.any(Object),
    )
    expect(store.clientOptions).toEqual([
      { label: 'Cliente permitido', value: 'client-1', active: true },
    ])
    expect(store.clientOptionsSynced).toBe(true)
  })

  it('does not request or retain the agency catalog in a client-account workspace', async () => {
    getFetchMock().mockResolvedValue({
      tenants: [{ id: 'client-1', name: 'Cliente permitido', isActive: true }],
    })
    const store = useTasksClientStore()

    await store.initialize(true)
    await store.initialize(false)

    expect(getFetchMock()).toHaveBeenCalledTimes(1)
    expect(store.clientOptions).toEqual([])
    expect(store.clientOptionsSynced).toBe(false)
  })

  it('shares the in-flight request so page boot waits for the catalog', async () => {
    let release!: (value: unknown) => void
    getFetchMock().mockReturnValue(new Promise((resolve) => (release = resolve)))
    const store = useTasksClientStore()

    const first = store.initialize(true)
    const second = store.refreshClientOptions(true)
    expect(store.loadingClientOptions).toBe(true)

    release({ tenants: [] })
    await Promise.all([first, second])

    expect(store.loadingClientOptions).toBe(false)
    expect(getFetchMock()).toHaveBeenCalledTimes(1)
  })

  it('does not repopulate the catalog when client scope is activated during a request', async () => {
    let release!: (value: unknown) => void
    getFetchMock().mockReturnValue(new Promise((resolve) => (release = resolve)))
    const store = useTasksClientStore()

    const request = store.initialize(true)
    await store.initialize(false)
    release({ tenants: [{ id: 'hidden-client', name: 'Cliente oculto' }] })
    await request

    expect(store.clientOptions).toEqual([])
    expect(store.clientOptionsSynced).toBe(false)
  })
})
