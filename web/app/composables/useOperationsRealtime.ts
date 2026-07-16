import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

import { useAppRuntimeStore } from '~/stores/app-runtime'
import { useAuthStore } from '~/stores/auth'
import { useOperationsStore } from '~/stores/operations'
import { buildRealtimeSocketURL } from '~/composables/useRealtimeConnection'
import { createApiRequest } from '~/utils/api-client'
import { refreshRuntimeStoreSettings } from '~/utils/runtime-remote'
import { useCoreAccountStore } from '../../layers/core/stores/account'

type OperationsRealtimeOptions = {
  scopeMode?: unknown
}

// Janela do debounce trailing do overview no modo "Todas as lojas". Rajadas de
// eventos dentro dessa janela colapsam em UM unico refreshOverview, disparado
// apos o periodo de silencio. Mantida curta (~meio segundo) para preservar a
// sensacao de tempo real; o ultimo evento SEMPRE garante o refresh final.
const OVERVIEW_REFRESH_DEBOUNCE_MS = 300

function resolveSourceValue(source, fallback = 'single') {
  if (typeof source === 'function') {
    const value = source()
    return value == null ? fallback : value
  }

  if (source && typeof source === 'object' && 'value' in source) {
    return source.value == null ? fallback : source.value
  }

  return source == null ? fallback : source
}

/**
 * Mantem conexoes realtime da operacao por loja e invalida snapshots sem acoplar a UI ao socket.
 *
 * Em modo `single`, acompanha apenas a loja ativa. Em modo `all`, abre uma conexao por loja
 * acessivel e revalida tanto o overview integrado quanto o snapshot local quando eventos chegam.
 * Tambem escuta atualizacoes de configuracao para sincronizar `runtime` e status da loja ativa.
 *
 * @param options Configuracoes opcionais de escopo, como `scopeMode` (`single` ou `all`).
 * @returns Estado resumido da conexao (`status`) e o ultimo payload recebido (`lastEvent`).
 *
 * @example
 * ```ts
 * const { status, lastEvent } = useOperationsRealtime({
 *   scopeMode: computed(() => 'all'),
 * })
 * ```
 *
 * @see ~/stores/operations
 * @see docs/operacao/operations.md
 */
export function useOperationsRealtime(options: OperationsRealtimeOptions = {}) {
  const runtimeConfig = useRuntimeConfig()
  const runtime = useAppRuntimeStore()
  const auth = useAuthStore()
  const operationsStore = useOperationsStore()
  const accountStore = useCoreAccountStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const status = ref('idle')
  const lastEvent = ref(null)

  const sockets = new Map()
  const reconnectTimers = new Map()
  const reconnectAttempts = new Map()
  const silencedCloses = new Set()

  let stopWatcher = null
  let snapshotRefreshPromise = null
  let snapshotRefreshQueued = false
  let overviewRefreshPromise = null
  let overviewRefreshQueued = false
  let overviewDebounceTimer = null
  let settingsRefreshPromise = null
  let settingsRefreshQueued = false
  let queuedSettingsStoreId = ''

  function desiredStoreIds() {
    if (!auth.isAuthenticated || !auth.accessToken) {
      return []
    }

    const mode = String(resolveSourceValue(options.scopeMode, 'single') || 'single').trim()
    const ids = mode === 'all' ? auth.accessibleStoreIds : [auth.activeStoreId]

    return [
      ...new Set(
        (Array.isArray(ids) ? ids : []).map((value) => String(value || '').trim()).filter(Boolean),
      ),
    ]
  }

  function updateStatus() {
    if (!sockets.size) {
      status.value = 'idle'
      return
    }

    const socketEntries = [...sockets.values()].map((entry) => entry.socket)
    if (socketEntries.some((socket) => socket.readyState === WebSocket.OPEN)) {
      status.value = 'connected'
      return
    }

    if (socketEntries.some((socket) => socket.readyState === WebSocket.CONNECTING)) {
      status.value = 'connecting'
      return
    }

    if (reconnectTimers.size > 0) {
      status.value = 'reconnecting'
      return
    }

    status.value = 'error'
  }

  async function refreshSnapshot(storeId) {
    const normalizedStoreId = String(storeId || '').trim()
    if (!normalizedStoreId) {
      return
    }

    if (snapshotRefreshPromise) {
      snapshotRefreshQueued = true
      return snapshotRefreshPromise
    }

    snapshotRefreshPromise = operationsStore
      .refreshOperationSnapshot(normalizedStoreId)
      .catch(() => null)
      .finally(async () => {
        snapshotRefreshPromise = null

        if (snapshotRefreshQueued) {
          snapshotRefreshQueued = false
          await refreshSnapshot(normalizedStoreId)
        }
      })

    return snapshotRefreshPromise
  }

  async function refreshOverview() {
    if (overviewRefreshPromise) {
      overviewRefreshQueued = true
      return overviewRefreshPromise
    }

    overviewRefreshPromise = operationsStore
      .refreshOverview()
      .catch(() => null)
      .finally(async () => {
        overviewRefreshPromise = null

        if (overviewRefreshQueued) {
          overviewRefreshQueued = false
          await refreshOverview()
        }
      })

    return overviewRefreshPromise
  }

  function clearOverviewDebounce() {
    if (overviewDebounceTimer != null) {
      window.clearTimeout(overviewDebounceTimer)
      overviewDebounceTimer = null
    }
  }

  /**
   * Debounce trailing do refresh de overview no modo "Todas as lojas".
   *
   * Cada evento reinicia a janela; o refresh so dispara apos OVERVIEW_REFRESH_DEBOUNCE_MS
   * sem novos eventos. Isso colapsa rajadas em UM unico refreshOverview e garante,
   * por construcao, um refresh trailing apos o ULTIMO evento (a janela so termina
   * quando os eventos param). O coalescing interno de refreshOverview segue como
   * guarda adicional caso o fetch anterior ainda esteja em voo.
   */
  function scheduleOverviewRefresh() {
    clearOverviewDebounce()
    overviewDebounceTimer = window.setTimeout(() => {
      overviewDebounceTimer = null
      void refreshOverview()
    }, OVERVIEW_REFRESH_DEBOUNCE_MS)
  }

  async function refreshStoreSettings(storeId) {
    const normalizedStoreId = String(
      storeId || auth.activeStoreId || runtime.state.activeStoreId || '',
    ).trim()

    if (!normalizedStoreId || !auth.isAuthenticated || !auth.accessToken) {
      return null
    }

    if (settingsRefreshPromise) {
      settingsRefreshQueued = true
      queuedSettingsStoreId = normalizedStoreId
      return settingsRefreshPromise
    }

    settingsRefreshPromise = refreshRuntimeStoreSettings(
      runtime,
      apiRequest,
      normalizedStoreId,
      auth.activeTenantId,
      // Conta sem o modulo queue nao recarrega /v1/settings (evita 403 +
      // aviso degradado). Quem tem queue mantem o refresh em tempo real.
      { canFetchQueueSettings: accountStore.enabledModules.includes('queue') },
    )
      .then((result) => {
        auth.applyRuntimeSettingsStatus(result)
        return result
      })
      .catch(() => null)
      .finally(async () => {
        settingsRefreshPromise = null

        if (settingsRefreshQueued) {
          const nextStoreId = queuedSettingsStoreId
          settingsRefreshQueued = false
          queuedSettingsStoreId = ''
          await refreshStoreSettings(nextStoreId)
        }
      })

    return settingsRefreshPromise
  }

  function clearReconnectTimer(storeId) {
    const timer = reconnectTimers.get(storeId)
    if (timer) {
      window.clearTimeout(timer)
    }
    reconnectTimers.delete(storeId)
  }

  function disconnectStore(storeId) {
    clearReconnectTimer(storeId)

    const entry = sockets.get(storeId)
    if (!entry) {
      updateStatus()
      return
    }

    silencedCloses.add(storeId)
    entry.socket.close()
    sockets.delete(storeId)
    reconnectAttempts.delete(storeId)
    updateStatus()
  }

  function disconnectAll() {
    for (const storeId of [...sockets.keys()]) {
      disconnectStore(storeId)
    }

    for (const storeId of [...reconnectTimers.keys()]) {
      clearReconnectTimer(storeId)
    }

    updateStatus()
  }

  function scheduleReconnect(storeId) {
    clearReconnectTimer(storeId)

    if (!desiredStoreIds().includes(storeId)) {
      updateStatus()
      return
    }

    const attempt = reconnectAttempts.get(storeId) || 0
    const delayMs = Math.min(10000, 1000 * Math.max(1, 2 ** attempt))
    const timer = window.setTimeout(() => {
      reconnectTimers.delete(storeId)
      void ensureSocket(storeId)
    }, delayMs)

    reconnectTimers.set(storeId, timer)
    updateStatus()
  }

  async function ensureSocket(storeId) {
    const normalizedStoreId = String(storeId || '').trim()
    const accessToken = String(auth.accessToken || '').trim()

    if (!normalizedStoreId || !accessToken || !desiredStoreIds().includes(normalizedStoreId)) {
      disconnectStore(normalizedStoreId)
      return
    }

    const connectionKey = `${normalizedStoreId}:${accessToken}`
    const currentEntry = sockets.get(normalizedStoreId)
    if (
      currentEntry &&
      currentEntry.key === connectionKey &&
      currentEntry.socket.readyState <= WebSocket.OPEN
    ) {
      updateStatus()
      return
    }

    if (currentEntry) {
      disconnectStore(normalizedStoreId)
    }

    let socketURL = ''
    try {
      socketURL = await buildRealtimeSocketURL(
        runtimeConfig,
        '/v1/realtime/operations',
        { storeId: normalizedStoreId },
        accessToken,
      )
    } catch (error) {
      if (
        desiredStoreIds().includes(normalizedStoreId) &&
        String(auth.accessToken || '').trim() === accessToken
      ) {
        status.value = 'error'
      }
      if (import.meta.client) {
        console.warn('[operations-ws] ticket request failed; socket not opened', error)
      }
      return
    }

    if (
      !desiredStoreIds().includes(normalizedStoreId) ||
      String(auth.accessToken || '').trim() !== accessToken
    ) {
      updateStatus()
      return
    }

    const entryAfterTicket = sockets.get(normalizedStoreId)
    if (
      entryAfterTicket &&
      entryAfterTicket.key === connectionKey &&
      entryAfterTicket.socket.readyState <= WebSocket.OPEN
    ) {
      updateStatus()
      return
    }
    if (entryAfterTicket) {
      disconnectStore(normalizedStoreId)
    }

    const nextSocket = new WebSocket(socketURL)
    sockets.set(normalizedStoreId, {
      key: connectionKey,
      socket: nextSocket,
    })
    updateStatus()

    nextSocket.addEventListener('open', () => {
      reconnectAttempts.set(normalizedStoreId, 0)
      updateStatus()
    })

    nextSocket.addEventListener('message', async (message) => {
      try {
        const payload = JSON.parse(String(message.data || '{}'))
        lastEvent.value = payload

        if (payload?.type !== 'operation.updated') {
          return
        }

        const payloadStoreId = String(payload?.storeId || '').trim()
        const payloadAction = String(payload?.action || '').trim()
        const mode = String(resolveSourceValue(options.scopeMode, 'single') || 'single').trim()

        if (payloadAction === 'settings-updated') {
          if (
            payloadStoreId &&
            payloadStoreId ===
              String(auth.activeStoreId || runtime.state.activeStoreId || '').trim()
          ) {
            await refreshStoreSettings(payloadStoreId)
          }

          return
        }

        // O roster (queue.consultants) mudou por edicao de usuario/consultor. A faixa
        // de participantes vem do roster de GESTAO (/v1/consultants), nao do roster
        // enxuto do snapshot — refreshSnapshot preserva o roster existente e nao
        // adicionaria/removeria consultor ao vivo. Reidratamos o store completo para
        // rebuscar /v1/consultants e refletir a entrada/saida na Lista da vez na hora.
        const isRosterChange = payloadAction === 'roster.updated'

        if (mode === 'all') {
          // O board integrado renderiza a partir do overview: colapsamos as rajadas
          // num unico refresh trailing (~300ms apos o ultimo evento).
          scheduleOverviewRefresh()

          // No modo "Todas as lojas" o snapshot por loja so e exibido para a loja
          // aberta no detalhe operavel (integratedStoreId). Revalidar o snapshot de
          // qualquer loja que emita evento gera refetches que ninguem ve. Atualizamos
          // imediatamente apenas a loja ativa, sem descartar a atualizacao dela; o
          // coalescing interno de refreshSnapshot ja protege contra rajadas dessa loja.
          const integratedStoreId = String(operationsStore.integratedStoreId || '').trim()
          if (payloadStoreId && payloadStoreId === integratedStoreId) {
            if (isRosterChange && payloadStoreId === String(auth.activeStoreId || '').trim()) {
              await operationsStore.refreshActiveStore()
            } else {
              await refreshSnapshot(payloadStoreId)
            }
          }

          return
        }

        if (payloadStoreId && payloadStoreId === String(auth.activeStoreId || '').trim()) {
          if (isRosterChange) {
            await operationsStore.refreshActiveStore()
          } else {
            await refreshSnapshot(payloadStoreId)
          }
        }
      } catch {
        // ignoramos payloads invalidos do socket
      }
    })

    nextSocket.addEventListener('close', () => {
      if (silencedCloses.has(normalizedStoreId)) {
        silencedCloses.delete(normalizedStoreId)
        updateStatus()
        return
      }

      sockets.delete(normalizedStoreId)
      reconnectAttempts.set(normalizedStoreId, (reconnectAttempts.get(normalizedStoreId) || 0) + 1)
      scheduleReconnect(normalizedStoreId)
    })

    nextSocket.addEventListener('error', () => {
      updateStatus()
    })
  }

  function syncConnections() {
    if (import.meta.server) {
      return
    }

    const expectedStoreIds = new Set(desiredStoreIds())
    for (const storeId of [...sockets.keys()]) {
      if (!expectedStoreIds.has(storeId)) {
        disconnectStore(storeId)
      }
    }

    if (!expectedStoreIds.size) {
      disconnectAll()
      return
    }

    for (const storeId of expectedStoreIds) {
      void ensureSocket(storeId)
    }

    updateStatus()
  }

  onMounted(() => {
    stopWatcher = watch(
      [
        () => auth.isAuthenticated,
        () => auth.activeStoreId,
        () => auth.accessToken,
        () => auth.accessibleStoreIds.join(','),
        () => String(resolveSourceValue(options.scopeMode, 'single') || 'single'),
      ],
      () => {
        syncConnections()
      },
      { immediate: true },
    )
  })

  onBeforeUnmount(() => {
    if (typeof stopWatcher === 'function') {
      stopWatcher()
      stopWatcher = null
    }

    clearOverviewDebounce()
    disconnectAll()
  })

  return {
    status,
    lastEvent,
  }
}
