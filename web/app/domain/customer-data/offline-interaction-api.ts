import type { createApiRequest } from '~/utils/api-client'
import type {
  CreateOfflineInteractionInput,
  OfflineInteractionsPage,
  OfflineInteractionView,
} from './offline-interaction-types'

type OfflineApi = ReturnType<typeof createApiRequest>

interface OfflineResponse {
  items?: OfflineInteractionView[]
  interactions?: OfflineInteractionView[]
  nextCursor?: string
  createDescriptor?: OfflineInteractionsPage['createDescriptor']
}

function query(clientAccountId: string, cursor = ''): string {
  const params = new URLSearchParams()
  if (clientAccountId.trim()) params.set('clientAccountId', clientAccountId.trim())
  if (cursor.trim()) params.set('cursor', cursor.trim())
  params.set('limit', '40')
  return params.toString()
}

export async function fetchOfflineInteractions(
  api: OfflineApi,
  relationshipId: string,
  clientAccountId: string,
  cursor = '',
  signal?: AbortSignal,
): Promise<OfflineInteractionsPage> {
  const response = (await api(
    `/v1/customer-data/relationships/${encodeURIComponent(relationshipId)}/offline-interactions?${query(clientAccountId, cursor)}`,
    { signal, dedupe: false },
  )) as OfflineResponse
  return {
    items: response.items ?? response.interactions ?? [],
    nextCursor: String(response.nextCursor ?? ''),
    createDescriptor: response.createDescriptor,
  }
}

export function createOfflineInteraction(
  api: OfflineApi,
  relationshipId: string,
  input: CreateOfflineInteractionInput,
): Promise<OfflineInteractionView> {
  return api(
    `/v1/customer-data/relationships/${encodeURIComponent(relationshipId)}/offline-interactions`,
    { method: 'POST', body: input },
  ) as Promise<OfflineInteractionView>
}
