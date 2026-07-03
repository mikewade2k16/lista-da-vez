import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useAuthStore } from './auth'
import { useCardapioStore } from './cardapio'

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

describe('useCardapioStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    authenticateSession()
  })

  it('accepts both response shapes for loadRestaurants and scopes the query', async () => {
    const fetchMock = getFetchMock()
    const store = useCardapioStore()

    fetchMock.mockResolvedValueOnce({ restaurants: [{ id: 'r1' }] })
    await store.loadRestaurants()
    expect(store.restaurants).toHaveLength(1)

    fetchMock.mockResolvedValueOnce([{ id: 'r1' }, { id: 'r2' }])
    await store.loadRestaurants()
    expect(store.restaurants).toHaveLength(2)

    fetchMock.mockResolvedValueOnce({ restaurants: [] })
    await store.loadRestaurants({ accountId: 'acc-1', q: 'pizza' })
    const [path] = fetchMock.mock.calls.at(-1)!
    expect(path).toContain('accountId=acc-1')
    expect(path).toContain('q=pizza')
  })

  it('records the error message on loadRestaurants failure without throwing', async () => {
    getFetchMock().mockRejectedValue(httpError(500, 'down'))
    const store = useCardapioStore()

    await store.loadRestaurants()
    expect(store.listError).toBe('down')
    expect(store.listPending).toBe(false)
  })

  it('loads the active restaurant with the account scope on every request', async () => {
    const fetchMock = getFetchMock()
    // $fetch sempre devolve Promise; o dedupe de GET do api-client faz
    // fetchPromise.finally(...), entao a implementacao do mock precisa ser async.
    fetchMock.mockImplementation(async (path: string) => {
      if (path.includes('/categories')) return { categories: [{ id: 'c1' }] }
      if (path.includes('/products')) return { products: [] }
      if (path.includes('/domains')) return { domains: [{ host: 'a.com', isPrimary: true }] }
      if (path.includes('/delivery-zones')) return { deliveryZones: [] }
      return { id: 'r1', name: 'Rest' }
    })
    const store = useCardapioStore()

    await store.loadRestaurant('r1', 'acc-9')

    expect(fetchMock).toHaveBeenCalledTimes(5)
    for (const [path] of fetchMock.mock.calls) {
      expect(path).toContain('accountId=acc-9')
    }
    expect(store.restaurant?.id).toBe('r1')
    expect(store.categories).toHaveLength(1)
    expect(store.primaryDomain).toBe('a.com')
    expect(store.detailError).toBe('')
  })

  it('resetActive clears the editor scope so the next list is unscoped', async () => {
    const fetchMock = getFetchMock()
    fetchMock.mockImplementation((path: string) => {
      if (path.includes('/categories')) return { categories: [] }
      if (path.includes('/products')) return { products: [] }
      if (path.includes('/domains')) return { domains: [] }
      if (path.includes('/delivery-zones')) return { deliveryZones: [] }
      return { id: 'r1', name: 'Rest' }
    })
    const store = useCardapioStore()

    await store.loadRestaurant('r1', 'acc-9')
    store.resetActive()
    expect(store.restaurant).toBeNull()

    fetchMock.mockClear()
    fetchMock.mockResolvedValue({ restaurants: [] })
    await store.loadRestaurants()
    const [path] = fetchMock.mock.calls[0]
    expect(path).not.toContain('accountId=')
  })

  it('accepts both response shapes for loadOrders and paginates', async () => {
    const fetchMock = getFetchMock()
    const store = useCardapioStore()

    fetchMock.mockResolvedValueOnce([{ id: 'o1' }, { id: 'o2' }])
    await store.loadOrders('r1')
    expect(store.orders.total).toBe(2)

    fetchMock.mockResolvedValueOnce({ orders: [{ id: 'o1' }], total: 42 })
    await store.loadOrders('r1')
    expect(store.orders.total).toBe(42)
    expect(store.orders.items).toHaveLength(1)
    const [path] = fetchMock.mock.calls.at(-1)!
    expect(path).toContain('page=1')
    expect(path).toContain('perPage=20')

    fetchMock.mockResolvedValueOnce({ orders: [], total: 0 })
    await store.loadOrders('r1', { status: 'pending' })
    expect(fetchMock.mock.calls.at(-1)![0]).toContain('status=pending')
  })

  it('records the error message on loadOrders failure without throwing', async () => {
    getFetchMock().mockRejectedValue(httpError(500, 'orders down'))
    const store = useCardapioStore()

    await store.loadOrders('r1')
    expect(store.ordersError).toBe('orders down')
  })

  it('replaces the matching order in place on updateOrderStatus', async () => {
    const fetchMock = getFetchMock()
    // Async: o dedupe de GET do api-client encadeia .finally na Promise do $fetch.
    fetchMock.mockImplementation(async (path: string) => {
      if (path.includes('/cardapio/orders/')) return { id: 'o1', status: 'done' }
      return { orders: [{ id: 'o1', status: 'pending' }], total: 1 }
    })
    const store = useCardapioStore()

    await store.loadOrders('r1')
    await store.updateOrderStatus('o1', 'done' as any)
    expect(store.orders.items[0].status).toBe('done')
  })

  it('returns the API error message on createRestaurant failure', async () => {
    getFetchMock().mockRejectedValue(httpError(409, 'Slug ja existe'))
    const store = useCardapioStore()

    await expect(store.createRestaurant({ slug: 'dup', name: 'Dup' })).resolves.toEqual({
      ok: false,
      message: 'Slug ja existe',
    })
  })
})
