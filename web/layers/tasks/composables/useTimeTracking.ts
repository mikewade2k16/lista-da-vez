import { ref, computed } from 'vue'

type TrackingStatus = 'running' | 'paused'

interface TrackingEntry {
  status: TrackingStatus
  startedAt: number | null
  accumulatedMs: number
}

const STORAGE_KEY = 'tasks-tracking-v1'

function loadFromStorage(): Record<string, TrackingEntry> {
  if (typeof localStorage === 'undefined') return {}
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return {}
    const parsed: Record<string, TrackingEntry> = JSON.parse(raw)
    const now = Date.now()
    for (const entry of Object.values(parsed)) {
      if (entry.status === 'running' && entry.startedAt !== null) {
        // Absorb time elapsed while the page was closed
        entry.accumulatedMs += now - entry.startedAt
        entry.startedAt = now
      }
    }
    return parsed
  } catch {
    return {}
  }
}

function saveToStorage(entries: Record<string, TrackingEntry>) {
  if (typeof localStorage === 'undefined') return
  localStorage.setItem(STORAGE_KEY, JSON.stringify(entries))
}

const _entries = ref<Record<string, TrackingEntry>>(loadFromStorage())
const _tick = ref(0)

if (typeof window !== 'undefined') {
  setInterval(() => { _tick.value++ }, 1000)
}

export function useTimeTracking() {
  const trackedTaskIds = computed(() => Object.keys(_entries.value))

  function startTracking(taskId: string) {
    const existing = _entries.value[taskId]
    _entries.value = {
      ..._entries.value,
      [taskId]: {
        status: 'running',
        startedAt: Date.now(),
        accumulatedMs: existing?.accumulatedMs ?? 0,
      },
    }
    saveToStorage(_entries.value)
  }

  function pauseTracking(taskId: string) {
    const entry = _entries.value[taskId]
    if (!entry) return
    const elapsed = entry.startedAt !== null ? Date.now() - entry.startedAt : 0
    _entries.value = {
      ..._entries.value,
      [taskId]: {
        status: 'paused',
        startedAt: null,
        accumulatedMs: entry.accumulatedMs + elapsed,
      },
    }
    saveToStorage(_entries.value)
  }

  function stopTracking(taskId: string) {
    const updated = { ..._entries.value }
    delete updated[taskId]
    _entries.value = updated
    saveToStorage(_entries.value)
  }

  function getElapsedMs(taskId: string): number {
    void _tick.value // reactive dependency so it updates every second
    const entry = _entries.value[taskId]
    if (!entry) return 0
    const live = entry.status === 'running' && entry.startedAt !== null
      ? Date.now() - entry.startedAt
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

  return {
    trackedTaskIds,
    startTracking,
    pauseTracking,
    stopTracking,
    getElapsedMs,
    formatElapsed,
    isTracking,
    isRunning,
  }
}
