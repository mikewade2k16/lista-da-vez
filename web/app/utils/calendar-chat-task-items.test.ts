import { describe, expect, it } from 'vitest'
import { taskChecklistToday } from '../../layers/tasks/utils/task-checklist'
import { applyCalendarChatTaskItem } from './calendar-chat-task-items'

const base = [
  {
    id: 'item-1',
    title: 'Reel um',
    completed: true,
    completedDate: '2026-08-12',
    status: 'approval' as const,
    statusDate: '2026-08-11',
  },
]

describe('applyCalendarChatTaskItem', () => {
  it('cria com ID estável e não duplica no retry', () => {
    const created = applyCalendarChatTaskItem({
      action: 'create',
      items: base,
      item: { title: 'Reel dois', status: 'editing', statusDate: '2026-08-13' },
      createId: 'crow:message:0',
    })
    expect(created.error).toBe('')
    expect(created.items[1]).toEqual({
      id: 'crow:message:0',
      title: 'Reel dois',
      completed: false,
      status: 'editing',
      statusDate: '2026-08-13',
    })

    const retry = applyCalendarChatTaskItem({
      action: 'create',
      items: created.items,
      item: { title: 'Reel dois' },
      createId: 'crow:message:0',
    })
    expect(retry).toMatchObject({ changed: false, error: '' })
    expect(retry.items).toHaveLength(2)
  })

  it('atualiza somente os campos presentes e usa hoje por padrão', () => {
    const result = applyCalendarChatTaskItem({
      action: 'update',
      items: base,
      item: { id: 'item-1', status: 'posted' },
      createId: 'unused',
    })
    expect(result.items[0]).toEqual({
      ...base[0],
      status: 'posted',
      statusDate: taskChecklistToday(),
    })
  })

  it('preserva status ao desmarcar finalização e remove somente completedDate', () => {
    const result = applyCalendarChatTaskItem({
      action: 'update',
      items: base,
      item: { id: 'item-1', completed: false },
      createId: 'unused',
    })
    expect(result.items[0]).toEqual({
      id: 'item-1',
      title: 'Reel um',
      completed: false,
      status: 'approval',
      statusDate: '2026-08-11',
    })
  })

  it('renomeia e exclui pelo ID autoritativo', () => {
    const renamed = applyCalendarChatTaskItem({
      action: 'update',
      items: base,
      item: { id: 'item-1', title: 'Reel principal' },
      createId: 'unused',
    })
    expect(renamed.items[0]?.title).toBe('Reel principal')

    const removed = applyCalendarChatTaskItem({
      action: 'delete',
      items: renamed.items,
      item: { id: 'item-1' },
      createId: 'unused',
    })
    expect(removed.items).toEqual([])

    const retry = applyCalendarChatTaskItem({
      action: 'delete',
      items: removed.items,
      item: { id: 'item-1' },
      createId: 'unused',
    })
    expect(retry).toEqual({ items: [], changed: false, error: '' })
  })

  it('não altera a lista quando o item ou os dados são inválidos', () => {
    const missing = applyCalendarChatTaskItem({
      action: 'update',
      items: base,
      item: { id: 'unknown', completed: true },
      createId: 'unused',
    })
    expect(missing.changed).toBe(false)
    expect(missing.items).toEqual(base)

    const invalid = applyCalendarChatTaskItem({
      action: 'update',
      items: base,
      item: { id: 'item-1', status: 'unknown' as never },
      createId: 'unused',
    })
    expect(invalid.changed).toBe(false)
    expect(invalid.items).toEqual(base)
  })
})
