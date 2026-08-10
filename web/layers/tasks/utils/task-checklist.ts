import type { TaskChecklistItem } from '../types/tasks'
import { normalizeText } from './text'

export const TASK_CHECKLIST_MAX_ITEMS = 200

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
    items.push({ id, title, completed: raw.completed === true })
  }
  return items
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
