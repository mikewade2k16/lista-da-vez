import type { TaskChecklistItem, TaskChecklistItemStatus } from '../types/tasks'
import { normalizeText } from './text'

export const TASK_CHECKLIST_MAX_ITEMS = 200
export const TASK_CHECKLIST_STATUS_OPTIONS: ReadonlyArray<{
  value: TaskChecklistItemStatus
  label: string
}> = [
  { value: 'captured', label: 'Gravado' },
  { value: 'editing', label: 'Em edição' },
  { value: 'approval', label: 'Em aprovação' },
  { value: 'approved', label: 'Aprovado' },
  { value: 'scheduled', label: 'Agendado' },
  { value: 'posted', label: 'Postado' },
]

const TASK_CHECKLIST_STATUS_VALUES = new Set<TaskChecklistItemStatus>(
  TASK_CHECKLIST_STATUS_OPTIONS.map((option) => option.value),
)

export function isTaskChecklistItemStatus(value: unknown): value is TaskChecklistItemStatus {
  return TASK_CHECKLIST_STATUS_VALUES.has(normalizeText(value, 40) as TaskChecklistItemStatus)
}

export function normalizeTaskChecklistStatus(value: unknown): TaskChecklistItemStatus | undefined {
  const status = normalizeText(value, 40) as TaskChecklistItemStatus
  return isTaskChecklistItemStatus(status) ? status : undefined
}

export function normalizeTaskChecklistDate(value: unknown): string | undefined {
  const date = normalizeText(value, 10)
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date)
  if (!match) return undefined

  const year = Number(match[1])
  const month = Number(match[2])
  const day = Number(match[3])
  const parsed = new Date(year, month - 1, day)
  if (
    parsed.getFullYear() !== year ||
    parsed.getMonth() !== month - 1 ||
    parsed.getDate() !== day
  ) {
    return undefined
  }
  return date
}

export function taskChecklistLocalDate(offsetDays = 0, baseDate = new Date()): string {
  const date = new Date(baseDate.getTime())
  date.setDate(date.getDate() + offsetDays)
  return [
    String(date.getFullYear()).padStart(4, '0'),
    String(date.getMonth() + 1).padStart(2, '0'),
    String(date.getDate()).padStart(2, '0'),
  ].join('-')
}

export function taskChecklistToday(baseDate = new Date()): string {
  return taskChecklistLocalDate(0, baseDate)
}

export function taskChecklistYesterday(baseDate = new Date()): string {
  return taskChecklistLocalDate(-1, baseDate)
}

export function createTaskChecklistItemId(): string {
  return (
    globalThis.crypto?.randomUUID?.() || `item-${Date.now()}-${Math.random().toString(36).slice(2)}`
  )
}

export function normalizeTaskChecklist(value: unknown): TaskChecklistItem[] {
  if (!Array.isArray(value)) return []

  const seen = new Set<string>()
  const items: TaskChecklistItem[] = []
  for (const [index, item] of value.entries()) {
    if (!item || typeof item !== 'object' || items.length >= TASK_CHECKLIST_MAX_ITEMS) continue
    const raw = item as Record<string, unknown>
    const title = normalizeText(raw.title, 220)
    if (!title) continue
    const id = normalizeText(raw.id, 120) || `item-${index + 1}`
    if (seen.has(id)) continue
    seen.add(id)
    const completed = raw.completed === true
    const status = normalizeTaskChecklistStatus(raw.status)
    const statusDate = status ? normalizeTaskChecklistDate(raw.statusDate) : undefined
    const completedDate = completed ? normalizeTaskChecklistDate(raw.completedDate) : undefined
    items.push({
      id,
      title,
      completed,
      ...(status ? { status } : {}),
      ...(statusDate ? { statusDate } : {}),
      ...(completedDate ? { completedDate } : {}),
    })
  }
  return items
}

function normalizeTaskChecklistItem(item: TaskChecklistItem): TaskChecklistItem {
  return (
    normalizeTaskChecklist([item])[0] || {
      id: item.id,
      title: item.title,
      completed: Boolean(item.completed),
    }
  )
}

function resolveTaskChecklistActionDate(value?: string | Date): string {
  if (value instanceof Date) return taskChecklistToday(value)
  return normalizeTaskChecklistDate(value) || taskChecklistToday()
}

export function withTaskChecklistCompleted(
  item: TaskChecklistItem,
  completed: boolean,
  date?: string | Date,
): TaskChecklistItem {
  const normalized = normalizeTaskChecklistItem(item)
  if (!completed) {
    const { completedDate: _completedDate, ...withoutCompletedDate } = normalized
    return { ...withoutCompletedDate, completed: false }
  }
  return {
    ...normalized,
    completed: true,
    completedDate: date
      ? resolveTaskChecklistActionDate(date)
      : normalized.completedDate || taskChecklistToday(),
  }
}

export function withTaskChecklistStatus(
  item: TaskChecklistItem,
  status?: TaskChecklistItemStatus | null,
  date?: string | Date,
): TaskChecklistItem {
  const normalized = normalizeTaskChecklistItem(item)
  const nextStatus = normalizeTaskChecklistStatus(status)
  if (!nextStatus) {
    const { status: _status, statusDate: _statusDate, ...withoutStatus } = normalized
    return withoutStatus
  }
  return {
    ...normalized,
    status: nextStatus,
    statusDate: date
      ? resolveTaskChecklistActionDate(date)
      : normalized.status === nextStatus && normalized.statusDate
        ? normalized.statusDate
        : taskChecklistToday(),
  }
}

export function withTaskChecklistStatusDate(
  item: TaskChecklistItem,
  value: unknown,
): TaskChecklistItem {
  const normalized = normalizeTaskChecklistItem(item)
  if (!normalized.status) return normalized
  const statusDate = normalizeTaskChecklistDate(value)
  if (!statusDate) {
    const { statusDate: _statusDate, ...withoutStatusDate } = normalized
    return withoutStatusDate
  }
  return { ...normalized, statusDate }
}

export function taskChecklistProgress(value: unknown) {
  const items = normalizeTaskChecklist(value)
  const completed = items.filter((item) => item.completed).length
  return {
    total: items.length,
    completed,
    percent: items.length ? Math.round((completed / items.length) * 100) : 0,
  }
}
