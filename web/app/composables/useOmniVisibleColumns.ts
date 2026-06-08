import type { ComputedRef, Ref } from 'vue'
import type { OmniTableColumn } from '~/types/omni/collection'

interface UseOmniVisibleColumnsOptions {
  preferenceKey: string
  allColumns: Ref<OmniTableColumn[]> | ComputedRef<OmniTableColumn[]>
  columnExcludeKeys?: Iterable<string> | string[]
  // DEPRECATED: usar `column.locked = true` nas definicoes ou ajustar via UI.
  // Mantido por compat com workspaces antigos. Quando presente, vira default
  // para `lockedColumnKeys` (admin ainda pode destravar via UI).
  alwaysVisibleColumnKeys?: Iterable<string>
  defaultVisibleColumnKeys?: Iterable<string> | string[]
}

function sameStringArray(a: string[], b: string[]) {
  if (a.length !== b.length) return false
  return a.every((value, index) => value === b[index])
}

function normalizeStringIterable(value: unknown) {
  if (!value) return [] as string[]
  if (Array.isArray(value)) {
    return value.map((item) => String(item ?? '').trim()).filter(Boolean)
  }
  if (typeof value === 'string') {
    const normalized = value.trim()
    return normalized ? [normalized] : []
  }
  if (typeof value === 'object' && Symbol.iterator in (value as object)) {
    return Array.from(value as Iterable<unknown>)
      .map((item) => String(item ?? '').trim())
      .filter(Boolean)
  }
  return []
}

function sanitizeKeys(current: string[], allowed: string[]) {
  const allowedSet = new Set(allowed)
  return current.map((item) => String(item ?? '').trim()).filter((item) => allowedSet.has(item))
}

// Sort columns by: order array first (if present in order), then defaultOrder,
// then original index. Stable, idempotente.
function orderColumns(columns: OmniTableColumn[], order: string[]): OmniTableColumn[] {
  if (order.length === 0) {
    // Sem ordem custom — usa defaultOrder declarado nas colunas.
    return [...columns].sort((a, b) => {
      const orderA = a.defaultOrder ?? Number.MAX_SAFE_INTEGER
      const orderB = b.defaultOrder ?? Number.MAX_SAFE_INTEGER
      return orderA - orderB
    })
  }
  const orderIndex = new Map<string, number>()
  order.forEach((key, idx) => orderIndex.set(key, idx))
  return [...columns].sort((a, b) => {
    const idxA = orderIndex.has(a.key) ? orderIndex.get(a.key)! : Number.MAX_SAFE_INTEGER
    const idxB = orderIndex.has(b.key) ? orderIndex.get(b.key)! : Number.MAX_SAFE_INTEGER
    if (idxA !== idxB) return idxA - idxB
    const defA = a.defaultOrder ?? Number.MAX_SAFE_INTEGER
    const defB = b.defaultOrder ?? Number.MAX_SAFE_INTEGER
    return defA - defB
  })
}

export function useOmniVisibleColumns(options: UseOmniVisibleColumnsOptions) {
  const excludeSet = new Set(normalizeStringIterable(options.columnExcludeKeys))
  // Defaults declarados pelo workspace: alwaysVisible (legado) + column.locked=true.
  const declaredLockedKeys = computed(() => {
    const legacy = normalizeStringIterable(options.alwaysVisibleColumnKeys)
    const fromColumns = options.allColumns.value.filter((c) => c.locked === true).map((c) => c.key)
    return [...new Set([...legacy, ...fromColumns])]
  })
  const preferredVisibleDefaults = normalizeStringIterable(options.defaultVisibleColumnKeys)

  const { ensureLoaded, readStringArray, writeStringArray } = useAdminPreferences()

  const visibleColumnKeys = ref<string[]>([])
  const lockedColumnKeys = ref<string[]>([])
  const columnOrder = ref<string[]>([])
  const hydratedFromPreferences = ref(false)

  const allowedColumnKeys = computed(() =>
    options.allColumns.value
      .filter((column) => !excludeSet.has(column.key))
      .map((column) => column.key),
  )

  const fallbackVisibleKeys = computed(() => {
    if (preferredVisibleDefaults.length === 0) {
      return [...allowedColumnKeys.value]
    }
    const allowedSet = new Set(allowedColumnKeys.value)
    const preferred = preferredVisibleDefaults.filter((key) => allowedSet.has(key))
    return preferred.length > 0 ? preferred : [...allowedColumnKeys.value]
  })

  watch(
    allowedColumnKeys,
    (allowedKeys) => {
      const fallbackKeys = fallbackVisibleKeys.value
      if (visibleColumnKeys.value.length === 0) {
        visibleColumnKeys.value = [...fallbackKeys]
      } else {
        const sanitized = sanitizeKeys(visibleColumnKeys.value, allowedKeys)
        const next = sanitized.length === 0 ? [...fallbackKeys] : sanitized
        if (!sameStringArray(next, visibleColumnKeys.value)) {
          visibleColumnKeys.value = next
        }
      }
      // Re-sanitiza locked + order quando colunas mudam.
      const allKeys = options.allColumns.value.map((c) => c.key)
      const sanitizedLocked = sanitizeKeys(lockedColumnKeys.value, allKeys)
      if (!sameStringArray(sanitizedLocked, lockedColumnKeys.value)) {
        lockedColumnKeys.value = sanitizedLocked
      }
      const sanitizedOrder = sanitizeKeys(columnOrder.value, allKeys)
      if (!sameStringArray(sanitizedOrder, columnOrder.value)) {
        columnOrder.value = sanitizedOrder
      }
    },
    { immediate: true },
  )

  // Colunas finais para a tabela: aplica ordem (custom ou defaultOrder) e
  // filtra mantendo as locked (excluded sempre passam, mantendo 'actions' etc).
  const tableColumns = computed(() => {
    const visibleSet = new Set(visibleColumnKeys.value)
    const lockedSet = new Set(lockedColumnKeys.value)
    const ordered = orderColumns(options.allColumns.value, columnOrder.value)
    return ordered.filter(
      (column) =>
        excludeSet.has(column.key) || lockedSet.has(column.key) || visibleSet.has(column.key),
    )
  })

  onMounted(async () => {
    if (import.meta.server) return
    await ensureLoaded()

    const allowed = allowedColumnKeys.value
    const allKeys = options.allColumns.value.map((c) => c.key)
    const fallback = fallbackVisibleKeys.value

    const savedVisible = readStringArray(['ui', 'columns', options.preferenceKey], fallback)
    visibleColumnKeys.value = (() => {
      const sanitized = sanitizeKeys(savedVisible, allowed)
      return sanitized.length === 0 ? [...fallback] : sanitized
    })()

    const savedLocked = readStringArray(
      ['ui', 'columns_locked', options.preferenceKey],
      declaredLockedKeys.value,
    )
    lockedColumnKeys.value = sanitizeKeys(savedLocked, allKeys)

    const savedOrder = readStringArray(['ui', 'columns_order', options.preferenceKey], [])
    columnOrder.value = sanitizeKeys(savedOrder, allKeys)

    hydratedFromPreferences.value = true
  })

  watch(
    visibleColumnKeys,
    (next) => {
      if (!hydratedFromPreferences.value || import.meta.server) return
      const allowed = allowedColumnKeys.value
      const sanitized = sanitizeKeys(next, allowed)
      const fallback = fallbackVisibleKeys.value
      const finalValue = sanitized.length === 0 ? [...fallback] : sanitized
      if (!sameStringArray(finalValue, next)) {
        visibleColumnKeys.value = finalValue
        return
      }
      writeStringArray(['ui', 'columns', options.preferenceKey], finalValue)
    },
    { deep: true },
  )

  watch(
    lockedColumnKeys,
    (next) => {
      if (!hydratedFromPreferences.value || import.meta.server) return
      writeStringArray(['ui', 'columns_locked', options.preferenceKey], next)
    },
    { deep: true },
  )

  watch(
    columnOrder,
    (next) => {
      if (!hydratedFromPreferences.value || import.meta.server) return
      writeStringArray(['ui', 'columns_order', options.preferenceKey], next)
    },
    { deep: true },
  )

  function resetToDefaults() {
    visibleColumnKeys.value = [...fallbackVisibleKeys.value]
    lockedColumnKeys.value = [...declaredLockedKeys.value]
    columnOrder.value = []
  }

  return {
    visibleColumnKeys,
    lockedColumnKeys,
    columnOrder,
    tableColumns,
    resetToDefaults,
  }
}
