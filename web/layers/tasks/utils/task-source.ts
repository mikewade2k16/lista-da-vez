import type { TaskProjectTaskSourceMode } from '../types/tasks'

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

export function normalizeTaskSourceMode(value: unknown): TaskProjectTaskSourceMode {
  const mode = String(value ?? '')
    .trim()
    .toLowerCase()
  return mode === 'all' || mode === 'selected' ? mode : 'own'
}

export function normalizeTaskSourceBoardIds(value: unknown, currentBoardId = ''): string[] {
  if (!Array.isArray(value)) return []
  const current = String(currentBoardId).trim().toLowerCase()
  const seen = new Set<string>()
  const ids: string[] = []
  value.forEach((item) => {
    const id = String(item ?? '')
      .trim()
      .toLowerCase()
    if (!UUID_PATTERN.test(id) || id === current || seen.has(id)) return
    seen.add(id)
    ids.push(id)
  })
  return ids
}
