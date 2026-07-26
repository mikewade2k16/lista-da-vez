import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useCoreAccountStore } from '../../layers/core/stores/account'
import { setApiAccountIdProvider } from '~/utils/api-client'
import { useAuthStore } from './auth'
import { useSocialPublishingPortfolioStore } from './social-publishing-portfolio'

function fetchMock() {
  return (globalThis as typeof globalThis & { $fetch: ReturnType<typeof vi.fn> }).$fetch
}

function authenticate(permissions = ['social_publishing.view', 'social_publishing.analytics']) {
  const auth = useAuthStore()
  auth.accessToken = 'test-token'
  auth.user = { id: 'user-1', name: 'Operador' }
  auth.principal = {
    role: 'owner',
    permissions,
    permissionsResolved: true,
  }
  auth.hydrated = true

  const account = useCoreAccountStore()
  account.accounts = [
    {
      id: 'agency-a',
      name: 'Agência A',
      slug: 'agency-a',
      organizationId: 'org-a',
      planCode: 'agency',
      modules: ['social_publishing'],
      isAgency: true,
      organizationName: 'Organização A',
    },
    {
      id: 'client-a',
      name: 'Cliente A',
      slug: 'client-a',
      organizationId: 'org-a',
      planCode: 'client',
      modules: ['social_publishing'],
      isAgency: false,
      organizationName: 'Organização A',
    },
  ]
  account.activeAccountId = 'client-a'
  setApiAccountIdProvider(() => account.activeAccountId)
  return account
}

describe('useSocialPublishingPortfolioStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    authenticate()
  })

  afterEach(() => {
    setApiAccountIdProvider(null)
  })

  it('loads selectable scope through the agency host while keeping the active client', async () => {
    fetchMock().mockResolvedValue({
      canSelect: true,
      clients: [{ id: 'client-a', name: 'Cliente A' }],
    })
    const store = useSocialPublishingPortfolioStore()

    await expect(store.loadScope()).resolves.toMatchObject({ canSelect: true })

    expect(store.scopeHostId).toBe('agency-a')
    expect(store.selectedClientId).toBe('client-a')
    expect(fetchMock()).toHaveBeenCalledWith(
      '/v1/social-publishing/scope',
      expect.objectContaining({
        headers: expect.objectContaining({ 'X-Account-Id': 'agency-a' }),
      }),
    )
  })

  it('does not request a consolidated portfolio without analytics permission', async () => {
    const auth = useAuthStore()
    auth.principal = {
      role: 'owner',
      permissions: ['social_publishing.view'],
      permissionsResolved: true,
    }
    const account = useCoreAccountStore()
    account.activeAccountId = 'agency-a'
    fetchMock().mockResolvedValue({
      canSelect: true,
      clients: [{ id: 'client-a', name: 'Cliente A' }],
    })
    const store = useSocialPublishingPortfolioStore()

    await store.loadScope()
    await expect(store.loadPortfolio()).resolves.toBe(false)

    expect(fetchMock().mock.calls.some(([path]) => path.endsWith('/portfolio'))).toBe(false)
  })

  it('rejects a portfolio row outside the view-authorized scope', async () => {
    const account = useCoreAccountStore()
    account.activeAccountId = 'agency-a'
    fetchMock().mockImplementation((path: string) => {
      if (path.endsWith('/scope')) {
        return Promise.resolve({
          canSelect: true,
          clients: [{ id: 'client-a', name: 'Cliente A' }],
        })
      }
      if (path.endsWith('/portfolio')) {
        return Promise.resolve({
          clientCount: 1,
          connectedClients: 1,
          clients: [{ accountId: 'client-forged', connected: true }],
        })
      }
      return Promise.reject(new Error(`Unexpected path: ${path}`))
    })
    const store = useSocialPublishingPortfolioStore()

    await store.loadScope()
    await expect(store.loadPortfolio()).resolves.toBe(false)

    expect(store.portfolio).toBeNull()
    expect(store.error).toContain('escopo de clientes inválido')
  })

  it('discards an old scope response after an account switch', async () => {
    let resolveScope: ((value: unknown) => void) | null = null
    fetchMock().mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveScope = resolve
        }),
    )
    const account = useCoreAccountStore()
    const store = useSocialPublishingPortfolioStore()
    const pending = store.loadScope()

    await vi.waitFor(() => expect(resolveScope).toBeTypeOf('function'))
    account.activeAccountId = 'agency-a'
    store.prepareAccountSwitch()
    resolveScope?.({
      canSelect: true,
      clients: [{ id: 'client-a', name: 'Cliente A' }],
    })

    await expect(pending).resolves.toBeNull()
    expect(store.scope).toBeNull()
    expect(store.loadingScope).toBe(false)
  })
})
