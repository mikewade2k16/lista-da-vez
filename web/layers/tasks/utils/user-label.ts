import { normalizeText } from './text'

interface UserLabelSource {
  nick?: unknown
  displayName?: unknown
  name?: unknown
  fullName?: unknown
  email?: unknown
}

function firstToken(value: unknown, max = 120): string {
  const normalized = normalizeText(value, max)
  if (!normalized) return ''
  return normalized.split(/\s+/).filter(Boolean)[0] || normalized
}

function compactEmail(value: unknown, max = 120): string {
  const normalized = normalizeText(value, max)
  if (!normalized) return ''
  const [localPart] = normalized.split('@')
  return normalizeText(localPart || normalized, max)
}

export function compactUserLabel(source?: UserLabelSource | null, max = 120): string {
  const nick = normalizeText(source?.nick, max)
  if (nick) return nick
  const displayName = firstToken(source?.displayName || source?.name || source?.fullName, max)
  if (displayName) return displayName
  return compactEmail(source?.email, max)
}