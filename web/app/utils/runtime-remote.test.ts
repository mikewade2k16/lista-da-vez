import { describe, expect, it } from 'vitest'

import { applyOperationSnapshotToState, applyRemoteStoreData } from './runtime-remote'

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
