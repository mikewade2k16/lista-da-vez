import { computed, ref } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

type TrackingStatus = 'running' | 'paused'

interface BackendTimeEntry {
  id?: string
  taskId?: string
  startedAt?: string
  pausedAt?: string
  resumedAt?: string
  stoppedAt?: string
  durationMs?: number
  version?: number
  updatedAt?: string
}

interface TrackingEntry {
  id: string
  status: TrackingStatus
  startedAt: number | null
  accumulatedMs: number
  version: number
}

const _entries = ref<Record<string, TrackingEntry>>({})
const _tick = ref(0)
const _serverOffsetMs = ref(0)
const _pending = ref<Record<string, boolean>>({})
const _lastError = ref('')
let _hydrated = false
let _hydratePromise: Promise<void> | null = null

if (typeof window !== 'undefined') {
  setInterval(() => { _tick.value++ }, 1000)
}

function normalizeText(value: unknown, max = 180) {
  return String(value ?? '').trim().slice(0, max)
}

function toMs(value: unknown) {
  const parsed = Date.parse(String(value || ''))
  return Number.isFinite(parsed) ? parsed : 0
}

function updateServerOffset(entry: BackendTimeEntry) {
  const serverNow = toMs(entry.updatedAt || entry.pausedAt || entry.resumedAt || entry.startedAt)
  if (serverNow > 0) _serverOffsetMs.value = serverNow - Date.now()
}

function entryFromBackend(entry: BackendTimeEntry): TrackingEntry | null {
  const taskId = normalizeText(entry.taskId, 80)
  if (!taskId || entry.stoppedAt) return null
  updateServerOffset(entry)
  const paused = Boolean(entry.pausedAt)
  const anchor = toMs(entry.resumedAt || entry.startedAt)
  return {
    id: normalizeText(entry.id, 80),
    status: paused ? 'paused' : 'running',
    startedAt: paused ? null : anchor,
    accumulatedMs: Math.max(0, Number(entry.durationMs || 0) || 0),
    version: Math.max(0, Number(entry.version || 0) || 0)
  }
}

function mergeBackendEntry(entry: BackendTimeEntry) {
  const taskId = normalizeText(entry.taskId, 80)
  if (!taskId) return
  const mapped = entryFromBackend(entry)
  const next = { ..._entries.value }
  if (mapped) next[taskId] = mapped
  else delete next[taskId]
  _entries.value = next
}

function replaceFromBackend(entries: BackendTimeEntry[]) {
  const next: Record<string, TrackingEntry> = {}
  entries.forEach((entry) => {
    const taskId = normalizeText(entry.taskId, 80)
    const mapped = entryFromBackend(entry)
    if (taskId && mapped) next[taskId] = mapped
  })
  _entries.value = next
}

function nowServerMs() {
  return Date.now() + _serverOffsetMs.value
}

function rollback(taskId: string, previous: TrackingEntry | undefined) {
  const next = { ..._entries.value }
  if (previous) next[taskId] = previous
  else delete next[taskId]
  _entries.value = next
}

function optimisticRun(taskId: string, accumulatedMs = 0, previous?: TrackingEntry) {
  _entries.value = {
    ..._entries.value,
    [taskId]: {
      id: previous?.id || '',
      status: 'running',
      startedAt: nowServerMs(),
      accumulatedMs,
      version: previous?.version || 0
    }
  }
}

export function useTimeTracking() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const trackedTaskIds = computed(() => Object.keys(_entries.value))
  const pendingTaskIds = computed(() => Object.keys(_pending.value).filter(taskId => _pending.value[taskId]))
  const lastTrackingError = computed(() => _lastError.value)

  async function requestTracking(path: string, options: Record<string, any> = {}) {
    if (auth.isAuthenticated) {
      await auth.ensureSession()
    }
    const accountId = normalizeText(auth.activeTenantId || auth.tenantContext?.[0]?.id, 80)
    return await apiRequest(path, {
      skipLoadingIndicator: true,
      ...options,
      headers: {
        ...(options.headers || {}),
        ...(accountId ? { 'X-Account-Id': accountId } : {})
      }
    })
  }

  async function refreshActiveTracking(force = false) {
    if (!import.meta.client || (!force && _hydrated)) return
    if (!auth.accessToken) return
    if (_hydratePromise) return _hydratePromise
    _hydratePromise = (async () => {
      try {
        const response = await requestTracking('/v1/tasks/tracking/active')
        replaceFromBackend(Array.isArray(response?.entries) ? response.entries : [])
        _hydrated = true
        _lastError.value = ''
      } catch (error) {
        _lastError.value = getApiErrorMessage(error, 'Nao foi possivel carregar tracking ativo.')
      } finally {
        _hydratePromise = null
      }
    })()
    return _hydratePromise
  }

  async function startTracking(taskId: string) {
    const id = normalizeText(taskId, 80)
    if (!id || _pending.value[id]) return
    const previous = _entries.value[id]
    const endpoint = previous?.status === 'paused' ? 'resume' : 'start'
    optimisticRun(id, previous?.accumulatedMs || 0, previous)
    _pending.value = { ..._pending.value, [id]: true }
    try {
      const response = await requestTracking(`/v1/tasks/${encodeURIComponent(id)}/tracking/${endpoint}`, {
        method: 'POST',
        headers: previous?.version ? { 'If-Match': String(previous.version) } : undefined
      })
      if (response?.entry) mergeBackendEntry(response.entry)
      _lastError.value = ''
    } catch (error) {
      rollback(id, previous)
      _lastError.value = getApiErrorMessage(error, 'Nao foi possivel iniciar tracking.')
      console.error(_lastError.value, error)
    } finally {
      const nextPending = { ..._pending.value }
      delete nextPending[id]
      _pending.value = nextPending
    }
  }

  async function pauseTracking(taskId: string) {
    const id = normalizeText(taskId, 80)
    const previous = _entries.value[id]
    if (!id || !previous || previous.status !== 'running' || _pending.value[id]) return
    const liveMs = previous.startedAt !== null ? Math.max(0, nowServerMs() - previous.startedAt) : 0
    _entries.value = {
      ..._entries.value,
      [id]: {
        ...previous,
        status: 'paused',
        startedAt: null,
        accumulatedMs: previous.accumulatedMs + liveMs
      }
    }
    _pending.value = { ..._pending.value, [id]: true }
    try {
      const response = await requestTracking(`/v1/tasks/${encodeURIComponent(id)}/tracking/pause`, {
        method: 'POST',
        headers: previous.version ? { 'If-Match': String(previous.version) } : undefined
      })
      if (response?.entry) mergeBackendEntry(response.entry)
      _lastError.value = ''
    } catch (error) {
      rollback(id, previous)
      _lastError.value = getApiErrorMessage(error, 'Nao foi possivel pausar tracking.')
      console.error(_lastError.value, error)
    } finally {
      const nextPending = { ..._pending.value }
      delete nextPending[id]
      _pending.value = nextPending
    }
  }

  async function stopTracking(taskId: string) {
    const id = normalizeText(taskId, 80)
    const previous = _entries.value[id]
    if (!id || !previous || _pending.value[id]) return
    const next = { ..._entries.value }
    delete next[id]
    _entries.value = next
    _pending.value = { ..._pending.value, [id]: true }
    try {
      const response = await requestTracking(`/v1/tasks/${encodeURIComponent(id)}/tracking/stop`, {
        method: 'POST',
        headers: previous.version ? { 'If-Match': String(previous.version) } : undefined
      })
      if (response?.entry) mergeBackendEntry(response.entry)
      _lastError.value = ''
    } catch (error) {
      rollback(id, previous)
      _lastError.value = getApiErrorMessage(error, 'Nao foi possivel parar tracking.')
      console.error(_lastError.value, error)
    } finally {
      const nextPending = { ..._pending.value }
      delete nextPending[id]
      _pending.value = nextPending
    }
  }

  function getElapsedMs(taskId: string): number {
    void _tick.value
    const entry = _entries.value[taskId]
    if (!entry) return 0
    const live = entry.status === 'running' && entry.startedAt !== null
      ? Math.max(0, nowServerMs() - entry.startedAt)
      : 0
    return entry.accumulatedMs + live
  }

  function formatElapsed(ms: number): string {
    const totalSeconds = Math.floor(ms / 1000)
    const h = Math.floor(totalSeconds / 3600)
    const m = Math.floor((totalSeconds % 3600) / 60)
    const s = totalSeconds % 60
    const mm = String(m).padStart(2, '0')
    const ss = String(s).padStart(2, '0')
    if (h > 0) return `${h}:${mm}:${ss}`
    return `${mm}:${ss}`
  }

  function isTracking(taskId: string): boolean {
    return taskId in _entries.value
  }

  function isRunning(taskId: string): boolean {
    return _entries.value[taskId]?.status === 'running'
  }

  if (import.meta.client) {
    void refreshActiveTracking()
  }

  return {
    trackedTaskIds,
    pendingTaskIds,
    lastTrackingError,
    refreshActiveTracking,
    startTracking,
    pauseTracking,
    stopTracking,
    getElapsedMs,
    formatElapsed,
    isTracking,
    isRunning,
  }
}
