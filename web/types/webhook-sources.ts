export type WebhookEntityType = 'leads' | 'products' | 'tracking'

export interface WebhookSourceItem {
  id: string
  accountId: string
  slug: string
  name: string
  entityType: WebhookEntityType
  isActive: boolean
  createdAt: string
  updatedAt: string
}

export interface WebhookSourceCreateInput {
  slug: string
  name: string
  entityType: WebhookEntityType
}

export interface WebhookSourceCreatedResponse {
  source: WebhookSourceItem
  secret: string
}

export interface WebhookSourceRotateResponse {
  secret: string
}
