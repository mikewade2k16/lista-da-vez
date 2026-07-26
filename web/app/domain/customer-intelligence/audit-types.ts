export interface IntelligenceAuditDiffItem {
  fieldLabel: string
  changeType: 'added' | 'removed' | 'changed'
  oldDisplay?: string
  newDisplay?: string
}

export interface IntelligenceAuditEventView {
  id: string
  clientAccountId?: string
  action: string
  entityType: string
  entityRef: string
  occurredAt: string
  actor: {
    type: string
    ref?: string
    display?: string
  }
  oldHash?: string
  newHash?: string
  diff?: IntelligenceAuditDiffItem[]
  sourceRef?: string
  observationRef?: string
  reasonCode?: string
  correlationCode?: string
  canOpenObservation: boolean
  canNavigate: boolean
}

export interface IntelligenceObservationView {
  id: string
  sourceKey: string
  provenanceRef?: string
  sensitivity: string
  purposeKey: string
  retentionState: string
  observedAt: string
  expiresAt?: string
  revealed?: boolean
  snapshotFields: Array<{
    label: string
    displayValue: string
    masked: boolean
  }>
}

export interface IntelligenceAuditFilterOptions {
  actions: Array<{ value: string; label: string }>
  entityTypes: Array<{ value: string; label: string }>
  statuses: Array<{ value: string; label: string }>
  provenances: Array<{ value: string; label: string }>
}

export interface IntelligenceAuditFilters {
  clientAccountId: string
  action?: string
  entityType?: string
  status?: string
  provenance?: string
  occurredFrom?: string
  occurredTo?: string
  cursor?: string
  limit?: number
}

export interface IntelligenceAuditPage {
  items: IntelligenceAuditEventView[]
  nextCursor: string
  filterOptions: IntelligenceAuditFilterOptions
}
