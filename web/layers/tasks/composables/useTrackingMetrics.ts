import { computed, ref } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { useCan } from './useCan'
import type { TaskItem } from '../types/tasks'

// ── Shape REAL do endpoint ─────────────────────────────────────────────────────
// GET /v1/tasks/tracking/metrics  (headers: Authorization Bearer + X-Account-Id)
//   query: accountId? (derivado do header), clientAccountId?, userId?, from?, to?
//   resposta: { "metrics": {
//     totalDurationMs, entryCount,
//     byClient: [{key,label,totalDurationMs,entryCount}],   // GROUP BY t.client_account_id
//     byUser:   [{key,label,totalDurationMs,entryCount}],   // GROUP BY e.user_id
//     byType:   [{key,label,totalDurationMs,entryCount}]    // GROUP BY ui_metadata->>'type'
//   } }
//
// O backend agrega tudo server-side numa unica chamada (sem N+1): label ja vem resolvido
// por join (core.accounts / core.users). Breakdowns respeitam o periodo (from/to); o total
// escalar respeita tambem clientAccountId/userId. Ver repository_postgres_tracking.go.
interface BackendBucket {
  key?: string
  label?: string
  totalDurationMs?: number
  entryCount?: number
}

interface BackendTrackingMetrics {
  totalDurationMs?: number
  entryCount?: number
  byClient?: BackendBucket[]
  byUser?: BackendBucket[]
  byType?: BackendBucket[]
}

export interface TrackingMetricsFilters {
  from?: string
  to?: string
}

export interface TrackingBreakdownRow {
  key: string
  label: string
  durationMs: number
  entryCount: number
}

export interface TrackedTypeRow {
  key: string
  label: string
  ms: number
  count: number
}

export interface TrackingBoardColumnView {
  id: string
  label: string
  color: string
  tasks: TaskItem[]
}

export interface TrackingProjectBoard {
  projectId: string
  projectName: string
  projectIcon: string
  columns: TrackingBoardColumnView[]
}

const BOARD_COLOR_OPTIONS = ['indigo', 'slate', 'blue', 'amber', 'emerald', 'violet', 'rose']

export function trackingNormalizeKey(value: unknown) {
  return String(value ?? '')
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
}

function trackingTaskSort(a: TaskItem, b: TaskItem) {
  const delta = Number(a.order || 0) - Number(b.order || 0)
  return delta !== 0 ? delta : a.createdAt.localeCompare(b.createdAt)
}

function normalizeText(value: unknown, max = 80) {
  return String(value ?? '')
    .trim()
    .slice(0, max)
}

function toBreakdown(rows: BackendBucket[] | undefined): TrackingBreakdownRow[] {
  if (!Array.isArray(rows)) return []
  return rows
    .map((row) => ({
      key: normalizeText(row.key, 80),
      label: normalizeText(row.label, 140) || 'Sem nome',
      durationMs: Math.max(0, Number(row.totalDurationMs ?? 0) || 0),
      entryCount: Math.max(0, Number(row.entryCount ?? 0) || 0),
    }))
    .filter((row) => row.durationMs > 0 || row.entryCount > 0)
}

interface ParsedTrackingMetrics {
  totalDurationMs: number
  entryCount: number
  byClient: TrackingBreakdownRow[]
  byUser: TrackingBreakdownRow[]
  byType: TrackingBreakdownRow[]
}

function toMetrics(response: unknown): ParsedTrackingMetrics {
  const metrics = (response as { metrics?: BackendTrackingMetrics } | null)?.metrics
  return {
    totalDurationMs: Math.max(0, Number(metrics?.totalDurationMs ?? 0) || 0),
    entryCount: Math.max(0, Number(metrics?.entryCount ?? 0) || 0),
    byClient: toBreakdown(metrics?.byClient),
    byUser: toBreakdown(metrics?.byUser),
    byType: toBreakdown(metrics?.byType),
  }
}

export function useTrackingMetrics() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const canViewAll = useCan('tasks.tracking.view_all')
  const canUse = useCan('tasks.tracking.use')

  const filters = ref<TrackingMetricsFilters>({ from: '', to: '' })
  const loading = ref(false)
  const errorMessage = ref('')

  const total = ref<{ totalDurationMs: number; entryCount: number }>({
    totalDurationMs: 0,
    entryCount: 0,
  })
  const porCliente = ref<TrackingBreakdownRow[]>([])
  const porUsuario = ref<TrackingBreakdownRow[]>([])
  const porTipo = ref<TrackingBreakdownRow[]>([])

  const accountId = computed(() =>
    normalizeText(auth.activeTenantId || auth.tenantContext?.[0]?.id),
  )

  function buildQuery() {
    const query: Record<string, string> = {}
    const from = normalizeText(filters.value.from, 32)
    const to = normalizeText(filters.value.to, 32)
    if (from) query.from = from
    if (to) query.to = to
    if (accountId.value) query.accountId = accountId.value
    return query
  }

  async function fetchMetrics(): Promise<ParsedTrackingMetrics> {
    if (auth.isAuthenticated) {
      await auth.ensureSession()
    }
    const response = await apiRequest('/v1/tasks/tracking/metrics', {
      skipLoadingIndicator: true,
      query: buildQuery(),
      headers: accountId.value ? { 'X-Account-Id': accountId.value } : undefined,
    })
    return toMetrics(response)
  }

  function clearMetrics() {
    total.value = { totalDurationMs: 0, entryCount: 0 }
    porCliente.value = []
    porUsuario.value = []
    porTipo.value = []
  }

  async function refresh() {
    // O endpoint /metrics exige tasks.tracking.view_all (403 caso contrario). Gate ANTES de
    // disparar, espelhando o back: quem so tem tasks.tracking.use ve apenas "Em andamento".
    if (!canViewAll.value) {
      clearMetrics()
      errorMessage.value = ''
      return
    }
    if (!auth.accessToken) return
    loading.value = true
    errorMessage.value = ''
    try {
      // Uma unica chamada: o backend agrega total + breakdowns (cliente/usuario/tipo) server-side.
      const metrics = await fetchMetrics()
      total.value = { totalDurationMs: metrics.totalDurationMs, entryCount: metrics.entryCount }
      porCliente.value = metrics.byClient
      porUsuario.value = metrics.byUser
      porTipo.value = metrics.byType
    } catch (error) {
      clearMetrics()
      errorMessage.value = getApiErrorMessage(
        error,
        'Nao foi possivel carregar as metricas de tracking.',
      )
      console.error(errorMessage.value, error)
    } finally {
      loading.value = false
    }
  }

  function setPeriod(from: string, to: string) {
    filters.value = { from: normalizeText(from, 32), to: normalizeText(to, 32) }
    void refresh()
  }

  function formatElapsed(ms: number): string {
    const totalSeconds = Math.floor(Math.max(0, ms) / 1000)
    const hours = Math.floor(totalSeconds / 3600)
    const minutes = Math.floor((totalSeconds % 3600) / 60)
    const mm = String(minutes).padStart(2, '0')
    if (hours > 0) return `${hours}h ${mm}min`
    if (minutes > 0) return `${minutes}min`
    return `${totalSeconds}s`
  }

  return {
    filters,
    loading,
    errorMessage,
    canViewAll,
    canUse,
    total,
    porCliente,
    porUsuario,
    porTipo,
    formatElapsed,
    refresh,
    setPeriod,
  }
}

// useTrackingBoards: deriva a visao "em andamento" (boards/colunas com timers ativos)
// e o tempo por tipo dos timers correntes. Separado de useTrackingMetrics para manter
// a pagina enxuta (principio: quebrar logica em composable). Recebe o workspace de
// tasks e os getters de tracking ja resolvidos pela pagina.
interface TrackingBoardsDeps {
  projects: () => Array<{
    id: string
    name: string
    icon: string
    columns: Array<{ label: string; color: string }>
  }>
  tasks: () => TaskItem[]
  trackedTaskIds: () => string[]
  getElapsedMs: (taskId: string) => number
}

export function useTrackingBoards(deps: TrackingBoardsDeps) {
  const trackingBoards = computed<TrackingProjectBoard[]>(() => {
    const ids = new Set(deps.trackedTaskIds())
    const boards: TrackingProjectBoard[] = []
    for (const project of deps.projects()) {
      const tracked = deps
        .tasks()
        .filter((task) => task.projectId === project.id && ids.has(task.id))
      if (!tracked.length) continue
      const schemaColors = new Map(
        project.columns.map((column) => [trackingNormalizeKey(column.label), column.color]),
      )
      const statuses = [...new Set(tracked.map((task) => task.status))]
      const columns: TrackingBoardColumnView[] = statuses.map((status, index) => ({
        id: `${project.id}-${trackingNormalizeKey(status) || 'empty'}`,
        label: status || 'Sem status',
        color:
          schemaColors.get(trackingNormalizeKey(status)) ||
          BOARD_COLOR_OPTIONS[index % BOARD_COLOR_OPTIONS.length]!,
        tasks: tracked
          .filter((task) => trackingNormalizeKey(task.status) === trackingNormalizeKey(status))
          .sort(trackingTaskSort),
      }))
      boards.push({
        projectId: project.id,
        projectName: project.name,
        projectIcon: project.icon || 'i-lucide-folder',
        columns,
      })
    }
    return boards
  })

  // Por tipo a partir dos timers ATIVOS (unico tempo por tipo disponivel — o backend
  // nao filtra metricas por type; ver useTrackingMetrics).
  const porTipoAndamento = computed<TrackedTypeRow[]>(() => {
    const ids = new Set(deps.trackedTaskIds())
    const bucket = new Map<string, TrackedTypeRow>()
    for (const task of deps.tasks()) {
      if (!ids.has(task.id)) continue
      const key = trackingNormalizeKey(task.type) || 'sem_tipo'
      const label = String(task.type || '').trim() || 'Sem tipo'
      const current = bucket.get(key) || { key, label, ms: 0, count: 0 }
      current.ms += deps.getElapsedMs(task.id)
      current.count += 1
      bucket.set(key, current)
    }
    return [...bucket.values()].sort((a, b) => b.ms - a.ms)
  })

  return { trackingBoards, porTipoAndamento }
}
