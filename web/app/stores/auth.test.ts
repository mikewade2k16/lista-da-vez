import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useAuthStore } from './auth'

describe('useAuthStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('only enables all-stores mode when the session can actually reach more than one store', () => {
    const store = useAuthStore()

    store.storeContext = [{ id: 'store-1' }] as any

    expect(store.setStoreScopeMode('all')).toBe('single')
    expect(store.storeScopeMode).toBe('single')

    store.storeContext = [{ id: 'store-1' }, { id: 'store-2' }] as any

    expect(store.setStoreScopeMode('all')).toBe('all')
    expect(store.storeScopeMode).toBe('all')
  })

  it('routes an authenticated user without any workspace to the universal profile page', () => {
    const store = useAuthStore()

    store.principal = {
      userId: 'user-without-permissions',
      role: '',
      permissions: [],
      permissionsResolved: true,
    } as any

    expect(store.allowedWorkspaces).toEqual([])
    expect(store.homeWorkspaceId).toBe('')
    expect(store.homePath).toBe('/perfil')
  })
})
