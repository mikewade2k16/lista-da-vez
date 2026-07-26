export type SegmentCombinator = 'and' | 'or'
export type SegmentScalar = string | number | boolean
export type SegmentValue = SegmentScalar | SegmentScalar[] | null

export interface SegmentPredicateNode {
  kind: 'predicate'
  nodeId: string
  fieldKey: string
  operator: string
  value: SegmentValue
}

export interface SegmentGroupNode {
  kind: 'group'
  nodeId: string
  combinator: SegmentCombinator
  children: SegmentFilterNode[]
}

export type SegmentFilterNode = SegmentGroupNode | SegmentPredicateNode

export interface SegmentFilterAst {
  schemaVersion: string
  root: SegmentGroupNode
}

export interface SegmentFieldDescriptor {
  fieldKey: string
  label: string
  description?: string
  valueType: 'string' | 'number' | 'boolean' | 'date' | 'enum'
  operators: string[]
  options?: Array<{ value: string; label: string }>
  availability: 'available' | 'partial' | 'unavailable'
  sourceKey?: string
  maxLength?: number
}

export interface SegmentFieldCatalog {
  version: string
  schemaVersion: string
  fields: SegmentFieldDescriptor[]
  caps: {
    maxDepth: number
    maxPredicates: number
    maxListItems: number
    maxStringLength: number
  }
}

export interface SegmentCapabilities {
  segmentationMode: 'off' | 'shadow' | 'canary' | 'on'
  exportMode: 'off' | 'shadow' | 'canary' | 'on'
  reasonCodes?: string[]
}

export interface CustomerSegmentListItem {
  id: string
  segmentKey: string
  name: string
  description?: string
  status: 'draft' | 'active' | 'archived'
  revision: number
  activeVersionId?: string
  activeVersionNumber?: number
  memberCountBucket?: string
  lastMaterializedAt?: string
  freshnessStatus?: 'fresh' | 'stale' | 'partial' | 'unknown'
  updatedAt: string
}

export interface SegmentVersionView {
  id: string
  versionNumber: number
  status: 'draft' | 'validated' | 'published' | 'superseded'
  revision: number
  fieldCatalogVersion: string
  definitionHash?: string
  filterAst?: SegmentFilterAst
  validationId?: string
  validationStatus?: 'pending' | 'valid' | 'invalid'
  validationReasonCodes?: string[]
  createdAt: string
  publishedAt?: string
}

export interface CustomerSegmentView {
  segment: CustomerSegmentListItem
  versions: SegmentVersionView[]
  draft?: SegmentVersionView
  capabilities: SegmentCapabilities
  exportDescriptor?: SegmentExportDescriptor
}

export interface SegmentEvaluationRun {
  id: string
  mode: 'preview' | 'materialization'
  status: 'queued' | 'running' | 'completed' | 'partial' | 'failed' | 'cancelled'
  versionId: string
  asOf: string
  definitionHash?: string
  pollAfterMs?: number
  candidateCount?: number
  eligibleCount?: number
  excludedCount?: number
  countBucket?: string
  reasonCodes?: string[]
  sourceStatuses?: Array<{
    sourceKey: string
    status: string
    asOf?: string
    reasonCode?: string
  }>
}

export interface SegmentMaterializationView {
  id: string
  versionId: string
  status: 'building' | 'current' | 'superseded' | 'expired' | 'failed'
  asOf: string
  countBucket?: string
  freshnessStatus: string
  reasonCodes?: string[]
}

export interface SegmentExportDescriptor {
  purposeOptions: Array<{ value: string; label: string }>
  channelOptions: Array<{ value: string; label: string }>
  formatOptions: Array<{ value: string; label: string }>
  fieldSetOptions: Array<{ value: string; label: string }>
  requiresReason: boolean
}

export interface SegmentExportView {
  id: string
  status: 'checking' | 'awaiting_approval' | 'ready' | 'failed' | 'expired'
  candidateCount?: number
  eligibleCount?: number
  excludedCount?: number
  reasonCodes?: string[]
  expiresAt?: string
}

export interface SegmentListFilters {
  clientAccountId: string
  status?: string
  cursor?: string
  limit?: number
}

export interface SegmentListPage {
  items: CustomerSegmentListItem[]
  nextCursor: string
}

export interface CreateSegmentInput {
  clientAccountId: string
  segmentKey: string
  name: string
  description: string
  draft: {
    filterSchemaVersion: string
    fieldCatalogVersion: string
    filterAst: SegmentFilterAst
    evaluationPolicy: Record<string, string | number | boolean>
  }
  idempotencyKey: string
}

export interface UpdateSegmentDraftInput {
  filterAst: SegmentFilterAst
  fieldCatalogVersion: string
  expectedRevision: number
}
