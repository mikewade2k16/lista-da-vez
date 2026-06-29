import { computed, onBeforeUnmount, onMounted, ref, watch, type ComputedRef, type Ref } from 'vue'
import { buildRealtimeSocketURL } from '~/composables/useRealtimeConnection'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../core/stores/account'

type PresenceSource<T> = T | Ref<T> | ComputedRef<T> | (() => T)

export type TaskPresenceStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'error'

export interface TaskPresenceUser {
  userId: string
  displayName: string
  avatarPath: string
  fieldKey: string
  lockId: string
  draftValue: string
  hasDraftValue: boolean
  updatedAt: string
  avatarText: string
}

interface TaskPresenceOptions {
  enabled: PresenceSource<boolean>
  scope?: PresenceSource<'task' | 'board'>
  taskId?: PresenceSource<string>
  boardId?: PresenceSource<string>
  accountId?: PresenceSource<string>
}

function sourceValue<T>(source: PresenceSource<T> | undefined, fallback: T): T {
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
  return String(value ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, max)
}

function initialsFor(value: string) {
  const words = normalizeText(value, 80).split(' ').filter(Boolean)
  if (!words.length) return 'U'
  return (
    words
      .slice(0, 2)
      .map((word) => word[0]?.toUpperCase() || '')
      .join('') || 'U'
  )
}

function normalizeFieldKey(value: unknown) {
  return normalizeText(value, 80)
}

function normalizeDraftValue(value: unknown, max = 24000) {
  // eslint-disable-next-line no-control-regex
  const text = String(value ?? '').replace(/\u0000/g, '')
  return text.length <= max ? text : text.slice(0, max)
}

function rawHasDraftValue(raw: Record<string, unknown>) {
  return (
    Object.prototype.hasOwnProperty.call(raw, 'draftValue') ||
    Object.prototype.hasOwnProperty.call(raw, 'fieldValue') ||
    Object.prototype.hasOwnProperty.call(raw, 'value')
  )
}

function presenceLog(
  level: 'info' | 'warn' | 'error',
  message: string,
  payload?: Record<string, unknown>,
) {
  if (!import.meta.client) return
  const logger = level === 'error' ? console.error : level === 'warn' ? console.warn : console.info
  logger(`[tasks-presence] ${message}`, payload || '')
}

function normalizePresenceUser(raw: Record<string, unknown>): TaskPresenceUser {
  const userId = normalizeText(raw.userId ?? raw.userID ?? raw.id, 120)
  const displayName = normalizeText(raw.displayName ?? raw.name ?? raw.email, 120) || 'Usuario'
  const avatarPath = normalizeText(raw.avatarPath ?? raw.avatarUrl ?? raw.avatarURL, 500)
  const fieldKey = normalizeFieldKey(raw.fieldKey)
  const lockId = normalizeText(raw.lockId ?? raw.lockID, 180)
  const hasDraftValue = rawHasDraftValue(raw)
  const draftValue = hasDraftValue
    ? normalizeDraftValue(raw.draftValue ?? raw.fieldValue ?? raw.value)
    : ''
  const updatedAt = normalizeText(raw.updatedAt ?? raw.savedAt, 80)

  return {
    userId,
    displayName,
    avatarPath,
    fieldKey,
    lockId,
    draftValue,
    hasDraftValue,
    updatedAt,
    avatarText: initialsFor(displayName),
  }
}

function resolveAccountId(
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

function resolveCurrentUserId(auth: ReturnType<typeof useAuthStore>) {
  return normalizeText(
    auth.principal?.userId ||
      auth.principal?.userID ||
      auth.user?.id ||
      auth.user?.userId ||
      auth.user?.userID ||
      auth.user?.email,
    160,
  )
}

export function useTaskPresence(options: TaskPresenceOptions) {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()

  const status = ref<TaskPresenceStatus>('idle')
  const lastEvent = ref<Record<string, unknown> | null>(null)
  const participantsById = ref<Record<string, TaskPresenceUser>>({})
  const activeFieldKey = ref('')
  const localFieldDrafts = ref<Record<string, string>>({})

  let socket: WebSocket | null = null
  let socketKey = ''
  let connectionGeneration = 0
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let heartbeatTimer: ReturnType<typeof setInterval> | null = null
  const draftTimers = new Map<string, ReturnType<typeof setTimeout>>()
  let reconnectAttempts = 0
  const silencedSockets = new WeakSet<WebSocket>()

  const currentUserId = computed(() => resolveCurrentUserId(auth))
  const participants = computed(() =>
    Object.values(participantsById.value)
      .filter((user) => user.userId && user.userId !== currentUserId.value)
      .sort((a, b) => a.displayName.localeCompare(b.displayName)),
  )

  function desiredConnection() {
    const enabled = Boolean(sourceValue(options.enabled, false))
    const scope = sourceValue(options.scope, 'task')
    const taskId = normalizeText(sourceValue(options.taskId, ''), 120)
    const boardId = normalizeText(sourceValue(options.boardId, ''), 120)
    const accountId = resolveAccountId(auth, accountStore, sourceValue(options.accountId, ''))
    const accessToken = normalizeText(auth.accessToken, 2000)

    if (!enabled || !auth.isAuthenticated || !accountId || !accessToken) return null
    if (scope === 'task' && !taskId) return null
    if (scope === 'board' && !boardId) return null

    return {
      key: `${scope}:${accountId}:${boardId}:${taskId}:${accessToken}`,
      scope,
      accountId,
      boardId,
      taskId,
      accessToken,
    }
  }

  function replaceParticipant(user: TaskPresenceUser) {
    if (!user.userId) return
    participantsById.value = { ...participantsById.value, [user.userId]: user }
  }

  function removeParticipant(userId: string) {
    if (!userId) return
    const next = { ...participantsById.value }
    delete next[userId]
    participantsById.value = next
  }

  function clearTimers() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (heartbeatTimer) {
      clearInterval(heartbeatTimer)
      heartbeatTimer = null
    }
  }

  function clearDraftTimer(fieldKey: string) {
    const key = normalizeFieldKey(fieldKey)
    if (!key) return
    const timer = draftTimers.get(key)
    if (!timer) return
    clearTimeout(timer)
    draftTimers.delete(key)
  }

  function clearDraftTimers() {
    draftTimers.forEach((timer) => clearTimeout(timer))
    draftTimers.clear()
  }

  function setLocalFieldDraft(fieldKey: string, draftValue: string | null) {
    const key = normalizeFieldKey(fieldKey)
    if (!key) return
    const next = { ...localFieldDrafts.value }
    if (draftValue == null) delete next[key]
    else next[key] = draftValue
    localFieldDrafts.value = next
  }

  function send(payload: Record<string, unknown>) {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      presenceLog('warn', 'envio ignorado; socket fechado', {
        type: normalizeText(payload.type, 80),
        fieldKey: normalizeFieldKey(payload.fieldKey),
      })
      return false
    }
    socket.send(JSON.stringify(payload))
    return true
  }

  function sendHeartbeat() {
    send({ type: 'presence.heartbeat' })
  }

  function sendFieldFocus(fieldKey: string) {
    const key = normalizeFieldKey(fieldKey)
    if (!key) return false
    const lockId = `${currentUserId.value || 'user'}:${key}:${Date.now()}`
    return send({ type: 'presence.field_focus', fieldKey: key, lockId })
  }

  function sendFieldDraft(fieldKey: string, value: unknown) {
    const key = normalizeFieldKey(fieldKey)
    if (!key || activeFieldKey.value !== key) return false
    return send({
      type: 'presence.field_draft',
      fieldKey: key,
      draftValue: normalizeDraftValue(value),
    })
  }

  function startHeartbeat() {
    if (heartbeatTimer) clearInterval(heartbeatTimer)
    sendHeartbeat()
    heartbeatTimer = setInterval(sendHeartbeat, 15000)
  }

  function applyEvent(payload: Record<string, unknown>) {
    const eventType = normalizeText(payload.type, 80)
    lastEvent.value = payload

    if (eventType === 'presence.snapshot') {
      const next: Record<string, TaskPresenceUser> = {}
      const rawParticipants = Array.isArray(payload.participants) ? payload.participants : []
      rawParticipants.forEach((participant) => {
        if (!participant || typeof participant !== 'object') return
        const user = normalizePresenceUser(participant as Record<string, unknown>)
        if (user.userId) next[user.userId] = user
      })
      participantsById.value = next
      presenceLog('info', 'snapshot recebido', {
        participants: Object.keys(next).length,
      })
      return
    }

    if (eventType === 'presence.user_left') {
      const userId = normalizeText(payload.userId ?? payload.userID, 120)
      removeParticipant(userId)
      presenceLog('info', 'usuario saiu', { userId })
      return
    }

    if (eventType === 'presence.user_joined' || eventType === 'presence.field_locked') {
      const user = normalizePresenceUser(payload)
      const existing = user.userId ? participantsById.value[user.userId] : null
      replaceParticipant({ ...(existing || user), ...user, hasDraftValue: user.hasDraftValue })
      presenceLog(
        'info',
        eventType === 'presence.field_locked' ? 'campo travado' : 'usuario entrou',
        {
          userId: user.userId,
          displayName: user.displayName,
          fieldKey: user.fieldKey,
        },
      )
      return
    }

    if (eventType === 'presence.field_draft') {
      const user = normalizePresenceUser(payload)
      const existing = user.userId ? participantsById.value[user.userId] : null
      replaceParticipant({ ...(existing || user), ...user, hasDraftValue: user.hasDraftValue })
      return
    }

    if (eventType === 'presence.field_unlocked') {
      const user = normalizePresenceUser(payload)
      const existing = user.userId ? participantsById.value[user.userId] : null
      if (!existing) return
      replaceParticipant({
        ...existing,
        displayName: user.displayName || existing.displayName,
        avatarPath: user.avatarPath || existing.avatarPath,
        fieldKey: '',
        lockId: '',
        draftValue: '',
        hasDraftValue: false,
        updatedAt: user.updatedAt || existing.updatedAt,
      })
      presenceLog('info', 'campo liberado', {
        userId: user.userId,
        displayName: user.displayName,
        fieldKey: user.fieldKey,
      })
    }
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

  function disconnect(clearParticipants = true, preserveActiveField = false) {
    connectionGeneration += 1
    clearTimers()
    clearDraftTimers()

    if (socket) {
      silencedSockets.add(socket)
      socket.close()
      socket = null
    }

    socketKey = ''
    reconnectAttempts = 0
    if (!preserveActiveField) activeFieldKey.value = ''
    if (!preserveActiveField) localFieldDrafts.value = {}
    if (clearParticipants) participantsById.value = {}
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

    const preserveActiveField = socketKey === desired.key
    disconnect(false, preserveActiveField)
    participantsById.value = {}
    socketKey = desired.key
    status.value = 'connecting'
    const requestGeneration = (connectionGeneration += 1)

    let socketURL = ''
    try {
      socketURL = await buildRealtimeSocketURL(
        runtimeConfig,
        '/v1/realtime/presence',
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
      presenceLog('error', 'ticket ERROR; socket nao aberto', {
        scope: desired.scope,
        accountId: desired.accountId,
        boardId: desired.boardId,
        taskId: desired.taskId,
        error,
      })
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
      startHeartbeat()
      if (activeFieldKey.value) {
        sendFieldFocus(activeFieldKey.value)
        const localDraft = localFieldDrafts.value[activeFieldKey.value]
        if (localDraft !== undefined) sendFieldDraft(activeFieldKey.value, localDraft)
      }
      updateStatus()
      presenceLog('info', 'socket OPEN', {
        scope: desired.scope,
        accountId: desired.accountId,
        boardId: desired.boardId,
        taskId: desired.taskId,
      })
    })

    nextSocket.addEventListener('message', (message) => {
      if (socket !== nextSocket) return
      try {
        const payload = JSON.parse(String(message.data || '{}'))
        if (payload && typeof payload === 'object') applyEvent(payload as Record<string, unknown>)
      } catch {
        // Payload invalido nao deve derrubar a tela de tasks.
      }
    })

    nextSocket.addEventListener('close', () => {
      if (socket === nextSocket && heartbeatTimer) {
        clearInterval(heartbeatTimer)
        heartbeatTimer = null
      }

      if (socket === nextSocket) socket = null
      if (silencedSockets.has(nextSocket)) {
        updateStatus()
        return
      }

      presenceLog('warn', 'socket CLOSED; agendando reconexao', {
        scope: desired.scope,
        accountId: desired.accountId,
        boardId: desired.boardId,
        taskId: desired.taskId,
      })
      reconnectAttempts += 1
      scheduleReconnect()
    })

    nextSocket.addEventListener('error', () => {
      status.value = 'error'
      presenceLog('error', 'socket ERROR', {
        scope: desired.scope,
        accountId: desired.accountId,
        boardId: desired.boardId,
        taskId: desired.taskId,
      })
    })
  }

  function focusField(fieldKey: string) {
    const key = normalizeFieldKey(fieldKey)
    if (!key) return
    if (activeFieldKey.value && activeFieldKey.value !== key) blurField(activeFieldKey.value)
    activeFieldKey.value = key
    if (sendFieldFocus(key)) {
      presenceLog('info', 'field_focus enviado', { fieldKey: key })
    }
  }

  function blurField(fieldKey = activeFieldKey.value) {
    const key = normalizeFieldKey(fieldKey)
    if (!key) return
    clearDraftTimer(key)
    setLocalFieldDraft(key, null)
    if (send({ type: 'presence.field_blur', fieldKey: key })) {
      presenceLog('info', 'field_blur enviado', { fieldKey: key })
    }
    if (activeFieldKey.value === key) activeFieldKey.value = ''
  }

  function scheduleFieldDraft(fieldKey: string, value: unknown, delayMs = 220) {
    const key = normalizeFieldKey(fieldKey)
    if (!key || activeFieldKey.value !== key) return
    const draftValue = normalizeDraftValue(value)
    setLocalFieldDraft(key, draftValue)
    clearDraftTimer(key)

    const sendDraft = () => {
      draftTimers.delete(key)
      sendFieldDraft(key, draftValue)
    }

    if (delayMs <= 0) {
      sendDraft()
      return
    }

    draftTimers.set(key, setTimeout(sendDraft, delayMs))
  }

  function releaseActiveField() {
    if (activeFieldKey.value) blurField(activeFieldKey.value)
  }

  function handleVisibilityChange() {
    if (document.visibilityState === 'hidden') releaseActiveField()
  }

  function usersForField(fieldKey: string) {
    const key = normalizeFieldKey(fieldKey)
    if (!key) return []
    return participants.value.filter((user) => user.fieldKey === key)
  }

  function fieldLabel(fieldKey: string) {
    const users = usersForField(fieldKey)
    if (!users.length) return ''
    if (users.length === 1) return `${users[0]!.displayName} editando`
    return `${users[0]!.displayName} +${users.length - 1} editando`
  }

  function draftValueForField(fieldKey: string) {
    const user = usersForField(fieldKey)[0]
    return user?.hasDraftValue ? user.draftValue : null
  }

  onMounted(() => {
    document.addEventListener('visibilitychange', handleVisibilityChange)
    window.addEventListener('pagehide', releaseActiveField)

    watch(
      [
        () => sourceValue(options.enabled, false),
        () => sourceValue(options.scope, 'task'),
        () => sourceValue(options.taskId, ''),
        () => sourceValue(options.boardId, ''),
        () => sourceValue(options.accountId, ''),
        () => auth.isAuthenticated,
        () => auth.accessToken,
        () => auth.activeTenantId,
        () => auth.principal?.tenantId,
      ],
      () => void ensureConnection(),
      { immediate: true },
    )
  })

  onBeforeUnmount(() => {
    document.removeEventListener('visibilitychange', handleVisibilityChange)
    window.removeEventListener('pagehide', releaseActiveField)
    disconnect()
  })

  return {
    status,
    lastEvent,
    participants,
    activeFieldKey,
    focusField,
    blurField,
    scheduleFieldDraft,
    usersForField,
    fieldLabel,
    draftValueForField,
    disconnect,
  }
}
