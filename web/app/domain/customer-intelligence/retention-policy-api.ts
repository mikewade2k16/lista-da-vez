import type { createApiRequest } from '~/utils/api-client'
import type {
  RetentionExpiryAction,
  RetentionPolicyDraftCommand,
  RetentionPolicyPublishCommand,
  RetentionPolicyStatus,
  RetentionPolicyVersion,
} from './retention-policy-types'
import { validRetentionPolicyKey } from './retention-policy-types'

type RetentionPolicyApi = ReturnType<typeof createApiRequest>

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function normalizedStatus(value: unknown): RetentionPolicyStatus | null {
  return value === 'draft' || value === 'published' ? value : null
}

function normalizedExpiryAction(value: unknown): RetentionExpiryAction | null {
  return value === 'tombstone' || value === 'crypto_shred' ? value : null
}

function normalizeRetentionPolicyVersion(value: unknown): RetentionPolicyVersion | null {
  if (!isRecord(value)) return null
  const status = normalizedStatus(value.status)
  const onExpiry = normalizedExpiryAction(value.onExpiry)
  const id = String(value.id ?? '').trim()
  const policyKey = String(value.policyKey ?? '').trim()
  const version = Number(value.version)
  const revision = Number(value.revision)
  const snapshotTtlSeconds = Number(value.snapshotTtlSeconds)
  if (
    !id ||
    !validRetentionPolicyKey(policyKey) ||
    !status ||
    !onExpiry ||
    !Number.isInteger(version) ||
    version < 1 ||
    !Number.isInteger(revision) ||
    revision < 1 ||
    !Number.isInteger(snapshotTtlSeconds)
  ) {
    return null
  }
  const publishedAt = String(value.publishedAt ?? '').trim()
  const createdByUserId = String(value.createdByUserId ?? '').trim()
  const publishedByUserId = String(value.publishedByUserId ?? '').trim()
  const publicationReasonCode = String(value.publicationReasonCode ?? '').trim()
  const approvalReference = String(value.approvalReference ?? '').trim()
  return {
    id,
    accountId: String(value.accountId ?? '').trim(),
    policyKey,
    version,
    status,
    snapshotTtlSeconds,
    onExpiry,
    legalHoldBehavior: String(value.legalHoldBehavior ?? '').trim(),
    blockReingestion: value.blockReingestion === true,
    revision,
    createdByUserId: createdByUserId || undefined,
    publishedByUserId: publishedByUserId || undefined,
    publicationReasonCode: publicationReasonCode || undefined,
    approvalReference: approvalReference || undefined,
    createdAt: String(value.createdAt ?? '').trim(),
    publishedAt: publishedAt || undefined,
  }
}

function requiredRetentionPolicyVersion(value: unknown): RetentionPolicyVersion {
  const normalized = normalizeRetentionPolicyVersion(value)
  if (!normalized) throw new Error('O servidor devolveu uma versao de retention policy invalida.')
  return normalized
}

export async function fetchRetentionPolicyVersions(
  api: RetentionPolicyApi,
  signal?: AbortSignal,
): Promise<RetentionPolicyVersion[]> {
  const response = (await api('/v1/customer-intelligence/retention-policies', {
    signal,
    dedupe: false,
  })) as unknown
  if (!Array.isArray(response)) return []
  return response
    .map(normalizeRetentionPolicyVersion)
    .filter((item): item is RetentionPolicyVersion => item !== null)
    .sort((left, right) => {
      if (left.policyKey === right.policyKey) return right.version - left.version
      return left.policyKey.localeCompare(right.policyKey)
    })
}

export async function createRetentionPolicyDraft(
  api: RetentionPolicyApi,
  input: RetentionPolicyDraftCommand,
  signal?: AbortSignal,
): Promise<RetentionPolicyVersion> {
  const response = await api(
    `/v1/customer-intelligence/retention-policies/${encodeURIComponent(input.policyKey)}/drafts`,
    {
      method: 'POST',
      body: {
        snapshotTtlSeconds: input.snapshotTtlSeconds,
        onExpiry: input.onExpiry,
      },
      signal,
    },
  )
  return requiredRetentionPolicyVersion(response)
}

export async function publishRetentionPolicyVersion(
  api: RetentionPolicyApi,
  versionId: string,
  input: RetentionPolicyPublishCommand,
  signal?: AbortSignal,
): Promise<RetentionPolicyVersion> {
  const response = await api(
    `/v1/customer-intelligence/retention-policy-versions/${encodeURIComponent(versionId)}/publish`,
    {
      method: 'POST',
      body: {
        expectedRevision: input.expectedRevision,
        reasonCode: input.reasonCode,
        approvalReference: input.approvalReference,
      },
      signal,
    },
  )
  return requiredRetentionPolicyVersion(response)
}
