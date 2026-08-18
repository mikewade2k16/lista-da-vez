import { describe, expect, it } from 'vitest'
import {
  isTaskChecklistItemStatus,
  normalizeTaskChecklist,
  normalizeTaskChecklistDate,
  taskChecklistProgress,
  taskChecklistToday,
  taskChecklistYesterday,
  withTaskChecklistCompleted,
  withTaskChecklistStatus,
  withTaskChecklistStatusDate,
} from './task-checklist'

describe('normalizeTaskChecklist', () => {
  it('normaliza titulos, conclusao e remove ids duplicados', () => {
    expect(
      normalizeTaskChecklist([
        { id: 'a', title: '  Filme   um  ', completed: true },
        { id: 'a', title: 'Duplicado', completed: false },
        { id: 'b', title: 'Filme dois', completed: 'true' },
        { id: 'c', title: '   ', completed: true },
      ]),
    ).toEqual([
      { id: 'a', title: 'Filme um', completed: true },
      { id: 'b', title: 'Filme dois', completed: false },
    ])
  })

  it('calcula o progresso da tarefa', () => {
    expect(
      taskChecklistProgress([
        { id: 'a', title: 'Um', completed: true },
        { id: 'b', title: 'Dois', completed: true },
        { id: 'c', title: 'Tres', completed: false },
      ]),
    ).toEqual({ total: 3, completed: 2, percent: 67 })
  })

  it('preserva status e datas válidos sem preencher itens legados', () => {
    expect(
      normalizeTaskChecklist([
        { id: 'legacy', title: 'Item legado', completed: false },
        {
          id: 'content',
          title: 'Reel aprovado',
          completed: true,
          status: 'approved',
          statusDate: '2026-08-12',
          completedDate: '2026-08-13',
        },
      ]),
    ).toEqual([
      { id: 'legacy', title: 'Item legado', completed: false },
      {
        id: 'content',
        title: 'Reel aprovado',
        completed: true,
        status: 'approved',
        statusDate: '2026-08-12',
        completedDate: '2026-08-13',
      },
    ])
  })

  it('descarta status e datas inválidos ou incoerentes', () => {
    expect(
      normalizeTaskChecklist([
        {
          id: 'a',
          title: 'Sem metadados válidos',
          completed: false,
          status: 'unknown',
          statusDate: '2026-02-31',
          completedDate: '2026-08-13',
        },
        {
          id: 'b',
          title: 'Status sem data válida',
          completed: true,
          status: 'editing',
          statusDate: '13/08/2026',
          completedDate: '2026-13-01',
        },
      ]),
    ).toEqual([
      { id: 'a', title: 'Sem metadados válidos', completed: false },
      { id: 'b', title: 'Status sem data válida', completed: true, status: 'editing' },
    ])
  })

  it('calcula hoje e ontem no calendário local', () => {
    const baseDate = new Date(2026, 0, 1, 12, 0, 0)
    expect(taskChecklistToday(baseDate)).toBe('2026-01-01')
    expect(taskChecklistYesterday(baseDate)).toBe('2025-12-31')
  })

  it('valida datas de calendário reais no formato ISO curto', () => {
    expect(normalizeTaskChecklistDate('2024-02-29')).toBe('2024-02-29')
    expect(normalizeTaskChecklistDate('2025-02-29')).toBeUndefined()
    expect(normalizeTaskChecklistDate('2026-8-13')).toBeUndefined()
  })

  it('expõe um guard estável para os status suportados', () => {
    expect(isTaskChecklistItemStatus('captured')).toBe(true)
    expect(isTaskChecklistItemStatus('posted')).toBe(true)
    expect(isTaskChecklistItemStatus('done')).toBe(false)
  })

  it('marca conclusão com data automática sem alterar o status', () => {
    const item = { id: 'a', title: 'Reel', completed: false, status: 'editing' as const }
    const completed = withTaskChecklistCompleted(item, true, new Date(2026, 7, 13, 12))
    expect(completed).toEqual({
      id: 'a',
      title: 'Reel',
      completed: true,
      status: 'editing',
      completedDate: '2026-08-13',
    })
    expect(withTaskChecklistCompleted(completed, false)).toEqual({
      id: 'a',
      title: 'Reel',
      completed: false,
      status: 'editing',
    })
  })

  it('aplica status com hoje, preserva a data no mesmo status e renova ao trocar', () => {
    const item = { id: 'a', title: 'Reel', completed: false }
    const editing = withTaskChecklistStatus(item, 'editing', new Date(2026, 7, 13, 12))
    expect(editing).toEqual({
      id: 'a',
      title: 'Reel',
      completed: false,
      status: 'editing',
      statusDate: '2026-08-13',
    })

    const yesterday = withTaskChecklistStatusDate(editing, '2026-08-12')
    expect(withTaskChecklistStatus(yesterday, 'editing')).toEqual(yesterday)
    expect(withTaskChecklistStatus(yesterday, 'approval', '2026-08-14')).toEqual({
      id: 'a',
      title: 'Reel',
      completed: false,
      status: 'approval',
      statusDate: '2026-08-14',
    })
  })

  it('permite limpar a data ou o status sem mexer no checkbox', () => {
    const item = {
      id: 'a',
      title: 'Reel',
      completed: true,
      completedDate: '2026-08-13',
      status: 'posted' as const,
      statusDate: '2026-08-12',
    }
    expect(withTaskChecklistStatusDate(item, '')).toEqual({
      id: 'a',
      title: 'Reel',
      completed: true,
      completedDate: '2026-08-13',
      status: 'posted',
    })
    expect(withTaskChecklistStatus(item, null)).toEqual({
      id: 'a',
      title: 'Reel',
      completed: true,
      completedDate: '2026-08-13',
    })
  })
})
