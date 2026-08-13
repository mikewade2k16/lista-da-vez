export function reorderMediaItems<T>(items: readonly T[], from: number, to: number): T[] {
  if (from === to || from < 0 || to < 0 || from >= items.length || to >= items.length) {
    return [...items]
  }

  const reordered = [...items]
  const [moved] = reordered.splice(from, 1)
  if (moved === undefined) return [...items]
  reordered.splice(to, 0, moved)
  return reordered
}

export function orderMediaItemsByIds<T extends { id: string }>(
  items: readonly T[],
  orderedIds: readonly string[],
): T[] {
  const byId = new Map(items.map((item) => [item.id, item]))
  const seen = new Set<string>()
  const ordered: T[] = []

  for (const id of orderedIds) {
    const item = byId.get(id)
    if (!item || seen.has(id)) continue
    ordered.push(item)
    seen.add(id)
  }
  for (const item of items) {
    if (seen.has(item.id)) continue
    ordered.push(item)
    seen.add(item.id)
  }
  return ordered
}
