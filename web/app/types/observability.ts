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

// --- Saude do sistema (fase 6) -----------------------------------------------

export type ObsHealthStatus = 'ok' | 'warn' | 'critical' | 'unknown'
export type ObsSeverity = 'critical' | 'high' | 'medium' | 'low'
export type ObsTabKey = 'endpoints' | 'db' | 'pages' | 'errors' | 'security' | 'system' | 'deploys'

export interface ObsHealthArea {
  key: string
  label: string
  status: ObsHealthStatus
  summary: string
}

export interface ObsHealthIssue {
  severity: ObsSeverity
  area: string
  title: string
  detail: string
  since: string
  link?: { tab: ObsTabKey; query?: Record<string, unknown> } | null
}

export interface ObsHealthErrorCounts {
  api5xx: number
  frontend: number
}

export interface ObsHealthTopError {
  source: 'api' | 'front'
  signature: string
  count: number
}

export interface ObsOnlineUser {
  userId: string
  name: string
  page: string
  lastSeen: string
  tabs: number
}

export interface ObsHealthAlertSummary {
  id: number
  severity: string
  title: string
  since: string
  notifiedAt: string | null
  acknowledgedAt?: string | null
  acknowledgedBy?: string | null
  silencedUntil?: string | null
  silencedBy?: string | null
}

export interface ObsSecuritySummary {
  checks: { pass: number; fail: number; partial: number; not_tested: number }
  findings: { open: Record<ObsSeverity, number>; fixed: number }
}

export interface ObsDiskSample {
  path: string
  totalBytes: number
  freeBytes: number
  usedPct: number
}

export interface ObsHealth {
  generatedAt: string
  overall: 'ok' | 'warn' | 'critical'
  areas: ObsHealthArea[]
  issues: ObsHealthIssue[]
  errors: {
    lastHour: ObsHealthErrorCounts
    last24h: ObsHealthErrorCounts
    top: ObsHealthTopError[]
  }
  online: { count: number; users: ObsOnlineUser[] }
  system: {
    api: {
      at: string
      goroutines: number
      heapMB: number
      poolUtilization: number
      pgConnections: number
      pgMaxConnections: number
    } | null
    agent: { at: string; disk: ObsDiskSample[]; containers: number; running: number } | null
  }
  alerts: {
    channel: string
    configured: boolean
    recipients: number
    whatsapp: string
    open: ObsHealthAlertSummary[]
  }
  security: ObsSecuritySummary
}

export type ObsCheckStatus = 'pass' | 'fail' | 'partial' | 'not_tested'

export interface ObsSecurityCheck {
  key: string
  category: string
  title: string
  standard: string
  status: ObsCheckStatus
  detail: string
  checkedAt: string | null
  source: string
}

export type ObsFindingStatus = 'open' | 'fixed' | 'accepted'

export interface ObsSecurityFinding {
  id: string
  severity: ObsSeverity
  title: string
  area: string
  location: string
  status: ObsFindingStatus | string
  owner: string
  dueDate: string | null
  note: string
  updatedBy: string
  updatedAt: string
  overdue: boolean
}

export interface ObsFindingPatch {
  status?: ObsFindingStatus
  owner?: string
  dueDate?: string | null
  note?: string
}

export interface ObsSecurity {
  checks: ObsSecurityCheck[]
  findings: ObsSecurityFinding[]
}

export interface ObsApiSystemMetrics {
  go?: { goroutines: number; heapAllocBytes: number; heapSysBytes: number; numGC: number }
  pool?: { max: number; acquired: number; idle: number; utilization: number }
  pg?: {
    connections: number
    maxConnections: number
    active: number
    idleInTransaction: number
    waitingOnLock: number
    longestQuerySeconds: number
    cacheHitRatio: number
    dbSizeBytes: number
  }
  jobs?: Array<{
    table: string
    pending: number
    due: number
    oldestDueLagSeconds: number
    stuckProcessing: number
    dead: number
  }>
}

export interface ObsAgentSystemMetrics {
  disk?: ObsDiskSample[]
  docker?: { containers: number; running: number }
}

export interface ObsSystemSample<T> {
  at: string
  metrics: T
}

export interface ObsContainerSample {
  container: string
  service: string
  state: string
  health: string
  restartCount: number
  cpuPct: number
  memBytes: number
  memLimitBytes: number
  at: string
}

export interface ObsSystem {
  api: Array<ObsSystemSample<ObsApiSystemMetrics>>
  agent: Array<ObsSystemSample<ObsAgentSystemMetrics>>
  containers: ObsContainerSample[]
}

export interface ObsAlert {
  id: number
  rule: string
  severity: string
  area: string
  title: string
  detail: { measured?: string } & Record<string, unknown>
  status: 'open' | 'resolved'
  firstSeen: string
  lastSeen: string
  resolvedAt: string | null
  notifiedAt: string | null
  notifyCount: number
  lastNotifyError: string | null
  acknowledgedAt: string | null
  acknowledgedBy: string | null
  silencedUntil: string | null
  silencedBy: string | null
}

export interface ObsIntegration {
  host: string
  calls: number
  errors: number
  errorRate: number
  avgMs: number
  p95Ms: number
  lastErrorAt: string | null
  lastError: string
}

export type ObsDeployVerdict = 'ok' | 'slow' | 'failed'

export interface ObsDeployStage {
  name: string
  depth: number
  status: string
  durationMs: number | null
}

export interface ObsDeploy {
  id: string
  command: string
  environment: string
  startedAt: string
  finishedAt: string | null
  durationMs: number | null
  exitCode: number | null
  status: 'success' | 'failed' | 'interrupted' | 'unknown'
  gitSha: string
  gitBranch: string
  imageTag: string
  verdict: ObsDeployVerdict
  stages: ObsDeployStage[]
}

export interface ObsDeployStats {
  count: number
  successRate: number
  medianMs: number
  p90Ms: number
  slowThresholdMs: number
  lastSuccessAt: string | null
}

export interface ObsDeploys {
  items: ObsDeploy[]
  stats: ObsDeployStats
}
