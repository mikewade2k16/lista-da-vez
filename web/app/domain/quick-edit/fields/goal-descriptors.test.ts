import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { OperationGoalTarget } from '~/stores/operation-goals'
import type {
  ConsultantGoalFlags,
  GoalQuickEditContext,
  GoalsStoreLike,
  StoreGoalFlags,
} from '~/domain/quick-edit/fields/goalContext'
import { consultantMonthlyGoal } from '~/domain/quick-edit/fields/consultantMonthlyGoal'
import { storePaGoal } from '~/domain/quick-edit/fields/storePaGoal'
import { storeTicketGoal } from '~/domain/quick-edit/fields/storeTicketGoal'

// Mocks tipados da store de metas: cada teste sobrescreve o retorno de loadGoals.
type GoalsMock = {
  loadGoals: ReturnType<typeof vi.fn>
  createGoal: ReturnType<typeof vi.fn>
  updateGoal: ReturnType<typeof vi.fn>
}

function createGoalsMock(rows: OperationGoalTarget[] = []): GoalsMock {
  return {
    loadGoals: vi.fn(() => Promise.resolve(rows)),
    createGoal: vi.fn(() => Promise.resolve({ ok: true })),
    updateGoal: vi.fn(() => Promise.resolve({ ok: true })),
  }
}

function makeStoreFlags(overrides: Partial<StoreGoalFlags> = {}): StoreGoalFlags {
  return {
    storeGoalSource: 'none',
    missingStoreGoal: false,
    missingTicketGoal: false,
    missingPaGoal: false,
    splitConsultantCount: 0,
    avgTicketGoal: 0,
    paGoal: 0,
    storeGoal: 0,
    ...overrides,
  }
}

function makeConsultantFlags(overrides: Partial<ConsultantGoalFlags> = {}): ConsultantGoalFlags {
  return {
    goalSource: 'none',
    missingMonthlyGoal: false,
    missingTicketGoal: false,
    missingPaGoal: false,
    monthlyGoal: 0,
    ...overrides,
  }
}

// Linha de meta completa para os cenarios de update (campos preservados pelo upsert).
function makeGoalRow(overrides: Partial<OperationGoalTarget> = {}): OperationGoalTarget {
  return {
    id: 'goal-1',
    tenantId: 'tenant-1',
    month: '2026-06',
    scope: 'store',
    storeId: 'store-1',
    storeCode: 'S1',
    storeName: 'Loja 1',
    consultantId: '',
    consultantName: '',
    monthlyGoal: 1000,
    avgTicketGoal: 200,
    conversionGoal: 30,
    paGoal: 2,
    createdAt: '2026-06-01T00:00:00Z',
    updatedAt: '2026-06-01T00:00:00Z',
    ...overrides,
  }
}

function makeContext(
  goals: GoalsMock,
  overrides: {
    store?: Partial<StoreGoalFlags>
    consultant?: Partial<ConsultantGoalFlags>
    canManageGoalTargets?: boolean
    refreshCrm?: GoalQuickEditContext['refreshCrm']
    consultantId?: string
  } = {},
): GoalQuickEditContext {
  return {
    permission: { canManageGoalTargets: overrides.canManageGoalTargets ?? true },
    tenantId: 'tenant-1',
    storeId: 'store-1',
    consultantId: overrides.consultantId ?? 'consultant-9',
    month: '2026-06',
    store: makeStoreFlags(overrides.store),
    consultant: makeConsultantFlags(overrides.consultant),
    goals: goals as unknown as GoalsStoreLike,
    refreshCrm: overrides.refreshCrm ?? vi.fn(() => Promise.resolve()),
  }
}

describe('goal quick-edit descriptors', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('isMissing', () => {
    it('storeTicketGoal segue ctx.store.missingTicketGoal', () => {
      expect(
        storeTicketGoal.isMissing(
          makeContext(createGoalsMock(), { store: { missingTicketGoal: true } }),
        ),
      ).toBe(true)
      expect(
        storeTicketGoal.isMissing(
          makeContext(createGoalsMock(), { store: { missingTicketGoal: false } }),
        ),
      ).toBe(false)
    })

    it('storePaGoal segue ctx.store.missingPaGoal', () => {
      expect(
        storePaGoal.isMissing(makeContext(createGoalsMock(), { store: { missingPaGoal: true } })),
      ).toBe(true)
      expect(
        storePaGoal.isMissing(makeContext(createGoalsMock(), { store: { missingPaGoal: false } })),
      ).toBe(false)
    })

    it('consultantMonthlyGoal exige missingMonthlyGoal e goalSource diferente de own', () => {
      expect(
        consultantMonthlyGoal.isMissing(
          makeContext(createGoalsMock(), {
            consultant: { missingMonthlyGoal: true, goalSource: 'none' },
          }),
        ),
      ).toBe(true)
      // goalSource === 'own' suprime o aviso mesmo com missingMonthlyGoal true.
      expect(
        consultantMonthlyGoal.isMissing(
          makeContext(createGoalsMock(), {
            consultant: { missingMonthlyGoal: true, goalSource: 'own' },
          }),
        ),
      ).toBe(false)
      // sem o flag de gap, tambem nao avisa.
      expect(
        consultantMonthlyGoal.isMissing(
          makeContext(createGoalsMock(), {
            consultant: { missingMonthlyGoal: false, goalSource: 'none' },
          }),
        ),
      ).toBe(false)
    })
  })

  describe('warning', () => {
    it('expoe textos curtos por campo', () => {
      const ctx = makeContext(createGoalsMock())
      expect(storeTicketGoal.warning(ctx)).toBe('sem TM')
      expect(storePaGoal.warning(ctx)).toBe('sem PA')
      expect(consultantMonthlyGoal.warning(ctx)).toBe('sem Meta')
    })
  })

  describe('canEdit', () => {
    it('libera apenas quando canManageGoalTargets e true', () => {
      for (const field of [storeTicketGoal, storePaGoal, consultantMonthlyGoal]) {
        expect(field.canEdit({ canManageGoalTargets: true })).toBe(true)
        expect(field.canEdit({ canManageGoalTargets: false })).toBe(false)
      }
    })
  })

  describe('current', () => {
    it('storeTicketGoal devolve avgTicketGoal quando > 0, senao null', () => {
      expect(
        storeTicketGoal.current(makeContext(createGoalsMock(), { store: { avgTicketGoal: 250 } })),
      ).toBe(250)
      expect(
        storeTicketGoal.current(makeContext(createGoalsMock(), { store: { avgTicketGoal: 0 } })),
      ).toBeNull()
    })

    it('storePaGoal devolve paGoal quando > 0, senao null', () => {
      expect(storePaGoal.current(makeContext(createGoalsMock(), { store: { paGoal: 3 } }))).toBe(3)
      expect(
        storePaGoal.current(makeContext(createGoalsMock(), { store: { paGoal: 0 } })),
      ).toBeNull()
    })

    it('consultantMonthlyGoal devolve monthlyGoal quando > 0, senao null', () => {
      expect(
        consultantMonthlyGoal.current(
          makeContext(createGoalsMock(), { consultant: { monthlyGoal: 5000 } }),
        ),
      ).toBe(5000)
      expect(
        consultantMonthlyGoal.current(
          makeContext(createGoalsMock(), { consultant: { monthlyGoal: 0 } }),
        ),
      ).toBeNull()
    })
  })

  describe('save (upsertGoalScope)', () => {
    it('storeTicketGoal: sem linha existente faz createGoal no escopo store', async () => {
      const goals = createGoalsMock([])
      const ctx = makeContext(goals)

      await storeTicketGoal.save(199, ctx)

      expect(goals.updateGoal).not.toHaveBeenCalled()
      expect(goals.createGoal).toHaveBeenCalledTimes(1)
      expect(goals.createGoal).toHaveBeenCalledWith(
        expect.objectContaining({
          storeId: 'store-1',
          consultantId: '',
          month: '2026-06',
          avgTicketGoal: 199,
        }),
      )
    })

    it('storePaGoal: sem linha existente faz createGoal no escopo store com paGoal', async () => {
      const goals = createGoalsMock([])
      const ctx = makeContext(goals)

      await storePaGoal.save(4, ctx)

      expect(goals.createGoal).toHaveBeenCalledTimes(1)
      expect(goals.createGoal).toHaveBeenCalledWith(
        expect.objectContaining({ consultantId: '', paGoal: 4 }),
      )
    })

    it('consultantMonthlyGoal: sem linha existente faz createGoal no escopo consultant', async () => {
      const goals = createGoalsMock([])
      const ctx = makeContext(goals, { consultantId: 'consultant-9' })

      await consultantMonthlyGoal.save(7000, ctx)

      expect(goals.createGoal).toHaveBeenCalledTimes(1)
      expect(goals.createGoal).toHaveBeenCalledWith(
        expect.objectContaining({
          storeId: 'store-1',
          consultantId: 'consultant-9',
          monthlyGoal: 7000,
        }),
      )
    })

    it('storeTicketGoal: com linha do escopo faz updateGoal naquele id', async () => {
      const existing = makeGoalRow({ id: 'store-goal-7', scope: 'store', consultantId: '' })
      const goals = createGoalsMock([existing])
      const ctx = makeContext(goals)

      await storeTicketGoal.save(300, ctx)

      expect(goals.createGoal).not.toHaveBeenCalled()
      expect(goals.updateGoal).toHaveBeenCalledTimes(1)
      expect(goals.updateGoal).toHaveBeenCalledWith(
        'store-goal-7',
        expect.objectContaining({ avgTicketGoal: 300 }),
      )
    })

    it('storePaGoal: com linha do escopo faz updateGoal naquele id', async () => {
      const existing = makeGoalRow({ id: 'store-goal-7', scope: 'store', consultantId: '' })
      const goals = createGoalsMock([existing])
      const ctx = makeContext(goals)

      await storePaGoal.save(5, ctx)

      expect(goals.updateGoal).toHaveBeenCalledTimes(1)
      expect(goals.updateGoal).toHaveBeenCalledWith(
        'store-goal-7',
        expect.objectContaining({ paGoal: 5 }),
      )
    })

    it('consultantMonthlyGoal: com linha do consultor faz updateGoal naquele id', async () => {
      const existing = makeGoalRow({
        id: 'consultant-goal-3',
        scope: 'consultant',
        consultantId: 'consultant-9',
      })
      const goals = createGoalsMock([existing])
      const ctx = makeContext(goals, { consultantId: 'consultant-9' })

      await consultantMonthlyGoal.save(8000, ctx)

      expect(goals.createGoal).not.toHaveBeenCalled()
      expect(goals.updateGoal).toHaveBeenCalledTimes(1)
      expect(goals.updateGoal).toHaveBeenCalledWith(
        'consultant-goal-3',
        expect.objectContaining({ monthlyGoal: 8000 }),
      )
    })

    it('escopo store ignora uma linha de consultor ao resolver a meta existente', async () => {
      // Linha de consultor nao deve casar com o upsert de loja -> cai no createGoal.
      const consultantRow = makeGoalRow({
        id: 'consultant-goal-3',
        scope: 'consultant',
        consultantId: 'consultant-9',
      })
      const goals = createGoalsMock([consultantRow])
      const ctx = makeContext(goals)

      await storeTicketGoal.save(123, ctx)

      expect(goals.updateGoal).not.toHaveBeenCalled()
      expect(goals.createGoal).toHaveBeenCalledWith(
        expect.objectContaining({ consultantId: '', avgTicketGoal: 123 }),
      )
    })
  })

  describe('afterSave', () => {
    it('chama ctx.refreshCrm em cada descriptor', async () => {
      for (const field of [storeTicketGoal, storePaGoal, consultantMonthlyGoal]) {
        const refreshCrm = vi.fn(() => Promise.resolve())
        const ctx = makeContext(createGoalsMock(), { refreshCrm })
        await field.afterSave(ctx)
        expect(refreshCrm).toHaveBeenCalledTimes(1)
      }
    })
  })
})
