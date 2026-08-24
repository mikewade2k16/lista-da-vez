// Camada de I/O do chat PERSISTIDO compartilhado: conversas + mensagens + escopo.
// O nome do arquivo permanece durante a migracao para preservar os imports do
// Calendario, mas as rotas canonicas vivem em /v1/assistant/chat e cada conversa
// registra a surface que a originou. Todas as rotas sao multi-tenant: o
// X-Account-Id vai no header pelo apiRequest e o account_id NUNCA viaja no body/query.
// SEGURANCA: o ACESSO (quais conversas o usuario ve, quais clientes) e SEMPRE resolvido
// server-side pela permissao (resolveChatAccess): agencia ve todas as conversas da
// conta, cliente-side so as suas; conversa/cliente fora do visivel => 404.
import type { ApiRequest } from './calendar-api'
import type { CalendarMediaItem } from '~/utils/calendar'

// Escopo do contexto que a IA usa: 'client' = um cliente especifico; 'all' = todos os
// clientes visiveis (so a agencia/multi-cliente pode escolher 'all').
export type CalendarChatScopeMode = 'client' | 'all'

/** Pagina que fornece o contexto default da conversa. */
export type AssistantChatSurface = 'calendar' | 'meta_ads' | 'global'

export type AssistantResourceKind = 'instagram_post' | 'meta_campaign' | 'meta_ad_account'

export interface AssistantResource {
  id: string
  kind: AssistantResourceKind
  title: string
  subtitle: string
  status: string
  imageUrl: string
  permalink: string
  metadata: Record<string, string>
}

const ASSISTANT_CHAT_BASE = '/v1/assistant/chat'

export function assistantTranscribePath(surface: AssistantChatSurface): string {
  return `${ASSISTANT_CHAT_BASE}/transcribe?surface=${encodeURIComponent(
    normalizeAssistantChatSurface(surface),
  )}`
}

/** Item lean da lista de conversas (GET /chat/conversations, contrato D3). */
export interface CalendarChatConversation {
  id: string
  title: string
  surface: AssistantChatSurface
  scopeMode: CalendarChatScopeMode
  scopeClientId: string
  createdByUserId: string
  createdByName: string
  updatedAt: string
}

/** Mensagem persistida de uma conversa (vem do banco, order by created_at). */
export interface CalendarChatStoredMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
  // proposals (multi-tarefa, WAVE 5.1): lista de propostas, cada uma com status proprio.
  proposals: CalendarChatStoredProposal[]
  calendarItems: CalendarChatCalendarItem[]
  resources: AssistantResource[]
  createdAt: string
}

export type CalendarChatProposalStatus = 'none' | 'pending' | 'accepted' | 'rejected'
export type CalendarChatProposalExecutionStatus =
  | 'pending'
  | 'executing'
  | 'succeeded'
  | 'failed'
  | 'unknown'
  | 'rejected'
  | 'unavailable'
// action reservado p/ CRUD futuro (create|update|delete); hoje o front so executa 'create'.
export type CalendarChatProposalAction = 'create' | 'update' | 'delete'

// Kinds de proposta: evento/task (WAVE 5.1), anotacao/perfil do cliente (WAVE 7)
// e item do checklist de uma task. `taskItem` continua sujeito ao mesmo cartao de
// confirmacao: a IA nunca altera a task diretamente.
export type CalendarChatProposalKind =
  | 'event'
  | 'task'
  | 'taskItem'
  | 'note'
  | 'clientProfile'
  | 'metaAction'

export type CalendarChatMetaActionKind =
  | 'create_campaign'
  | 'duplicate_campaign'
  | 'update_campaign'
  | 'pause_campaign'
  | 'resume_campaign'
  | 'promote_instagram_post'

export type CalendarChatMetaActionStatus =
  | 'pending'
  | 'executing'
  | 'succeeded'
  | 'failed'
  | 'unknown'
  | 'cancelled'
  | 'expired'

export interface CalendarChatMetaBudget {
  type: 'daily' | 'lifetime'
  amount: number
}

/** Snapshot seguro de uma proposta duravel criada server-side pelo modulo Meta Ads. */
export interface CalendarChatMetaAction {
  action: CalendarChatMetaActionKind
  adAccountId: string
  adAccountName: string
  campaignId: string
  campaignName: string
  currency: string
  name: string
  objective: string
  specialAdCategories: string[]
  budget?: CalendarChatMetaBudget
  instagramPostId: string
  instagramPostTitle: string
  adSetName: string
  adName: string
  countries: string[]
  ageMin: number
  ageMax: number
  actionProposalId: string
  summary: string
  actionStatus: CalendarChatMetaActionStatus
  executionAvailable: boolean
  canConfirm: boolean
  requiresSpendAcknowledgement: boolean
  expiresAt: string
  errorCode: string
  errorMessage: string
}

export type CalendarChatTaskItemStatus =
  | 'captured'
  | 'editing'
  | 'approval'
  | 'approved'
  | 'scheduled'
  | 'posted'

/** Sub-objeto da proposta de item do checklist (kind=taskItem). */
export interface CalendarChatProposalTaskItem {
  id?: string
  title?: string
  /** Snapshot autoritativo usado no cartao em update/delete. */
  itemTitle?: string
  /** Titulo autoritativo da task pai, resolvido pelo backend. */
  taskTitle?: string
  status?: CalendarChatTaskItemStatus
  statusDate?: string
  completed?: boolean
  completedDate?: string
}

/** Sub-objeto da proposta de anotacao do mes (kind=note, WAVE 7). */
export interface CalendarChatProposalNote {
  month?: string
  content?: string
  mode?: 'append' | 'replace'
}

/** Campos livres do brief no perfil (kind=clientProfile, WAVE 7). */
export interface CalendarChatProposalProfileExtra {
  audience?: string
  offer?: string
  pillars?: string
  cadence?: string
  restrictions?: string
  performance?: string
  assets?: string
}

/** Sub-objeto da proposta de perfil estrategico do cliente (kind=clientProfile, WAVE 7). */
export interface CalendarChatProposalProfile {
  segment?: string
  positioning?: string
  description?: string
  history?: string
  siteUrl?: string
  instagram?: string
  address?: string
  objectives?: string
  brandVoice?: string
  extra?: CalendarChatProposalProfileExtra
  clearFields?: string[]
  clearAll?: boolean
}

export interface CalendarChatProposalFields {
  title?: string
  date?: string
  time?: string
  type?: string
  status?: string
  priority?: string
  responsibleId?: string
  involvedIds?: string[]
  description?: string
  contentHtml?: string
  dueDate?: string
  startDate?: string
  dueEndDate?: string
  columnId?: string
  clientId?: string
  clientName?: string
  archived?: boolean
  targetId?: string
  // taskItem/note/profile: sub-objetos fechados por kind.
  taskItem?: CalendarChatProposalTaskItem
  note?: CalendarChatProposalNote
  profile?: CalendarChatProposalProfile
  metaAction?: CalendarChatMetaAction
}

/** Proposta PERSISTIDA: id estavel (indice na mensagem) + status proprio (aprova/recusa por item). */
export interface CalendarChatStoredProposal {
  id: string
  action: CalendarChatProposalAction
  kind: CalendarChatProposalKind
  fields: CalendarChatProposalFields
  status: CalendarChatProposalStatus
  execution?: {
    status: CalendarChatProposalExecutionStatus
    canConfirm: boolean
    errorCode: string
    message: string
  }
}

export interface CalendarChatCalendarItem {
  id: string
  date: string
  time: string
  type: string
  title: string
  status: string
  priority: string
  responsibleId?: string
  involvedIds?: string[]
  clientId: string
  clientName: string
  description?: string
  taskId?: string
  media: CalendarMediaItem[]
}

/** Conversa + mensagens (GET /chat/conversations/{id}, contrato D3). */
export interface CalendarChatConversationDetail {
  id: string
  title: string
  surface: AssistantChatSurface
  scopeMode: CalendarChatScopeMode
  scopeClientId: string
  messages: CalendarChatStoredMessage[]
}

/** Cliente visivel para o SELECT de escopo (GET /chat/scope). */
export interface CalendarChatScopeClient {
  id: string
  name: string
}

/**
 * Alimenta o SELECT de escopo do front (GET /chat/scope). O acesso e resolvido no
 * back: `canSelect=false` => usuario-cliente (esconde o select; a IA fica travada em
 * `lockedClientId`); `canSelect=true` => agencia/multi-cliente (mostra o select com
 * "Todos os clientes" + cada `clients`).
 */
export interface CalendarChatScope {
  canSelect: boolean
  lockedClientId: string
  clients: CalendarChatScopeClient[]
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' ? (value as Record<string, unknown>) : {}
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

const RESOURCE_KINDS: AssistantResourceKind[] = [
  'instagram_post',
  'meta_campaign',
  'meta_ad_account',
]
const RESOURCE_MAX = 20
const RESOURCE_ID_MAX = 192
const RESOURCE_TITLE_MAX = 180
const RESOURCE_TEXT_MAX = 240
const RESOURCE_URL_MAX = 2048
const RESOURCE_METADATA_MAX = 8
const RESOURCE_SUFFIX_RE = /^[A-Za-z0-9._-]+$/

function resourceText(value: unknown, limit: number): string {
  const text = asString(value).trim().replace(/\s+/g, ' ')
  return Array.from(text).slice(0, limit).join('')
}

function safeHttpsUrl(value: unknown): string {
  const raw = asString(value).trim()
  if (!raw || raw.length > RESOURCE_URL_MAX) return ''
  try {
    const url = new URL(raw)
    if (url.protocol !== 'https:' || !url.hostname || url.username || url.password) return ''
    return url.toString()
  } catch {
    return ''
  }
}

function resourceKindLabel(kind: AssistantResourceKind): string {
  if (kind === 'instagram_post') return 'Post do Instagram'
  if (kind === 'meta_campaign') return 'Campanha Meta'
  return 'Conta de anuncios Meta'
}

export function normalizeAssistantResources(value: unknown): AssistantResource[] {
  if (!Array.isArray(value)) return []
  const out: AssistantResource[] = []
  const seen = new Set<string>()
  for (const raw of value) {
    if (out.length >= RESOURCE_MAX) break
    const item = asRecord(raw)
    const kind = asString(item.kind).trim().toLowerCase() as AssistantResourceKind
    if (!RESOURCE_KINDS.includes(kind)) continue
    const id = asString(item.id).trim()
    const prefix = `${kind}:`
    const suffix = id.startsWith(prefix) ? id.slice(prefix.length) : ''
    if (
      !suffix ||
      Array.from(id).length > RESOURCE_ID_MAX ||
      !RESOURCE_SUFFIX_RE.test(suffix) ||
      seen.has(id)
    ) {
      continue
    }
    const metadata: Record<string, string> = {}
    const metadataSource = asRecord(item.metadata)
    for (const key of Object.keys(metadataSource).sort()) {
      if (Object.keys(metadata).length >= RESOURCE_METADATA_MAX) break
      const cleanKey = resourceText(key, 40)
      const cleanValue = resourceText(metadataSource[key], RESOURCE_TEXT_MAX)
      if (!cleanKey || !cleanValue || !RESOURCE_SUFFIX_RE.test(cleanKey)) continue
      metadata[cleanKey] = cleanValue
    }
    seen.add(id)
    out.push({
      id,
      kind,
      title: resourceText(item.title, RESOURCE_TITLE_MAX) || resourceKindLabel(kind),
      subtitle: resourceText(item.subtitle, RESOURCE_TEXT_MAX),
      status: resourceText(item.status, RESOURCE_TEXT_MAX),
      imageUrl: safeHttpsUrl(item.imageUrl),
      permalink: safeHttpsUrl(item.permalink),
      metadata,
    })
  }
  return out
}

export function assistantResourceInstruction(resource: AssistantResource): string {
  const label = resource.title || resourceKindLabel(resource.kind)
  if (resource.kind === 'instagram_post') {
    return `Use o post "${label}" (${resource.id}) como criativo para uma campanha. Prepare a proposta para eu revisar; nao execute nada ainda.`
  }
  if (resource.kind === 'meta_campaign') {
    return `Use a campanha "${label}" (${resource.id}) como referencia para esta nova instrucao: `
  }
  return `Considere a conta de anuncios "${label}" (${resource.id}) nesta nova instrucao: `
}

function normalizeScopeMode(value: unknown): CalendarChatScopeMode {
  return value === 'all' ? 'all' : 'client'
}

export function normalizeAssistantChatSurface(value: unknown): AssistantChatSurface {
  if (value === 'meta_ads' || value === 'global') return value
  return 'calendar'
}

function normalizeConversation(raw: unknown): CalendarChatConversation {
  const o = asRecord(raw)
  return {
    id: asString(o.id),
    title: asString(o.title),
    surface: normalizeAssistantChatSurface(o.surface ?? o.entrySurface),
    scopeMode: normalizeScopeMode(o.scopeMode),
    scopeClientId: asString(o.scopeClientId),
    createdByUserId: asString(o.createdByUserId),
    createdByName: asString(o.createdByName),
    updatedAt: asString(o.updatedAt),
  }
}

const META_ACTIONS: CalendarChatMetaActionKind[] = [
  'create_campaign',
  'duplicate_campaign',
  'update_campaign',
  'pause_campaign',
  'resume_campaign',
  'promote_instagram_post',
]
const META_ACTION_STATUSES: CalendarChatMetaActionStatus[] = [
  'pending',
  'executing',
  'succeeded',
  'failed',
  'unknown',
  'cancelled',
  'expired',
]

function boundedText(value: unknown, max: number): string {
  return Array.from(asString(value).trim().replace(/\s+/g, ' ')).slice(0, max).join('')
}

function normalizeMetaAction(value: unknown): CalendarChatMetaAction | null {
  const raw = asRecord(value)
  const action = asString(raw.action) as CalendarChatMetaActionKind
  if (!META_ACTIONS.includes(action)) return null
  const actionStatus = asString(raw.actionStatus) as CalendarChatMetaActionStatus
  if (!META_ACTION_STATUSES.includes(actionStatus)) return null
  const budgetRaw = asRecord(raw.budget)
  const budgetType = asString(budgetRaw.type)
  const budgetAmount = budgetRaw.amount
  const budget =
    (budgetType === 'daily' || budgetType === 'lifetime') &&
    typeof budgetAmount === 'number' &&
    Number.isFinite(budgetAmount) &&
    budgetAmount > 0
      ? { type: budgetType, amount: budgetAmount }
      : undefined
  return {
    action,
    adAccountId: boundedText(raw.adAccountId, 64),
    adAccountName: boundedText(raw.adAccountName, 300),
    campaignId: boundedText(raw.campaignId, 64),
    campaignName: boundedText(raw.campaignName, 300),
    currency: boundedText(raw.currency, 3).toUpperCase(),
    name: boundedText(raw.name, 240),
    objective: boundedText(raw.objective, 80),
    specialAdCategories: Array.isArray(raw.specialAdCategories)
      ? raw.specialAdCategories
          .map((entry) => boundedText(entry, 80))
          .filter(Boolean)
          .slice(0, 6)
      : [],
    ...(budget ? { budget } : {}),
    instagramPostId: boundedText(raw.instagramPostId, 80),
    instagramPostTitle: boundedText(raw.instagramPostTitle, 180),
    adSetName: boundedText(raw.adSetName, 240),
    adName: boundedText(raw.adName, 240),
    countries: Array.isArray(raw.countries)
      ? raw.countries
          .map((entry) => asString(entry).trim().toUpperCase())
          .filter((entry) => /^[A-Z]{2}$/.test(entry))
          .slice(0, 10)
      : [],
    ageMin: Number.isInteger(raw.ageMin) ? Number(raw.ageMin) : 0,
    ageMax: Number.isInteger(raw.ageMax) ? Number(raw.ageMax) : 0,
    actionProposalId: boundedText(raw.actionProposalId, 64),
    summary: boundedText(raw.summary, 1000),
    actionStatus,
    executionAvailable: raw.executionAvailable === true,
    canConfirm: raw.canConfirm === true,
    requiresSpendAcknowledgement: raw.requiresSpendAcknowledgement === true,
    expiresAt: boundedText(raw.expiresAt, 64),
    errorCode: boundedText(raw.errorCode, 100),
    errorMessage: boundedText(raw.errorMessage, 500),
  }
}

function normalizeProposal(raw: unknown, index: number): CalendarChatStoredProposal | null {
  const o = asRecord(raw)
  const action: CalendarChatProposalAction = ['create', 'update', 'delete'].includes(
    asString(o.action),
  )
    ? (asString(o.action) as CalendarChatProposalAction)
    : 'create'
  const status: CalendarChatProposalStatus = ['pending', 'accepted', 'rejected'].includes(
    asString(o.status),
  )
    ? (asString(o.status) as CalendarChatProposalStatus)
    : 'pending'
  const kind: CalendarChatProposalKind = (
    ['event', 'task', 'taskItem', 'note', 'clientProfile', 'metaAction'] as string[]
  ).includes(asString(o.kind))
    ? (asString(o.kind) as CalendarChatProposalKind)
    : 'event'
  const rawFields = asRecord(o.fields)
  const metaAction = kind === 'metaAction' ? normalizeMetaAction(rawFields.metaAction) : null
  if (kind === 'metaAction' && !metaAction) return null
  const rawExecution = asRecord(o.execution)
  const executionStatus = asString(rawExecution.status) as CalendarChatProposalExecutionStatus
  const validExecutionStatus = (
    [
      'pending',
      'executing',
      'succeeded',
      'failed',
      'unknown',
      'rejected',
      'unavailable',
    ] as string[]
  ).includes(executionStatus)
  return {
    id: asString(o.id) || String(index),
    action,
    kind,
    fields:
      kind === 'metaAction'
        ? { metaAction: metaAction! }
        : (rawFields as CalendarChatProposalFields),
    status,
    ...(validExecutionStatus
      ? {
          execution: {
            status: executionStatus,
            canConfirm: rawExecution.canConfirm === true,
            errorCode: boundedText(rawExecution.errorCode, 100),
            message: boundedText(rawExecution.message, 500),
          },
        }
      : {}),
  }
}

function stableValue(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(stableValue)
  const object = asRecord(value)
  if (!Object.keys(object).length) return value
  return Object.fromEntries(
    Object.keys(object)
      .sort()
      .map((key) => [key, stableValue(object[key])]),
  )
}

function proposalFingerprint(proposal: CalendarChatStoredProposal): string {
  return JSON.stringify({
    action: proposal.action,
    kind: proposal.kind,
    fields: stableValue(proposal.fields || {}),
    status: proposal.status === 'pending' ? 'pending' : proposal.id,
  })
}

function dedupeProposals(proposals: CalendarChatStoredProposal[]): CalendarChatStoredProposal[] {
  const seen = new Set<string>()
  const out: CalendarChatStoredProposal[] = []
  for (const proposal of proposals) {
    const key = proposalFingerprint(proposal)
    if (seen.has(key)) continue
    seen.add(key)
    out.push(proposal)
  }
  return out
}

export function normalizeCalendarChatStoredMessage(raw: unknown): CalendarChatStoredMessage {
  const o = asRecord(raw)
  let proposals: CalendarChatStoredProposal[] = Array.isArray(o.proposals)
    ? o.proposals
        .map((p, i) => normalizeProposal(p, i))
        .filter((proposal): proposal is CalendarChatStoredProposal => proposal !== null)
    : []
  // Retrocompat: mensagem no shape antigo (proposal singular + proposalStatus) vira lista de 1.
  if (!proposals.length && o.proposal) {
    const p = asRecord(o.proposal)
    const legacyStatus = ['pending', 'accepted', 'rejected'].includes(asString(o.proposalStatus))
      ? asString(o.proposalStatus)
      : 'pending'
    const legacy = normalizeProposal(
      { id: '0', kind: p.kind, fields: p.fields, status: legacyStatus },
      0,
    )
    proposals = legacy ? [legacy] : []
  }
  return {
    id: asString(o.id),
    role: o.role === 'assistant' ? 'assistant' : 'user',
    content: asString(o.content),
    proposals: dedupeProposals(proposals),
    calendarItems: Array.isArray(o.calendarItems)
      ? (o.calendarItems as CalendarChatCalendarItem[])
      : [],
    resources: normalizeAssistantResources(o.resources),
    createdAt: asString(o.createdAt),
  }
}

export async function updateProposalStatus(
  api: ApiRequest,
  conversationId: string,
  messageId: string,
  proposalId: string,
  status: 'accepted' | 'rejected',
  signal?: AbortSignal,
): Promise<CalendarChatStoredMessage> {
  const path = `${ASSISTANT_CHAT_BASE}/conversations/${encodeURIComponent(conversationId)}/messages/${encodeURIComponent(messageId)}/proposals/${encodeURIComponent(proposalId)}/status`
  const raw = signal
    ? await api(path, { method: 'PATCH', body: { status }, signal })
    : await api(path, { method: 'PATCH', body: { status } })
  return normalizeCalendarChatStoredMessage(raw)
}

export function calendarChatProposalConfirmationKey(messageId: string, proposalId: string): string {
  return `assistant-confirm:${messageId}:${proposalId}`
}

/**
 * Executa um card local exclusivamente no backend. O retorno traz a mensagem
 * reidratada da mesma transacao que gravou o efeito e o receipt.
 */
export async function confirmCalendarChatProposal(
  api: ApiRequest,
  conversationId: string,
  messageId: string,
  proposalId: string,
  idempotencyKey: string,
  fields?: CalendarChatProposalFields,
  clientId = '',
  signal?: AbortSignal,
): Promise<CalendarChatStoredMessage> {
  const path = `${ASSISTANT_CHAT_BASE}/conversations/${encodeURIComponent(conversationId)}/messages/${encodeURIComponent(messageId)}/proposals/${encodeURIComponent(proposalId)}/confirm`
  const body: Record<string, unknown> = {}
  if (fields) body.fields = fields
  if (clientId) body.clientId = clientId
  const options = {
    method: 'POST' as const,
    body,
    headers: { 'Idempotency-Key': idempotencyKey },
    ...(signal ? { signal } : {}),
  }
  const raw = asRecord(await api(path, options))
  return normalizeCalendarChatStoredMessage(raw.message)
}

// Lista as conversas visiveis (agency=todas; cliente-side=so as suas — o back ja
// filtra pela permissao). Filtra itens sem id (defesa contra shape inesperado).
export async function fetchConversations(
  api: ApiRequest,
  signal?: AbortSignal,
): Promise<CalendarChatConversation[]> {
  const path = `${ASSISTANT_CHAT_BASE}/conversations`
  const res = asRecord(signal ? await api(path, { signal }) : await api(path))
  const list = Array.isArray(res.conversations) ? res.conversations : []
  return list.map(normalizeConversation).filter((c: CalendarChatConversation) => c.id)
}

// Carrega uma conversa + suas mensagens do banco (order by created_at). Acesso
// resolvido no back (dono OU agencia); fora do visivel => 404 (erro no chamador).
export async function getConversation(
  api: ApiRequest,
  id: string,
  signal?: AbortSignal,
): Promise<CalendarChatConversationDetail> {
  const path = `${ASSISTANT_CHAT_BASE}/conversations/${encodeURIComponent(id)}`
  const res = asRecord(signal ? await api(path, { signal }) : await api(path))
  const messages = Array.isArray(res.messages) ? res.messages : []
  return {
    id: asString(res.id) || id,
    title: asString(res.title),
    surface: normalizeAssistantChatSurface(res.surface ?? res.entrySurface),
    scopeMode: normalizeScopeMode(res.scopeMode),
    scopeClientId: asString(res.scopeClientId),
    messages: messages
      .map(normalizeCalendarChatStoredMessage)
      .filter((m: CalendarChatStoredMessage) => m.id),
  }
}

// Cria uma conversa vazia (POST -> 201 { id }). O escopo e validado no back contra a
// permissao. scopeClientId vazio nao viaja (conversa scope='all'). Devolve o id novo.
export async function createConversation(
  api: ApiRequest,
  surface: AssistantChatSurface,
  scopeMode: CalendarChatScopeMode,
  scopeClientId: string,
  title = '',
  signal?: AbortSignal,
): Promise<string> {
  const body: Record<string, unknown> = { surface, scopeMode }
  if (scopeClientId) body.scopeClientId = scopeClientId
  if (title) body.title = title
  const path = `${ASSISTANT_CHAT_BASE}/conversations`
  const res = asRecord(
    signal
      ? await api(path, { method: 'POST', body, signal })
      : await api(path, { method: 'POST', body }),
  )
  return asString(res.id)
}

// Soft-delete de uma conversa (dono ou agencia; fora do visivel => 404 no back).
export async function deleteConversation(
  api: ApiRequest,
  id: string,
  signal?: AbortSignal,
): Promise<void> {
  const path = `${ASSISTANT_CHAT_BASE}/conversations/${encodeURIComponent(id)}`
  if (signal) await api(path, { method: 'DELETE', signal })
  else await api(path, { method: 'DELETE' })
}

// Escopo do SELECT (GET /chat/scope): acesso 100% resolvido no back pela permissao.
export async function fetchChatScope(
  api: ApiRequest,
  signal?: AbortSignal,
): Promise<CalendarChatScope> {
  const path = `${ASSISTANT_CHAT_BASE}/scope`
  const res = asRecord(signal ? await api(path, { signal }) : await api(path))
  const clients = Array.isArray(res.clients) ? res.clients : []
  return {
    canSelect: res.canSelect === true,
    lockedClientId: asString(res.lockedClientId),
    clients: clients
      .map((c: unknown) => {
        const o = asRecord(c)
        return { id: asString(o.id), name: asString(o.name) }
      })
      .filter((c: CalendarChatScopeClient) => c.id),
  }
}
