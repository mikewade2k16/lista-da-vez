import { describe, expect, it } from 'vitest'
import { normalizeTaskChecklist, taskChecklistProgress } from './task-checklist'

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
})
