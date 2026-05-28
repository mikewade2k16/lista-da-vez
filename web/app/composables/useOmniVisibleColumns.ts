import type { ComputedRef, Ref } from 'vue'
import type { OmniTableColumn } from '~/types/omni/collection'

interface UseOmniVisibleColumnsOptions {
  preferenceKey: string
  allColumns: Ref<OmniTableColumn[]> | ComputedRef<OmniTableColumn[]>
  columnExcludeKeys?: Iterable<string> | string[]
  alwaysVisibleColumnKeys?: Iterable<string>
  defaultVisibleColumnKeys?: Iterable<string> | string[]
}

function sameStringArray(a: string[], b: string[]) {
  if (a.length !== b.length) {
    return false
  }

  return a.every((value, index) => value === b[index])
}

function sanitizeVisibleColumns(current: string[], allowedKeys: string[], fallbackKeys: string[]) {
  const allowedSet = new Set(allowedKeys)
  const cleaned = current
    .map((item) => String(item ?? '').trim())
    .filter((item) => allowedSet.has(item))

  if (cleaned.length > 0) {
    return cleaned
  }

  return [...fallbackKeys]
}

function normalizeStringIterable(value: unknown) {
  if (!value) {
    return [] as string[]
  }

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

export function useOmniVisibleColumns(options: UseOmniVisibleColumnsOptions) {
  const excludeSet = new Set(normalizeStringIterable(options.columnExcludeKeys))
  const alwaysVisibleSet = new Set(normalizeStringIterable(options.alwaysVisibleColumnKeys))
  const preferredDefaults = normalizeStringIterable(options.defaultVisibleColumnKeys)

  const { ensureLoaded, readStringArray, writeStringArray } = useAdminPreferences()
  const visibleColumnKeys = ref<string[]>([])
  const hydratedFromPreferences = ref(false)

  const allowedColumnKeys = computed(() => {
    return options.allColumns.value
      .filter((column) => !excludeSet.has(column.key))
      .map((column) => column.key)
  })

  const fallbackVisibleKeys = computed(() => {
    if (preferredDefaults.length === 0) {
      return [...allowedColumnKeys.value]
    }

    const allowedSet = new Set(allowedColumnKeys.value)
    const preferred = preferredDefaults.filter((key) => allowedSet.has(key))
    if (preferred.length > 0) {
      return preferred
    }

    return [...allowedColumnKeys.value]
  })

  watch(
    allowedColumnKeys,
    (allowedKeys) => {
      const fallbackKeys = fallbackVisibleKeys.value
      if (visibleColumnKeys.value.length === 0) {
        visibleColumnKeys.value = [...fallbackKeys]
        return
      }

      const sanitized = sanitizeVisibleColumns(visibleColumnKeys.value, allowedKeys, fallbackKeys)
      if (!sameStringArray(sanitized, visibleColumnKeys.value)) {
        visibleColumnKeys.value = sanitized
      }
    },
    { immediate: true },
  )

  const tableColumns = computed(() => {
    const visibleSet = new Set(visibleColumnKeys.value)
    return options.allColumns.value.filter(
      (column) => alwaysVisibleSet.has(column.key) || visibleSet.has(column.key),
    )
  })

  onMounted(async () => {
    if (import.meta.server) {
      return
    }

    await ensureLoaded()
    const allowed = allowedColumnKeys.value
    const fallback = fallbackVisibleKeys.value
    const saved = readStringArray(['ui', 'columns', options.preferenceKey], fallback)
    const sanitized = sanitizeVisibleColumns(saved, allowed, fallback)
    visibleColumnKeys.value = sanitized
    hydratedFromPreferences.value = true
  })

  watch(
    visibleColumnKeys,
    (next) => {
      if (!hydratedFromPreferences.value || import.meta.server) {
        return
      }

      const allowed = allowedColumnKeys.value
      const fallback = fallbackVisibleKeys.value
      const sanitized = sanitizeVisibleColumns(next, allowed, fallback)
      if (!sameStringArray(sanitized, next)) {
        visibleColumnKeys.value = sanitized
        return
      }

      writeStringArray(['ui', 'columns', options.preferenceKey], sanitized)
    },
    { deep: true },
  )

  return {
    visibleColumnKeys,
    tableColumns,
  }
}
