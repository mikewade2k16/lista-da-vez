import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useAuthStore } from './auth'
import { useBiStore } from './bi'

function fetchMock() {
  return (globalThis as typeof globalThis & { $fetch: ReturnType<typeof vi.fn> }).$fetch
}

const datasetCatalog = {
  datasets: [
    {
      id: 'inventario',
      label: 'Inventário',
      description: 'Movimentos',
      defaultLimit: 10,
      maxLimit: 25,
      defaultOrderBy: { field: 'data', direction: 'DESC' },
      allowedOrderFields: ['id', 'data'],
      filters: [{ field: 'itemSaldoId', valueType: 'integer', operators: ['eq'] }],
      requiredFilterRule: 'itemSaldoId obrigatório.',
      requiredFilterAlternatives: [[{ field: 'itemSaldoId', operator: 'eq' }]],
      dateRange: { field: 'data', maxDays: 31 },
    },
  ],
}

describe('useBiStore typed datasets', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    useAuthStore().hydrated = true
  })

  it('blocks every BI API action by default', async () => {
    const store = useBiStore()
    store.updateManualConfig({ companyKey: 'company', login: 'user', pass: 'secret' })
    const input = {
      pageNumber: 1,
      limit: 10,
      orderBy: { field: 'data', direction: 'DESC' as const },
      filters: [{ field: 'itemSaldoId', operator: 'eq', value: 99 }],
    }

    const results = await Promise.all([
      store.loginPerola(),
      store.refreshOverview(),
      store.loadPerolaDatasetCatalog(),
      store.queryPerolaDataset('inventario', input),
    ])

    expect(results.every((result) => result.blocked)).toBe(true)
    expect(fetchMock()).not.toHaveBeenCalled()
  })

  it('loads the safe catalog lazily and reuses it', async () => {
    fetchMock().mockResolvedValue(datasetCatalog)
    const store = useBiStore()
    store.setApiBlocked(false)

    expect(fetchMock()).not.toHaveBeenCalled()

    await store.loadPerolaDatasetCatalog()
    await store.loadPerolaDatasetCatalog()

    expect(store.datasetCatalog).toHaveLength(1)
    expect(fetchMock()).toHaveBeenCalledTimes(1)
    expect(fetchMock().mock.calls[0]?.[0]).toContain('/v1/bi/perola/datasets')
  })

  it('posts one typed page and stores only the structured response', async () => {
    fetchMock().mockResolvedValue({
      datasetId: 'inventario',
      datasetLabel: 'Inventário',
      pageNumber: 1,
      limit: 10,
      totalRecords: 2,
      totalPages: 1,
      returned: 2,
      hasMore: false,
      orderBy: { field: 'data', direction: 'DESC' },
      filterCount: 1,
      durationMs: 15,
      records: [{ id: 1 }, { id: 2 }],
    })
    const store = useBiStore()
    store.setApiBlocked(false)
    const input = {
      pageNumber: 1,
      limit: 10,
      orderBy: { field: 'data', direction: 'DESC' as const },
      filters: [{ field: 'itemSaldoId', operator: 'eq', value: 99 }],
    }

    const result = await store.queryPerolaDataset('inventario', input)

    expect(result.ok).toBe(true)
    expect(store.datasetQueryResponse?.returned).toBe(2)
    expect(fetchMock().mock.calls[0]?.[0]).toContain('/datasets/inventario/query')
    const request = fetchMock().mock.calls[0]?.[1] as { method?: string; body?: unknown }
    expect(request.method).toBe('POST')
    expect(JSON.parse(String(request.body))).toEqual(input)
  })

  it('aborts an in-flight BI request when the absolute block is enabled', async () => {
    fetchMock().mockImplementation(
      (_path: string, options: { signal?: AbortSignal } = {}) =>
        new Promise((_resolve, reject) => {
          options.signal?.addEventListener(
            'abort',
            () => {
              const error = new Error('aborted')
              error.name = 'AbortError'
              reject(error)
            },
            { once: true },
          )
        }),
    )
    const store = useBiStore()
    store.setApiBlocked(false)

    const request = store.loadPerolaDatasetCatalog()
    await vi.waitFor(() => expect(fetchMock()).toHaveBeenCalledTimes(1))
    store.setApiBlocked(true)
    const result = await request

    expect(result.blocked).toBe(true)
    expect(store.datasetCatalogLoading).toBe(false)
  })
})
