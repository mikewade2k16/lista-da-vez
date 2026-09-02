import { ref, type Ref } from 'vue'
import {
  parseRealtimeInvalidationEnvelope,
  useOmnichannelScopeInvalidation,
  type ParsedRealtimeInvalidation,
} from '~/composables/omnichannel/useOmnichannelScopeInvalidation'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../../layers/core/stores/account'
import {
  resolveRealtimeAccountId,
  useRealtimeSocket,
} from '../../../layers/tasks/composables/useRealtimeSocket'

export type RealtimeConnectionState =
  | 'disconnected'
  | 'connecting'
  | 'connected'
  | 'module_denied'
  | 'auth_error'

interface AuthorizedBootstrapOptions {
  invalidateInFlight?: boolean
  clearAll?: boolean
}

interface RealtimeCoordinatorDependencies {
  currentAccountId: () => string
  publish: (event: ParsedRealtimeInvalidation) => boolean
  bootstrap: (options: AuthorizedBootstrapOptions) => Promise<void>
}

export function createOmnichannelRealtimeCoordinator(
  dependencies: RealtimeCoordinatorDependencies,
) {
  let hasOpened = false

  function handleEnvelope(envelope: unknown): boolean {
    const event = parseRealtimeInvalidationEnvelope(envelope)
    if (!event || event.accountId !== dependencies.currentAccountId()) return false
    return dependencies.publish(event)
  }

  async function handleOpen(): Promise<boolean> {
    const isReconnect = hasOpened
    hasOpened = true
    if (!isReconnect) return false

    await dependencies.bootstrap({
      invalidateInFlight: true,
      clearAll: true,
    })
    return true
  }

  function resetLifecycle(): void {
    hasOpened = false
  }

  return {
    handleEnvelope,
    handleOpen,
    resetLifecycle,
  }
}

// O WS por account entrega somente `omnichannel.invalidate`. A resposta autoritativa
// sempre volta do REST; nenhum payload de conversa ou mensagem e aplicado localmente.
export function useOmnichannelInboxRealtime(options: {
  token: Ref<string | null>
  loadWhatsAppStatus: () => Promise<void>
  bootstrapAuthorizedState: (options?: AuthorizedBootstrapOptions) => Promise<void>
}) {
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()
  const scopeInvalidation = useOmnichannelScopeInvalidation()

  const realtimeConnectionState = ref<RealtimeConnectionState>('disconnected')
  const socketEnabled = ref(false)
  let socketManuallyClosed = false
  let authorizedRefreshInFlight: Promise<void> | null = null
  let lastAuthorizedRefreshAt = 0
  let whatsappStatusPollTimer: ReturnType<typeof setTimeout> | null = null
  let whatsappStatusPollingActive = false
  let whatsappStatusPollingInFlight = false
  let staleFallbackPollTimer: ReturnType<typeof setTimeout> | null = null
  let staleFallbackPollingActive = false
  let staleFallbackPollingInFlight = false
  let connectedHeartbeatTimer: ReturnType<typeof setTimeout> | null = null
  let connectedHeartbeatActive = false
  let visibilityChangeHandler: (() => void) | null = null

  const WHATSAPP_STATUS_POLL_INTERVAL_MS = 45_000
  const AUTHORIZED_REFRESH_COOLDOWN_MS = 5_000
  const STALE_FALLBACK_POLL_INTERVAL_MS = 20_000
  const STALE_FALLBACK_POLL_START_DELAY_MS = 4_000
  const CONNECTED_HEARTBEAT_INTERVAL_MS = 5 * 60_000
  const PAGE_VISIBILITY_STALE_THRESHOLD_MS = 5 * 60_000

  function currentAccountId(): string {
    return resolveRealtimeAccountId(auth, accountStore, '')
  }

  async function refreshAuthorizedState(
    refreshOptions: AuthorizedBootstrapOptions & { force?: boolean } = {},
  ): Promise<void> {
    const now = Date.now()
    if (authorizedRefreshInFlight) {
      await authorizedRefreshInFlight
      return
    }
    if (
      !refreshOptions.force &&
      now - lastAuthorizedRefreshAt < AUTHORIZED_REFRESH_COOLDOWN_MS
    ) {
      return
    }

    const request = options.bootstrapAuthorizedState({
      invalidateInFlight: refreshOptions.invalidateInFlight,
      clearAll: refreshOptions.clearAll,
    })
    authorizedRefreshInFlight = request
    try {
      await request
    } finally {
      lastAuthorizedRefreshAt = Date.now()
      if (authorizedRefreshInFlight === request) authorizedRefreshInFlight = null
    }
  }

  const coordinator = createOmnichannelRealtimeCoordinator({
    currentAccountId,
    publish: scopeInvalidation.publishRealtimeInvalidation,
    bootstrap: (refreshOptions) =>
      refreshAuthorizedState({ ...refreshOptions, force: true }),
  })

  function clearWhatsAppStatusPollTimer(): void {
    if (!whatsappStatusPollTimer) return
    clearTimeout(whatsappStatusPollTimer)
    whatsappStatusPollTimer = null
  }

  function scheduleWhatsAppStatusPoll(
    delay = WHATSAPP_STATUS_POLL_INTERVAL_MS,
  ): void {
    if (!whatsappStatusPollingActive) return
    clearWhatsAppStatusPollTimer()
    whatsappStatusPollTimer = setTimeout(() => {
      void runWhatsAppStatusPollCycle()
    }, delay)
  }

  async function runWhatsAppStatusPollCycle(): Promise<void> {
    if (!whatsappStatusPollingActive) return
    if (import.meta.client && document.visibilityState === 'hidden') {
      scheduleWhatsAppStatusPoll()
      return
    }
    if (whatsappStatusPollingInFlight) {
      scheduleWhatsAppStatusPoll()
      return
    }

    whatsappStatusPollingInFlight = true
    try {
      await options.loadWhatsAppStatus()
    } finally {
      whatsappStatusPollingInFlight = false
      scheduleWhatsAppStatusPoll()
    }
  }

  function clearStaleFallbackPollTimer(): void {
    if (!staleFallbackPollTimer) return
    clearTimeout(staleFallbackPollTimer)
    staleFallbackPollTimer = null
  }

  function scheduleStaleFallbackPoll(
    delay = STALE_FALLBACK_POLL_INTERVAL_MS,
  ): void {
    if (!staleFallbackPollingActive) return
    clearStaleFallbackPollTimer()
    staleFallbackPollTimer = setTimeout(() => {
      void runStaleFallbackPollCycle()
    }, delay)
  }

  async function runStaleFallbackPollCycle(): Promise<void> {
    if (!staleFallbackPollingActive) return
    if (import.meta.client && document.visibilityState === 'hidden') {
      scheduleStaleFallbackPoll()
      return
    }
    if (staleFallbackPollingInFlight) {
      scheduleStaleFallbackPoll()
      return
    }

    staleFallbackPollingInFlight = true
    try {
      await refreshAuthorizedState({ force: true })
    } finally {
      staleFallbackPollingInFlight = false
      scheduleStaleFallbackPoll()
    }
  }

  function startStaleFallbackPolling(): void {
    if (staleFallbackPollingActive) return
    staleFallbackPollingActive = true
    scheduleStaleFallbackPoll(STALE_FALLBACK_POLL_START_DELAY_MS)
  }

  function stopStaleFallbackPolling(): void {
    staleFallbackPollingActive = false
    staleFallbackPollingInFlight = false
    clearStaleFallbackPollTimer()
  }

  function clearConnectedHeartbeatTimer(): void {
    if (!connectedHeartbeatTimer) return
    clearTimeout(connectedHeartbeatTimer)
    connectedHeartbeatTimer = null
  }

  function scheduleConnectedHeartbeat(): void {
    if (!connectedHeartbeatActive) return
    clearConnectedHeartbeatTimer()
    connectedHeartbeatTimer = setTimeout(() => {
      void runConnectedHeartbeatCycle()
    }, CONNECTED_HEARTBEAT_INTERVAL_MS)
  }

  async function runConnectedHeartbeatCycle(): Promise<void> {
    if (!connectedHeartbeatActive) return
    if (import.meta.client && document.visibilityState === 'hidden') {
      scheduleConnectedHeartbeat()
      return
    }
    await refreshAuthorizedState({ force: true })
    scheduleConnectedHeartbeat()
  }

  function startConnectedHeartbeat(): void {
    connectedHeartbeatActive = true
    scheduleConnectedHeartbeat()
  }

  function stopConnectedHeartbeat(): void {
    connectedHeartbeatActive = false
    clearConnectedHeartbeatTimer()
  }

  function handleVisibilityChange(): void {
    if (!import.meta.client || document.visibilityState !== 'visible') return
    if (Date.now() - lastAuthorizedRefreshAt < PAGE_VISIBILITY_STALE_THRESHOLD_MS) return
    void refreshAuthorizedState({ force: true })
  }

  function removeVisibilityChangeListener(): void {
    if (!visibilityChangeHandler || !import.meta.client) return
    document.removeEventListener('visibilitychange', visibilityChangeHandler)
    visibilityChangeHandler = null
  }

  function addVisibilityChangeListener(): void {
    if (!import.meta.client) return
    removeVisibilityChangeListener()
    visibilityChangeHandler = handleVisibilityChange
    document.addEventListener('visibilitychange', visibilityChangeHandler)
  }

  function handleSocketOpen(): void {
    if (socketManuallyClosed) return
    realtimeConnectionState.value = 'connected'
    stopStaleFallbackPolling()
    startConnectedHeartbeat()
    addVisibilityChangeListener()
    void coordinator.handleOpen().catch(() => {
      startStaleFallbackPolling()
    })
  }

  function handleSocketClosed(): void {
    if (socketManuallyClosed) return
    realtimeConnectionState.value = 'disconnected'
    stopConnectedHeartbeat()
    removeVisibilityChangeListener()
    startStaleFallbackPolling()
  }

  function handleTicketError(): void {
    if (socketManuallyClosed) return
    realtimeConnectionState.value = 'auth_error'
    startStaleFallbackPolling()
  }

  const socket = useRealtimeSocket({
    enabled: () => socketEnabled.value,
    scope: 'account',
    path: '/v1/realtime/omnichannel',
    scopeDefault: 'account',
    normalizeScope: () => 'account',
    isValid: ({ accountId }) => Boolean(accountId),
    watchSources: [
      () => socketEnabled.value,
      () => auth.isAuthenticated,
      () => auth.accessToken,
      () => auth.activeTenantId,
      () => accountStore.activeAccountId,
    ],
    onOpen: handleSocketOpen,
    onMessage: (envelope) => {
      coordinator.handleEnvelope(envelope)
    },
    logClose: handleSocketClosed,
    logTicketError: handleTicketError,
  })

  function connectSocket(): void {
    socketManuallyClosed = false
    if (!options.token.value) {
      realtimeConnectionState.value = 'disconnected'
      startStaleFallbackPolling()
      return
    }

    realtimeConnectionState.value = 'connecting'
    socketEnabled.value = true
    void socket.ensureConnection()
  }

  function disconnectSocket(): void {
    socketManuallyClosed = true
    coordinator.resetLifecycle()
    realtimeConnectionState.value = 'disconnected'
    stopStaleFallbackPolling()
    stopConnectedHeartbeat()
    removeVisibilityChangeListener()
    socketEnabled.value = false
    socket.disconnect()
  }

  function stopWhatsAppStatusPolling(): void {
    whatsappStatusPollingActive = false
    clearWhatsAppStatusPollTimer()
  }

  function startWhatsAppStatusPolling(): void {
    stopWhatsAppStatusPolling()
    whatsappStatusPollingActive = true
    scheduleWhatsAppStatusPoll()
  }

  return {
    realtimeConnectionState,
    connectSocket,
    disconnectSocket,
    startWhatsAppStatusPolling,
    stopWhatsAppStatusPolling,
  }
}
