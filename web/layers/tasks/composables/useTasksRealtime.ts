import { computed, ref, type ComputedRef, type Ref } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { normalizeText } from '../utils/text'
import { sourceValue, useRealtimeSocket } from './useRealtimeSocket'

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

function normalizeScope(value: unknown): TasksRealtimeScope {
  const scope = normalizeText(value, 20)
  if (scope === 'board' || scope === 'task') return scope
  return 'account'
}

export function useTasksRealtime(options: TasksRealtimeOptions) {
  const auth = useAuthStore()

  const lastEvent = ref<TasksRealtimeEvent | null>(null)

  function applyEvent(payload: TasksRealtimeEvent) {
    lastEvent.value = payload
    options.onEvent?.(payload)
  }

  // Canal stateless: so encaminha eventos parseados ao consumidor (sem heartbeat/draft/snapshot).
  // A maquina de ciclo de vida (desiredConnection/scheduleReconnect/silencedSockets/ensureConnection)
  // vive em useRealtimeSocket; aqui ficam apenas o scope/validacao especificos e o handler.
  const socket = useRealtimeSocket({
    enabled: options.enabled,
    scope: options.scope ?? 'account',
    accountId: options.accountId,
    boardId: options.boardId,
    taskId: options.taskId,
    path: '/v1/realtime/tasks',
    scopeDefault: 'account',
    normalizeScope,
    isValid: ({ scope, boardId, taskId }) => {
      if (scope === 'board' && !boardId) return false
      if (scope === 'task' && !taskId) return false
      return true
    },
    watchSources: [
      () => sourceValue(options.enabled, false),
      () => sourceValue(options.scope, 'account'),
      () => sourceValue(options.accountId, ''),
      () => sourceValue(options.boardId, ''),
      () => sourceValue(options.taskId, ''),
      () => auth.isAuthenticated,
      () => auth.accessToken,
      () => auth.activeTenantId,
      () => auth.principal?.tenantId,
    ],
    onOpen: (_socket, desired) => {
      if (import.meta.client) {
        console.info('[tasks-ws] socket OPEN', {
          scope: desired.scope,
          accountId: desired.accountId,
          boardId: desired.boardId || undefined,
          taskId: desired.taskId || undefined,
        })
      }
    },
    onMessage: (payload) => applyEvent(payload as TasksRealtimeEvent),
    logTicketError: (_desired, error) => {
      if (import.meta.client) console.error('[tasks-ws] ticket ERROR', error)
    },
    logClose: (_desired, event) => {
      if (import.meta.client) {
        console.warn('[tasks-ws] socket CLOSED — agendando reconexao', {
          code: event.code,
          reason: event.reason,
          wasClean: event.wasClean,
        })
      }
    },
    onError: () => {
      if (import.meta.client) console.error('[tasks-ws] socket ERROR')
    },
  })

  const status = socket.status as Ref<TasksRealtimeStatus>
  const isConnected = computed(() => status.value === 'connected')

  return {
    status,
    isConnected,
    lastEvent,
    disconnect: socket.disconnect,
  }
}
