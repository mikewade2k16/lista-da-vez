export type IntelligenceRunStatus =
  | 'queued'
  | 'running'
  | 'completed'
  | 'partial'
  | 'failed'
  | 'cancelled'

export interface RuntimeRunListItem {
  id: string
  clientAccountId?: string
  status: IntelligenceRunStatus
  processKey?: string
  pipelineKey?: string
  promptBindingRef?: string
  promptVersionRef?: string
  schemaVersionRef?: string
  executorType?: string
  providerName?: string
  modelName?: string
  queuedAt?: string
  startedAt?: string
  finishedAt?: string
  durationMs?: number
  latencyMs?: number
  attempts: number
  inputUnits?: number
  outputUnits?: number
  costAmount?: number
  currency?: string
  sourceCount?: number
  sourceStatus?: string
  toolCount?: number
  toolStatus?: string
  reasonCode?: string
  errorCode?: string
  correlationRef?: string
  contextRef?: {
    label: string
    path: string
  }
}

export interface IntelligenceRunsFilterOptions {
  statuses: Array<{ value: string; label: string }>
  processes: Array<{ value: string; label: string }>
  pipelines: Array<{ value: string; label: string }>
  executors: Array<{ value: string; label: string }>
}

export interface IntelligenceRunsPage {
  items: RuntimeRunListItem[]
  nextCursor: string
  filterOptions: IntelligenceRunsFilterOptions
}

export interface IntelligenceRunsFilters {
  clientAccountId: string
  status?: string
  processKey?: string
  pipelineKey?: string
  executorType?: string
  startedFrom?: string
  startedTo?: string
  cursor?: string
  limit?: number
}
