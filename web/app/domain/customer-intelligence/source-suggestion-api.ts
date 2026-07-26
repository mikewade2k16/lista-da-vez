import type { createApiRequest } from '~/utils/api-client'
import type {
  SourceSuggestionReviewInput,
  SourceSuggestionStatus,
  SourceSuggestionView,
} from './source-suggestion-types'

type SourceSuggestionApi = ReturnType<typeof createApiRequest>
type UnknownRecord = Record<string, unknown>

const STATUSES = new Set<SourceSuggestionStatus>(['proposed', 'accepted', 'rejected', 'expired'])

function record(value: unknown): UnknownRecord {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as UnknownRecord) : {}
}

function text(value: unknown): string {
  return String(value ?? '').trim()
}

function confidence(value: unknown): number {
  const normalized = Number(value)
  if (!Number.isFinite(normalized)) return 0
  return Math.min(1, Math.max(0, normalized))
}

function strings(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return [...new Set(value.map(text).filter(Boolean))]
}

function status(value: unknown): SourceSuggestionStatus {
  const normalized = text(value) as SourceSuggestionStatus
  return STATUSES.has(normalized) ? normalized : 'unknown'
}

function stillValid(expiresAt: string): boolean {
  if (!expiresAt) return true
  const timestamp = Date.parse(expiresAt)
  return Number.isFinite(timestamp) && timestamp > Date.now()
}

function normalizeSourceSuggestion(value: unknown): SourceSuggestionView {
  const item = record(value)
  const normalizedStatus = status(item.status)
  const expiresAt = text(item.expiresAt)
  return {
    id: text(item.id),
    relationshipId: text(item.relationshipId),
    sourceKey: text(item.sourceKey),
    gapCodes: strings(item.gapCodes),
    rationaleCode: text(item.rationaleCode),
    rationale: text(item.rationale),
    confidence: confidence(item.confidence),
    status: normalizedStatus,
    expiresAt,
    createdAt: text(item.createdAt),
    allowedActions:
      normalizedStatus === 'proposed' && stillValid(expiresAt) ? ['accepted', 'rejected'] : [],
  }
}

function clientScopeQuery(clientAccountId: string, includeLimit = false): string {
  const query = new URLSearchParams()
  const client = clientAccountId.trim()
  if (client) query.set('clientAccountId', client)
  if (includeLimit) query.set('limit', '50')
  return query.toString()
}

export async function fetchSourceSuggestions(
  api: SourceSuggestionApi,
  relationshipId: string,
  clientAccountId: string,
  signal?: AbortSignal,
): Promise<SourceSuggestionView[]> {
  const query = clientScopeQuery(clientAccountId, true)
  const response = await api(
    `/v1/customer-intelligence/relationships/${encodeURIComponent(relationshipId)}/source-suggestions?${query}`,
    { signal, dedupe: false },
  )
  return Array.isArray(response) ? response.map(normalizeSourceSuggestion) : []
}

export async function reviewSourceSuggestion(
  api: SourceSuggestionApi,
  suggestionId: string,
  clientAccountId: string,
  input: SourceSuggestionReviewInput,
  signal?: AbortSignal,
): Promise<SourceSuggestionView> {
  const query = clientScopeQuery(clientAccountId)
  const suffix = query ? `?${query}` : ''
  const response = await api(
    `/v1/customer-intelligence/source-suggestions/${encodeURIComponent(suggestionId)}/review${suffix}`,
    {
      method: 'POST',
      body: {
        status: input.status,
        reason: input.reason.trim(),
      },
      signal,
    },
  )
  return normalizeSourceSuggestion(response)
}
