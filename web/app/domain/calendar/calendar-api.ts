// Camada de acesso a dados do calendario (I/O puro sobre o apiRequest).
// Separada do store (stores/calendar.ts) para manter cada arquivo < 450 linhas e
// isolar a construcao de URL/parse de resposta da orquestracao de estado.
// Todas as rotas sao multi-tenant: o X-Account-Id vai no header pelo apiRequest.
import type { createApiRequest } from '~/utils/api-client'
import {
  defaultCalendarConfig,
  type CalendarConfig,
  type CalendarEvent,
  type CalendarEventInput,
  type CalendarHoliday,
  type CalendarMember,
  type CalendarPerson,
} from '~/utils/calendar'

// Tipo exato do apiRequest devolvido por createApiRequest (evita incompatibilidade
// de contravariancia ao passar apiRequest para estas funcoes).
export type ApiRequest = ReturnType<typeof createApiRequest>

function rangeQuery(from: string, to: string): string {
  return new URLSearchParams({ from, to }).toString()
}

export function normalizeConfig(res: unknown): CalendarConfig {
  const cfg = { ...defaultCalendarConfig(), ...((res as Partial<CalendarConfig>) || {}) }
  if (!Array.isArray(cfg.responsibleUserIds)) cfg.responsibleUserIds = []
  return cfg
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
