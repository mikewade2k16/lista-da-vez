import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useAuthStore } from './auth'
import { useCoreAccountStore } from '../../layers/core/stores/account'

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

  it('uses calendar as the home for an agency account with calendar access', () => {
    const auth = useAuthStore()
    const account = useCoreAccountStore()

    Reflect.set(auth, 'principal', {
      userId: 'agency-user',
      role: 'owner',
      permissionsResolved: false,
    })
    account.accounts = [
      {
        id: 'crow-account',
        name: 'Crow Visuals',
        slug: 'crow',
        organizationId: 'crow-org',
        planCode: 'agency',
        modules: ['calendar'],
        isAgency: true,
        organizationName: 'Crow Visuals',
      },
    ]
    account.activeAccountId = 'crow-account'

    expect(auth.homeWorkspaceId).toBe('calendar')
    expect(auth.homePath).toBe('/calendario')
  })

  it('preserves the normal home for a client account', () => {
    const auth = useAuthStore()
    const account = useCoreAccountStore()

    Reflect.set(auth, 'principal', {
      userId: 'client-user',
      role: 'owner',
      permissionsResolved: false,
    })
    account.accounts = [
      {
        id: 'client-account',
        name: 'Cliente',
        slug: 'cliente',
        organizationId: 'crow-org',
        planCode: 'default',
        modules: ['queue'],
        isAgency: false,
        organizationName: 'Crow Visuals',
      },
    ]
    account.activeAccountId = 'client-account'

    expect(auth.homeWorkspaceId).toBe('operacao')
    expect(auth.homePath).toBe('/operacao')
  })

  it('does not force calendar when the agency role cannot access it', () => {
    const auth = useAuthStore()
    const account = useCoreAccountStore()

    Reflect.set(auth, 'principal', {
      userId: 'restricted-agency-user',
      role: 'manager',
      permissionsResolved: false,
    })
    account.accounts = [
      {
        id: 'crow-account',
        name: 'Crow Visuals',
        slug: 'crow',
        organizationId: 'crow-org',
        planCode: 'agency',
        modules: ['calendar'],
        isAgency: true,
        organizationName: 'Crow Visuals',
      },
    ]
    account.activeAccountId = 'crow-account'

    expect(auth.allowedWorkspaces).not.toContain('calendar')
    expect(auth.homeWorkspaceId).toBe('operacao')
    expect(auth.homePath).toBe('/operacao')
  })
})
