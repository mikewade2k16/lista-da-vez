import type { createApiRequest } from '~/utils/api-client'
import type { OmniAiToolApproval, OmniAiToolRun } from '~/domain/omnichannel/config-types'

export type ApiRequest = ReturnType<typeof createApiRequest>

const AGENTS = '/v1/omnichannel/agents'

export function fetchAiToolRuns(
  api: ApiRequest,
  agentId: string,
  options: { status?: OmniAiToolRun['status']; limit?: number } = {},
): Promise<OmniAiToolRun[]> {
  const query = new URLSearchParams()
  if (options.status) query.set('status', options.status)
  if (options.limit) query.set('limit', String(options.limit))
  const suffix = query.toString() ? `?${query.toString()}` : ''
  return api(`${AGENTS}/${encodeURIComponent(agentId)}/tool-runs${suffix}`, {
    dedupe: false,
  }) as Promise<OmniAiToolRun[]>
}

export function fetchAiToolApprovals(
  api: ApiRequest,
  agentId: string,
  limit = 30,
): Promise<OmniAiToolApproval[]> {
  const query = new URLSearchParams({ limit: String(limit) }).toString()
  return api(`${AGENTS}/${encodeURIComponent(agentId)}/tool-approvals?${query}`, {
    dedupe: false,
  }) as Promise<OmniAiToolApproval[]>
}

export function approveAiToolApproval(
  api: ApiRequest,
  agentId: string,
  approvalId: string,
): Promise<OmniAiToolApproval> {
  return api(
    `${AGENTS}/${encodeURIComponent(agentId)}/tool-approvals/${encodeURIComponent(approvalId)}/approve`,
    { method: 'POST' },
  ) as Promise<OmniAiToolApproval>
}

export function rejectAiToolApproval(
  api: ApiRequest,
  agentId: string,
  approvalId: string,
): Promise<OmniAiToolApproval> {
  return api(
    `${AGENTS}/${encodeURIComponent(agentId)}/tool-approvals/${encodeURIComponent(approvalId)}/reject`,
    { method: 'POST' },
  ) as Promise<OmniAiToolApproval>
}
