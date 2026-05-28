type PreferencesRecord = Record<string, unknown>

const STORAGE_KEY = 'fila-reference-admin-preferences'

function normalizePreferences(value: unknown): PreferencesRecord {
  if (typeof value === 'string') {
    const raw = value.trim()
    if (!raw) {
      return {}
    }

    try {
      const parsed = JSON.parse(raw) as unknown
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        return parsed as PreferencesRecord
      }
    } catch {
      return {}
    }

    return {}
  }

  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return value as PreferencesRecord
  }

  return {}
}

function clonePreferences(value: PreferencesRecord) {
  try {
    return JSON.parse(JSON.stringify(value)) as PreferencesRecord
  } catch {
    return {}
  }
}

function pathSegments(path: string | string[]) {
  if (Array.isArray(path)) {
    return path.map((segment) => String(segment ?? '').trim()).filter(Boolean)
  }

  return String(path ?? '')
    .split('.')
    .map((segment) => segment.trim())
    .filter(Boolean)
}

function getNestedValue(source: PreferencesRecord, path: string | string[]) {
  const segments = pathSegments(path)
  let cursor: unknown = source

  for (const segment of segments) {
    if (!cursor || typeof cursor !== 'object' || Array.isArray(cursor)) {
      return undefined
    }

    cursor = (cursor as PreferencesRecord)[segment]
  }

  return cursor
}

function setNestedValue(source: PreferencesRecord, path: string | string[], value: unknown) {
  const segments = pathSegments(path)
  if (segments.length === 0) {
    return source
  }

  const next = clonePreferences(source)
  let cursor: PreferencesRecord = next

  for (let index = 0; index < segments.length - 1; index += 1) {
    const segment = segments[index]!
    const current = cursor[segment]

    if (!current || typeof current !== 'object' || Array.isArray(current)) {
      cursor[segment] = {}
    }

    cursor = cursor[segment] as PreferencesRecord
  }

  cursor[segments[segments.length - 1]!] = value
  return next
}

function sameStringArray(a: string[], b: string[]) {
  if (a.length !== b.length) {
    return false
  }

  return a.every((value, index) => value === b[index])
}

export function useAdminPreferences() {
  const preferences = useState<PreferencesRecord>('reference-admin.preferences', () => ({}))
  const loaded = useState<boolean>('reference-admin.preferences.loaded', () => false)

  function persist() {
    if (import.meta.server) {
      return
    }

    localStorage.setItem(STORAGE_KEY, JSON.stringify(preferences.value))
  }

  async function ensureLoaded() {
    if (loaded.value) {
      return
    }

    if (import.meta.server) {
      loaded.value = true
      return
    }

    preferences.value = normalizePreferences(localStorage.getItem(STORAGE_KEY))
    loaded.value = true
  }

  function readStringArray(path: string | string[], fallback: string[] = []) {
    const current = getNestedValue(preferences.value, path)
    if (!Array.isArray(current)) {
      return [...fallback]
    }

    const sanitized = current.map((item) => String(item ?? '').trim()).filter(Boolean)

    return sanitized.length > 0 ? sanitized : [...fallback]
  }

  function writeStringArray(path: string | string[], value: string[]) {
    const sanitized = value.map((item) => String(item ?? '').trim()).filter(Boolean)

    const current = readStringArray(path)
    if (sameStringArray(current, sanitized)) {
      return
    }

    preferences.value = setNestedValue(preferences.value, path, sanitized)
    persist()
  }

  return {
    preferences,
    loaded,
    ensureLoaded,
    readStringArray,
    writeStringArray,
  }
}
