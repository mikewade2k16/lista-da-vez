import { describe, expect, it, vi } from 'vitest'

import {
  applyOperationSnapshotToState,
  applyRemoteStoreData,
  fetchRemoteStoreData,
} from './runtime-remote'

const baseState = () => ({
  activeStoreId: 'store-1',
  stores: [{ id: 'store-1', tenantId: 'tenant-1', name: 'Loja 1', code: 'L1' }],
  storeSnapshots: {},
  roster: [],
})

const leanSnapshot = () => ({
  storeId: 'store-1',
  roster: [
    {
      id: 'person-1',
      storeId: 'store-1',
      name: 'Daiane C.',
      role: 'Atendimento',
      initials: 'DC',
      color: '#168aad',
    },
  ],
  waitingList: [],
  activeServices: [],
  pausedEmployees: [],
})

describe('runtime-remote roster fallback', () => {
  it('uses the lean snapshot roster when the management roster is empty (operador sem /v1/consultants)', () => {
    const next = applyRemoteStoreData(baseState(), 'store-1', {}, [], leanSnapshot())

    expect(next.roster.map((person) => person.id)).toEqual(['person-1'])
    expect(next.roster[0].name).toBe('Daiane C.')
    // projecao enxuta: sem meta/comissao reais (default 0)
    expect(next.roster[0].monthlyGoal).toBe(0)
    expect(next.roster[0].commissionRate).toBe(0)
  })

  it('prefers the full management roster over the lean snapshot roster', () => {
    const consultants = [
      {
        id: 'person-9',
        storeId: 'store-1',
        name: 'Gerente',
        role: 'Atendimento',
        initials: 'GE',
        color: '#fff',
        monthlyGoal: 50000,
        commissionRate: 3.5,
      },
    ]

    const next = applyRemoteStoreData(baseState(), 'store-1', {}, consultants, leanSnapshot())

    expect(next.roster.map((person) => person.id)).toEqual(['person-9'])
    expect(next.roster[0].monthlyGoal).toBe(50000)
  })

  it('fills the active-store roster from the snapshot when no roster was loaded yet', () => {
    const next = applyOperationSnapshotToState(baseState(), 'store-1', leanSnapshot())

    expect(next.roster.map((person) => person.id)).toEqual(['person-1'])
  })
})

describe('fetchRemoteStoreData queue settings gate', () => {
  it('skips /v1/settings (no 403 / no degraded) when the account has no queue module', async () => {
    const apiRequest = vi.fn(async (path: string) => {
      if (path.startsWith('/v1/operations/snapshot')) return leanSnapshot()
      if (path.startsWith('/v1/consultants')) return { consultants: [] }
      return {}
    })

    const result = await fetchRemoteStoreData(apiRequest, 'store-1', 'tenant-1', {
      canFetchQueueSettings: false,
      canFetchConsultants: false,
    })

    const requestedPaths = apiRequest.mock.calls.map((call) => String(call[0]))
    expect(requestedPaths.some((path) => path.startsWith('/v1/settings'))).toBe(false)
    expect(result.settingsLoadState).toBe('skipped')
    expect(result.settingsBundle).toBeNull()
    expect(result.settingsErrorMessage).toBe('')
  })

  it('fetches /v1/settings as usual when the account has the queue module', async () => {
    const apiRequest = vi.fn(async (path: string) => {
      if (path.startsWith('/v1/operations/snapshot')) return leanSnapshot()
      if (path.startsWith('/v1/consultants')) return { consultants: [] }
      if (path.startsWith('/v1/settings')) return { storeId: 'store-1' }
      return {}
    })

    const result = await fetchRemoteStoreData(apiRequest, 'store-1', 'tenant-1', {
      canFetchQueueSettings: true,
      canFetchConsultants: false,
    })

    const requestedPaths = apiRequest.mock.calls.map((call) => String(call[0]))
    expect(requestedPaths.some((path) => path.startsWith('/v1/settings'))).toBe(true)
    expect(result.settingsLoadState).toBe('loaded')
  })
})

// Login blindado: nem o roster de gestao nem o snapshot da operacao podem derrubar
// o boot. Numa VPS de schema stale, /v1/consultants e /v1/operations/snapshot
// retornam 500 e ANTES jogavam o usuario de volta pro login com "Erro ao processar
// o consultor". Agora qualquer erro degrada para vazio e o login completa.
describe('fetchRemoteStoreData login resilience', () => {
  const apiError = (status: number) => Object.assign(new Error('boom'), { statusCode: status })

  it('degrades (no throw) when /v1/consultants returns 500 — login keeps going', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const apiRequest = vi.fn(async (path: string) => {
      if (path.startsWith('/v1/operations/snapshot')) return leanSnapshot()
      if (path.startsWith('/v1/consultants')) throw apiError(500)
      if (path.startsWith('/v1/settings')) return { storeId: 'store-1' }
      return {}
    })

    const result = await fetchRemoteStoreData(apiRequest, 'store-1', 'tenant-1', {
      canFetchQueueSettings: true,
      canFetchConsultants: true,
    })

    expect(result.consultantsLoadState).toBe('degraded')
    expect(result.consultants).toEqual([])
    expect(warn).toHaveBeenCalled()
    warn.mockRestore()
  })

  it('degrades (no throw) when /v1/operations/snapshot returns 500 — login keeps going', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const apiRequest = vi.fn(async (path: string) => {
      if (path.startsWith('/v1/operations/snapshot')) throw apiError(500)
      if (path.startsWith('/v1/consultants')) return { consultants: [] }
      if (path.startsWith('/v1/settings')) return { storeId: 'store-1' }
      return {}
    })

    const result = await fetchRemoteStoreData(apiRequest, 'store-1', 'tenant-1', {
      canFetchQueueSettings: true,
      canFetchConsultants: true,
    })

    expect(result.operationsSnapshotLoadState).toBe('degraded')
    expect(result.operationsSnapshot).toBeNull()
    expect(warn).toHaveBeenCalled()
    warn.mockRestore()
  })
})
