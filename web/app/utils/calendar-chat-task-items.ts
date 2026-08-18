import type {
  CalendarChatProposalAction,
  CalendarChatProposalTaskItem,
} from '~/domain/calendar/calendar-chat-api'
import type { TaskChecklistItem } from '../../layers/tasks/types/tasks'
import {
  TASK_CHECKLIST_MAX_ITEMS,
  isTaskChecklistItemStatus,
  normalizeTaskChecklist,
  normalizeTaskChecklistDate,
  withTaskChecklistCompleted,
  withTaskChecklistStatus,
  withTaskChecklistStatusDate,
} from '../../layers/tasks/utils/task-checklist'
import { normalizeText } from '../../layers/tasks/utils/text'

export interface ApplyCalendarChatTaskItemInput {
  action: CalendarChatProposalAction
  items: unknown
  item: CalendarChatProposalTaskItem
  /** ID estavel derivado da mensagem/proposta; impede duplicacao num retry. */
  createId: string
}

export interface ApplyCalendarChatTaskItemResult {
  items: TaskChecklistItem[]
  changed: boolean
  error: string
}

function has(item: CalendarChatProposalTaskItem, key: keyof CalendarChatProposalTaskItem): boolean {
  return Object.prototype.hasOwnProperty.call(item, key)
}

function sameChecklist(left: TaskChecklistItem[], right: TaskChecklistItem[]): boolean {
  return JSON.stringify(left) === JSON.stringify(right)
}

function applyItemFields(
  source: TaskChecklistItem,
  fields: CalendarChatProposalTaskItem,
): TaskChecklistItem | null {
  let next = { ...source }

  if (has(fields, 'title')) {
    const title = normalizeText(fields.title, 220)
    if (!title) return null
    next.title = title
  }

  if (has(fields, 'status')) {
    if (!isTaskChecklistItemStatus(fields.status)) return null
    next = withTaskChecklistStatus(next, fields.status, fields.statusDate)
  } else if (has(fields, 'statusDate')) {
    const date = normalizeTaskChecklistDate(fields.statusDate)
    if (!date || !next.status) return null
    next = withTaskChecklistStatusDate(next, date)
  }

  if (has(fields, 'completed')) {
    if (typeof fields.completed !== 'boolean') return null
    next = withTaskChecklistCompleted(next, fields.completed, fields.completedDate)
  } else if (has(fields, 'completedDate')) {
    const date = normalizeTaskChecklistDate(fields.completedDate)
    if (!date || !next.completed) return null
    next = withTaskChecklistCompleted(next, true, date)
  }

  return normalizeTaskChecklist([next])[0] || null
}

/**
 * Aplica localmente UMA proposta confirmada do Crow. A funcao e pura: a escrita real
 * continua no Tasks store, com a versao/escopo da task pai.
 */
export function applyCalendarChatTaskItem(
  input: ApplyCalendarChatTaskItemInput,
): ApplyCalendarChatTaskItemResult {
  const current = normalizeTaskChecklist(input.items)
  const itemID = normalizeText(input.item.id, 120)

  if (input.action === 'create') {
    const title = normalizeText(input.item.title, 220)
    const createID = normalizeText(input.createId, 120)
    if (!title || !createID) {
      return { items: current, changed: false, error: 'A proposta não tem um item válido.' }
    }
    const alreadyCreated = current.find((item) => item.id === createID)
    if (alreadyCreated) {
      return { items: current, changed: false, error: '' }
    }
    if (current.length >= TASK_CHECKLIST_MAX_ITEMS) {
      return { items: current, changed: false, error: 'Essa task já atingiu o limite de itens.' }
    }
    const created = applyItemFields(
      { id: createID, title, completed: false },
      { ...input.item, id: undefined, title },
    )
    if (!created) {
      return {
        items: current,
        changed: false,
        error: 'Os dados sugeridos para o item são inválidos.',
      }
    }
    return { items: [...current, created], changed: true, error: '' }
  }

  const index = current.findIndex((item) => item.id === itemID)
  if (index < 0) {
    // Delete e idempotente: a confirmacao pode ser repetida depois de a primeira chamada ter
    // removido o item, inclusive quando a resposta se perdeu. Update continua exigindo alvo real.
    if (input.action === 'delete') {
      return { items: current, changed: false, error: '' }
    }
    return { items: current, changed: false, error: 'Não encontrei esse item na task.' }
  }

  if (input.action === 'delete') {
    return { items: current.filter((item) => item.id !== itemID), changed: true, error: '' }
  }

  const updated = applyItemFields(current[index]!, input.item)
  if (!updated) {
    return {
      items: current,
      changed: false,
      error: 'Os dados sugeridos para o item são inválidos.',
    }
  }
  const items = current.map((item, itemIndex) => (itemIndex === index ? updated : item))
  return { items, changed: !sameChecklist(current, items), error: '' }
}
