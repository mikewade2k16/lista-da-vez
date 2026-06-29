import { onBeforeUnmount, onMounted, ref, watch, type ComputedRef, type Ref } from 'vue'
import { buildRealtimeSocketURL } from '~/composables/useRealtimeConnection'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../core/stores/account'
import { normalizeText } from '../utils/text'

// Maquina de ciclo de vida do WebSocket compartilhada por useTaskPresence e useTasksRealtime.
// Centraliza o que era duplicado nos dois composables: desiredConnection (resolucao de conta +
// montagem da chave), scheduleReconnect (backoff exponencial 1-10s), silencedSockets (isolamento
// por conta — fecha o socket antigo sem disparar reconexao), ensureConnection (handshake via
// ticket + abertura) e updateStatus. As features especificas (heartbeat/draft/snapshot da presence;
// stateless do realtime) ficam fora, injetadas por hooks. Comportamento de reconexao/isolamento
// por conta e' identico ao das implementacoes anteriores.

export type RealtimeSource<T> = T | Ref<T> | ComputedRef<T> | (() => T)

export type RealtimeSocketStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'error'

export function sourceValue<T>(source: RealtimeSource<T> | undefined, fallback: T): T {
  if (typeof source === 'function') {
    const value = (source as () => T)()
    return value == null ? fallback : value
  }

  if (source && typeof source === 'object' && 'value' in source) {
    const value = (source as Ref<T>).value
    return value == null ? fallback : value
  }

  // Apos descartar funcao e Ref/ComputedRef, sobra o valor cru (T); o cast e so
  // para o vue-tsc, ja que o narrowing do union nao estreita o generico aqui.
  return source == null ? fallback : (source as T)
}

export function resolveRealtimeAccountId(
  auth: ReturnType<typeof useAuthStore>,
  accountStore: ReturnType<typeof useCoreAccountStore>,
  explicitAccountId = '',
) {
  // Espelha a fonte do REST (stores/tasks.ts): conta do switcher v2 primeiro, fallback legado
  // depois. Sem accountStore.activeAccountId aqui, um chamador que nao passe o prop cairia em
  // auth.activeTenantId (seed aaaa... pro platform_admin) e o canal de board seria rejeitado (1006).
  return normalizeText(
    explicitAccountId ||
      accountStore.activeAccountId ||
      auth.activeTenantId ||
      auth.principal?.tenantId ||
      auth.tenantContext?.[0]?.id,
    120,
  )
}

// Dados resolvidos da conexao desejada. scope fica como string generica porque cada composable
// tem o proprio union ('task' | 'board' ou 'account' | 'board' | 'task').
export interface RealtimeDesiredConnection {
  key: string
  scope: string
  accountId: string
  boardId: string
  taskId: string
  accessToken: string
}

export interface RealtimeSocketOptions {
  // Fontes reativas comuns: enabled + ids. accountId e' resolvido contra o switcher v2.
  enabled: RealtimeSource<boolean>
  scope: RealtimeSource<string>
  accountId?: RealtimeSource<string>
  boardId?: RealtimeSource<string>
  taskId?: RealtimeSource<string>
  // Path do endpoint WS (ex.: '/v1/realtime/tasks' ou '/v1/realtime/presence').
  path: string
  // Defaults usados pelo sourceValue de cada fonte (ex.: scope 'account' vs 'task').
  scopeDefault: string
  // Normaliza o scope cru no union do composable (realtime usa normalizeScope; presence passa direto).
  normalizeScope: (value: unknown) => string
  // Validacao extra alem de enabled/auth/accountId/accessToken (ex.: board exige boardId).
  isValid: (desired: Omit<RealtimeDesiredConnection, 'key'>) => boolean
  // Lista de getters reativos extras observados pelo watch que dispara ensureConnection
  // (ex.: presence inclui as fontes options.* alem das de auth).
  watchSources: Array<() => unknown>
  // Hooks de ciclo de vida do socket — chamados nos MESMOS pontos das implementacoes originais.
  onOpen?: (socket: WebSocket, desired: RealtimeDesiredConnection) => void
  onMessage?: (payload: Record<string, unknown>, desired: RealtimeDesiredConnection) => void
  // Antes de agendar reconexao no close (ex.: presence limpa o heartbeat especificamente).
  // `isCurrent` reflete `socket === nextSocket` no momento do close (guarda do heartbeat original).
  onSocketClosed?: (socket: WebSocket, event: CloseEvent, isCurrent: boolean) => void
  onError?: (desired: RealtimeDesiredConnection) => void
  // Log do close (cada composable tem mensagem/level proprio).
  logClose?: (desired: RealtimeDesiredConnection, event: CloseEvent) => void
  logTicketError?: (desired: RealtimeDesiredConnection, error: unknown) => void
  // Reset extra dentro do disconnect/ensureConnection (ex.: presence limpa draftTimers/participants).
  onDisconnect?: (clearState: boolean, preserveActiveField: boolean) => void
  // Antes de (re)conectar, depois do disconnect — ex.: presence zera participantsById.
  onBeforeConnect?: (desired: RealtimeDesiredConnection) => void
  // Decide se o disconnect interno do ensureConnection deve preservar estado (presence: mesma key).
  // Realtime sempre faz disconnect "limpo".
  preserveOnReconnect?: (desired: RealtimeDesiredConnection, currentKey: string) => boolean
}

export function useRealtimeSocket(options: RealtimeSocketOptions) {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()

  const status = ref<RealtimeSocketStatus>('idle')

  let socket: WebSocket | null = null
  let socketKey = ''
  let connectionGeneration = 0
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let reconnectAttempts = 0
  const silencedSockets = new WeakSet<WebSocket>()

  function desiredConnection(): RealtimeDesiredConnection | null {
    const enabled = Boolean(sourceValue(options.enabled, false))
    const scope = options.normalizeScope(sourceValue(options.scope, options.scopeDefault))
    const accountId = resolveRealtimeAccountId(
      auth,
      accountStore,
      sourceValue(options.accountId, ''),
    )
    const boardId = normalizeText(sourceValue(options.boardId, ''), 120)
    const taskId = normalizeText(sourceValue(options.taskId, ''), 120)
    const accessToken = normalizeText(auth.accessToken, 2000)

    if (!enabled || !auth.isAuthenticated || !accountId || !accessToken) return null
    if (!options.isValid({ scope, accountId, boardId, taskId, accessToken })) return null

    return {
      key: `${scope}:${accountId}:${boardId}:${taskId}:${accessToken}`,
      scope,
      accountId,
      boardId,
      taskId,
      accessToken,
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

  function disconnect(clearState = true, preserveActiveField = false) {
    connectionGeneration += 1
    clearReconnectTimer()
    options.onDisconnect?.(clearState, preserveActiveField)

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
      void ensureConnection()
    }, delayMs)
    updateStatus()
  }

  async function ensureConnection() {
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

    const preserveActiveField = options.preserveOnReconnect
      ? options.preserveOnReconnect(desired, socketKey)
      : false
    disconnect(false, preserveActiveField)
    options.onBeforeConnect?.(desired)
    socketKey = desired.key
    status.value = 'connecting'
    const requestGeneration = (connectionGeneration += 1)

    let socketURL = ''
    try {
      socketURL = await buildRealtimeSocketURL(
        runtimeConfig,
        options.path,
        {
          scope: desired.scope,
          accountId: desired.accountId,
          boardId: desired.boardId,
          taskId: desired.taskId,
        },
        desired.accessToken,
      )
    } catch (error) {
      if (requestGeneration === connectionGeneration && socketKey === desired.key) {
        socketKey = ''
        status.value = 'error'
      }
      options.logTicketError?.(desired, error)
      return
    }

    const latest = desiredConnection()
    if (
      requestGeneration !== connectionGeneration ||
      !latest ||
      latest.key !== desired.key ||
      socketKey !== desired.key
    ) {
      return
    }

    const nextSocket = new WebSocket(socketURL)
    socket = nextSocket
    updateStatus()

    nextSocket.addEventListener('open', () => {
      if (socket !== nextSocket) return
      reconnectAttempts = 0
      options.onOpen?.(nextSocket, desired)
      updateStatus()
    })

    nextSocket.addEventListener('message', (message) => {
      if (socket !== nextSocket) return
      try {
        const payload = JSON.parse(String(message.data || '{}'))
        if (payload && typeof payload === 'object')
          options.onMessage?.(payload as Record<string, unknown>, desired)
      } catch {
        // Payload invalido nao deve derrubar a tela.
      }
    })

    nextSocket.addEventListener('close', (event) => {
      options.onSocketClosed?.(nextSocket, event, socket === nextSocket)
      if (socket === nextSocket) socket = null
      if (silencedSockets.has(nextSocket)) {
        updateStatus()
        return
      }
      options.logClose?.(desired, event)
      reconnectAttempts += 1
      scheduleReconnect()
    })

    nextSocket.addEventListener('error', () => {
      status.value = 'error'
      options.onError?.(desired)
    })
  }

  // Acesso ao socket vivo para os hooks que precisam enviar mensagens (heartbeat/draft da presence).
  function currentSocket() {
    return socket
  }

  onMounted(() => {
    watch(options.watchSources, () => void ensureConnection(), { immediate: true })
  })

  onBeforeUnmount(() => {
    disconnect()
  })

  return {
    status,
    desiredConnection,
    ensureConnection,
    disconnect,
    updateStatus,
    currentSocket,
  }
}
