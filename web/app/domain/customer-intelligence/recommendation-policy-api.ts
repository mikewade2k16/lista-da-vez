import type { createApiRequest } from '~/utils/api-client'
import type {
  RecommendationPolicyValue,
  RecommendationPolicyVersion,
  RecommendationPolicyView,
} from './recommendation-policy-types'

type PolicyApi = ReturnType<typeof createApiRequest>

interface PoliciesResponse {
  items?: RecommendationPolicyView[]
  policies?: RecommendationPolicyView[]
}

export async function fetchRecommendationPolicies(
  api: PolicyApi,
  signal?: AbortSignal,
): Promise<RecommendationPolicyView[]> {
  const response = (await api('/v1/customer-intelligence/recommendation-policies', {
    signal,
    dedupe: false,
  })) as PoliciesResponse | RecommendationPolicyView[]
  return Array.isArray(response) ? response : (response.items ?? response.policies ?? [])
}

export function createRecommendationPolicyDraft(
  api: PolicyApi,
  policyKey: string,
): Promise<RecommendationPolicyVersion> {
  return api(
    `/v1/customer-intelligence/recommendation-policies/${encodeURIComponent(policyKey)}/drafts`,
    { method: 'POST', body: {} },
  ) as Promise<RecommendationPolicyVersion>
}

export function updateRecommendationPolicyDraft(
  api: PolicyApi,
  versionId: string,
  values: Record<string, RecommendationPolicyValue>,
  expectedRevision: number,
): Promise<RecommendationPolicyVersion> {
  return api(
    `/v1/customer-intelligence/recommendation-policy-versions/${encodeURIComponent(versionId)}`,
    { method: 'PATCH', body: { values, expectedRevision } },
  ) as Promise<RecommendationPolicyVersion>
}

export function runRecommendationPolicyAction(
  api: PolicyApi,
  versionId: string,
  action: 'validate' | 'publish',
  expectedRevision: number,
): Promise<RecommendationPolicyVersion> {
  return api(
    `/v1/customer-intelligence/recommendation-policy-versions/${encodeURIComponent(versionId)}/${action}`,
    { method: 'POST', body: { expectedRevision } },
  ) as Promise<RecommendationPolicyVersion>
}

export function rollbackRecommendationPolicyBinding(
  api: PolicyApi,
  bindingId: string,
  expectedRevision: number,
): Promise<RecommendationPolicyView> {
  return api(
    `/v1/customer-intelligence/recommendation-policy-bindings/${encodeURIComponent(bindingId)}/rollback`,
    {
      method: 'POST',
      body: { expectedRevision, reasonCode: 'panel_rollback' },
    },
  ) as Promise<RecommendationPolicyView>
}
