export const CUSTOMER_CLAIM_STATUS_OPTIONS = [
  { value: 'candidate', label: 'Candidatas' },
  { value: 'accepted', label: 'Aceitas' },
  { value: 'rejected', label: 'Rejeitadas' },
] as const

export type CustomerClaimStatus = (typeof CUSTOMER_CLAIM_STATUS_OPTIONS)[number]['value']
export type CustomerClaimReviewStatus = Exclude<CustomerClaimStatus, 'candidate'>

export interface CustomerClaimEvidenceRef {
  observationId: string
  sourceKey: string
  locator: string
}

export interface CustomerClaimView {
  id: string
  relationshipId: string
  factKey: string
  valueType: string
  value: unknown
  extractionMethod: string
  extractorKey: string
  extractorVersion: string
  promptBindingId: string
  runtimeRunId: string
  confidence: number
  verificationState: string
  validFrom: string
  validUntil: string
  sensitivity: string
  status: CustomerClaimStatus
  sourceOutcomeEventId: string
  sourceClaimOrdinal: number | null
  revision: number
  reviewedAt: string
  reviewReasonCode: string
  evidence: CustomerClaimEvidenceRef[]
  createdAt: string
  updatedAt: string
}

export interface CustomerClaimReviewInput {
  status: CustomerClaimReviewStatus
  reasonCode: string
  expectedRevision: number
}

export const CUSTOMER_CLAIM_REASON_CODE_PATTERN = /^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$/

export function validCustomerClaimReasonCode(value: string): boolean {
  const normalized = value.trim()
  return (
    normalized.length > 0 &&
    normalized.length <= 160 &&
    CUSTOMER_CLAIM_REASON_CODE_PATTERN.test(normalized)
  )
}
