import type { createApiRequest } from '~/utils/api-client'
import type {
  IntelligenceAuditEventView,
  IntelligenceAuditFilters,
  IntelligenceAuditPage,
  IntelligenceObservationView,
} from './audit-types'

type AuditApi = ReturnType<typeof createApiRequest>

interface BackendAuditEvent {
  id: string
  clientAccountId?: string
  actorUserId?: string
  eventType: string
  aggregateType: string
  aggregateId: string
  correlationId?: string
  reasonCode?: string
  occurredAt: string
}

interface BackendAuditPage {
  items?: BackendAuditEvent[]
  nextCursor?: string
}

function append(query: URLSearchParams, key: string, value: string | undefined): void {
  const normalized = String(value ?? '').trim()
  if (normalized) query.set(key, normalized)
}

function label(value: string): string {
  return value.replace(/[._-]+/g, ' ').replace(/\b\w/g, (character) => character.toUpperCase())
}

function appendTimestamp(query: URLSearchParams, key: string, value: string | undefined): void {
  const normalized = String(value ?? '').trim()
  if (!normalized) return
  const parsed = new Date(normalized)
  query.set(key, Number.isNaN(parsed.getTime()) ? normalized : parsed.toISOString())
}

function normalizeAuditEvent(item: BackendAuditEvent): IntelligenceAuditEventView {
  const isObservationAggregate =
    item.aggregateType === 'source_observation' || item.aggregateType === 'source_observations'
  const canOpenObservation =
    isObservationAggregate && item.eventType !== 'source.observation_retention_applied'
  return {
    id: item.id,
    clientAccountId: item.clientAccountId,
    action: item.eventType,
    entityType: item.aggregateType,
    entityRef: item.aggregateId,
    occurredAt: item.occurredAt,
    actor: {
      type: item.actorUserId ? 'user' : 'system',
      ref: item.actorUserId || undefined,
      display: item.actorUserId ? 'Usuario autorizado' : 'Sistema',
    },
    diff: [],
    reasonCode: item.reasonCode || undefined,
    correlationCode: item.correlationId || undefined,
    observationRef: canOpenObservation ? item.aggregateId : undefined,
    canOpenObservation,
    // aggregateType ainda nao possui descriptor de rotas navegaveis.
    canNavigate: false,
  }
}

export async function fetchIntelligenceAuditEvents(
  api: AuditApi,
  filters: IntelligenceAuditFilters,
  signal?: AbortSignal,
): Promise<IntelligenceAuditPage> {
  const query = new URLSearchParams()
  append(query, 'clientAccountId', filters.clientAccountId)
  append(query, 'action', filters.action)
  append(query, 'entityType', filters.entityType)
  appendTimestamp(query, 'occurredFrom', filters.occurredFrom)
  appendTimestamp(query, 'occurredTo', filters.occurredTo)
  append(query, 'cursor', filters.cursor)
  query.set('limit', String(Math.min(Math.max(filters.limit ?? 50, 1), 100)))
  const response = (await api(`/v1/customer-intelligence/audit-events?${query.toString()}`, {
    signal,
    dedupe: false,
  })) as BackendAuditPage | BackendAuditEvent[]
  const rows = Array.isArray(response)
    ? response
    : Array.isArray(response?.items)
      ? response.items
      : []
  const normalized = rows.map(normalizeAuditEvent)
  const actions = [
    ...new Set([...normalized.map((item) => item.action), String(filters.action ?? '').trim()]),
  ]
    .filter(Boolean)
    .sort()
  const entityTypes = [
    ...new Set([
      ...normalized.map((item) => item.entityType),
      String(filters.entityType ?? '').trim(),
    ]),
  ]
    .filter(Boolean)
    .sort()
  return {
    items: normalized,
    nextCursor: Array.isArray(response) ? '' : String(response?.nextCursor ?? '').trim(),
    filterOptions: {
      actions: actions.map((value) => ({ value, label: label(value) })),
      entityTypes: entityTypes.map((value) => ({ value, label: label(value) })),
      statuses: [],
      provenances: [],
    },
  }
}

export function fetchIntelligenceObservation(
  api: AuditApi,
  observationId: string,
  clientAccountId: string,
  signal?: AbortSignal,
): Promise<IntelligenceObservationView> {
  const query = new URLSearchParams()
  append(query, 'clientAccountId', clientAccountId)
  return api(
    `/v1/customer-intelligence/observations/${encodeURIComponent(observationId)}?${query.toString()}`,
    { signal, dedupe: false },
  ) as Promise<IntelligenceObservationView>
}

export function revealIntelligenceObservation(
  api: AuditApi,
  observationId: string,
  clientAccountId: string,
  reasonCode: string,
  signal?: AbortSignal,
): Promise<IntelligenceObservationView> {
  const query = new URLSearchParams()
  append(query, 'clientAccountId', clientAccountId)
  return api(
    `/v1/customer-intelligence/observations/${encodeURIComponent(observationId)}/reveal?${query.toString()}`,
    {
      method: 'POST',
      body: { reasonCode },
      signal,
      dedupe: false,
    },
  ) as Promise<IntelligenceObservationView>
}
