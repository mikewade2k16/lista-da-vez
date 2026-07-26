export type PortfolioOpportunityStatus =
  | 'proposed'
  | 'approved'
  | 'rejected'
  | 'expired'
  | 'suppressed'

export interface PortfolioOpportunityView {
  id: string
  status: PortfolioOpportunityStatus
  title: string
  summary: string
  purposeKey: string
  cohortClass: string
  cohortSizeBucket?: string
  targetClients: Array<{
    clientAccountRef: string
    displayName: string
    rationale: string
  }>
  freshnessStatus: string
  asOf: string
  policyVersionRef: string
  promptBindingRef?: string
  modelRef?: string
  evaluationRef?: string
  estimatedCost?: {
    amount: number
    currency: string
  }
  protection: {
    aggregateOnly: boolean
    contributorsSuppressed: boolean
    piiSuppressed: boolean
    reasonCodes: string[]
  }
  allowedActions: Array<'approve' | 'reject'>
  revision: number
}

export interface PortfolioFilterDescriptor {
  key: 'status' | 'purposeKey' | 'cohortClass' | 'freshnessStatus'
  label: string
  options: Array<{ value: string; label: string }>
}

export interface PortfolioOpportunitiesPage {
  items: PortfolioOpportunityView[]
  nextCursor: string
  filters: PortfolioFilterDescriptor[]
  decisionReasons: {
    approve: Array<{ value: string; label: string }>
    reject: Array<{ value: string; label: string }>
  }
  policySummary: {
    purposeKey: string
    policyVersionRef: string
    minimumCohortLabel: string
    suppressionMode: string
    freshnessPolicy: string
  }
}

export interface PortfolioFilters {
  targetClientAccountId?: string
  status?: string
  purposeKey?: string
  cohortClass?: string
  freshnessStatus?: string
  cursor?: string
  limit?: number
}

export interface PortfolioDecisionResult {
  opportunityId: string
  status: PortfolioOpportunityStatus
  revision: number
}
