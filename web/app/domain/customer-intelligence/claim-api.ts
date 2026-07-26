import type { createApiRequest } from '~/utils/api-client'
import type {
  CustomerClaimEvidenceRef,
  CustomerClaimReviewInput,
  CustomerClaimStatus,
  CustomerClaimView,
} from './claim-types'

type CustomerClaimApi = ReturnType<typeof createApiRequest>
type UnknownRecord = Record<string, unknown>

const STATUSES = new Set<CustomerClaimStatus>(['candidate', 'accepted', 'rejected'])

function record(value: unknown): UnknownRecord {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as UnknownRecord) : {}
}

function text(value: unknown): string {
  return String(value ?? '').trim()
}

function number(value: unknown): number {
  const normalized = Number(value)
  return Number.isFinite(normalized) ? normalized : 0
}

function status(value: unknown): CustomerClaimStatus {
  const normalized = text(value) as CustomerClaimStatus
  return STATUSES.has(normalized) ? normalized : 'candidate'
}

function evidence(value: unknown): CustomerClaimEvidenceRef[] {
  if (!Array.isArray(value)) return []
  return value.map((entry) => {
    const item = record(entry)
    // O contrato visual aceita somente a referencia minimizada do backend.
    return {
      observationId: text(item.observationId),
      sourceKey: text(item.sourceKey),
      locator: text(item.locator),
    }
  })
}

function normalizeClaim(value: unknown): CustomerClaimView {
  const item = record(value)
  const ordinal = item.sourceClaimOrdinal
  return {
    id: text(item.id),
    relationshipId: text(item.relationshipId),
    factKey: text(item.factKey),
    valueType: text(item.valueType),
    value: item.value,
    extractionMethod: text(item.extractionMethod),
    extractorKey: text(item.extractorKey),
    extractorVersion: text(item.extractorVersion),
    promptBindingId: text(item.promptBindingId),
    runtimeRunId: text(item.runtimeRunId),
    confidence: number(item.confidence),
    verificationState: text(item.verificationState),
    validFrom: text(item.validFrom),
    validUntil: text(item.validUntil),
    sensitivity: text(item.sensitivity),
    status: status(item.status),
    sourceOutcomeEventId: text(item.sourceOutcomeEventId),
    sourceClaimOrdinal:
      ordinal === null || ordinal === undefined || !Number.isFinite(Number(ordinal))
        ? null
        : Number(ordinal),
    revision: number(item.revision),
    reviewedAt: text(item.reviewedAt),
    reviewReasonCode: text(item.reviewReasonCode),
    evidence: evidence(item.evidence),
    createdAt: text(item.createdAt),
    updatedAt: text(item.updatedAt),
  }
}

function clientScopeQuery(clientAccountId: string): URLSearchParams {
  const query = new URLSearchParams()
  if (clientAccountId.trim()) query.set('clientAccountId', clientAccountId.trim())
  return query
}

function listQuery(clientAccountId: string, status: CustomerClaimStatus): string {
  const query = clientScopeQuery(clientAccountId)
  query.set('status', status)
  query.set('limit', '50')
  return query.toString()
}

export async function fetchCustomerClaims(
  api: CustomerClaimApi,
  relationshipId: string,
  clientAccountId: string,
  claimStatus: CustomerClaimStatus,
  signal?: AbortSignal,
): Promise<CustomerClaimView[]> {
  const response = await api(
    `/v1/customer-intelligence/relationships/${encodeURIComponent(relationshipId)}/claims?${listQuery(clientAccountId, claimStatus)}`,
    { signal, dedupe: false },
  )
  return Array.isArray(response) ? response.map(normalizeClaim) : []
}

export async function reviewCustomerClaim(
  api: CustomerClaimApi,
  claimId: string,
  clientAccountId: string,
  input: CustomerClaimReviewInput,
): Promise<CustomerClaimView> {
  const query = clientScopeQuery(clientAccountId).toString()
  const response = await api(
    `/v1/customer-intelligence/claims/${encodeURIComponent(claimId)}/review?${query}`,
    { method: 'POST', body: input },
  )
  return normalizeClaim(response)
}
