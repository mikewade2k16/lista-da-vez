import type { createApiRequest } from '~/utils/api-client'
import type {
  IntelligenceFactsPage,
  IntelligenceFactView,
  IntelligenceSummaryView,
  IntelligenceTimelinePage,
} from './types'

type CustomerIntelligenceApi = ReturnType<typeof createApiRequest>

interface BackendEvidenceRef {
  observationId: string
  sourceKey: string
  locator?: string
}

interface BackendFact {
  id: string
  key: string
  value: unknown
  confidence?: number
  verificationState: string
  validFrom?: string
  updatedAt: string
  evidence?: BackendEvidenceRef[]
}

interface BackendSummary {
  id: string
  summaryType: string
  body: unknown
  generatedAt: string
}

interface BackendProfile {
  facts?: BackendFact[]
  summary?: BackendSummary
}

function buildClientQuery(clientAccountId: string): string {
  const query = new URLSearchParams()
  if (clientAccountId.trim()) query.set('clientAccountId', clientAccountId.trim())
  query.set('limit', '100')
  return query.toString()
}

function label(key: string): string {
  return key.replace(/[._-]+/g, ' ').replace(/\b\w/g, (character) => character.toUpperCase())
}

function normalizeFact(item: BackendFact): IntelligenceFactView {
  return {
    id: item.id,
    factKey: item.key,
    label: label(item.key),
    value: item.value,
    state: item.verificationState,
    confidence: item.confidence,
    asOf: item.validFrom ?? item.updatedAt,
    evidenceRefs: (item.evidence ?? []).map((evidence) => ({
      id: evidence.observationId,
      sourceKey: evidence.sourceKey,
      excerpt: evidence.locator,
      masked: true,
    })),
  }
}

function summaryText(body: unknown): string {
  if (typeof body === 'string') return body
  if (body && typeof body === 'object' && !Array.isArray(body)) {
    const record = body as Record<string, unknown>
    for (const key of ['text', 'summary', 'content', 'narrative']) {
      if (typeof record[key] === 'string') return record[key]
    }
  }
  const serialized = JSON.stringify(body) ?? ''
  return serialized.length > 2000 ? `${serialized.slice(0, 2000)}...` : serialized
}

export async function fetchRelationshipFacts(
  api: CustomerIntelligenceApi,
  relationshipId: string,
  clientAccountId: string,
  _cursor = '',
  signal?: AbortSignal,
): Promise<IntelligenceFactsPage> {
  const response = (await api(
    `/v1/customer-intelligence/relationships/${encodeURIComponent(relationshipId)}/facts?${buildClientQuery(clientAccountId)}`,
    { signal, dedupe: false },
  )) as BackendFact[]
  return {
    items: (Array.isArray(response) ? response : []).map(normalizeFact),
    nextCursor: '',
  }
}

export async function fetchRelationshipSummaries(
  api: CustomerIntelligenceApi,
  relationshipId: string,
  clientAccountId: string,
  signal?: AbortSignal,
): Promise<IntelligenceSummaryView[]> {
  const response = (await api(
    `/v1/customer-intelligence/relationships/${encodeURIComponent(relationshipId)}/profile?${buildClientQuery(clientAccountId)}`,
    { signal, dedupe: false },
  )) as BackendProfile
  if (!response.summary) return []
  return [
    {
      id: response.summary.id,
      summaryType: response.summary.summaryType,
      text: summaryText(response.summary.body),
      status: 'generated',
      asOf: response.summary.generatedAt,
    },
  ]
}

export async function fetchRelationshipTimeline(
  _api: CustomerIntelligenceApi,
  _relationshipId: string,
  _clientAccountId: string,
  _cursor = '',
  _signal?: AbortSignal,
): Promise<IntelligenceTimelinePage> {
  // Nao existe rota de timeline na API atual. O historico deterministico continua
  // vindo do Customer Data; esta colecao fica vazia ate haver descriptor proprio.
  return { items: [], nextCursor: '' }
}
