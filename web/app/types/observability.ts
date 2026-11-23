// Contratos da API do painel (observability/SPEC.md). Campos opcionais onde o
// backend pode omitir (ex.: sem rodada do runner ainda).

export interface ObsLiveStats {
  requests: number
  errorRate: number
  p95Ms: number
  droppedEvents: number
  enabled: boolean
}

export interface ObsModuleScore {
  module: string
  score: number
  components?: Record<string, number>
  coverage?: number
}

export interface ObsTrendPoint {
  runId: string
  startedAt: string
  scores: Record<string, number>
  coverage?: Record<string, number> | number
}

export interface ObsOverview {
  live: ObsLiveStats
  modules: ObsModuleScore[]
  trend: ObsTrendPoint[]
}

export interface ObsEndpoint {
  route: string
  method: string
  module: string
  calls: number
  errorRate: number
  p50Ms: number
  p95Ms: number
  p99Ms: number
  avgQueries: number
  maxQueries: number
}

export interface ObsSlowQuery {
  correlationId?: string
  route?: string
  fingerprint?: string
  statement: string
  durationMs: number
  createdAt?: string
}

export interface ObsNPlusOne {
  route: string
  fingerprint: string
  statement: string
  occurrences: number
  maxRepetitions: number
  lastCorrelationId?: string
}

export interface ObsTopQuery {
  fingerprint: string
  statement: string
  calls: number
  totalMs: number
  maxMs: number
  avgMs: number
}

export interface ObsRunResult {
  kind: string
  module?: string
  target: string
  status: 'pass' | 'fail' | 'warn' | 'skip'
  reason?: string
  metrics?: Record<string, unknown>
}

export interface ObsDb {
  slow: ObsSlowQuery[]
  nPlusOne: ObsNPlusOne[]
  top: ObsTopQuery[]
  checks?: unknown
}

export interface ObsPageLive {
  page: string
  samples: number
  lcpP75?: number | null
  inpP75?: number | null
  clsP75?: number | null
  ttfbP75?: number | null
  loadP75?: number | null
  jsErrors: number
  apiFailures: number
}

export interface ObsPages {
  live: ObsPageLive[]
  lastRun: ObsRunResult[]
}

export interface ObsErrorGroup {
  source: 'api' | 'front'
  signature: string
  route?: string
  page?: string
  count: number
  lastSeen: string
  correlationIds: string[]
}

export interface ObsTrace {
  request?: Record<string, unknown> | null
  slowQueries: ObsSlowQuery[]
  nPlusOne: ObsNPlusOne[]
  externalCalls: Array<Record<string, unknown>>
  frontendEvents: Array<Record<string, unknown>>
}

export interface ObsRun {
  id: string
  startedAt: string
  finishedAt?: string | null
  env?: string
  filters?: Record<string, unknown> | null
  summary?: Record<string, unknown> | null
  coverage?: Record<string, unknown> | null
  scores?: Record<string, unknown> | null
  exitCode?: number | null
}

export interface ObsRunChange {
  kind: string
  target: string
  metric: string
  before: number | null
  after: number | null
  deltaPct: number | null
  verdict: 'regression' | 'improvement' | 'same'
}

export interface ObsRunCompare {
  a: ObsRun
  b: ObsRun
  changes: ObsRunChange[]
}

export interface ObsRunRequest {
  id: string | number
  module?: string
  only?: string
  status: 'queued' | 'running' | 'done' | 'failed'
  runId?: string | null
  requestedAt?: string
  createdAt?: string
  log?: string
}

export interface ObsRunRequests {
  canRun: boolean
  workerOnline: boolean
  items: ObsRunRequest[]
}
