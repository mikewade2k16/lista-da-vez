export type CustomerApiErrorKind =
  | 'aborted'
  | 'capability_off'
  | 'forbidden'
  | 'not_found'
  | 'unavailable'
  | 'error'

export interface CustomerApiErrorState {
  kind: CustomerApiErrorKind
  message: string
  reasonCode: string
  statusCode: number
}

interface ErrorShape {
  name?: unknown
  message?: unknown
  status?: unknown
  statusCode?: unknown
  response?: { status?: unknown }
  data?: {
    error?: {
      code?: unknown
      message?: unknown
      reasonCode?: unknown
      details?: { reasonCode?: unknown }
    }
  }
}

function normalizeText(value: unknown): string {
  return String(value ?? '').trim()
}

function readStatus(error: ErrorShape): number {
  const status = Number(error.statusCode ?? error.status ?? error.response?.status ?? 0)
  return Number.isFinite(status) ? status : 0
}

export function classifyCustomerApiError(
  cause: unknown,
  fallback = 'Nao foi possivel carregar os dados.',
): CustomerApiErrorState {
  const error = cause && typeof cause === 'object' ? (cause as ErrorShape) : {}
  const statusCode = readStatus(error)
  const reasonCode = normalizeText(
    error.data?.error?.reasonCode ??
      error.data?.error?.details?.reasonCode ??
      error.data?.error?.code,
  )
  const message =
    normalizeText(error.data?.error?.message ?? error.message) || normalizeText(fallback)
  const normalizedReason = reasonCode.toLowerCase()

  if (normalizeText(error.name) === 'AbortError' || normalizedReason === 'aborted') {
    return { kind: 'aborted', message, reasonCode, statusCode }
  }
  if (
    normalizedReason.includes('capability') ||
    normalizedReason.includes('feature_off') ||
    normalizedReason.includes('module_disabled') ||
    normalizedReason.includes('disabled')
  ) {
    return { kind: 'capability_off', message, reasonCode, statusCode }
  }
  if (statusCode === 403) {
    return { kind: 'forbidden', message, reasonCode, statusCode }
  }
  if (statusCode === 404) {
    return { kind: 'not_found', message, reasonCode, statusCode }
  }
  if (statusCode === 0 || statusCode >= 500) {
    return { kind: 'unavailable', message, reasonCode, statusCode }
  }
  return { kind: 'error', message, reasonCode, statusCode }
}
