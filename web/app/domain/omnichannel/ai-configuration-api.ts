import type { ApiRequest } from '~/domain/omnichannel/config-api'
import type { OmniAgentVersion, OmniAgentVersionInput } from '~/domain/omnichannel/config-types'

const AGENTS = '/v1/omnichannel/agents'

export function saveAgentConfiguration(
  api: ApiRequest,
  id: string,
  input: OmniAgentVersionInput,
): Promise<OmniAgentVersion> {
  return api(`${AGENTS}/${encodeURIComponent(id)}/configuration`, {
    method: 'PUT',
    body: input,
  }) as Promise<OmniAgentVersion>
}
