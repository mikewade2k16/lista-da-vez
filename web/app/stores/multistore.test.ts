import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useAuthStore } from './auth'
import { useMultiStoreStore } from './multistore'

function getFetchMock() {
  return (globalThis as any).$fetch as ReturnType<typeof vi.fn>
}

function authenticateSession(partial: Record<string, unknown> = {}) {
  const auth = useAuthStore()
  auth.accessToken = 'test-token'
  auth.user = { id: 'user-1', name: 'Teste' } as any
  auth.principal = { role: 'owner', permissions: [], permissionsResolved: true } as any
  auth.hydrated = true // ensureSession() curto-circuita (auth.ts)
  auth.activeTenantId = 'tenant-1'
  Object.assign(auth, partial)
  return auth
}

const httpError = (status: number, message: string) =>
  Object.assign(new Error(message), { statusCode: status, data: { error: { message } } })

describe('useMultiStoreStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('fails fast on mutations without a session and never calls the API', async () => {
    const store = useMultiStoreStore()
    const fetchMock = getFetchMock()

    const unavailable = { ok: false, message: 'Sessao indisponivel.' }
    await expect(store.createStore({})).resolves.toEqual(unavailable)
    await expect(store.updateStore('s1', {})).resolves.toEqual(unavailable)
    await expect(store.archiveStore('s1')).resolves.toEqual(unavailable)
    await expect(store.deleteStore('s1')).resolves.toEqual(unavailable)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('normalizes managed stores and scopes the request to the tenant', async () => {
    authenticateSession()
    const fetchMock = getFetchMock()
    // O $fetch real do Nuxt sempre devolve Promise; apiRequest encadeia
    // fetchPromise.finally no caminho de dedupe de GET, entao o mock precisa
    // resolver de forma assincrona (nao retornar o objeto cru).
    fetchMock.mockImplementation((path: string) =>
      Promise.resolve(
        path.includes('/v1/stores')
          ? {
              stores: [
                { id: 's1', code: 'ab', storeType: 'SHOPPING', monthlyGoal: -5 },
                { id: 's2', storeType: 'mall' },
              ],
            }
          : {},
      ),
    )
    const store = useMultiStoreStore()

    const result = await store.refreshManagedStores()

    expect(result[0]).toEqual(
      expect.objectContaining({
        code: 'AB',
        storeType: 'shopping',
        monthlyGoal: 0,
        isActive: true,
      }),
    )
    expect(result[1].storeType).toBe('bairro')
    const [path] = fetchMock.mock.calls[0]
    expect(path).toContain('tenantId=tenant-1')
    expect(path).toContain('includeInactive=true')
  })

  it('degrades a 403 overview into a silent empty state', async () => {
    authenticateSession()
    getFetchMock().mockRejectedValue(httpError(403, 'forbidden'))
    const store = useMultiStoreStore()

    await expect(store.refreshOverview()).resolves.toBeNull()
    expect(store.overview).toBeNull()
    expect(store.errorMessage).toBe('')
    expect(store.ready).toBe(false)
  })

  it('rethrows non-403 overview errors and records the message', async () => {
    authenticateSession()
    getFetchMock().mockRejectedValue(httpError(500, 'db down'))
    const store = useMultiStoreStore()

    await expect(store.refreshOverview()).rejects.toBeTruthy()
    expect(store.errorMessage).toContain('db down')
  })

  it('validates the create payload before hitting the API', async () => {
    authenticateSession()
    const fetchMock = getFetchMock()
    fetchMock.mockImplementation((path: string) =>
      Promise.resolve(path.includes('/v1/stores') ? { stores: [] } : {}),
    )
    const store = useMultiStoreStore()

    await expect(store.createStore({ name: 'Loja' })).resolves.toEqual({
      ok: false,
      message: 'Preencha nome, codigo e tenant da loja.',
    })
    const postCalls = fetchMock.mock.calls.filter(
      ([path, options]: [string, any]) => path === '/v1/stores' && options?.method === 'POST',
    )
    expect(postCalls).toHaveLength(0)
  })

  it('sends only fields explicitly provided in a partial store update', async () => {
    authenticateSession()
    const fetchMock = getFetchMock()
    const currentStore = {
      id: 's1',
      tenantId: 'tenant-1',
      name: 'Loja Jardins',
      code: 'JAR',
      city: 'Aracaju',
      storeType: 'bairro',
      isActive: true,
    }
    fetchMock.mockImplementation((path: string, options?: Record<string, unknown>) => {
      if (path === '/v1/stores/s1' && options?.method === 'PATCH') {
        return Promise.resolve({ store: { ...currentStore, storeType: 'shopping' } })
      }
      if (path.includes('/v1/stores')) {
        return Promise.resolve({ stores: [currentStore] })
      }
      return Promise.resolve({})
    })
    const store = useMultiStoreStore()
    await store.refreshManagedStores()

    await expect(store.updateStore('s1', { storeType: 'shopping' })).resolves.toEqual(
      expect.objectContaining({ ok: true }),
    )

    const patchCall = fetchMock.mock.calls.find(
      ([path, options]: [string, any]) => path === '/v1/stores/s1' && options?.method === 'PATCH',
    )
    expect(JSON.parse(String(patchCall?.[1]?.body))).toEqual({ storeType: 'shopping' })
  })
})
