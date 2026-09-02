import type { createApiRequest } from '~/utils/api-client'

type ApiRequest = ReturnType<typeof createApiRequest>

export type OperationalStatus = 'ok' | 'degraded' | 'configured' | 'disabled'

export interface OperationalComponentHealth {
  status: OperationalStatus
  detail: string
}

export interface OperationalAlert {
  code: string
  severity: 'info' | 'warning' | 'critical'
  message: string
  action: string
  owner: string
  runbook: string
}

export interface OperationalHealth {
  status: 'ok' | 'degraded'
  generatedAt: string
  process: OperationalComponentHealth
  database: OperationalComponentHealth
  n8n: OperationalComponentHealth
  outbox: { pending: number; processing: number; dead: number; oldestPendingSeconds: number }
  ai: { queued: number; processing: number; stuckProcessing: number; failed24h: number }
  provider: {
    activeInstances: number
    missingCredentials: number
    webhookEvents24h: number
    lastWebhookReceivedAt: string | null
  }
  retention: { lastFinishedAt: string | null; lastError: string }
  bindings: { enabledProfiles: number; mismatches: number }
  alerts: OperationalAlert[]
}

export type RolloutMode =
  | 'off'
  | 'observe'
  | 'shadow'
  | 'assist'
  | 'auto_pilot'
  | 'active'
  | 'paused'

export interface RolloutWindow {
  days: number[]
  start: string
  end: string
}

export interface RolloutConfig {
  mode: RolloutMode
  allowedInstanceIds: string[]
  allowedInstagramAccountIds: string[]
  allowedQueueIds: string[]
  autoReplyPercent: number
  allowedHours: { timezone: string; windows: RolloutWindow[] }
  excludedTags: string[]
  maxDailyAutoReplies: number
  killSwitchReason: string | null
  revision: number
  legacyDefault: boolean
  updatedByUserId: string | null
  updatedAt: string | null
}

export interface RolloutConfigInput extends Omit<
  RolloutConfig,
  'revision' | 'legacyDefault' | 'updatedByUserId' | 'updatedAt'
> {
  expectedRevision: number
  reason: string
}

const BASE = '/v1/omnichannel'

export function fetchOperationalHealth(api: ApiRequest): Promise<OperationalHealth> {
  return api(`${BASE}/operations/health`, { dedupe: false }) as Promise<OperationalHealth>
}

export function fetchRolloutConfig(api: ApiRequest): Promise<RolloutConfig> {
  return api(`${BASE}/settings/rollout`, { dedupe: false }) as Promise<RolloutConfig>
}

export function putRolloutConfig(
  api: ApiRequest,
  input: RolloutConfigInput,
): Promise<RolloutConfig> {
  return api(`${BASE}/settings/rollout`, { method: 'PUT', body: input }) as Promise<RolloutConfig>
}
