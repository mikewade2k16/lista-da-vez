export interface CustomerIdentityView {
  id: string
  kind: string
  maskedValue: string
  verificationStatus: string
  provenance?: string
}

export interface CustomerRelationshipSummary {
  id: string
  clientAccountId: string
  displayName: string
  preferredName?: string | null
  lifecycleStatus: string
  tags: string[]
  ownerUserId?: string | null
  firstSeenAt?: string | null
  lastSeenAt?: string | null
  revision: number
  updatedAt: string
}

export interface CustomerSubjectListItem {
  subjectId: string
  subjectType: string
  relationship: CustomerRelationshipSummary
  primaryIdentities?: CustomerIdentityView[]
}

export interface CustomerSubjectsPage {
  items: CustomerSubjectListItem[]
  nextCursor: string
  hasMore: boolean
}

export interface CustomerSourceLink {
  sourceKey: string
  status: string
  freshness?: string | null
  reasonCode?: string
}

export interface CustomerNoteView {
  id: string
  content: string
  createdAt: string
  authorDisplayName?: string
}

export interface CustomerConsentView {
  purposeKey: string
  channel: string
  status: string
  effectiveAt?: string | null
  expiresAt?: string | null
}

export interface CustomerRelationshipProfile {
  subject: {
    id: string
    subjectType: string
    displayName?: string
    preferredName?: string | null
    aliases?: string[]
    mergeStatus?: string
  }
  relationship: CustomerRelationshipSummary
  identities?: CustomerIdentityView[]
  sourceLinks?: CustomerSourceLink[]
  notes?: CustomerNoteView[]
  consents?: CustomerConsentView[]
  touchpointRefs?: Array<{
    id: string
    sourceKind: string
    occurredAt: string
  }>
  capabilities?: Record<string, boolean>
}

export interface CustomerSubjectListFilters {
  clientAccountId: string
  query?: string
  lifecycleStatus?: string
  cursor?: string
  limit?: number
}
