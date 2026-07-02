const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i
const LEGACY_RECURRING_PREFIX = 'recurring-client-'
const RECURRING_ROW_UUID_RE = /^00000000-0000-4000-8000-([0-9a-f]{12})$/i
const RECURRING_STORE_ROW_UUID_RE = /^11111111-1111-4111-8111-([0-9a-f]{12})$/i

function normalizeDeterministicSeed(value: unknown) {
  return String(value ?? '')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, ' ')
}

function hashSeedToHex(seed: string) {
  let hashA = 0x811c9dc5
  let hashB = 0x9e3779b1

  for (let index = 0; index < seed.length; index += 1) {
    const code = seed.charCodeAt(index)
    hashA = Math.imul(hashA ^ code, 0x01000193)
    hashB = Math.imul(hashB ^ code, 0x85ebca6b)
  }

  const hexA = (hashA >>> 0).toString(16).padStart(8, '0')
  const hexB = (hashB >>> 0).toString(16).padStart(8, '0')
  return `${hexA}${hexB}`
}

export function isFinanceUuid(value: unknown): value is string {
  return UUID_RE.test(String(value ?? '').trim())
}

export function createFinanceUuid() {
  const randomUUID = globalThis.crypto?.randomUUID?.bind(globalThis.crypto)
  if (randomUUID) {
    return randomUUID().toLowerCase()
  }

  const bytes = new Uint8Array(16)
  if (globalThis.crypto?.getRandomValues) {
    globalThis.crypto.getRandomValues(bytes)
  } else {
    for (let index = 0; index < bytes.length; index += 1) {
      bytes[index] = Math.floor(Math.random() * 256)
    }
  }

  bytes[6] = (bytes[6]! & 0x0f) | 0x40
  bytes[8] = (bytes[8]! & 0x3f) | 0x80

  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
  return [
    hex.slice(0, 8),
    hex.slice(8, 12),
    hex.slice(12, 16),
    hex.slice(16, 20),
    hex.slice(20, 32),
  ].join('-')
}

export function financeRecurringRowId(sourceCoreTenantId: string) {
  const normalized = normalizeDeterministicSeed(sourceCoreTenantId)
  if (!normalized) return ''

  const hash = hashSeedToHex(normalized).slice(0, 12)
  return `00000000-0000-4000-8000-${hash}`
}

export function financeRecurringStoreRowId(sourceCoreTenantId: string, storeName: string) {
  const normalizedClientId = normalizeDeterministicSeed(sourceCoreTenantId)
  const normalizedStoreName = normalizeDeterministicSeed(storeName)
  if (!normalizedClientId || !normalizedStoreName) {
    return ''
  }

  const hash = hashSeedToHex(`${normalizedClientId}:${normalizedStoreName}`).slice(0, 12)
  return `11111111-1111-4111-8111-${hash}`
}

export function isFinanceRecurringRowId(value: unknown) {
  const raw = String(value ?? '')
    .trim()
    .toLowerCase()
  return RECURRING_ROW_UUID_RE.test(raw)
}

export function parseFinanceRecurringClientId(value: unknown) {
  const raw = String(value ?? '')
    .trim()
    .toLowerCase()
  if (!raw) return 0

  if (raw.startsWith(LEGACY_RECURRING_PREFIX)) {
    const parsed = Number.parseInt(raw.slice(LEGACY_RECURRING_PREFIX.length), 10)
    return Number.isFinite(parsed) && parsed > 0 ? parsed : 0
  }

  const recurringMatch = raw.match(RECURRING_ROW_UUID_RE)
  if (!recurringMatch) return 0

  const parsed = Number.parseInt(recurringMatch[1] || '', 16)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0
}

export function isFinanceRecurringStoreRowId(value: unknown) {
  const raw = String(value ?? '')
    .trim()
    .toLowerCase()
  return RECURRING_STORE_ROW_UUID_RE.test(raw)
}

export function normalizeFinanceEntityId(value: unknown) {
  const raw = String(value ?? '')
    .trim()
    .toLowerCase()
  return isFinanceUuid(raw) ? raw : createFinanceUuid()
}

export function normalizeFinanceLinkedUuid(value: unknown) {
  const raw = String(value ?? '')
    .trim()
    .toLowerCase()
  if (!raw) return ''

  if (isFinanceRecurringRowId(raw) || isFinanceRecurringStoreRowId(raw)) {
    return raw
  }

  return isFinanceUuid(raw) ? raw : ''
}
