import type { createApiRequest } from '~/utils/api-client'
import type {
  IntelligenceRunsFilters,
  IntelligenceRunsPage,
  IntelligenceRunsFilterOptions,
  IntelligenceRunStatus,
  RuntimeRunListItem,
} from './runs-types'

type RunsApi = ReturnType<typeof createApiRequest>

interface BackendRuntimeRun {
  id: string
  requestId: string
  clientAccountId?: string
  processKey: string
  promptBindingId?: string
  agentVersionId?: string
  modelId?: string
  outputSchemaVersion?: string
  status: string
  errorCode?: string
  usage?: {
    promptTokens?: number
    completionTokens?: number
    totalTokens?: number
    latencyMs?: number
  }
  createdAt: string
  completedAt?: string
}

function normalizeStatus(status: string): IntelligenceRunStatus {
  if (status === 'succeeded') return 'completed'
  if (
    status === 'queued' ||
    status === 'running' ||
    status === 'completed' ||
    status === 'partial' ||
    status === 'failed' ||
    status === 'cancelled'
  ) {
    return status
  }
  return 'failed'
}

function durationMs(startedAt: string, finishedAt?: string): number | undefined {
  if (!finishedAt) return undefined
  const duration = Date.parse(finishedAt) - Date.parse(startedAt)
  return Number.isFinite(duration) && duration >= 0 ? duration : undefined
}

function normalizeRuntimeRun(item: BackendRuntimeRun): RuntimeRunListItem {
  return {
    id: item.id,
    clientAccountId: item.clientAccountId,
    status: normalizeStatus(item.status),
    processKey: item.processKey,
    promptBindingRef: item.promptBindingId || undefined,
    promptVersionRef: item.agentVersionId || undefined,
    schemaVersionRef: item.outputSchemaVersion || undefined,
    executorType: 'customer_intelligence',
    modelName: item.modelId || undefined,
    queuedAt: item.createdAt,
    startedAt: item.createdAt,
    finishedAt: item.completedAt,
    durationMs: durationMs(item.createdAt, item.completedAt),
    latencyMs: item.usage?.latencyMs,
    attempts: 1,
    inputUnits: item.usage?.promptTokens,
    outputUnits: item.usage?.completionTokens,
    errorCode: item.errorCode || undefined,
    correlationRef: item.requestId || undefined,
  }
}

function options(values: string[]): Array<{ value: string; label: string }> {
  return [...new Set(values.filter(Boolean))]
    .sort()
    .map((value) => ({ value, label: value.replace(/[._-]+/g, ' ') }))
}

export async function fetchIntelligenceRuns(
  api: RunsApi,
  filters: IntelligenceRunsFilters,
  signal?: AbortSignal,
): Promise<IntelligenceRunsPage> {
  const query = new URLSearchParams()
  if (filters.clientAccountId.trim()) {
    query.set('clientAccountId', filters.clientAccountId.trim())
  }
  query.set('limit', String(Math.min(Math.max(filters.limit ?? 50, 1), 100)))
  const response = (await api(`/v1/customer-intelligence/runs?${query.toString()}`, {
    signal,
    dedupe: false,
  })) as BackendRuntimeRun[]
  const normalized = (Array.isArray(response) ? response : []).map(normalizeRuntimeRun)
  const items = normalized.filter((item) => {
    if (filters.status && item.status !== filters.status) return false
    if (filters.processKey && item.processKey !== filters.processKey) return false
    if (filters.pipelineKey && item.pipelineKey !== filters.pipelineKey) return false
    if (filters.executorType && item.executorType !== filters.executorType) return false
    if (filters.startedFrom && String(item.startedAt ?? '') < filters.startedFrom) {
      return false
    }
    if (filters.startedTo && String(item.startedAt ?? '') > filters.startedTo) {
      return false
    }
    return true
  })
  const filterOptions: IntelligenceRunsFilterOptions = {
    statuses: options(normalized.map((item) => item.status)),
    processes: options(normalized.map((item) => item.processKey ?? '')),
    pipelines: options(normalized.map((item) => item.pipelineKey ?? '')),
    executors: options(normalized.map((item) => item.executorType ?? '')),
  }
  return {
    items,
    nextCursor: '',
    filterOptions,
  }
}
