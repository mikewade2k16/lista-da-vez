import type { TaskProjectClientScopeMode } from '../types/tasks'

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

export interface TaskClientScopeOption {
  value: string
  active: boolean
}

export function normalizeTaskClientScopeMode(value: unknown): TaskProjectClientScopeMode {
  const mode = String(value ?? '')
    .trim()
    .toLowerCase()
  if (mode === 'all' || mode === 'selected') return mode
  return 'active'
}

export function normalizeTaskClientScopeIds(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  const seen = new Set<string>()
  const result: string[] = []
  for (const item of value) {
    const clientId = String(item ?? '')
      .trim()
      .toLowerCase()
      .slice(0, 80)
    if (!UUID_PATTERN.test(clientId) || seen.has(clientId)) continue
    seen.add(clientId)
    result.push(clientId)
  }
  return result
}

export function taskClientVisibleInScope(
  client: TaskClientScopeOption,
  mode: TaskProjectClientScopeMode,
  selectedClientIds: readonly string[],
): boolean {
  if (mode === 'all') return true
  if (mode === 'selected') return selectedClientIds.includes(client.value)
  return client.active
}
