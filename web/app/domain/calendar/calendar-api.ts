// Camada de acesso a dados do calendario (I/O puro sobre o apiRequest).
// Separada do store (stores/calendar.ts) para manter cada arquivo < 450 linhas e
// isolar a construcao de URL/parse de resposta da orquestracao de estado.
// Todas as rotas sao multi-tenant: o X-Account-Id vai no header pelo apiRequest.
import type { createApiRequest } from '~/utils/api-client'
import {
  AI_SECRET_PROVIDERS,
  defaultCalendarConfig,
  defaultCalendarMediaLimits,
  normalizeClientAiOverride,
  normalizeClientProfile,
  normalizePlan,
  normalizePlanIndexItem,
  type CalendarAiKeyStatus,
  type CalendarAiPlan,
  type CalendarAiPlanIndexItem,
  type CalendarAiSecretProvider,
  type CalendarClientAiOverride,
  type CalendarClientProfile,
  type CalendarClientProfileIndexItem,
  type CalendarConfig,
  type CalendarEvent,
  type CalendarEventInput,
  type CalendarHoliday,
  type CalendarMediaLimits,
  type CalendarMember,
  type CalendarPerson,
} from '~/utils/calendar'

// Tipo exato do apiRequest devolvido por createApiRequest (evita incompatibilidade
// de contravariancia ao passar apiRequest para estas funcoes).
export type ApiRequest = ReturnType<typeof createApiRequest>

function rangeQuery(from: string, to: string, clientId = ''): string {
  const query = new URLSearchParams({ from, to })
  const normalizedClientId = clientId.trim()
  if (normalizedClientId) query.set('clientId', normalizedClientId)
  return query.toString()
}

export interface CalendarScopeClient {
  id: string
  name: string
}

export interface CalendarScope {
  canSelect: boolean
  lockedClientId: string
  clients: CalendarScopeClient[]
}

export function normalizeCalendarScope(res: unknown): CalendarScope {
  const raw = (res && typeof res === 'object' ? res : {}) as {
    canSelect?: unknown
    lockedClientId?: unknown
    clients?: unknown
  }
  const clients: CalendarScopeClient[] = []
  const seen = new Set<string>()
  for (const entry of Array.isArray(raw.clients) ? raw.clients : []) {
    if (!entry || typeof entry !== 'object') continue
    const client = entry as { id?: unknown; name?: unknown }
    const id = typeof client.id === 'string' ? client.id.trim() : ''
    if (!id || seen.has(id)) continue
    seen.add(id)
    clients.push({ id, name: typeof client.name === 'string' ? client.name.trim() : '' })
  }
  return {
    canSelect: raw.canSelect === true,
    lockedClientId: typeof raw.lockedClientId === 'string' ? raw.lockedClientId.trim() : '',
    clients,
  }
}

function asStringMap(value: unknown): Record<string, string> {
  if (!value || typeof value !== 'object') return {}
  const out: Record<string, string> = {}
  for (const [key, raw] of Object.entries(value as Record<string, unknown>)) {
    if (typeof raw === 'string') out[key] = raw
  }
  return out
}

// Clamp do tamanho da janela de chat (px): 0 = default da posicao; teto de seguranca.
function clampChatSize(value: unknown): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) return 0
  return Math.min(2000, Math.max(0, Math.round(value)))
}

// normalizeConfig faz merge POR SECAO (nao spread raso): linha antiga do banco sem
// os campos novos (weekStartsOn/clientColors/typeColors/whiteLabel/ai/tasks/chat) ainda
// ganha o shape completo (C2/C6/CFG v4) com os defaults preenchidos. O merge por secao
// tambem garante que `tasks`/`chat` existam sempre e nunca apaga o valor persistido no
// full-replace do PUT. Os campos v4 da IA (enabled/useGlobalKeys/transcribeProvider) e o
// `chat.position` sao coeridos por enum/bool: jsonb antigo ou editado a mao nao quebra o
// shape. IMPORTANTE: a config NUNCA carrega a chave de API crua (vive nos secrets/SEC).
export function normalizeConfig(res: unknown): CalendarConfig {
  const base = defaultCalendarConfig()
  const raw = (res as Partial<CalendarConfig>) || {}
  const rawAi = (raw.ai || {}) as Partial<CalendarConfig['ai']>
  const rawChat = (raw.chat || {}) as Partial<CalendarConfig['chat']>
  return {
    responsibleUserIds: Array.isArray(raw.responsibleUserIds) ? raw.responsibleUserIds : [],
    holidays: { ...base.holidays, ...(raw.holidays || {}) },
    weekStartsOn: raw.weekStartsOn === 'monday' ? 'monday' : 'sunday',
    clientColors: asStringMap(raw.clientColors),
    typeColors: asStringMap(raw.typeColors),
    whiteLabel: { ...base.whiteLabel, ...(raw.whiteLabel || {}) },
    ai: {
      ...base.ai,
      ...rawAi,
      // Default de enabled/useGlobalKeys = true (so `false` explicito desliga/troca).
      enabled: rawAi.enabled !== false,
      useGlobalKeys: rawAi.useGlobalKeys !== false,
      transcribeProvider: (['local', 'openai', 'gemini'] as const).includes(
        rawAi.transcribeProvider as 'local' | 'openai' | 'gemini',
      )
        ? (rawAi.transcribeProvider as 'local' | 'openai' | 'gemini')
        : 'local',
      // WAVE 3.1: escopo por cliente. Enum general|perClient (default general); a lista
      // de exclusoes filtra so strings (defesa contra jsonb antigo/editado a mao).
      scopeMode: rawAi.scopeMode === 'perClient' ? 'perClient' : 'general',
      disabledClientIds: Array.isArray(rawAi.disabledClientIds)
        ? rawAi.disabledClientIds.filter((id): id is string => typeof id === 'string')
        : [],
    },
    tasks: { ...base.tasks, ...(raw.tasks || {}) },
    chat: {
      position:
        rawChat.position === 'left' || rawChat.position === 'right' ? rawChat.position : 'center',
      width: clampChatSize(rawChat.width),
      height: clampChatSize(rawChat.height),
    },
    // Atalhos (WAVE 11): acao ausente cai no default; valor do banco vence (inclusive '').
    shortcuts: { ...base.shortcuts, ...asStringMap(raw.shortcuts) },
  }
}

export async function fetchMediaLimits(api: ApiRequest): Promise<CalendarMediaLimits> {
  const res = await api('/v1/calendar/media-limits')
  return { ...defaultCalendarMediaLimits(), ...((res as Partial<CalendarMediaLimits>) || {}) }
}

export async function putMediaLimits(
  api: ApiRequest,
  next: CalendarMediaLimits,
): Promise<CalendarMediaLimits> {
  const res = await api('/v1/calendar/media-limits', { method: 'PUT', body: next })
  return { ...defaultCalendarMediaLimits(), ...((res as Partial<CalendarMediaLimits>) || {}) }
}

export async function fetchEventsInRange(
  api: ApiRequest,
  from: string,
  to: string,
  clientId = '',
): Promise<CalendarEvent[]> {
  const res = await api(`/v1/calendar/events?${rangeQuery(from, to, clientId)}`)
  return Array.isArray(res?.events) ? (res.events as CalendarEvent[]) : []
}

export async function fetchScope(api: ApiRequest): Promise<CalendarScope> {
  return normalizeCalendarScope(await api('/v1/calendar/scope'))
}

export async function fetchHolidaysInRange(
  api: ApiRequest,
  from: string,
  to: string,
): Promise<CalendarHoliday[]> {
  const res = await api(`/v1/calendar/holidays?${rangeQuery(from, to)}`)
  return Array.isArray(res?.holidays) ? (res.holidays as CalendarHoliday[]) : []
}

// createEventTask cria (e vincula) uma task para um evento SEM task (WAVE 6, botão do badge
// "evento sem task"). Idempotente no back. Devolve o taskId criado/existente.
export async function createEventTask(api: ApiRequest, eventId: string): Promise<string> {
  const res = await api(`/v1/calendar/events/${eventId}/task`, { method: 'POST' })
  return String(res?.taskId || '')
}

export async function fetchNotesForMonth(api: ApiRequest, month: string): Promise<string> {
  const res = await api(`/v1/calendar/notes/${month}`)
  return String(res?.content || '')
}

export async function putNotesForMonth(
  api: ApiRequest,
  month: string,
  content: string,
): Promise<void> {
  await api(`/v1/calendar/notes/${month}`, { method: 'PUT', body: { content } })
}

export async function fetchResponsibles(api: ApiRequest): Promise<CalendarPerson[]> {
  const res = await api('/v1/calendar/responsibles')
  return Array.isArray(res?.responsibles) ? (res.responsibles as CalendarPerson[]) : []
}

export async function fetchMembers(api: ApiRequest): Promise<CalendarMember[]> {
  const res = await api('/v1/calendar/members')
  return Array.isArray(res?.members) ? (res.members as CalendarMember[]) : []
}

export async function fetchConfig(api: ApiRequest): Promise<CalendarConfig> {
  return normalizeConfig(await api('/v1/calendar/config'))
}

export async function putConfig(api: ApiRequest, next: CalendarConfig): Promise<CalendarConfig> {
  return normalizeConfig(await api('/v1/calendar/config', { method: 'PUT', body: next }))
}

// --- Chaves de API da IA (contrato SEC) -----------------------------------------
// SEGURANCA: o front so recebe status MASCARADO {set,last4}, NUNCA a chave crua. A
// key so existe server-side (resolver/dispatch). GET /ai-keys devolve a FONTE ATIVA
// (global|conta, por config salva); PUT grava (apiKey vazio = LIMPAR). O par /global
// exige platform_admin no back; os demais rodam no accountScope.

/** Escopo ativo + status mascarado das chaves por provider (contrato SEC). */
export interface CalendarAiKeys {
  scope: 'global' | 'account'
  keys: Record<CalendarAiSecretProvider, CalendarAiKeyStatus>
}

function normalizeKeyStatus(raw: unknown): CalendarAiKeyStatus {
  const obj = (raw && typeof raw === 'object' ? raw : {}) as { set?: unknown; last4?: unknown }
  return { set: obj.set === true, last4: typeof obj.last4 === 'string' ? obj.last4 : '' }
}

function normalizeAiKeys(res: unknown): CalendarAiKeys {
  const raw = (res && typeof res === 'object' ? res : {}) as {
    scope?: unknown
    keys?: Record<string, unknown>
  }
  const rawKeys = raw.keys && typeof raw.keys === 'object' ? raw.keys : {}
  const keys = {} as Record<CalendarAiSecretProvider, CalendarAiKeyStatus>
  for (const provider of AI_SECRET_PROVIDERS) {
    keys[provider] = normalizeKeyStatus((rawKeys as Record<string, unknown>)[provider])
  }
  return { scope: raw.scope === 'global' ? 'global' : 'account', keys }
}

export async function fetchAiKeys(api: ApiRequest): Promise<CalendarAiKeys> {
  return normalizeAiKeys(await api('/v1/calendar/ai-keys'))
}

// Grava a chave DESTA conta; apiKey vazio = limpar. Valido so com escopo=conta
// (useGlobalKeys=false); o back rejeita fora disso. Retorna void: o chamador re-le
// via fetchAiKeys (fonte unica = banco), nunca guarda a key no front.
export async function putAiKey(
  api: ApiRequest,
  provider: CalendarAiSecretProvider,
  apiKey: string,
): Promise<void> {
  await api('/v1/calendar/ai-keys', { method: 'PUT', body: { provider, apiKey } })
}

export async function fetchGlobalAiKeys(api: ApiRequest): Promise<CalendarAiKeys> {
  return normalizeAiKeys(await api('/v1/calendar/ai-keys/global'))
}

// fetchAiModels lista os modelos de CHAT do provedor (Opcao C): o back resolve a chave
// server-side e bate no /models do provedor. Devolve so os IDs (o front popula o select).
// Erros propagam o codigo do back (ai_key_missing / models_unavailable / invalid_provider)
// para o chamador escolher a mensagem. dedupe:false garante um refetch real no botao
// "Tentar novamente" (senao o GET em voo seria deduplicado).
export async function fetchAiModels(api: ApiRequest, provider: string): Promise<string[]> {
  const q = new URLSearchParams({ provider }).toString()
  const res = await api(`/v1/calendar/ai/models?${q}`, { dedupe: false })
  const list = Array.isArray((res as { models?: unknown })?.models)
    ? (res as { models: unknown[] }).models
    : []
  return list.filter((id): id is string => typeof id === 'string' && id.trim() !== '')
}

// Grava a chave GLOBAL da plataforma (contrato SEC): so platform_admin (o back
// devolve 403 fora disso). apiKey vazio = limpar.
export async function putGlobalAiKey(
  api: ApiRequest,
  provider: CalendarAiSecretProvider,
  apiKey: string,
): Promise<void> {
  await api('/v1/calendar/ai-keys/global', { method: 'PUT', body: { provider, apiKey } })
}

// --- Override de IA por cliente (WAVE 3.1, contrato SEC+) ------------------------
// So COMPORTAMENTO por cliente (enabled/provider/modelo/baseUrl/prompt/temperatura) —
// as CHAVES seguem no nivel conta/global (SEC), nunca aqui. account_id NUNCA vai no
// body/query: o back resolve pelo Principal (accountScope); so o clientId trafega.
// Override vazio ({}) = o cliente usa a config geral da conta.
export async function fetchClientAiConfig(
  api: ApiRequest,
  clientId: string,
): Promise<CalendarClientAiOverride> {
  const q = new URLSearchParams({ clientId }).toString()
  return normalizeClientAiOverride(await api(`/v1/calendar/ai-config/client?${q}`))
}

export async function putClientAiConfig(
  api: ApiRequest,
  clientId: string,
  override: CalendarClientAiOverride,
): Promise<CalendarClientAiOverride> {
  // O back le o clientId da query (autoridade), nao do body; espelhar o GET.
  const q = new URLSearchParams({ clientId }).toString()
  const res = await api(`/v1/calendar/ai-config/client?${q}`, { method: 'PUT', body: override })
  return normalizeClientAiOverride(res)
}

// --- Perfil estrategico do cliente (contrato C3) --------------------------------
// account_id NUNCA vai no body/query: o back resolve pelo Principal (accountScope).
// Aqui so o clientId (conta-cliente) trafega.
export async function fetchClientProfile(
  api: ApiRequest,
  clientId: string,
): Promise<CalendarClientProfile> {
  const q = new URLSearchParams({ clientId }).toString()
  const res = await api(`/v1/calendar/client-profile?${q}`)
  return normalizeClientProfile(res, clientId)
}

export async function putClientProfile(
  api: ApiRequest,
  profile: CalendarClientProfile,
): Promise<CalendarClientProfile> {
  // O back le o clientId da query (autoridade C3), nao do body; espelhar o GET.
  const q = new URLSearchParams({ clientId: profile.clientId }).toString()
  const res = await api(`/v1/calendar/client-profile?${q}`, { method: 'PUT', body: profile })
  return normalizeClientProfile(res, profile.clientId)
}

export async function fetchClientProfilesIndex(
  api: ApiRequest,
): Promise<CalendarClientProfileIndexItem[]> {
  const res = await api('/v1/calendar/client-profiles')
  const list = Array.isArray(res?.profiles) ? res.profiles : []
  return list
    .filter((entry: { clientId?: string }) => typeof entry?.clientId === 'string' && entry.clientId)
    .map((entry: { clientId: string; filled?: unknown; updatedAt?: unknown }) => ({
      clientId: entry.clientId,
      filled: entry.filled === true,
      updatedAt: typeof entry.updatedAt === 'string' ? entry.updatedAt : '',
    }))
}

// --- Plano de IA do mes (contrato C4) -------------------------------------------
// account_id NUNCA vai no body: o back resolve pelo Principal (accountScope). Aqui
// so month + clientIds trafegam. Retorno normalizado (shape estavel no front).
export async function createAiPlan(
  api: ApiRequest,
  month: string,
  clientIds: string[],
): Promise<{ id: string; status: string }> {
  const res = await api('/v1/calendar/ai/plan', { method: 'POST', body: { month, clientIds } })
  return { id: String(res?.id || ''), status: String(res?.status || '') }
}

export async function fetchAiPlans(
  api: ApiRequest,
  month: string,
): Promise<CalendarAiPlanIndexItem[]> {
  const q = month ? `?${new URLSearchParams({ month }).toString()}` : ''
  const res = await api(`/v1/calendar/ai/plans${q}`)
  const list = Array.isArray(res?.plans) ? res.plans : []
  return list.map(normalizePlanIndexItem)
}

export async function fetchAiPlan(api: ApiRequest, id: string): Promise<CalendarAiPlan> {
  const res = await api(`/v1/calendar/ai/plans/${encodeURIComponent(id)}`)
  return normalizePlan(res)
}

export async function markAiPlanApplied(api: ApiRequest, id: string): Promise<CalendarAiPlan> {
  const res = await api(`/v1/calendar/ai/plans/${encodeURIComponent(id)}/applied`, {
    method: 'POST',
  })
  return normalizePlan(res)
}

export async function deleteAiPlan(api: ApiRequest, id: string): Promise<void> {
  await api(`/v1/calendar/ai/plans/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

/**
 * Resultado transitorio do POST de evento (contrato C10): so a resposta de criacao
 * traz esses campos do vinculo com task. `taskWarning` != '' = o evento salvou (201)
 * mas a task nao pode ser criada (aviso, nao erro).
 */
export interface PostEventResult {
  taskId: string
  taskWarning: string
}

export async function postEvent(
  api: ApiRequest,
  input: CalendarEventInput,
): Promise<PostEventResult> {
  const res = await api('/v1/calendar/events', { method: 'POST', body: input })
  return {
    taskId: typeof res?.taskId === 'string' ? res.taskId : '',
    taskWarning: typeof res?.taskWarning === 'string' ? res.taskWarning : '',
  }
}

export async function putEvent(
  api: ApiRequest,
  id: string,
  input: CalendarEventInput,
  version?: number,
): Promise<void> {
  // C12 (optimistic locking): If-Match carrega a version que o front leu. O back devolve
  // 409 version_conflict se divergir (alguem salvou antes). Sem version = comportamento
  // antigo (compat, sem checagem).
  const headers =
    typeof version === 'number' && version > 0 ? { 'If-Match': String(version) } : undefined
  await api(`/v1/calendar/events/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: input,
    ...(headers ? { headers } : {}),
  })
}

// deleteEvent apaga o evento. archiveTask=true (WAVE 6, política "excluir os dois") arquiva também
// a task vinculada; false = só remove a relation (a task fica).
export async function deleteEvent(api: ApiRequest, id: string, archiveTask = false): Promise<void> {
  const query = archiveTask ? '?archiveTask=true' : ''
  await api(`/v1/calendar/events/${encodeURIComponent(id)}${query}`, { method: 'DELETE' })
}
