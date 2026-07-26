import type { createApiRequest } from '~/utils/api-client'

export type ApiRequest = ReturnType<typeof createApiRequest>
export type ChannelBindingChannel = 'WHATSAPP' | 'INSTAGRAM'
export type ChannelBindingMode = 'legacy' | 'shadow' | 'enforced'
export type CustomerIntelligenceMode = 'off' | 'shadow' | 'on'
export type CustomerIntelligenceFailurePolicy =
  | 'legacy_fallback'
  | 'retry_then_handoff'
  | 'immediate_handoff'

export interface ChannelClientBinding {
  id: string
  clientAccountId: string
  clientAccountName?: string
  channel: ChannelBindingChannel
  channelResource: {
    type: 'whatsapp_instance' | 'instagram_account'
    id: string
    label: string
  }
  effectiveFrom: string
  effectiveTo: string | null
  source: 'manual' | 'automation_profile_backfill' | 'standalone_default'
  reason: string
  revision: number
  createdAt: string
  updatedAt: string
}

export interface ChannelClientBindingPage {
  items: ChannelClientBinding[]
  hasMore: boolean
  nextCursor?: string
}

export interface ChannelClientBindingException {
  channel: ChannelBindingChannel
  channelResourceId?: string
  bindingState: 'unresolved' | 'quarantined'
  reasonCode: string
  conversationCount: number
  touchpointCount: number
  latestConversationAt?: string
}

export interface ChannelClientBindingPolicy {
  channelBindingMode: ChannelBindingMode
  customerIntelligenceMode: CustomerIntelligenceMode
  customerIntelligenceFailurePolicy: CustomerIntelligenceFailurePolicy
  revision: number
  updatedAt: string
}

export interface ChannelClientBindingRepairJob {
  id: string
  channel: ChannelBindingChannel
  channelResourceId: string
  clientAccountId: string
  bindingId: string
  mode: 'preview' | 'apply'
  status: 'queued' | 'processing' | 'completed' | 'partial' | 'failed' | 'cancelled'
  watermark: string
  previewJobId?: string
  previewChecksum: string
  scannedCount: number
  eligibleCount: number
  repairedCount: number
  quarantinedCount: number
  skippedCount: number
  lastErrorCode?: string
  createdAt: string
  updatedAt: string
}

const BASE = '/v1/omnichannel'

export function fetchChannelClientBindings(api: ApiRequest): Promise<ChannelClientBindingPage> {
  return api(`${BASE}/channel-client-bindings?state=active&limit=100`, {
    dedupe: false,
  }) as Promise<ChannelClientBindingPage>
}

export function fetchChannelClientBindingExceptions(
  api: ApiRequest,
): Promise<{ items: ChannelClientBindingException[] }> {
  return api(`${BASE}/channel-client-binding-exceptions`, {
    dedupe: false,
  }) as Promise<{ items: ChannelClientBindingException[] }>
}

export function fetchChannelClientBindingPolicy(
  api: ApiRequest,
): Promise<ChannelClientBindingPolicy> {
  return api(`${BASE}/channel-client-binding-policy`, {
    dedupe: false,
  }) as Promise<ChannelClientBindingPolicy>
}

export function createChannelClientBinding(
  api: ApiRequest,
  input: {
    clientAccountId: string
    channel: ChannelBindingChannel
    channelResourceId: string
    reason: string
    idempotencyKey: string
  },
): Promise<ChannelClientBinding> {
  return api(`${BASE}/channel-client-bindings`, {
    method: 'POST',
    body: input,
  }) as Promise<ChannelClientBinding>
}

export function reassignChannelClientBinding(
  api: ApiRequest,
  bindingId: string,
  input: {
    targetClientAccountId: string
    effectiveAt: string
    reason: string
    expectedRevision: number
    idempotencyKey: string
  },
): Promise<ChannelClientBinding> {
  return api(`${BASE}/channel-client-bindings/${encodeURIComponent(bindingId)}/reassign`, {
    method: 'POST',
    body: input,
  }) as Promise<ChannelClientBinding>
}

export function endChannelClientBinding(
  api: ApiRequest,
  bindingId: string,
  input: {
    effectiveAt: string
    reason: string
    expectedRevision: number
    idempotencyKey: string
  },
): Promise<ChannelClientBinding> {
  return api(`${BASE}/channel-client-bindings/${encodeURIComponent(bindingId)}/end`, {
    method: 'POST',
    body: input,
  }) as Promise<ChannelClientBinding>
}

export function updateChannelClientBindingPolicy(
  api: ApiRequest,
  input: {
    channelBindingMode: ChannelBindingMode
    customerIntelligenceMode: CustomerIntelligenceMode
    customerIntelligenceFailurePolicy: CustomerIntelligenceFailurePolicy
    expectedRevision: number
  },
): Promise<ChannelClientBindingPolicy> {
  return api(`${BASE}/channel-client-binding-policy`, {
    method: 'PUT',
    body: input,
  }) as Promise<ChannelClientBindingPolicy>
}

export function createChannelClientBindingRepairPreview(
  api: ApiRequest,
  input: {
    bindingId: string
    watermark: string
    reason: string
    idempotencyKey: string
    includeClosed: boolean
    confirmNoRetroactiveMove: true
  },
): Promise<ChannelClientBindingRepairJob> {
  return api(`${BASE}/channel-client-binding-repair-previews`, {
    method: 'POST',
    body: input,
  }) as Promise<ChannelClientBindingRepairJob>
}

export function applyChannelClientBindingRepair(
  api: ApiRequest,
  input: {
    previewId: string
    previewChecksum: string
    reason: string
    idempotencyKey: string
    confirm: true
  },
): Promise<ChannelClientBindingRepairJob> {
  return api(`${BASE}/channel-client-binding-repair-jobs`, {
    method: 'POST',
    body: input,
  }) as Promise<ChannelClientBindingRepairJob>
}
