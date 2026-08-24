import type { ApiRequest } from '~/domain/calendar/calendar-api'
import type {
  CalendarChatMetaActionKind,
  CalendarChatMetaActionStatus,
} from '~/domain/calendar/calendar-chat-api'

const ACTION_PROPOSALS_BASE = '/v1/meta-ads/action-proposals'
const ACTIONS: CalendarChatMetaActionKind[] = [
  'create_campaign',
  'duplicate_campaign',
  'update_campaign',
  'pause_campaign',
  'resume_campaign',
  'promote_instagram_post',
]
const STATUSES: CalendarChatMetaActionStatus[] = [
  'pending',
  'executing',
  'succeeded',
  'failed',
  'unknown',
  'cancelled',
  'expired',
]

export interface MetaAdsActionProposalView {
  id: string
  action: CalendarChatMetaActionKind
  source: 'assistant' | 'manual'
  adAccountId: string
  metaAdAccountId: string
  adAccountName: string
  currency: string
  targetCampaignId: string
  targetMetaCampaignId: string
  payload: Record<string, unknown>
  summary: string
  status: CalendarChatMetaActionStatus
  idempotencyKey: string
  confirmationIdempotencyKey: string
  cancellationIdempotencyKey: string
  executionAvailable: boolean
  canConfirm: boolean
  requiresSpendAcknowledgement: boolean
  externalEntityId: string
  result: Record<string, unknown>
  errorCode: string
  errorMessage: string
  confirmedAt: string
  executionStartedAt: string
  completedAt: string
  reconciledAt: string
  createdAt: string
  expiresAt: string
  updatedAt: string
}

function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {}
}

function text(value: unknown, max = 1000): string {
  const normalized = typeof value === 'string' ? value.trim().replace(/\s+/g, ' ') : ''
  return Array.from(normalized).slice(0, max).join('')
}

export function normalizeMetaAdsActionProposal(value: unknown): MetaAdsActionProposalView {
  const raw = record(value)
  const action = text(raw.action, 80) as CalendarChatMetaActionKind
  const status = text(raw.status, 40) as CalendarChatMetaActionStatus
  return {
    id: text(raw.id, 64),
    action: ACTIONS.includes(action) ? action : 'pause_campaign',
    source: raw.source === 'assistant' ? 'assistant' : 'manual',
    adAccountId: text(raw.adAccountId, 64),
    metaAdAccountId: text(raw.metaAdAccountId, 80),
    adAccountName: text(raw.adAccountName, 300),
    currency: text(raw.currency, 3).toUpperCase(),
    targetCampaignId: text(raw.targetCampaignId, 64),
    targetMetaCampaignId: text(raw.targetMetaCampaignId, 80),
    payload: record(raw.payload),
    summary: text(raw.summary),
    status: STATUSES.includes(status) ? status : 'unknown',
    idempotencyKey: text(raw.idempotencyKey, 160),
    confirmationIdempotencyKey: text(raw.confirmationIdempotencyKey, 160),
    cancellationIdempotencyKey: text(raw.cancellationIdempotencyKey, 160),
    executionAvailable: raw.executionAvailable === true,
    canConfirm: raw.canConfirm === true,
    requiresSpendAcknowledgement: raw.requiresSpendAcknowledgement === true,
    externalEntityId: text(raw.externalEntityId, 160),
    result: record(raw.result),
    errorCode: text(raw.errorCode, 100),
    errorMessage: text(raw.errorMessage, 500),
    confirmedAt: text(raw.confirmedAt, 64),
    executionStartedAt: text(raw.executionStartedAt, 64),
    completedAt: text(raw.completedAt, 64),
    reconciledAt: text(raw.reconciledAt, 64),
    createdAt: text(raw.createdAt, 64),
    expiresAt: text(raw.expiresAt, 64),
    updatedAt: text(raw.updatedAt, 64),
  }
}

export function metaActionConfirmationKey(messageId: string, proposalId: string): string {
  return `assistant-confirm:${messageId.trim()}:${proposalId.trim()}`
}

export function metaActionCancellationKey(messageId: string, proposalId: string): string {
  return `assistant-cancel:${messageId.trim()}:${proposalId.trim()}`
}

export async function getMetaActionProposal(
  api: ApiRequest,
  proposalId: string,
  signal?: AbortSignal,
): Promise<MetaAdsActionProposalView> {
  const path = `${ACTION_PROPOSALS_BASE}/${encodeURIComponent(proposalId)}`
  const raw = signal ? await api(path, { signal }) : await api(path)
  return normalizeMetaAdsActionProposal(raw)
}

export async function listMetaActionProposals(
  api: ApiRequest,
  limit = 20,
  signal?: AbortSignal,
): Promise<MetaAdsActionProposalView[]> {
  const boundedLimit = Math.max(1, Math.min(100, Math.trunc(limit) || 20))
  const options = {
    query: { limit: boundedLimit },
    ...(signal ? { signal } : {}),
  }
  const response = record(await api(ACTION_PROPOSALS_BASE, options))
  return Array.isArray(response.proposals)
    ? response.proposals.map(normalizeMetaAdsActionProposal).filter((proposal) => proposal.id)
    : []
}

export async function confirmMetaActionProposal(
  api: ApiRequest,
  proposalId: string,
  confirmationKey: string,
  acknowledgeSpend: boolean,
  signal?: AbortSignal,
): Promise<MetaAdsActionProposalView> {
  const path = `${ACTION_PROPOSALS_BASE}/${encodeURIComponent(proposalId)}/confirm`
  const options = {
    method: 'POST' as const,
    headers: { 'Idempotency-Key': confirmationKey },
    body: acknowledgeSpend ? { acknowledgeSpend: true } : {},
    ...(signal ? { signal } : {}),
  }
  return normalizeMetaAdsActionProposal(await api(path, options))
}

export async function reconcileMetaActionProposal(
  api: ApiRequest,
  proposalId: string,
  signal?: AbortSignal,
): Promise<MetaAdsActionProposalView> {
  const path = `${ACTION_PROPOSALS_BASE}/${encodeURIComponent(proposalId)}/reconcile`
  const options = { method: 'POST' as const, ...(signal ? { signal } : {}) }
  return normalizeMetaAdsActionProposal(await api(path, options))
}

export async function cancelMetaActionProposal(
  api: ApiRequest,
  proposalId: string,
  cancellationKey: string,
  signal?: AbortSignal,
): Promise<MetaAdsActionProposalView> {
  const path = `${ACTION_PROPOSALS_BASE}/${encodeURIComponent(proposalId)}/cancel`
  const options = {
    method: 'POST' as const,
    headers: { 'Idempotency-Key': cancellationKey },
    ...(signal ? { signal } : {}),
  }
  return normalizeMetaAdsActionProposal(await api(path, options))
}
