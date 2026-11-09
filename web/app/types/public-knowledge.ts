// Contrato de /v1/knowledge/public (back/internal/modules/knowledge/public_curation*.go).

export type PublicReviewerKind = 'owner' | 'ai' | 'accountant'
export type PublicVerdict = 'approved' | 'changes_requested' | 'rejected'

export interface PublicReviewSummary {
  owner: PublicVerdict | 'pending'
  aiApproved: number
  aiConcerns: number
  accountant: PublicVerdict | 'pending'
}

export interface PublicCatalogEntry {
  key: string
  title: string
  url: string
  contentType: 'text/html' | 'application/pdf'
  authority: string
  jurisdiction: string
  regime: string
  notes: string
}

export interface PublicCollection {
  id: string
  catalogKey: string
  version: number
  title: string
  sourceRef: string
  contentType: string
  sha256: string
  byteSize: number
  pages?: number
  chunkCount: number
  excluded: number
  status: 'collected' | 'published' | 'superseded' | 'discarded'
  collectedAt: string
  publishedAt?: string
  review: PublicReviewSummary
}

export interface PublicAttempt {
  outcome: 'collected' | 'unchanged' | 'failed'
  errorCode?: string
  collectionId?: string
  createdAt: string
}

export interface PublicCatalogStatus {
  entry: PublicCatalogEntry
  lastAttempt?: PublicAttempt
  latest?: PublicCollection
  published?: PublicCollection
}

export interface PublicChunk {
  id: string
  index: number
  page: number
  heading: string
  title: string
  text: string
  status: 'draft' | 'published' | 'withdrawn'
}

export interface PublicReview {
  id: string
  reviewerKind: PublicReviewerKind
  reviewerName: string
  verdict: PublicVerdict
  notes: string
  actorId: string
  createdAt: string
}

export interface PublicReviewInput {
  reviewerKind: PublicReviewerKind
  reviewerName: string
  verdict: PublicVerdict
  notes: string
}
