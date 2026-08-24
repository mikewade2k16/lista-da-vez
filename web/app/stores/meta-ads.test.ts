import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import type { Mock } from 'vitest'

import { useCoreAccountStore } from '../../layers/core/stores/account'
import { useAuthStore } from './auth'
import { useMetaAdsStore } from './meta-ads'

interface DeferredFetch {
  path: string
  options: Record<string, unknown>
  resolve: (value: unknown) => void
}

function getFetchMock() {
  return (globalThis as typeof globalThis & { $fetch: Mock }).$fetch
}

function authenticateSession(): void {
  const auth = useAuthStore()
  auth.accessToken = 'test-token'
  auth.user = { id: 'user-1', name: 'Agency Owner' }
  auth.principal = { role: 'owner', permissions: [], permissionsResolved: true }
  auth.hydrated = true
  useCoreAccountStore().activeAccountId = 'account-1'
}

function deferredFetches(): DeferredFetch[] {
  const requests: DeferredFetch[] = []
  getFetchMock().mockImplementation(
    (path: string, options: Record<string, unknown> = {}) =>
      new Promise((resolve) => {
        requests.push({ path: String(path), options, resolve })
      }),
  )
  return requests
}

function resolveSelectionRequest(request: DeferredFetch, key: string): void {
  if (request.path.includes('/overview')) {
    request.resolve({
      connection: { connected: true, name: key },
      kpis: { spend: key === 'B' ? 200 : 100 },
      adAccountId: `ad-account-${key.toLowerCase()}`,
    })
    return
  }
  if (request.path.includes('/campaigns')) {
    request.resolve({ campaigns: [{ id: `campaign-${key}` }] })
    return
  }
  request.resolve({ insights: [{ date: key, spend: key === 'B' ? 20 : 10 }] })
}

describe('useMetaAdsStore account isolation', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    authenticateSession()
  })

  it('aborts and discards an old account response after reset', async () => {
    let resolveRequest: ((value: unknown) => void) | undefined
    getFetchMock().mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveRequest = resolve
        }),
    )
    const store = useMetaAdsStore()

    const oldLoad = store.init()
    const requestSignal = getFetchMock().mock.calls[0]?.[1]?.signal as AbortSignal
    expect(store.pending).toBe(true)

    store.resetState()
    expect(requestSignal.aborted).toBe(true)
    expect(store.pending).toBe(false)

    resolveRequest?.({
      connection: { connected: true, name: 'Conta antiga' },
      kpis: { spend: 99 },
    })
    await oldLoad

    expect(store.connection).toBeNull()
    expect(store.overview).toBeNull()
    expect(store.error).toBe('')
  })

  it('updates the client mapping through the canonical endpoint', async () => {
    const store = useMetaAdsStore()
    store.adAccounts = [
      {
        id: 'ad-account-1',
        metaAdAccountId: 'act_123',
        name: 'Conta principal',
        currency: 'BRL',
        status: 'ACTIVE',
        clientAccountId: null,
      },
    ]
    getFetchMock().mockResolvedValue({
      ...store.adAccounts[0],
      clientAccountId: 'client-1',
    })

    await store.setAdAccountClient('ad-account-1', 'client-1')

    const [path, options] = getFetchMock().mock.calls[0] ?? []
    expect(path).toContain('/v1/meta-ads/ad-accounts/ad-account-1/client')
    expect(options?.method).toBe('PATCH')
    expect(JSON.parse(options?.body as string)).toEqual({ clientAccountId: 'client-1' })
    expect(store.adAccounts[0]?.clientAccountId).toBe('client-1')
  })

  it('updates the Instagram identity mapping through the canonical endpoint', async () => {
    const store = useMetaAdsStore()
    store.instagramIdentities = [
      {
        igUserId: '17841400000000001',
        username: 'marca',
        pageId: '100000000000001',
        pageName: 'Marca',
        clientAccountId: null,
      },
    ]
    getFetchMock().mockResolvedValue({
      ...store.instagramIdentities[0],
      clientAccountId: 'client-1',
    })

    await store.setInstagramIdentityClient('17841400000000001', 'client-1')

    const [path, options] = getFetchMock().mock.calls[0] ?? []
    expect(path).toContain('/v1/meta-ads/instagram-identities/17841400000000001/client')
    expect(options?.method).toBe('PATCH')
    expect(JSON.parse(options?.body as string)).toEqual({ clientAccountId: 'client-1' })
    expect(store.instagramIdentities[0]?.clientAccountId).toBe('client-1')
  })

  it('discards Instagram identities loaded for an old account', async () => {
    const store = useMetaAdsStore()
    const requests = deferredFetches()

    const oldLoad = store.loadInstagramIdentities()
    expect(requests).toHaveLength(1)
    const request = requests[0]
    expect(request).toBeDefined()

    store.resetState()
    expect((request?.options.signal as AbortSignal | undefined)?.aborted).toBe(true)
    request?.resolve([
      {
        igUserId: '17841400000000001',
        username: 'old-account',
        pageId: '100000000000001',
        pageName: 'Old',
        clientAccountId: 'old-client',
      },
    ])
    await oldLoad

    expect(store.instagramIdentities).toEqual([])
    expect(store.instagramIdentityMappingError).toBe('')
  })

  it('keeps connection and sync actions closed for view-only users', async () => {
    const auth = useAuthStore()
    auth.principal = {
      role: 'consultant',
      permissions: ['meta_ads.view'],
      permissionsResolved: true,
    }
    const store = useMetaAdsStore()
    store.selectedAdAccountId = 'ad-account-1'
    getFetchMock().mockClear()

    await store.saveConnection('secret')
    await store.startConnectionOAuth()
    await store.deleteConnection()
    await store.sync()

    expect(store.canManageMetaAds).toBe(false)
    expect(store.canConnectMetaAds).toBe(false)
    expect(getFetchMock()).not.toHaveBeenCalled()
  })

  it('fails closed while effective permissions are unresolved', () => {
    const auth = useAuthStore()
    auth.principal = {
      role: 'consultant',
      permissions: ['meta_ads.manage', 'meta_ads.connect'],
      permissionsResolved: false,
    }

    const store = useMetaAdsStore()

    expect(store.canManageMetaAds).toBe(false)
    expect(store.canConnectMetaAds).toBe(false)
  })

  it('starts first-party OAuth through the account-scoped backend contract', async () => {
    const store = useMetaAdsStore()
    getFetchMock().mockResolvedValue({
      authorizationUrl: 'https://www.facebook.com/v24.0/dialog/oauth?state=opaque',
      expiresAt: '2026-08-18T12:10:00Z',
    })

    const result = await store.startConnectionOAuth()

    const [path, options] = getFetchMock().mock.calls[0] ?? []
    expect(path).toContain('/v1/meta-ads/oauth/start')
    expect(options?.method).toBe('POST')
    expect(result?.authorizationUrl).toContain('facebook.com')
    expect(store.connecting).toBe(false)
  })

  it('keeps only the latest ad account selection when responses arrive out of order', async () => {
    const store = useMetaAdsStore()
    const requests = deferredFetches()

    const firstSelection = store.selectAdAccount('ad-account-a')
    expect(requests).toHaveLength(3)
    const firstRequests = [...requests]

    const secondSelection = store.selectAdAccount('ad-account-b')
    expect(requests).toHaveLength(6)
    expect(
      firstRequests.every(
        (request) => (request.options.signal as AbortSignal | undefined)?.aborted === true,
      ),
    ).toBe(true)

    const secondRequests = requests.slice(3)
    for (const request of secondRequests) {
      expect(request.path).toContain('adAccountId=ad-account-b')
      expect(request.options.headers).toMatchObject({ 'X-Account-Id': 'account-1' })
      resolveSelectionRequest(request, 'B')
    }
    await secondSelection

    for (const request of firstRequests) resolveSelectionRequest(request, 'A')
    await firstSelection

    expect(store.selectedAdAccountId).toBe('ad-account-b')
    expect(store.overview?.adAccountId).toBe('ad-account-b')
    expect(store.campaigns[0]?.id).toBe('campaign-B')
    expect(store.insights[0]?.date).toBe('B')
    expect(store.pending).toBe(false)
  })

  it('keeps only the latest range when insight responses arrive out of order', async () => {
    const store = useMetaAdsStore()
    const requests = deferredFetches()
    store.selectedAdAccountId = 'ad-account-1'

    const firstRange = store.setRange('last_7d')
    expect(requests).toHaveLength(1)
    const firstRequest = requests[0]
    expect(firstRequest).toBeDefined()

    const secondRange = store.setRange('last_90d')
    expect(requests).toHaveLength(2)
    expect((firstRequest?.options.signal as AbortSignal | undefined)?.aborted).toBe(true)

    const secondRequest = requests[1]
    expect(secondRequest?.path).toContain('range=last_90d')
    secondRequest?.resolve({ insights: [{ date: 'new-range', spend: 90 }] })
    await secondRange

    firstRequest?.resolve({ insights: [{ date: 'old-range', spend: 7 }] })
    await firstRange

    expect(store.range).toBe('last_90d')
    expect(store.insights).toEqual([{ date: 'new-range', spend: 90 }])
    expect(store.pending).toBe(false)
  })
})
