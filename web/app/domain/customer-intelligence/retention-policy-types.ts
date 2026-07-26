export type RetentionPolicyStatus = 'draft' | 'published'
export type RetentionExpiryAction = 'tombstone' | 'crypto_shred'

export interface RetentionPolicyVersion {
  id: string
  accountId: string
  policyKey: string
  version: number
  status: RetentionPolicyStatus
  snapshotTtlSeconds: number
  onExpiry: RetentionExpiryAction
  legalHoldBehavior: string
  blockReingestion: boolean
  revision: number
  createdByUserId?: string
  publishedByUserId?: string
  publicationReasonCode?: string
  approvalReference?: string
  createdAt: string
  publishedAt?: string
}

export interface RetentionPolicyDraftCommand {
  policyKey: string
  snapshotTtlSeconds: number
  onExpiry: RetentionExpiryAction
}

export interface RetentionPolicyPublishCommand {
  expectedRevision: number
  reasonCode: RetentionPublicationReasonCode
  approvalReference: string
}

export const RETENTION_EXPIRY_OPTIONS = [
  {
    value: 'tombstone',
    label: 'Tombstone',
    meta: 'Remove o snapshot e preserva somente a proveniencia.',
  },
  {
    value: 'crypto_shred',
    label: 'Crypto-shred',
    meta: 'Invalida o material cifrado e preserva metadados auditaveis.',
  },
] as const satisfies ReadonlyArray<{
  value: RetentionExpiryAction
  label: string
  meta: string
}>

export const RETENTION_PUBLICATION_REASON_OPTIONS = [
  {
    value: 'legal_review_approved',
    label: 'Revisao juridica aprovada',
    meta: 'A area juridica aprovou prazo e forma de expiracao.',
  },
  {
    value: 'privacy_review_approved',
    label: 'Revisao de privacidade aprovada',
    meta: 'A area de privacidade aprovou finalidade e minimizacao.',
  },
  {
    value: 'data_governance_approved',
    label: 'Governanca de dados aprovada',
    meta: 'A governanca aprovou o ciclo de vida desta categoria.',
  },
  {
    value: 'regulatory_requirement_approved',
    label: 'Exigencia regulatoria aprovada',
    meta: 'A versao atende uma obrigacao regulatoria documentada.',
  },
] as const

export type RetentionPublicationReasonCode =
  (typeof RETENTION_PUBLICATION_REASON_OPTIONS)[number]['value']

const SAFE_KEY_PATTERN = /^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$/
const APPROVAL_REFERENCE_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/
const MIN_RETENTION_SECONDS = 86_400
const MAX_RETENTION_SECONDS = 315_360_000
const PUBLICATION_REASON_CODES = new Set<string>(
  RETENTION_PUBLICATION_REASON_OPTIONS.map((option) => option.value),
)

export function validRetentionPolicyKey(value: string): boolean {
  const normalized = String(value || '').trim()
  return (
    normalized === value &&
    normalized.length > 0 &&
    normalized.length <= 160 &&
    SAFE_KEY_PATTERN.test(normalized)
  )
}

export function validRetentionDraftCommand(input: RetentionPolicyDraftCommand): boolean {
  return (
    validRetentionPolicyKey(input.policyKey) &&
    Number.isInteger(input.snapshotTtlSeconds) &&
    input.snapshotTtlSeconds >= MIN_RETENTION_SECONDS &&
    input.snapshotTtlSeconds <= MAX_RETENTION_SECONDS &&
    RETENTION_EXPIRY_OPTIONS.some((option) => option.value === input.onExpiry)
  )
}

export function isRetentionPublicationReasonCode(
  value: string,
): value is RetentionPublicationReasonCode {
  return PUBLICATION_REASON_CODES.has(String(value || '').trim())
}

export function validRetentionPublishCommand(input: RetentionPolicyPublishCommand): boolean {
  const approvalReference = String(input.approvalReference || '').trim()
  return (
    Number.isInteger(input.expectedRevision) &&
    input.expectedRevision > 0 &&
    isRetentionPublicationReasonCode(input.reasonCode) &&
    approvalReference === input.approvalReference &&
    APPROVAL_REFERENCE_PATTERN.test(approvalReference)
  )
}
