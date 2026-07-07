import { computed, onBeforeUnmount, onMounted, ref, type ComputedRef, type Ref } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import { normalizeText } from '../../layers/tasks/utils/text'
import { sourceValue, useRealtimeSocket } from '../../layers/tasks/composables/useRealtimeSocket'

// Presenca do calendario (contrato C11), versao REDUZIDA da presenca de tasks
// (useTaskPresence): so INDICADOR (quem esta na tela + quem edita qual campo), sem lock
// exclusivo e sem sync de draft na v1. Topico presence:calendar:{accountId} via
// /v1/realtime/presence (scope=calendar). fieldKey: "notes:YYYY-MM" (editor de notas do
// mes) e "event:<id>" (form de edicao de evento aberto).

type PresenceSource<T> = T | Ref<T> | ComputedRef<T> | (() => T)

export type CalendarPresenceStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'error'

export interface CalendarPresenceUser {
  userId: string
  displayName: string
  avatarPath: string
  avatarText: string
  fieldKey: string
}

interface CalendarPresenceOptions {
  enabled: PresenceSource<boolean>
  accountId?: PresenceSource<string>
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

function normalizePresenceUser(raw: Record<string, unknown>): CalendarPresenceUser {
  const displayName = normalizeText(raw.displayName ?? raw.name ?? raw.email, 120) || 'Usuario'
  return {
    userId: normalizeText(raw.userId ?? raw.userID ?? raw.id, 120),
    displayName,
    avatarPath: normalizeText(raw.avatarPath ?? raw.avatarUrl ?? raw.avatarURL, 500),
    avatarText: initialsFor(displayName),
    fieldKey: normalizeFieldKey(raw.fieldKey),
  }
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

export function useCalendarPresence(options: CalendarPresenceOptions) {
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()

  const participantsById = ref<Record<string, CalendarPresenceUser>>({})
  const activeFieldKey = ref('')

  // Heartbeat 15s (mantem o usuario "vivo" na presenca). O ciclo de vida do socket
  // (reconnect/isolamento por conta) fica na base useRealtimeSocket.
  let heartbeatTimer: ReturnType<typeof setInterval> | null = null

  const currentUserId = computed(() => resolveCurrentUserId(auth))
  const participants = computed(() =>
    Object.values(participantsById.value)
      .filter((user) => user.userId && user.userId !== currentUserId.value)
      .sort((a, b) => a.displayName.localeCompare(b.displayName)),
  )

  function replaceParticipant(user: CalendarPresenceUser) {
    if (!user.userId) return
    participantsById.value = { ...participantsById.value, [user.userId]: user }
  }

  function removeParticipant(userId: string) {
    if (!userId) return
    const next = { ...participantsById.value }
    delete next[userId]
    participantsById.value = next
  }

  function clearHeartbeatTimer() {
    if (!heartbeatTimer) return
    clearInterval(heartbeatTimer)
    heartbeatTimer = null
  }

  function send(payload: Record<string, unknown>) {
    const socket = presenceSocket.currentSocket()
    if (!socket || socket.readyState !== WebSocket.OPEN) return false
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

  function startHeartbeat() {
    clearHeartbeatTimer()
    sendHeartbeat()
    heartbeatTimer = setInterval(sendHeartbeat, 15000)
  }

  function applyEvent(payload: Record<string, unknown>) {
    const eventType = normalizeText(payload.type, 80)

    if (eventType === 'presence.snapshot') {
      const next: Record<string, CalendarPresenceUser> = {}
      const rawParticipants = Array.isArray(payload.participants) ? payload.participants : []
      rawParticipants.forEach((participant) => {
        if (!participant || typeof participant !== 'object') return
        const user = normalizePresenceUser(participant as Record<string, unknown>)
        if (user.userId) next[user.userId] = user
      })
      participantsById.value = next
      return
    }

    if (eventType === 'presence.user_left') {
      removeParticipant(normalizeText(payload.userId ?? payload.userID, 120))
      return
    }

    if (eventType === 'presence.user_joined' || eventType === 'presence.field_locked') {
      const user = normalizePresenceUser(payload)
      const existing = user.userId ? participantsById.value[user.userId] : null
      replaceParticipant({ ...(existing || user), ...user })
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
      })
    }
  }

  const presenceSocket = useRealtimeSocket({
    enabled: options.enabled,
    scope: 'calendar',
    accountId: options.accountId,
    path: '/v1/realtime/presence',
    scopeDefault: 'calendar',
    normalizeScope: () => 'calendar',
    isValid: ({ accountId }) => Boolean(accountId),
    watchSources: [
      () => sourceValue(options.enabled, false),
      () => sourceValue(options.accountId, ''),
      () => auth.isAuthenticated,
      () => auth.accessToken,
      () => auth.activeTenantId,
      () => accountStore.activeAccountId,
    ],
    // Limpa heartbeat; preserva o campo ativo no reconnect com a mesma key; so zera
    // participantes quando o disconnect e' "limpo" (troca de conta / unmount).
    onDisconnect: (clearParticipants, preserveActiveField) => {
      clearHeartbeatTimer()
      if (!preserveActiveField) activeFieldKey.value = ''
      if (clearParticipants) participantsById.value = {}
    },
    preserveOnReconnect: (desired, currentKey) => currentKey === desired.key,
    onBeforeConnect: () => {
      participantsById.value = {}
    },
    onOpen: () => {
      startHeartbeat()
      if (activeFieldKey.value) sendFieldFocus(activeFieldKey.value)
    },
    onMessage: (payload) => applyEvent(payload),
    onSocketClosed: (_socket, _event, isCurrent) => {
      if (isCurrent) clearHeartbeatTimer()
    },
  })

  const status = presenceSocket.status as Ref<CalendarPresenceStatus>

  function focusField(fieldKey: string) {
    const key = normalizeFieldKey(fieldKey)
    if (!key) return
    if (activeFieldKey.value && activeFieldKey.value !== key) blurField(activeFieldKey.value)
    activeFieldKey.value = key
    sendFieldFocus(key)
  }

  function blurField(fieldKey = activeFieldKey.value) {
    const key = normalizeFieldKey(fieldKey)
    if (!key) return
    send({ type: 'presence.field_blur', fieldKey: key })
    if (activeFieldKey.value === key) activeFieldKey.value = ''
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

  // O watch que dispara ensureConnection e o disconnect no unmount ficam na base. Aqui
  // so os listeners que liberam o campo ativo ao sair/esconder a aba.
  onMounted(() => {
    document.addEventListener('visibilitychange', handleVisibilityChange)
    window.addEventListener('pagehide', releaseActiveField)
  })

  onBeforeUnmount(() => {
    document.removeEventListener('visibilitychange', handleVisibilityChange)
    window.removeEventListener('pagehide', releaseActiveField)
  })

  return {
    status,
    participants,
    activeFieldKey,
    focusField,
    blurField,
    usersForField,
    fieldLabel,
    disconnect: presenceSocket.disconnect,
  }
}
