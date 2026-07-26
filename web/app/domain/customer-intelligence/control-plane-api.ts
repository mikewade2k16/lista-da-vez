import type { createApiRequest } from '~/utils/api-client'
import type {
  IntelligenceCapabilityKey,
  IntelligenceCapabilityView,
  IntelligenceCapabilityWriteInput,
} from './control-plane-types'

type ControlPlaneApi = ReturnType<typeof createApiRequest>

function clientQuery(clientAccountId: string): string {
  const query = new URLSearchParams()
  if (clientAccountId.trim()) query.set('clientAccountId', clientAccountId.trim())
  return query.toString()
}

export function fetchIntelligenceCapability(
  api: ControlPlaneApi,
  key: IntelligenceCapabilityKey,
  clientAccountId: string,
  signal?: AbortSignal,
): Promise<IntelligenceCapabilityView> {
  return api(
    `/v1/customer-intelligence/capabilities/${encodeURIComponent(key)}?${clientQuery(clientAccountId)}`,
    { signal, dedupe: false },
  ) as Promise<IntelligenceCapabilityView>
}

export function updateIntelligenceCapability(
  api: ControlPlaneApi,
  key: IntelligenceCapabilityKey,
  input: IntelligenceCapabilityWriteInput,
): Promise<IntelligenceCapabilityView> {
  return api(`/v1/customer-intelligence/capabilities/${encodeURIComponent(key)}`, {
    method: 'PUT',
    body: input,
  }) as Promise<IntelligenceCapabilityView>
}
