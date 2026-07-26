import type { createApiRequest } from '~/utils/api-client'
import type {
  CreateSegmentInput,
  CustomerSegmentListItem,
  CustomerSegmentView,
  SegmentEvaluationRun,
  SegmentExportView,
  SegmentFieldCatalog,
  SegmentListFilters,
  SegmentListPage,
  SegmentMaterializationView,
  SegmentVersionView,
  UpdateSegmentDraftInput,
} from './segment-types'

type SegmentApi = ReturnType<typeof createApiRequest>

interface ListResponse<T> {
  items?: T[]
  segments?: T[]
  materializations?: T[]
  nextCursor?: string
}

function clientQuery(clientAccountId: string, cursor = '', status = ''): string {
  const query = new URLSearchParams()
  if (clientAccountId.trim()) query.set('clientAccountId', clientAccountId.trim())
  if (cursor.trim()) query.set('cursor', cursor.trim())
  if (status.trim()) query.set('status', status.trim())
  return query.toString()
}

export function fetchSegmentFieldCatalog(
  api: SegmentApi,
  clientAccountId: string,
  signal?: AbortSignal,
): Promise<SegmentFieldCatalog> {
  return api(`/v1/customer-data/segment-fields?${clientQuery(clientAccountId)}`, {
    signal,
    dedupe: false,
  }) as Promise<SegmentFieldCatalog>
}

export async function fetchSegments(
  api: SegmentApi,
  filters: SegmentListFilters,
  signal?: AbortSignal,
): Promise<SegmentListPage> {
  const query = new URLSearchParams(
    clientQuery(filters.clientAccountId, filters.cursor, filters.status),
  )
  query.set('limit', String(Math.min(Math.max(filters.limit ?? 40, 1), 100)))
  const response = (await api(`/v1/customer-data/segments?${query.toString()}`, {
    signal,
    dedupe: false,
  })) as ListResponse<CustomerSegmentListItem>
  return {
    items: response.items ?? response.segments ?? [],
    nextCursor: String(response.nextCursor ?? ''),
  }
}

export function fetchSegment(
  api: SegmentApi,
  segmentId: string,
  clientAccountId: string,
  signal?: AbortSignal,
): Promise<CustomerSegmentView> {
  return api(
    `/v1/customer-data/segments/${encodeURIComponent(segmentId)}?${clientQuery(clientAccountId)}`,
    { signal, dedupe: false },
  ) as Promise<CustomerSegmentView>
}

export function createSegment(
  api: SegmentApi,
  input: CreateSegmentInput,
): Promise<CustomerSegmentView> {
  return api('/v1/customer-data/segments', {
    method: 'POST',
    body: input,
  }) as Promise<CustomerSegmentView>
}

export function updateSegmentDraft(
  api: SegmentApi,
  versionId: string,
  input: UpdateSegmentDraftInput,
): Promise<SegmentVersionView> {
  return api(`/v1/customer-data/segment-versions/${encodeURIComponent(versionId)}`, {
    method: 'PATCH',
    body: input,
  }) as Promise<SegmentVersionView>
}

export function runSegmentVersionAction(
  api: SegmentApi,
  versionId: string,
  action: 'validate' | 'preview' | 'publish',
  body: Record<string, string | number>,
): Promise<SegmentVersionView | SegmentEvaluationRun> {
  return api(`/v1/customer-data/segment-versions/${encodeURIComponent(versionId)}/${action}`, {
    method: 'POST',
    body,
  }) as Promise<SegmentVersionView | SegmentEvaluationRun>
}

export async function fetchSegmentMaterializations(
  api: SegmentApi,
  segmentId: string,
  clientAccountId: string,
  signal?: AbortSignal,
): Promise<SegmentMaterializationView[]> {
  const response = (await api(
    `/v1/customer-data/segments/${encodeURIComponent(segmentId)}/materializations?${clientQuery(clientAccountId)}`,
    { signal, dedupe: false },
  )) as ListResponse<SegmentMaterializationView> | SegmentMaterializationView[]
  return Array.isArray(response) ? response : (response.items ?? response.materializations ?? [])
}

export function createSegmentExport(
  api: SegmentApi,
  input: {
    clientAccountId: string
    materializationId: string
    purposeKey: string
    channelKey: string
    formatKey: string
    fieldSetKey: string
    reason?: string
    idempotencyKey: string
  },
): Promise<SegmentExportView> {
  return api('/v1/customer-data/segment-exports', {
    method: 'POST',
    body: input,
  }) as Promise<SegmentExportView>
}
