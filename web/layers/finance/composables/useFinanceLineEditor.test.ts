import { describe, expect, it, vi } from 'vitest'
import { useFinanceLineEditor } from './useFinanceLineEditor'
import { makeLine, lineTotal, ensureLineAdjustments } from '../utils/finance-helpers'

describe('exclusão de ajustes financeiros', () => {
  it.each([15, -5])('remove o último ajuste de %s sem recriar legado', (amount) => {
    const persist = vi.fn()
    const editor = useFinanceLineEditor(persist)
    const row = makeLine()
    row.amount = 100
    row.adjustmentAmount = amount
    row.adjustments = [{ id: 'adjustment', amount, note: 'Teste', date: '2026-11-01' }]

    editor.removeLineAdjustment(row, 'adjustment')

    expect(row.adjustments).toEqual([])
    expect(row.adjustmentAmount).toBe(0)
    expect(lineTotal(row)).toBe(100)
    expect(persist).toHaveBeenCalledOnce()
  })

  it('preserva e recalcula os ajustes restantes', () => {
    const row = makeLine()
    row.adjustmentAmount = 10
    row.adjustments = [
      { id: 'positive', amount: 15, note: '', date: '' },
      { id: 'negative', amount: -5, note: '', date: '' },
    ]
    useFinanceLineEditor(vi.fn()).removeLineAdjustment(row, 'positive')
    expect(row.adjustments.map((item) => item.id)).toEqual(['negative'])
    expect(row.adjustmentAmount).toBe(-5)
  })

  it('mantém compatibilidade com dados antigos e permite excluir seu ajuste', () => {
    const row = makeLine()
    row.amount = 100
    row.adjustmentAmount = 15
    ensureLineAdjustments(row)
    expect(row.adjustments[0]?.note).toBe('Ajuste legado')
    useFinanceLineEditor(vi.fn()).removeLineAdjustment(row, row.adjustments[0]!.id)
    ensureLineAdjustments(row)
    expect(row.adjustments).toEqual([])
    expect(lineTotal(row)).toBe(100)
  })
})
