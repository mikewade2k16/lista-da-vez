export type UserRole = 'ADMIN' | 'SUPERVISOR' | 'AGENT' | 'VIEWER'

export interface AuthUser {
  id: string
  tenantId: string
  tenantSlug: string
  email: string
  name: string
  nick?: string | null
  profileImage?: string | null
  role: UserRole
}

export interface TenantSettings {
  id: string
  slug: string
  name: string
  whatsappInstance: string | null
  whatsappInstances: WhatsAppInstanceRecord[]
  maxChannels: number
  maxUsers: number
  retentionDays: number
  maxUploadMb: number
  canManageAtendimentoLimits: boolean
  currentChannels: number
  currentUsers: number
  hasEvolutionApiKey: boolean
  webhookUrl: string
  createdAt: string
  updatedAt: string
  canViewSensitive: boolean
  evolutionApiKey: string | null
}

// No legado era `interface ClientRecord extends TenantSettings {}` — interface
// vazia, que o ESLint da casa reprova (@typescript-eslint/no-empty-object-type).
// O alias e equivalente para todos os consumidores e mantem o nome exportado.
export type ClientRecord = TenantSettings

export type WhatsAppInstanceUserScopePolicy = 'MULTI_INSTANCE' | 'SINGLE_INSTANCE'

export interface WhatsAppInstanceRecord {
  id: string
  tenantId?: string
  instanceName: string
  displayName: string | null
  phoneNumber: string | null
  queueLabel?: string | null
  userScopePolicy?: WhatsAppInstanceUserScopePolicy
  responsibleUserId?: string | null
  responsibleUserName?: string | null
  responsibleUserEmail?: string | null
  isDefault: boolean
  isActive: boolean
  hasEvolutionApiKey?: boolean
  assignedUserIds: string[]
  createdAt?: string
  updatedAt?: string
}

export interface WhatsAppInstanceAssignableUser {
  id: string
  email: string
  name: string
  role: UserRole
  atendimentoAccess: boolean
}

export interface WhatsAppInstanceManagementResponse {
  maxChannels: number
  currentChannels: number
  instances: WhatsAppInstanceRecord[]
  users: WhatsAppInstanceAssignableUser[]
}

export interface WhatsAppInstanceAccessResponse {
  hasMultipleActiveInstances: boolean
  instances: WhatsAppInstanceRecord[]
}

export interface TenantUser {
  id: string
  tenantId: string
  email: string
  name: string
  role: UserRole
  createdAt: string
  updatedAt: string
}

export type MessageDirection = 'INBOUND' | 'OUTBOUND'
export type MessageStatus = 'PENDING' | 'SENT' | 'DELIVERED' | 'READ' | 'FAILED' | 'DELETED'
export type MessageType = 'TEXT' | 'IMAGE' | 'AUDIO' | 'VIDEO' | 'DOCUMENT'
export type MessageOrigin = 'contact' | 'human' | 'ai' | 'provider_device' | 'system'
export type MessageMediaState = '' | 'pending' | 'ready' | 'failed'
export type ConversationStatus = 'OPEN' | 'PENDING' | 'CLOSED'
export type ConversationAIStatus =
  | 'idle'
  | 'analyzing'
  | 'transferring'
  | 'awaiting_client'
  | 'human'
  | 'closed'

export interface MessageReplyTo {
  messageId: string | null
  externalMessageId: string
  senderName: string
  content: string
  messageType: MessageType | ''
}

export interface Message {
  id: string
  tenantId: string
  conversationId: string
  senderUserId?: string | null
  direction: MessageDirection
  messageType?: MessageType
  senderName: string | null
  senderAvatarUrl: string | null
  content: string
  mediaUrl?: string | null
  mediaMimeType?: string | null
  mediaFileName?: string | null
  mediaFileSizeBytes?: number | null
  mediaCaption?: string | null
  mediaDurationSeconds?: number | null
  metadataJson?: Record<string, unknown> | null
  status: MessageStatus
  origin: MessageOrigin
  replyTo: MessageReplyTo | null
  providerStatusAt: string | null
  providerErrorCode: string
  mediaState: MessageMediaState
  canRetryMedia: boolean
  externalMessageId: string | null
  createdAt: string
  updatedAt: string
}

export interface GroupParticipant {
  jid: string
  name: string
  phone: string
  avatarUrl: string | null
}

export interface SavedSticker {
  id: string
  name: string
  dataUrl: string
  mimeType: string
  sizeBytes: number
  createdAt: string
  updatedAt: string
  createdByUserId?: string | null
}

export interface Contact {
  id: string
  tenantId: string
  name: string
  phone: string
  avatarUrl: string | null
  source: string
  createdAt: string
  updatedAt: string
  lastConversationId: string | null
  lastConversationAt: string | null
  lastConversationChannel: 'WHATSAPP' | 'INSTAGRAM' | null
  lastConversationStatus: ConversationStatus | null
}

export interface Conversation {
  id: string
  instanceId?: string | null
  instanceScopeKey?: string | null
  instanceName?: string | null
  instanceDisplayName?: string | null
  channel: 'WHATSAPP' | 'INSTAGRAM'
  status: ConversationStatus
  aiStatus?: ConversationAIStatus
  externalId: string
  contactId?: string | null
  contactName: string | null
  contactAvatarUrl: string | null
  contactPhone: string | null
  assignedToId: string | null
  createdAt: string
  updatedAt: string
  lastMessageAt: string
  lastMessage: {
    id: string
    content: string
    messageType?: MessageType
    mediaUrl?: string | null
    direction: MessageDirection
    status: MessageStatus
    createdAt: string
  } | null
}

export interface WhatsAppStatusResponse {
  configured: boolean
  message?: string
  instanceId?: string
  instanceName?: string
  webhookUrl?: string
  connectionState?: Record<string, unknown>
  webhook?: Record<string, unknown>
  providerUnavailable?: boolean
  degraded?: boolean
}

export interface WhatsAppBootstrapResponse {
  success: boolean
  instanceId?: string
  instanceName: string
  webhookUrl?: string
  created?: boolean
  createResult?: Record<string, unknown> | null
  webhookResult?: Record<string, unknown>
  connectResult?: Record<string, unknown>
  mode?: 'QRCODE' | 'PAIRING_CODE'
  connectionState?: Record<string, unknown>
}

export interface WhatsAppQrCodeResponse {
  configured: boolean
  message?: string
  instanceId?: string
  instanceName?: string
  qrCode?: string | null
  pairingCode?: string | null
  source?: string
  connectionState?: Record<string, unknown>
}

export interface WhatsAppConversationHistoryClearResponse {
  tenantId: string
  scope: 'tenant' | 'instance'
  instanceId: string | null
  instanceName: string | null
  deletedAuditEvents: number
  deletedMessages: number
  deletedConversations: number
  message: string
}

export type WhatsAppEndpointValidationStatus =
  | 'ok'
  | 'validation_error'
  | 'missing_route'
  | 'auth_error'
  | 'provider_error'
  | 'network_error'
  | 'unexpected_error'

export interface WhatsAppEndpointValidationEntry {
  key: 'text' | 'media' | 'audio' | 'contact' | 'sticker' | 'reaction'
  label: string
  pathTemplate: string
  resolvedPath: string | null
  status: WhatsAppEndpointValidationStatus
  available: boolean
  httpStatus: number | null
  message: string
}

export interface WhatsAppEndpointValidationSummary {
  total: number
  available: number
  missingRoute: number
  authError: number
  providerError: number
  networkError: number
}

export interface WhatsAppEndpointValidationResponse {
  instanceName: string
  generatedAt: string
  baseUrl: string
  timeoutMs: number
  endpoints: WhatsAppEndpointValidationEntry[]
  summary: WhatsAppEndpointValidationSummary
}

export interface FailureByTypeEntry {
  messageType: MessageType
  total: number
}

export interface FailureDailySeriesEntry {
  day: string
  total: number
  byType: Record<string, number>
}

export interface RecentFailureEntry {
  id: string
  conversationId: string
  messageType: MessageType
  status: MessageStatus
  createdAt: string
  content: string
  contactName: string | null
  externalId: string
}

export interface TenantFailuresDashboardResponse {
  generatedAt: string
  windowDays: number
  since: string
  failedTotal: number
  failedByType: FailureByTypeEntry[]
  dailySeries: FailureDailySeriesEntry[]
  recentFailures: RecentFailureEntry[]
}

export type HttpMetricsSortBy = 'p95' | 'avg' | 'errors' | 'requests'
export type HttpMetricsSortOrder = 'asc' | 'desc'

export interface HttpEndpointMetricEntry {
  key: string
  method: string
  route: string
  totalRequests: number
  errors: number
  clientErrors: number
  serverErrors: number
  errorRatePercent: number
  avgMs: number
  p95Ms: number
  minMs: number
  maxMs: number
  lastStatusCode: number
  lastSeenAt: string
}

export interface TenantHttpEndpointMetricsSummary {
  totalRequests: number
  totalErrors: number
  clientErrors: number
  serverErrors: number
  errorRatePercent: number
}

export interface TenantHttpEndpointMetricsResponse {
  generatedAt: string
  startedAt: string
  uptimeSeconds: number
  summary: TenantHttpEndpointMetricsSummary
  endpoints: HttpEndpointMetricEntry[]
}
