import { computed, onBeforeUnmount, onMounted, ref, watch, type ComputedRef, type Ref } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { getWebSocketBase } from '~/utils/api-client'

type RealtimeSource<T> = T | Ref<T> | ComputedRef<T> | (() => T)

export type TasksRealtimeStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'error'
export type TasksRealtimeScope = 'account' | 'board' | 'task'

export interface TasksRealtimeEvent {
  type?: string
  accountId?: string
  boardId?: string
  taskId?: string
  version?: number
  savedAt?: string
  [key: string]: unknown
}

interface TasksRealtimeOptions {
  enabled: RealtimeSource<boolean>
  scope?: RealtimeSource<TasksRealtimeScope>
  accountId?: RealtimeSource<string>
  boardId?: RealtimeSource<string>
  taskId?: RealtimeSource<string>
  onEvent?: (event: TasksRealtimeEvent) => void
}

function sourceValue<T>(source: RealtimeSource<T> | undefined, fallback: T): T {
  if (typeof source === 'function') {
    const value = (source as () => T)()
    return value == null ? fallback : value
  }

  if (source && typeof source === 'object' && 'value' in source) {
    const value = (source as Ref<T>).value
    return value == null ? fallback : value
  }

  return source == null ? fallback : source
}

function normalizeText(value: unknown, max = 240) {
  return String(value ?? '').replace(/\s+/g, ' ').trim().slice(0, max)
}

function normalizeScope(value: unknown): TasksRealtimeScope {
  const scope = normalizeText(value, 20)
  if (scope === 'board' || scope === 'task') return scope
  return 'account'
}

function resolveAccountId(auth: ReturnType<typeof useAuthStore>, explicitAccountId = '') {
  return normalizeText(
    explicitAccountId ||
      auth.activeTenantId ||
      auth.principal?.tenantId ||
      auth.tenantContext?.[0]?.id,
    120
  )
}

function buildSocketURL(runtimeConfig: ReturnType<typeof useRuntimeConfig>, params: {
  scope: TasksRealtimeScope
  accountId: string
  boardId: string
  taskId: string
  accessToken: string
}) {
  const url = new URL('/v1/realtime/tasks', getWebSocketBase(runtimeConfig))
  url.searchParams.set('scope', params.scope)
  url.searchParams.set('accountId', params.accountId)
  if (params.boardId) url.searchParams.set('boardId', params.boardId)
  if (params.taskId) url.searchParams.set('taskId', params.taskId)
  url.searchParams.set('access_token', params.accessToken)
  return url.toString()
}

export function useTasksRealtime(options: TasksRealtimeOptions) {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()

  const status = ref<TasksRealtimeStatus>('idle')
  const lastEvent = ref<TasksRealtimeEvent | null>(null)

  let socket: WebSocket | null = null
  let socketKey = ''
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let reconnectAttempts = 0
  const silencedSockets = new WeakSet<WebSocket>()

  const isConnected = computed(() => status.value === 'connected')

  function desiredConnection() {
    const enabled = Boolean(sourceValue(options.enabled, false))
    const scope = normalizeScope(sourceValue(options.scope, 'account'))
    const accountId = resolveAccountId(auth, sourceValue(options.accountId, ''))
    const boardId = normalizeText(sourceValue(options.boardId, ''), 120)
    const taskId = normalizeText(sourceValue(options.taskId, ''), 120)
    const accessToken = normalizeText(auth.accessToken, 2000)

    if (!enabled || !auth.isAuthenticated || !accountId || !accessToken) return null
    if (scope === 'board' && !boardId) return null
    if (scope === 'task' && !taskId) return null

    return {
      key: `${scope}:${accountId}:${boardId}:${taskId}:${accessToken}`,
      scope,
      accountId,
      boardId,
      taskId,
      accessToken
    }
  }

  function clearReconnectTimer() {
    if (!reconnectTimer) return
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }

  function updateStatus() {
    if (!socket) {
      status.value = reconnectTimer ? 'reconnecting' : 'idle'
      return
    }
    if (socket.readyState === WebSocket.OPEN) {
      status.value = 'connected'
      return
    }
    if (socket.readyState === WebSocket.CONNECTING) {
      status.value = 'connecting'
      return
    }
    status.value = reconnectTimer ? 'reconnecting' : 'error'
  }

  function disconnect() {
    clearReconnectTimer()
    if (socket) {
      silencedSockets.add(socket)
      socket.close()
      socket = null
    }
    socketKey = ''
    reconnectAttempts = 0
    updateStatus()
  }

  function scheduleReconnect() {
    if (reconnectTimer || !desiredConnection()) {
      updateStatus()
      return
    }

    const delayMs = Math.min(10000, 1000 * Math.max(1, 2 ** reconnectAttempts))
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      ensureConnection()
    }, delayMs)
    updateStatus()
  }

  function applyEvent(payload: TasksRealtimeEvent) {
    lastEvent.value = payload
    options.onEvent?.(payload)
  }

  function ensureConnection() {
    if (import.meta.server) return

    const desired = desiredConnection()
    if (!desired) {
      disconnect()
      return
    }

    if (socket && socketKey === desired.key && socket.readyState <= WebSocket.OPEN) {
      updateStatus()
      return
    }

    disconnect()
    socketKey = desired.key
    const nextSocket = new WebSocket(buildSocketURL(runtimeConfig, desired))
    socket = nextSocket
    updateStatus()

    nextSocket.addEventListener('open', () => {
      if (socket !== nextSocket) return
      reconnectAttempts = 0
      updateStatus()
      if (import.meta.client) {
        console.info('[tasks-ws] socket OPEN', {
          scope: desired.scope,
          accountId: desired.accountId,
          boardId: desired.boardId || undefined,
          taskId: desired.taskId || undefined
        })
      }
    })

    nextSocket.addEventListener('message', (message) => {
      if (socket !== nextSocket) return
      try {
        const payload = JSON.parse(String(message.data || '{}'))
        if (payload && typeof payload === 'object') applyEvent(payload as TasksRealtimeEvent)
      } catch {
        // Evento realtime invalido nao deve derrubar a tela.
      }
    })

    nextSocket.addEventListener('close', (event) => {
      if (socket === nextSocket) socket = null
      if (silencedSockets.has(nextSocket)) {
        updateStatus()
        return
      }
      if (import.meta.client) {
        console.warn('[tasks-ws] socket CLOSED — agendando reconexao', {
          code: event.code,
          reason: event.reason,
          wasClean: event.wasClean
        })
      }
      reconnectAttempts += 1
      scheduleReconnect()
    })

    nextSocket.addEventListener('error', () => {
      status.value = 'error'
      if (import.meta.client) console.error('[tasks-ws] socket ERROR')
    })
  }

  onMounted(() => {
    watch(
      [
        () => sourceValue(options.enabled, false),
        () => sourceValue(options.scope, 'account'),
        () => sourceValue(options.accountId, ''),
        () => sourceValue(options.boardId, ''),
        () => sourceValue(options.taskId, ''),
        () => auth.isAuthenticated,
        () => auth.accessToken,
        () => auth.activeTenantId,
        () => auth.principal?.tenantId
      ],
      () => ensureConnection(),
      { immediate: true }
    )
  })

  onBeforeUnmount(() => {
    disconnect()
  })

  return {
    status,
    isConnected,
    lastEvent,
    disconnect
  }
}
