import type { createApiRequest } from '~/utils/api-client'

export type ContentAlertSeverity = 'critical' | 'attention' | 'info'

export interface ContentOperationsAlert {
  id: string
  type: string
  severity: ContentAlertSeverity
  title: string
  body: string
  clientId: string
  clientName: string
  sourceKind?: string
  sourceId?: string
  occurredOn?: string
  linkPath: string
}

export interface ContentClientHealth {
  clientId: string
  clientName: string
  critical: number
  attention: number
  info: number
  lastPostedOn?: string
}

export interface ContentOperationsBrief {
  generatedAt: string
  today: string
  mode: 'planning' | 'closing' | 'follow_up'
  headline: string
  summary: string
  counts: { critical: number; attention: number; info: number; total: number }
  clients: ContentClientHealth[]
  alerts: ContentOperationsAlert[]
}

type ApiRequest = ReturnType<typeof createApiRequest>

export async function fetchContentOperationsBrief(api: ApiRequest) {
  return api('/v1/content-operations/brief', {
    method: 'GET',
    skipLoadingIndicator: true,
    dedupe: false,
  }) as Promise<ContentOperationsBrief>
}
