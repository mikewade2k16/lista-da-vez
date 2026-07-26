export interface OfflineInteractionView {
  id: string
  relationshipId: string
  interactionType: string
  title: string
  content: string
  occurredAt: string
  timezone: string
  purposeKey: string
  status: 'active' | 'archived'
  revision: number
  authorDisplay?: string
  attachmentMetadata?: Array<{
    id: string
    fileName: string
    mediaType: string
    scanStatus: string
  }>
}

export interface OfflineInteractionCreateDescriptor {
  interactionTypes: Array<{ value: string; label: string }>
  purposeOptions: Array<{ value: string; label: string }>
  timezoneOptions: Array<{ value: string; label: string }>
  maxTitleLength: number
  maxContentLength: number
}

export interface OfflineInteractionsPage {
  items: OfflineInteractionView[]
  nextCursor: string
  createDescriptor?: OfflineInteractionCreateDescriptor
}

export interface CreateOfflineInteractionInput {
  clientAccountId: string
  interactionType: string
  title: string
  content: string
  occurredAt: string
  timezone: string
  purposeKey: string
  attachmentRefs: string[]
  idempotencyKey: string
}
