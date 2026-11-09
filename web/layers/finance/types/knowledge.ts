export interface KnowledgeMetadata {
  authority: string
  jurisdiction: string
  regime: string
  classification: string
  publishedOn: string | null
  validFrom: string | null
  validTo: string | null
  validitySource: string
}
export const emptyKnowledgeMetadata = (): KnowledgeMetadata => ({
  authority: '',
  jurisdiction: 'unknown',
  regime: 'unknown',
  classification: 'informational',
  publishedOn: null,
  validFrom: null,
  validTo: null,
  validitySource: '',
})
export interface KnowledgeBase {
  id: string
  name: string
  enabled: boolean
  version: number
}
export interface KnowledgeDocument {
  id: string
  baseId: string
  title: string
  sourceRef: string
  checksum: string
  status: string
  version: number
  lockVersion: number
  metadata: KnowledgeMetadata
  reviewedBy: string | null
  chunkCount: number
}
export interface KnowledgeEvidence {
  documentId: string
  version: number
  chunkId: string
  excerpt: string
  sourceRef: string
  applicable: boolean
  applicabilityReason: string
  location: { start?: number; end?: number }
  score: number
}
export interface KnowledgeResult {
  truncated?: boolean
  status: string
  evidence: KnowledgeEvidence[]
  potentialConflict: boolean
  message: string
}
