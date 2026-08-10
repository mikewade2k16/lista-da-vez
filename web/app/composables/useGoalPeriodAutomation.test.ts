import { ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { OperationGoalTarget } from '~/stores/operation-goals'
import { useGoalPeriodAutomation } from './useGoalPeriodAutomation'

const goalActions = vi.hoisted(() => ({
  createGoal: vi.fn(),
  updateGoal: vi.fn(),
}))

vi.mock('~/stores/operation-goals', () => ({
  useOperationGoalsStore: () => goalActions,
}))

function monthlyStoreGoal(): OperationGoalTarget {
  return {
    id: 'goal-month',
    tenantId: 'tenant-1',
    month: '2026-07',
    week: 0,
    scope: 'store',
    storeId: 'store-1',
    storeCode: 'S1',
    storeName: 'Loja 1',
    consultantId: '',
    consultantName: '',
    monthlyGoal: 120_000,
    avgTicketGoal: 850,
    conversionGoal: 30,
    paGoal: 1.9,
    createdAt: '',
    updatedAt: '',
  }
}

describe('goal period automation', () => {
  beforeEach(() => {
    goalActions.createGoal.mockReset().mockResolvedValue({ ok: true })
    goalActions.updateGoal.mockReset().mockResolvedValue({ ok: true })
  })

  it('creates weeks 1 to 4 when a monthly store goal is saved', async () => {
    const source = monthlyStoreGoal()
    const { syncMonthlyGoal } = useGoalPeriodAutomation({ goals: ref([source]) })

    await expect(syncMonthlyGoal(source)).resolves.toBe(true)
    expect(goalActions.createGoal).toHaveBeenCalledTimes(4)
    expect(goalActions.createGoal.mock.calls.map(([payload]) => payload.week)).toEqual([1, 2, 3, 4])
    expect(goalActions.createGoal.mock.calls.map(([payload]) => payload.monthlyGoal)).toEqual([
      30_000, 30_000, 30_000, 30_000,
    ])
    expect(
      goalActions.createGoal.mock.calls.every(
        ([payload]) => payload.avgTicketGoal === 850 && payload.paGoal === 1.9,
      ),
    ).toBe(true)
  })
})
