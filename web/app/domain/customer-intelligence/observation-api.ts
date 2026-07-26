import type { createApiRequest } from '~/utils/api-client'
import type { IntelligenceObservationView } from './audit-types'

type CustomerIntelligenceApi = ReturnType<typeof createApiRequest>

function observationQuery(clientAccountId: string, sourceKeys: string[]): string {
  const query = new URLSearchParams()
  const client = clientAccountId.trim()
  if (client) query.set('clientAccountId', client)
  query.set('limit', '100')
  for (const sourceKey of [...new Set(sourceKeys.map((item) => item.trim()).filter(Boolean))]) {
    query.append('sourceKey', sourceKey)
  }
  return query.toString()
}

export async function fetchRelationshipObservations(
  api: CustomerIntelligenceApi,
  relationshipId: string,
  clientAccountId: string,
  sourceKeys: string[] = [],
  signal?: AbortSignal,
): Promise<IntelligenceObservationView[]> {
  const response = (await api(
    `/v1/customer-intelligence/relationships/${encodeURIComponent(relationshipId)}/observations?${observationQuery(clientAccountId, sourceKeys)}`,
    { signal, dedupe: false },
  )) as IntelligenceObservationView[]
  return Array.isArray(response) ? response : []
}
