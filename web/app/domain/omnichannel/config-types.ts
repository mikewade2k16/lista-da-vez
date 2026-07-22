// Tipos do contrato das telas de config do omnichannel (F10). Espelham as views
// camelCase servidas pelo back (messaging.*): instancias/sessao/credencial (F2/F4),
// setores/filas/regras (F8), agente/versao/simulate (F9). O escopo (account_id) NUNCA
// aparece aqui — vai no header X-Account-Id injetado pelo api-client.
//
// A credencial NUNCA volta crua: so o status mascarado {set,last4}.

// ============================================================================
// Numeros / instancias / providers
// ============================================================================

// Providers do enum canonico (§7.2). O back valida; o front so oferece estes.
export type OmniProvider = 'meta_whatsapp_cloud' | 'evolution' | 'waha' | 'mock'

export interface OmniInstagramAccount {
  id: string
  igUserId: string
  username: string | null
  displayName: string | null
  pageId: string | null
  isActive: boolean
  webhookStatus: string
  credentialsSet: boolean
  updatedAt: string
}

export interface OmniInstagramComment {
  id: string
  instagramAccountId: string
  externalCommentId: string
  externalMediaId: string | null
  parentCommentId: string | null
  contactId: string | null
  authorScopedId: string
  username: string | null
  text: string
  eventKind: 'comment' | 'mention'
  status: string
  isLive: boolean
  occurredAt: string
  metadata: Record<string, unknown>
  createdAt: string
  updatedAt: string
}

export interface OmniInstagramAction {
  id: string
  commentId: string
  actionKind: 'public_reply' | 'private_reply' | 'hide' | 'ignore'
  status: string
  proposedText: string | null
  approvedText: string | null
  approvedByUserId: string | null
  externalMessageId: string | null
  idempotencyKey: string
  privateReplyExpiresAt: string | null
  lastError: string
  createdAt: string
  executedAt: string | null
}

export interface OmniInstance {
  id: string
  tenantId: string
  instanceName: string
  provider: OmniProvider
  displayName: string | null
  phoneNumber: string | null
  queueLabel: string | null
  userScopePolicy: string
  responsibleUserId: string | null
  responsibleUserName: string | null
  responsibleUserEmail: string | null
  isDefault: boolean
  isActive: boolean
  hasEvolutionApiKey: boolean
  assignedUserIds: string[]
  createdAt: string
  updatedAt: string
}

export interface OmniAssignableUser {
  id: string
  email: string
  name: string
  role: string
  atendimentoAccess: boolean
}

// Resposta de GET /tenant/whatsapp/instances (InstanceManagementView).
export interface OmniInstanceManagement {
  maxChannels: number
  currentChannels: number
  instances: OmniInstance[]
  users: OmniAssignableUser[]
}

export interface OmniChannelLimit {
  maxChannels: number
  currentChannels: number
}

// Estado da sessao/QR (SessionView) — traz `provider` e `connected` que a view de
// gestao nao projeta. Lido por instancia sob demanda (status/qrcode/connect/logout).
export interface OmniSession {
  instanceName: string
  provider: string
  isDefault: boolean
  isActive: boolean
  connected: boolean
  phoneNumber: string | null
  qrCode: string | null
  credentialsSet: boolean
}

// Status mascarado da credencial (secretbox.Status). A chave crua nunca chega aqui.
export interface OmniCredentialStatus {
  set: boolean
  last4: string
}

// Projecao de Capabilities() do adapter daquele numero (a UI degrada por numero).
export interface OmniCapabilities {
  supportsTemplates: boolean
  requires24hWindow: boolean
  supportsReaction: boolean
  supportsSticker: boolean
  supportsGroups: boolean
  maxMediaBytes: number
}

// Body de POST/PATCH /tenant/whatsapp/instances[/{id}] (InstanceWriteInput). Campos
// opcionais omitidos = nao enviados; evolutionApiKey so-se-presente (cifrado no back).
export interface OmniInstanceWriteInput {
  instanceName?: string
  displayName?: string
  phoneNumber?: string
  evolutionApiKey?: string
  queueLabel?: string
  userScopePolicy?: string
  responsibleUserId?: string
  provider?: string
  isDefault?: boolean
  isActive?: boolean
}

// ============================================================================
// Setores / filas / membros / regras (F8, /settings/*)
// ============================================================================

export interface OmniDepartment {
  id: string
  slug: string
  name: string
  isDefault: boolean
  isActive: boolean
  createdAt: string
  updatedAt: string
}

export interface OmniQueue {
  id: string
  departmentId: string
  slug: string
  name: string
  isDefault: boolean
  isActive: boolean
  createdAt: string
  updatedAt: string
}

export interface OmniQueueMember {
  id: string
  queueId: string
  userId: string
  userName: string
  userEmail: string
  isActive: boolean
  createdAt: string
}

export type OmniConditionOp = 'eq' | 'neq' | 'contains' | 'in' | 'exists'

export interface OmniCondition {
  field: string
  op: OmniConditionOp
  value: unknown
}

export interface OmniRoutingRule {
  id: string
  name: string
  priority: number
  isActive: boolean
  conditions: OmniCondition[]
  targetQueueId: string
  createdAt: string
  updatedAt: string
}

export interface OmniDepartmentInput {
  slug?: string
  name: string
  isDefault?: boolean
}

export interface OmniQueueInput {
  departmentId: string
  slug?: string
  name: string
  isDefault?: boolean
}

export interface OmniRoutingRuleInput {
  name: string
  priority: number
  isActive?: boolean
  conditions: OmniCondition[]
  targetQueueId: string
}

export type OmniHandoffPolicyConditions = Record<string, unknown>

export interface OmniHandoffPolicy {
  id: string
  name: string
  priority: number
  isActive: boolean
  conditions: OmniHandoffPolicyConditions
  targetQueueId: string | null
  fallbackQueueId: string | null
  customerNoticeTemplate: string
  createdAt: string
  updatedAt: string
}

export interface OmniHandoffPolicyInput {
  name: string
  priority: number
  isActive?: boolean
  conditions: OmniHandoffPolicyConditions
  targetQueueId?: string | null
  fallbackQueueId?: string | null
  customerNoticeTemplate?: string
}

// ============================================================================
// Agente de IA (F9, /agents/*)
// ============================================================================

export interface OmniAgent {
  id: string
  slug: string
  name: string
  enabled: boolean
  activeVersionId: string | null
  providerKey: OmniCredentialStatus
  createdAt: string
  updatedAt: string
}

export interface OmniProviderKeyStatusView {
  keys: Partial<Record<'gemini' | 'glm' | 'openai', OmniCredentialStatus>>
}

export interface OmniAgentVersion {
  id: string
  agentId: string
  version: number
  status: string
  provider: string
  model: string
  temperature: number
  layers: unknown
  outputSchema: unknown
  mediaConfig: OmniMediaConfig
  schemaVersion: string
  debounceMs: number
  maxContextMessages: number
  maxAiTurns: number
  minConfidence: number
  handoffOnError: boolean
  handoffOnLimit: boolean
  workflowContractVersion: string
  publishedAt: string | null
  createdAt: string
}

export interface OmniMediaSectionConfig {
  enabled?: boolean
  provider?: string
  model?: string
  maxSeconds?: number
  maxBytes?: number
  allowedMime?: string[]
  maxPages?: number
}

export interface OmniMediaConfig {
  audio?: OmniMediaSectionConfig
  image?: OmniMediaSectionConfig
  document?: OmniMediaSectionConfig
  retentionDays?: number
  includeInReply?: boolean
}

export interface OmniMediaAnalysis {
  id: string
  messageId: string
  conversationId: string
  analysisKind: 'transcription' | 'vision' | 'document_text'
  status: 'queued' | 'processing' | 'completed' | 'failed' | 'blocked'
  provider: string
  model: string
  agentVersionId: string
  resultText: string | null
  resultJson: unknown
  promptTokens: number
  completionTokens: number
  costUsd: number
  latencyMs: number
  attempts: number
  lastError: string
  createdAt: string
  completedAt: string | null
  expiresAt: string | null
}

export interface OmniCollectField {
  id: string
  agentId: string
  key: string
  label: string
  fieldType: string
  enumOptions: unknown
  required: boolean
  sortOrder: number
}

// Tools e conhecimento (E6). Credenciais nunca aparecem nestes contratos; toolId é
// apenas o identificador lógico de um registry Go explicitamente injetado.
export interface OmniAiToolBinding {
  id: string
  agentId: string
  toolId: string
  isEnabled: boolean
  mode: 'read' | 'propose_write' | 'approved_write'
  allowedOperations: string[]
  inputSchema: unknown
  outputSchema: unknown
  timeoutMs: number
  maxCallsPerDispatch: number
  config: Record<string, unknown>
  createdAt: string
  updatedAt: string
}

export interface OmniAiToolBindingInput {
  toolId: string
  isEnabled?: boolean
  mode?: OmniAiToolBinding['mode']
  allowedOperations?: string[]
  inputSchema?: unknown
  outputSchema?: unknown
  timeoutMs?: number
  maxCallsPerDispatch?: number
  config?: Record<string, unknown>
}

export interface OmniAiToolRun {
  id: string
  conversationId: string | null
  dispatchId: string | null
  agentId: string
  bindingId: string
  toolId: string
  callId: string
  status: 'requested' | 'approved' | 'denied' | 'running' | 'completed' | 'failed' | 'timeout'
  operation: string
  inputMasked: Record<string, unknown>
  outputMasked: Record<string, unknown>
  latencyMs: number
  error: string
  createdAt: string
  completedAt: string | null
}

export interface OmniAiToolApproval {
  id: string
  runId: string
  agentId: string
  toolId: string
  bindingId: string
  conversationId: string | null
  dispatchId: string | null
  callId: string
  operation: string
  status: 'pending' | 'approved' | 'rejected' | 'expired'
  inputMasked: Record<string, unknown>
  reason: string
  error: string
  latencyMs: number
  requestedAt: string
  decidedAt: string | null
  decidedBy: string | null
  expiresAt: string
}

export interface OmniKnowledgeBase {
  id: string
  name: string
  isEnabled: boolean
  searchConfig: Record<string, unknown>
  createdAt: string
  updatedAt: string
}

export interface OmniKnowledgeDocument {
  id: string
  knowledgeBaseId: string
  sourceRef: string
  title: string
  checksum: string
  status: 'draft' | 'processing' | 'published' | 'failed' | 'archived'
  version: number
  chunkCount: number
  metadata: Record<string, unknown>
  error: string
  createdAt: string
  updatedAt: string
}

export interface OmniKnowledgeChunkInput {
  ordinal: number
  bodyText: string
  tokenCount?: number
}

export interface OmniAiKnowledgeBinding {
  id: string
  agentId: string
  knowledgeBaseId: string
  baseName: string
  isEnabled: boolean
  topK: number
  minScore: number
  createdAt: string
  updatedAt: string
}

export interface OmniAgentInput {
  slug?: string
  name: string
  enabled: boolean
}

export interface OmniAgentVersionInput {
  provider: string
  model: string
  temperature: number
  layers: unknown
  outputSchema?: unknown
  mediaConfig?: OmniMediaConfig
  schemaVersion?: string
  debounceMs?: number
  maxContextMessages?: number
  maxAiTurns?: number
  minConfidence?: number
  handoffOnError?: boolean
  handoffOnLimit?: boolean
  workflowContractVersion?: string
}

// ============================================================================
// Simulador (F9, C9.7) — dry-run: grava ai_runs e consome o limite mensal
// ============================================================================

export interface OmniSimMessage {
  role: 'contact' | 'agent'
  text: string
}

export interface OmniSimulateInput {
  versionId?: string
  messages: OmniSimMessage[]
  contact?: { name: string }
}

export interface OmniSimMatchedRule {
  id: string
  name: string
  priority: number
}

export interface OmniSimWouldRoute {
  departmentId: string | null
  queueId: string | null
}

export interface OmniSimUsage {
  promptTokens: number
  completionTokens: number
  totalTokens: number
  costUsd: number
}

export interface OmniSimulateResult {
  output: unknown
  schemaVersion: string
  valid: boolean
  validationErrors: string[]
  extractedFields: Record<string, unknown>
  matchedRule: OmniSimMatchedRule | null
  wouldRoute: OmniSimWouldRoute | null
  usage: OmniSimUsage
}

// Rotulo humano de cada provider (nunca cor/estado — so texto).
export const OMNI_PROVIDER_LABEL: Record<OmniProvider, string> = {
  meta_whatsapp_cloud: 'Meta WhatsApp Cloud (oficial)',
  evolution: 'Evolution (nao-oficial)',
  waha: 'WAHA (nao-oficial)',
  mock: 'Mock (teste, sem numero real)',
}

export const OMNI_PROVIDER_OPTIONS: Array<{ value: OmniProvider; label: string }> = (
  Object.keys(OMNI_PROVIDER_LABEL) as OmniProvider[]
).map((value) => ({ value, label: OMNI_PROVIDER_LABEL[value] }))
