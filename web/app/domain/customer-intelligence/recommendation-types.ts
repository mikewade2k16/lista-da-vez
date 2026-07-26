export type RecommendationType = 'follow_up' | 'offer' | 'important_date' | 'next_action'

export type RecommendationStatus =
  | 'proposed'
  | 'accepted'
  | 'approved'
  | 'rejected'
  | 'executing'
  | 'executed'
  | 'expired'
  | 'invalidated'
  | 'stale'

export interface CustomerRecommendationView {
  id: string
  relationshipId: string
  type: RecommendationType
  status: RecommendationStatus
  title: string
  rationale: string
  confidence?: number
  validFrom?: string
  validUntil?: string
  freshnessStatus?: string
  generatedByAi: boolean
  revision: number
  evidenceRefs: Array<{
    ref: string
    label: string
    sourceKey?: string
  }>
  promptVersionRef?: string
  modelRef?: string
  policyVersionRef?: string
  constraints: Array<{
    label: string
    status: 'passed' | 'warning' | 'blocked'
    reasonCode?: string
  }>
  allowedActions: Array<'approve' | 'reject'>
}

export interface RecommendationsPage {
  items: CustomerRecommendationView[]
  nextCursor: string
  decisionOptions: RecommendationDecisionOptions
}

export interface RecommendationDecisionOptions {
  approveReasons: Array<{ value: string; label: string }>
  rejectReasons: Array<{ value: string; label: string }>
  invalidateReasons: Array<{ value: string; label: string }>
  executeReasons: Array<{ value: string; label: string }>
}

export interface RecommendationDecisionInput {
  reason: string
}

export interface RecommendationActionResult {
  recommendationId: string
  status: RecommendationStatus
  revision: number
  jobRef?: string
  ownerModule?: string
}
