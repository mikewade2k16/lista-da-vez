import type { createApiRequest } from '~/utils/api-client'
import type {
  CustomerRelationshipProfile,
  CustomerSubjectListFilters,
  CustomerSubjectListItem,
  CustomerSubjectsPage,
} from './profile-types'

type CustomerDataApi = ReturnType<typeof createApiRequest>

interface CustomerSubjectsResponse {
  items?: CustomerSubjectListItem[]
  subjects?: CustomerSubjectListItem[]
  nextCursor?: string
  hasMore?: boolean
}

function appendIfPresent(query: URLSearchParams, key: string, value: string | undefined): void {
  const normalized = String(value ?? '').trim()
  if (normalized) query.set(key, normalized)
}

export async function fetchCustomerSubjects(
  api: CustomerDataApi,
  filters: CustomerSubjectListFilters,
  signal?: AbortSignal,
): Promise<CustomerSubjectsPage> {
  const query = new URLSearchParams()
  appendIfPresent(query, 'clientAccountId', filters.clientAccountId)
  appendIfPresent(query, 'q', filters.query)
  appendIfPresent(query, 'lifecycleStatus', filters.lifecycleStatus)
  appendIfPresent(query, 'cursor', filters.cursor)
  query.set('limit', String(Math.min(Math.max(filters.limit ?? 50, 1), 100)))

  const response = (await api(`/v1/customer-data/subjects?${query.toString()}`, {
    signal,
    dedupe: false,
  })) as CustomerSubjectsResponse
  const items = Array.isArray(response.items)
    ? response.items
    : Array.isArray(response.subjects)
      ? response.subjects
      : []

  return {
    items,
    nextCursor: String(response.nextCursor ?? ''),
    hasMore: response.hasMore === true || Boolean(response.nextCursor),
  }
}

export function fetchCustomerRelationshipProfile(
  api: CustomerDataApi,
  relationshipId: string,
  clientAccountId: string,
  signal?: AbortSignal,
): Promise<CustomerRelationshipProfile> {
  const query = new URLSearchParams()
  appendIfPresent(query, 'clientAccountId', clientAccountId)
  const suffix = query.size ? `?${query.toString()}` : ''
  return api(
    `/v1/customer-data/relationships/${encodeURIComponent(relationshipId)}/profile${suffix}`,
    { signal, dedupe: false },
  ) as Promise<CustomerRelationshipProfile>
}
