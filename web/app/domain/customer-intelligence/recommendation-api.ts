import type { createApiRequest } from '~/utils/api-client'
import type {
  CustomerRecommendationView,
  RecommendationDecisionInput,
  RecommendationStatus,
  RecommendationType,
  RecommendationsPage,
} from './recommendation-types'

type RecommendationApi = ReturnType<typeof createApiRequest>

interface BackendRecommendation {
  id: string
  clientAccountId: string
  relationshipId: string
  type: string
  status: string
  confidence?: number
  reasonCodes?: string[]
  payload?: unknown
  validUntil?: string
  createdAt: string
}

function recommendationType(value: string): RecommendationType {
  if (
    value === 'follow_up' ||
    value === 'offer' ||
    value === 'important_date' ||
    value === 'next_action'
  ) {
    return value
  }
  return 'next_action'
}

function recommendationStatus(value: string): RecommendationStatus {
  if (value === 'proposed' || value === 'accepted' || value === 'rejected' || value === 'expired') {
    return value
  }
  if (value === 'superseded') return 'stale'
  return 'proposed'
}

function title(type: RecommendationType): string {
  const labels: Record<RecommendationType, string> = {
    follow_up: 'Proximo follow-up',
    offer: 'Oferta sugerida',
    important_date: 'Data importante',
    next_action: 'Proxima acao',
  }
  return labels[type]
}

function normalizeRecommendation(item: BackendRecommendation): CustomerRecommendationView {
  const type = recommendationType(item.type)
  const status = recommendationStatus(item.status)
  const reasons = (item.reasonCodes ?? []).filter(Boolean)
  const payload =
    item.payload && typeof item.payload === 'object' && !Array.isArray(item.payload)
      ? (item.payload as Record<string, unknown>)
      : {}
  const rationale =
    (typeof payload.conversationBrief === 'string' && payload.conversationBrief) ||
    (typeof payload.fitNarrative === 'string' && payload.fitNarrative) ||
    (typeof payload.dateValue === 'string' &&
      `${String(payload.dateKind || 'data importante')}: ${payload.dateValue}`) ||
    reasons.join(', ') ||
    'Racional protegido no registro server-side.'
  const evidence = Array.isArray(payload.evidenceRefs)
    ? payload.evidenceRefs.flatMap((entry) => {
        if (!entry || typeof entry !== 'object' || Array.isArray(entry)) return []
        const record = entry as Record<string, unknown>
        const ref = String(record.observationId || '').trim()
        const sourceKey = String(record.sourceKey || '').trim()
        if (!ref) return []
        return [{ ref, sourceKey, label: sourceKey || 'Evidencia registrada' }]
      })
    : []
  const allowedActions: CustomerRecommendationView['allowedActions'] =
    status === 'proposed' ? ['approve', 'reject'] : []
  return {
    id: item.id,
    relationshipId: item.relationshipId,
    type,
    status,
    title: title(type),
    rationale,
    confidence: item.confidence,
    validFrom: item.createdAt,
    validUntil: item.validUntil,
    freshnessStatus: item.validUntil ? 'valid_until' : 'current',
    generatedByAi: true,
    revision: 0,
    evidenceRefs: evidence,
    constraints: [],
    allowedActions,
  }
}

function query(clientAccountId: string): string {
  const params = new URLSearchParams()
  if (clientAccountId.trim()) params.set('clientAccountId', clientAccountId.trim())
  params.set('limit', '50')
  return params.toString()
}

export async function fetchRecommendations(
  api: RecommendationApi,
  relationshipId: string,
  clientAccountId: string,
  _cursor = '',
  signal?: AbortSignal,
): Promise<RecommendationsPage> {
  const response = (await api(
    `/v1/customer-intelligence/relationships/${encodeURIComponent(relationshipId)}/recommendations?${query(clientAccountId)}`,
    { signal, dedupe: false },
  )) as BackendRecommendation[]
  return {
    items: (Array.isArray(response) ? response : []).map(normalizeRecommendation),
    nextCursor: '',
    decisionOptions: {
      approveReasons: [
        { value: 'relevant_to_customer', label: 'Relevante para o cliente' },
        { value: 'timing_confirmed', label: 'Momento confirmado' },
        { value: 'offer_verified', label: 'Oferta verificada' },
      ],
      rejectReasons: [
        { value: 'not_relevant', label: 'Nao relevante' },
        { value: 'outdated_context', label: 'Contexto desatualizado' },
        { value: 'consent_or_policy_risk', label: 'Risco de consentimento/politica' },
      ],
      invalidateReasons: [],
      executeReasons: [],
    },
  }
}

export function decideRecommendation(
  api: RecommendationApi,
  recommendationId: string,
  clientAccountId: string,
  action: 'approve' | 'reject',
  input: RecommendationDecisionInput,
  signal?: AbortSignal,
): Promise<unknown> {
  const params = new URLSearchParams()
  if (clientAccountId.trim()) params.set('clientAccountId', clientAccountId.trim())
  const suffix = params.size ? `?${params.toString()}` : ''
  return api(
    `/v1/customer-intelligence/recommendations/${encodeURIComponent(recommendationId)}/review${suffix}`,
    {
      method: 'POST',
      body: {
        status: action === 'approve' ? 'accepted' : 'rejected',
        reason: input.reason.trim(),
        metadata: {},
      },
      signal,
      dedupe: false,
    },
  )
}
