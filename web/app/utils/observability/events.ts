import { sanitizeText } from './sanitize'

export type ObsEventKind =
  | 'vital'
  | 'pageload'
  | 'js_error'
  | 'api_call'
  | 'api_failure'
  | 'heartbeat'

export interface ObsEvent {
  kind: ObsEventKind
  name: string
  value?: number
  status?: number
  correlationId?: string
  detail?: Record<string, unknown>
  at?: string
}

export const MAX_BATCH_SIZE = 100
export const MAX_QUEUE_SIZE = 500
export const SLOW_CALL_MS = 1000
export const FAST_CALL_SAMPLE_RATE = 0.1

// api_call: grava todo lento (>= 1s) ou falho; os demais so por amostragem.
export function shouldRecordApiCall(
  durationMs: number,
  failed: boolean,
  random: () => number = Math.random,
): boolean {
  if (failed || durationMs >= SLOW_CALL_MS) return true
  return random() < FAST_CALL_SAMPLE_RATE
}

export function clampSampleRate(raw: unknown): number {
  const value = Number(raw)
  if (!Number.isFinite(value)) return 1
  return Math.min(1, Math.max(0, value))
}

// Fila limitada: cheia = descarta o mais antigo (nunca cresce sem limite).
export function createEventQueue(maxSize = MAX_QUEUE_SIZE) {
  let items: ObsEvent[] = []
  return {
    push(event: ObsEvent) {
      items.push(event)
      if (items.length > maxSize) items = items.slice(items.length - maxSize)
    },
    drain(limit = MAX_BATCH_SIZE): ObsEvent[] {
      return items.splice(0, limit)
    },
    size() {
      return items.length
    },
  }
}

export function buildErrorDetail(
  message: unknown,
  stack: unknown,
  extra: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    ...extra,
    message: sanitizeText(message),
    stack: sanitizeText(stack),
  }
}

// Limite do coletor: 64 KB por requisicao. Remove eventos do fim ate caber.
export const MAX_BODY_CHARS = 60000

export interface CollectBody {
  pageViewId: string
  page: string
  events: ObsEvent[]
}

export function fitBatchToLimit(body: CollectBody, maxChars = MAX_BODY_CHARS): CollectBody {
  const events = [...body.events]
  while (events.length > 1 && JSON.stringify({ ...body, events }).length > maxChars) {
    events.pop()
  }
  return { ...body, events }
}
