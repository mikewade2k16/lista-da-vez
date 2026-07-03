// Camada de acesso a dados do calendario (I/O puro sobre o apiRequest).
// Separada do store (stores/calendar.ts) para manter cada arquivo < 450 linhas e
// isolar a construcao de URL/parse de resposta da orquestracao de estado.
// Todas as rotas sao multi-tenant: o X-Account-Id vai no header pelo apiRequest.
import type { createApiRequest } from '~/utils/api-client'
import {
  defaultCalendarConfig,
  defaultCalendarMediaLimits,
  normalizeClientProfile,
  normalizePlan,
  normalizePlanIndexItem,
  type CalendarAiPlan,
  type CalendarAiPlanIndexItem,
  type CalendarClientProfile,
  type CalendarClientProfileIndexItem,
  type CalendarConfig,
  type CalendarEvent,
  type CalendarEventInput,
  type CalendarHoliday,
  type CalendarMediaItem,
  type CalendarMediaLimits,
  type CalendarMember,
  type CalendarPerson,
} from '~/utils/calendar'

/** Uma entrada de anexos avulsos por dia (`GET /v1/calendar/day-media`). */
export interface CalendarDayMedia {
  date: string
  media: CalendarMediaItem[]
}

// Tipo exato do apiRequest devolvido por createApiRequest (evita incompatibilidade
// de contravariancia ao passar apiRequest para estas funcoes).
export type ApiRequest = ReturnType<typeof createApiRequest>

function rangeQuery(from: string, to: string): string {
  return new URLSearchParams({ from, to }).toString()
}

function asStringMap(value: unknown): Record<string, string> {
  if (!value || typeof value !== 'object') return {}
  const out: Record<string, string> = {}
  for (const [key, raw] of Object.entries(value as Record<string, unknown>)) {
    if (typeof raw === 'string') out[key] = raw
  }
  return out
}

// normalizeConfig faz merge POR SECAO (nao spread raso): linha antiga do banco sem
// os campos novos (weekStartsOn/clientColors/typeColors/whiteLabel/ai) ainda ganha
// o shape completo C2 com os defaults preenchidos.
export function normalizeConfig(res: unknown): CalendarConfig {
  const base = defaultCalendarConfig()
  const raw = (res as Partial<CalendarConfig>) || {}
  return {
    responsibleUserIds: Array.isArray(raw.responsibleUserIds) ? raw.responsibleUserIds : [],
    holidays: { ...base.holidays, ...(raw.holidays || {}) },
    weekStartsOn: raw.weekStartsOn === 'monday' ? 'monday' : 'sunday',
    clientColors: asStringMap(raw.clientColors),
    typeColors: asStringMap(raw.typeColors),
    whiteLabel: { ...base.whiteLabel, ...(raw.whiteLabel || {}) },
    ai: { ...base.ai, ...(raw.ai || {}) },
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
): Promise<CalendarEvent[]> {
  const res = await api(`/v1/calendar/events?${rangeQuery(from, to)}`)
  return Array.isArray(res?.events) ? (res.events as CalendarEvent[]) : []
}

export async function fetchHolidaysInRange(
  api: ApiRequest,
  from: string,
  to: string,
): Promise<CalendarHoliday[]> {
  const res = await api(`/v1/calendar/holidays?${rangeQuery(from, to)}`)
  return Array.isArray(res?.holidays) ? (res.holidays as CalendarHoliday[]) : []
}

export async function fetchDayMediaInRange(
  api: ApiRequest,
  from: string,
  to: string,
): Promise<CalendarDayMedia[]> {
  const res = await api(`/v1/calendar/day-media?${rangeQuery(from, to)}`)
  const days = Array.isArray(res?.days) ? res.days : []
  return days
    .filter((entry: { date?: string }) => typeof entry?.date === 'string' && entry.date)
    .map((entry: { date: string; media?: unknown }) => ({
      date: entry.date,
      media: Array.isArray(entry.media) ? (entry.media as CalendarMediaItem[]) : [],
    }))
}

export async function putDayMedia(
  api: ApiRequest,
  date: string,
  media: CalendarMediaItem[],
): Promise<void> {
  await api(`/v1/calendar/day-media/${date}`, { method: 'PUT', body: { media } })
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

export async function postEvent(api: ApiRequest, input: CalendarEventInput): Promise<void> {
  await api('/v1/calendar/events', { method: 'POST', body: input })
}

export async function putEvent(
  api: ApiRequest,
  id: string,
  input: CalendarEventInput,
): Promise<void> {
  await api(`/v1/calendar/events/${encodeURIComponent(id)}`, { method: 'PUT', body: input })
}

export async function deleteEvent(api: ApiRequest, id: string): Promise<void> {
  await api(`/v1/calendar/events/${encodeURIComponent(id)}`, { method: 'DELETE' })
}
