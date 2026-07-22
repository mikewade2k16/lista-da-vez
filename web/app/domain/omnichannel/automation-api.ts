import type { ApiRequest } from '~/domain/omnichannel/config-api'

const BASE = '/v1/omnichannel/automation'

export interface AutomationClientRef {
  id: string
  slug: string
  name: string
}

export interface AutomationBusinessContext {
  clientId: string
  segment: string
  positioning: string
  description: string
  history: string
  siteUrl: string
  instagram: string
  address: string
  objectives: string
  brandVoice: string
  extra: Record<string, string>
  updatedAt: string
}

export interface AutomationClosePolicy {
  autoCloseEnabled: boolean
  minimumConfidence: number
  requireAllRequiredFields: boolean
  blockOnHumanRequest: boolean
  blockSensitiveTopics: boolean
  validGenerationRequired: true
}

export interface AutomationProfile {
  id?: string
  configured: boolean
  client: AutomationClientRef
  whatsappInstance: {
    id: string
    instanceName: string
    provider: string
    displayName: string | null
    phoneNumber: string | null
    active: boolean
  } | null
  aiAgent: {
    id: string
    name: string
    enabled: boolean
    activeVersionId: string | null
  } | null
  enabled: boolean
  ready: boolean
  readinessIssues: string[]
  closePolicy: AutomationClosePolicy
  strategicContext?: {
    source: string
    available: boolean
    filled: boolean
    profile: AutomationBusinessContext
  }
  revision: number
  createdAt?: string
  updatedAt?: string
}

export interface AutomationProfileInput {
  whatsappInstanceId: string
  aiAgentId: string
  enabled: boolean
  closePolicy: Omit<AutomationClosePolicy, 'validGenerationRequired'>
}

export interface AutomationIntervention {
  id: string
  client: AutomationClientRef
  conversationId: string
  contactName: string
  contactPhone: string
  whatsappInstanceId: string
  instanceName: string
  reasonCode: string
  summary: string
  collectedFieldKeys: string[]
  status: string
  conversationState: string
  targetQueueId: string | null
  waitingSince: string
}

export type AutomationAttendanceMode = 'ai_active' | 'ai_stopped'

export interface AutomationAttendance {
  id: string
  mode: AutomationAttendanceMode
  client: AutomationClientRef
  conversationId: string
  contactName: string
  contactPhone: string
  whatsappInstanceId: string
  instanceName: string
  conversationState: string
  dispatchStatus: string
  handoffId: string | null
  reasonCode: string
  summary: string
  aiConfidence: number | null
  minimumConfidence: number | null
  maxAiTurns: number | null
  unansweredCount: number
  pendingMessagePreview: string
  pendingSince: string | null
  activitySince: string
}

export function fetchAutomationProfiles(api: ApiRequest): Promise<AutomationProfile[]> {
  return api(`${BASE}/profiles`, { dedupe: false }) as Promise<AutomationProfile[]>
}

export function fetchAutomationProfile(
  api: ApiRequest,
  clientId: string,
): Promise<AutomationProfile> {
  return api(`${BASE}/profiles/${encodeURIComponent(clientId)}`, {
    dedupe: false,
  }) as Promise<AutomationProfile>
}

export function saveAutomationProfile(
  api: ApiRequest,
  clientId: string,
  input: AutomationProfileInput,
): Promise<AutomationProfile> {
  return api(`${BASE}/profiles/${encodeURIComponent(clientId)}`, {
    method: 'PUT',
    body: input,
  }) as Promise<AutomationProfile>
}

export function fetchAutomationInterventions(
  api: ApiRequest,
  clientId = '',
): Promise<AutomationIntervention[]> {
  const query = new URLSearchParams({ limit: '100' })
  if (clientId) query.set('clientId', clientId)
  return api(`${BASE}/interventions?${query.toString()}`, { dedupe: false }) as Promise<
    AutomationIntervention[]
  >
}

export function fetchAutomationAttendances(
  api: ApiRequest,
  clientId = '',
): Promise<AutomationAttendance[]> {
  const query = new URLSearchParams({ limit: '100' })
  if (clientId) query.set('clientId', clientId)
  return api(`${BASE}/attendances?${query.toString()}`, { dedupe: false }) as Promise<
    AutomationAttendance[]
  >
}

export function pauseAutomationAI(
  api: ApiRequest,
  conversationId: string,
  idempotencyKey: string,
): Promise<unknown> {
  return api(`${BASE}/conversations/${encodeURIComponent(conversationId)}/pause-ai`, {
    method: 'POST',
    body: { idempotencyKey },
  })
}

export function replyAutomationWithAI(
  api: ApiRequest,
  conversationId: string,
  idempotencyKey: string,
): Promise<unknown> {
  return api(`${BASE}/conversations/${encodeURIComponent(conversationId)}/reply-with-ai`, {
    method: 'POST',
    body: { idempotencyKey },
  })
}

export function resumeAutomationOnNextInbound(
  api: ApiRequest,
  conversationId: string,
): Promise<unknown> {
  return api(`/v1/omnichannel/conversations/${encodeURIComponent(conversationId)}/status`, {
    method: 'PATCH',
    body: { status: 'CLOSED' },
  })
}
