// Camada de I/O do chat PERSISTIDO do calendario: conversas + mensagens + escopo
// (contrato D3/D4 da wave 4). Separada de calendar-api.ts para manter cada arquivo
// < 450 linhas e isolar a persistencia do chat da orquestracao de estado. Todas as
// rotas sao multi-tenant: o X-Account-Id vai no header pelo apiRequest e o account_id
// NUNCA viaja no body/query (o back resolve pelo Principal via accountScope).
// SEGURANCA: o ACESSO (quais conversas o usuario ve, quais clientes) e SEMPRE resolvido
// server-side pela permissao (resolveChatAccess): agencia ve todas as conversas da
// conta, cliente-side so as suas; conversa/cliente fora do visivel => 404.
import type { ApiRequest } from './calendar-api'

// Escopo do contexto que a IA usa: 'client' = um cliente especifico; 'all' = todos os
// clientes visiveis (so a agencia/multi-cliente pode escolher 'all').
export type CalendarChatScopeMode = 'client' | 'all'

/** Item lean da lista de conversas (GET /chat/conversations, contrato D3). */
export interface CalendarChatConversation {
  id: string
  title: string
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
  createdAt: string
}

/** Conversa + mensagens (GET /chat/conversations/{id}, contrato D3). */
export interface CalendarChatConversationDetail {
  id: string
  title: string
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

function normalizeScopeMode(value: unknown): CalendarChatScopeMode {
  return value === 'all' ? 'all' : 'client'
}

function normalizeConversation(raw: unknown): CalendarChatConversation {
  const o = asRecord(raw)
  return {
    id: asString(o.id),
    title: asString(o.title),
    scopeMode: normalizeScopeMode(o.scopeMode),
    scopeClientId: asString(o.scopeClientId),
    createdByUserId: asString(o.createdByUserId),
    createdByName: asString(o.createdByName),
    updatedAt: asString(o.updatedAt),
  }
}

function normalizeStoredMessage(raw: unknown): CalendarChatStoredMessage {
  const o = asRecord(raw)
  return {
    id: asString(o.id),
    role: o.role === 'assistant' ? 'assistant' : 'user',
    content: asString(o.content),
    createdAt: asString(o.createdAt),
  }
}

// Lista as conversas visiveis (agency=todas; cliente-side=so as suas — o back ja
// filtra pela permissao). Filtra itens sem id (defesa contra shape inesperado).
export async function fetchConversations(api: ApiRequest): Promise<CalendarChatConversation[]> {
  const res = await api('/v1/calendar/chat/conversations')
  const list = Array.isArray(res?.conversations) ? res.conversations : []
  return list.map(normalizeConversation).filter((c: CalendarChatConversation) => c.id)
}

// Carrega uma conversa + suas mensagens do banco (order by created_at). Acesso
// resolvido no back (dono OU agencia); fora do visivel => 404 (erro no chamador).
export async function getConversation(
  api: ApiRequest,
  id: string,
): Promise<CalendarChatConversationDetail> {
  const res = await api(`/v1/calendar/chat/conversations/${encodeURIComponent(id)}`)
  const messages = Array.isArray(res?.messages) ? res.messages : []
  return {
    id: asString(res?.id) || id,
    title: asString(res?.title),
    scopeMode: normalizeScopeMode(res?.scopeMode),
    scopeClientId: asString(res?.scopeClientId),
    messages: messages.map(normalizeStoredMessage).filter((m: CalendarChatStoredMessage) => m.id),
  }
}

// Cria uma conversa vazia (POST -> 201 { id }). O escopo e validado no back contra a
// permissao. scopeClientId vazio nao viaja (conversa scope='all'). Devolve o id novo.
export async function createConversation(
  api: ApiRequest,
  scopeMode: CalendarChatScopeMode,
  scopeClientId: string,
  title = '',
): Promise<string> {
  const body: Record<string, unknown> = { scopeMode }
  if (scopeClientId) body.scopeClientId = scopeClientId
  if (title) body.title = title
  const res = await api('/v1/calendar/chat/conversations', { method: 'POST', body })
  return asString(res?.id)
}

// Soft-delete de uma conversa (dono ou agencia; fora do visivel => 404 no back).
export async function deleteConversation(api: ApiRequest, id: string): Promise<void> {
  await api(`/v1/calendar/chat/conversations/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

// Escopo do SELECT (GET /chat/scope): acesso 100% resolvido no back pela permissao.
export async function fetchChatScope(api: ApiRequest): Promise<CalendarChatScope> {
  const res = await api('/v1/calendar/chat/scope')
  const clients = Array.isArray(res?.clients) ? res.clients : []
  return {
    canSelect: res?.canSelect === true,
    lockedClientId: asString(res?.lockedClientId),
    clients: clients
      .map((c: unknown) => {
        const o = asRecord(c)
        return { id: asString(o.id), name: asString(o.name) }
      })
      .filter((c: CalendarChatScopeClient) => c.id),
  }
}
