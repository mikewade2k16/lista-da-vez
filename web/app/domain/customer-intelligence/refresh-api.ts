import type { createApiRequest } from '~/utils/api-client'

type RefreshApi = ReturnType<typeof createApiRequest>

export interface RelationshipRefreshJob {
  id?: string
  status: 'pending' | 'existing' | string
  created: boolean
}

export function enqueueRelationshipIntelligenceRefresh(
  api: RefreshApi,
  relationshipId: string,
  clientAccountId: string,
  subjectId: string,
  idempotencyKey: string,
): Promise<RelationshipRefreshJob> {
  return api(
    `/v1/customer-intelligence/relationships/${encodeURIComponent(relationshipId)}/refresh`,
    {
      method: 'POST',
      body: {
        clientAccountId,
        subjectId,
        purposeKey: 'customer_profile',
        idempotencyKey,
      },
    },
  ) as Promise<RelationshipRefreshJob>
}
