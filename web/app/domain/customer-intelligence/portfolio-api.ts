import type { createApiRequest } from '~/utils/api-client'
import type {
  PortfolioFilters,
  PortfolioOpportunitiesPage,
  PortfolioOpportunityStatus,
  PortfolioOpportunityView,
} from './portfolio-types'

type PortfolioApi = ReturnType<typeof createApiRequest>

interface BackendPortfolioOpportunity {
  id: string
  targetClientAccountId: string
  segmentKey: string
  cohortClass: string
  opportunityType: string
  rationaleCode: string
  confidence: number
  status: string
  expiresAt?: string
  createdAt: string
}

const EMPTY_POLICY: PortfolioOpportunitiesPage['policySummary'] = {
  purposeKey: '',
  policyVersionRef: '',
  minimumCohortLabel: '',
  suppressionMode: 'contributors_hidden',
  freshnessPolicy: '',
}

function status(value: string): PortfolioOpportunityStatus {
  if (
    value === 'proposed' ||
    value === 'approved' ||
    value === 'rejected' ||
    value === 'expired' ||
    value === 'suppressed'
  ) {
    return value
  }
  return 'proposed'
}

function normalize(item: BackendPortfolioOpportunity): PortfolioOpportunityView {
  return {
    id: item.id,
    status: status(item.status),
    title: item.opportunityType.replace(/[._-]+/g, ' '),
    summary: item.rationaleCode,
    purposeKey: 'portfolio_opportunity',
    cohortClass: item.cohortClass || 'suppressed',
    cohortSizeBucket: item.cohortClass || 'suppressed',
    targetClients: [
      {
        clientAccountRef: item.targetClientAccountId,
        displayName: 'Cliente selecionado',
        rationale: item.rationaleCode,
      },
    ],
    freshnessStatus: item.expiresAt ? 'valid_until' : 'current',
    asOf: item.createdAt,
    policyVersionRef: 'server-side',
    protection: {
      aggregateOnly: true,
      contributorsSuppressed: true,
      piiSuppressed: true,
      reasonCodes: [item.rationaleCode],
    },
    // A API atual nao oferece endpoint de decisao/review para portfolio.
    allowedActions: [],
    revision: 0,
  }
}

export async function fetchPortfolioOpportunities(
  api: PortfolioApi,
  filters: PortfolioFilters,
  signal?: AbortSignal,
): Promise<PortfolioOpportunitiesPage> {
  const query = new URLSearchParams()
  if (filters.targetClientAccountId?.trim()) {
    query.set('targetClientAccountId', filters.targetClientAccountId.trim())
  }
  query.set('limit', String(Math.min(Math.max(filters.limit ?? 40, 1), 100)))
  const response = (await api(
    `/v1/customer-intelligence/portfolio/opportunities?${query.toString()}`,
    { signal, dedupe: false },
  )) as BackendPortfolioOpportunity[]
  const normalized = (Array.isArray(response) ? response : []).map(normalize)
  const items = normalized.filter((item) => {
    if (filters.status && item.status !== filters.status) return false
    if (filters.purposeKey && item.purposeKey !== filters.purposeKey) return false
    if (filters.cohortClass && item.cohortClass !== filters.cohortClass) return false
    if (filters.freshnessStatus && item.freshnessStatus !== filters.freshnessStatus) {
      return false
    }
    return true
  })
  return {
    items,
    nextCursor: '',
    filters: [],
    decisionReasons: { approve: [], reject: [] },
    policySummary: EMPTY_POLICY,
  }
}
