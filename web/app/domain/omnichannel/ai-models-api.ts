import type { ApiRequest } from '~/domain/omnichannel/config-api'

const AGENTS = '/v1/omnichannel/agents'

export async function fetchAgentModels(
  api: ApiRequest,
  agentId: string,
  provider: string,
): Promise<string[]> {
  const query = new URLSearchParams({ provider }).toString()
  const response = (await api(`${AGENTS}/${encodeURIComponent(agentId)}/models?${query}`, {
    dedupe: false,
  })) as { models?: unknown }
  return Array.isArray(response.models)
    ? response.models.filter((model): model is string => typeof model === 'string')
    : []
}
