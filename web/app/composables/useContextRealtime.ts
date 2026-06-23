import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

import { useAuthStore } from '~/stores/auth'
import { useAccessControlStore } from '~/stores/access-control'
import { useAlertsStore } from '~/stores/alerts'
import { useCrmStore } from '~/stores/crm'
import { useUiStore } from '~/stores/ui'
import { useAppRuntimeStore } from '~/stores/app-runtime'
import { useMultiStoreStore } from '~/stores/multistore'
import { useOperationGoalsStore } from '~/stores/operation-goals'
import { useUsersStore } from '~/stores/users'
import { buildRealtimeSocketURL } from '~/composables/useRealtimeConnection'
import { createApiRequest } from '~/utils/api-client'
import { refreshRuntimeStoreSettings } from '~/utils/runtime-remote'
import { useCoreAccountStore } from '../../layers/core/stores/account'

/**
 * Sincroniza o contexto global do tenant via WebSocket e dispara refresh dos stores transversais.
 *
 * Quando o backend publica `context.updated`, este composable reidrata contexto de autenticacao,
 * acessos, alertas, usuarios e visoes multi-loja conforme o papel atual. Alertas com `displayKind`
 * igual a `toast` tambem viram notificacoes efemeras no `ui`.
 *
 * @returns Estado resumido da conexao (`status`) e o ultimo evento recebido (`lastEvent`).
 *
 * @example
 * ```ts
 * const { status } = useContextRealtime()
 * ```
 *
 * @see ~/stores/auth
 * @see ~/stores/alerts
 */
export function useContextRealtime() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const accessControl = useAccessControlStore()
  const alertsStore = useAlertsStore()
  const ui = useUiStore()
  const runtime = useAppRuntimeStore()
  const accountStore = useCoreAccountStore()

  const toastedAlertIds = new Set<string>()
  const multiStore = useMultiStoreStore()
  const usersStore = useUsersStore()
  const operationGoalsStore = useOperationGoalsStore()
  const crmStore = useCrmStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const status = ref('idle')
  const lastEvent = ref(null)

  let socket = null
  let reconnectTimer = null
  let reconnectAttempt = 0
  let stopWatcher = null
  let currentConnectionKey = ''
  let connectionGeneration = 0
  let intentionallyClosed = false
  let refreshPromise = null
  let refreshQueued = false
  let settingsRefreshPromise = null
  let settingsRefreshQueued = false
  let queuedSettingsStoreId = ''

  async function refreshContextState() {
    if (refreshPromise) {
      refreshQueued = true
      return refreshPromise
    }

    refreshPromise = (async () => {
      try {
        await auth.fetchContext()

        const followUps = []
        if (
          auth.role === 'platform_admin' ||
          auth.role === 'owner' ||
          auth.role === 'director' ||
          auth.role === 'marketing'
        ) {
          followUps.push(multiStore.refreshOverview().catch(() => null))
        }

        if (auth.role === 'platform_admin' || auth.role === 'owner') {
          followUps.push(multiStore.refreshManagedStores().catch(() => null))
          followUps.push(usersStore.refreshUsers({ silent: true }).catch(() => null))
        }

        await Promise.allSettled(followUps)
      } finally {
        refreshPromise = null
        if (refreshQueued) {
          refreshQueued = false
          await refreshContextState()
        }
      }
    })()

    return refreshPromise
  }

  async function refreshActiveStoreSettings(storeId = '') {
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
          await refreshActiveStoreSettings(nextStoreId)
        }
      })

    return settingsRefreshPromise
  }

  function clearReconnectTimer() {
    if (reconnectTimer) {
      window.clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
  }

  function disconnect() {
    connectionGeneration += 1
    intentionallyClosed = true
    clearReconnectTimer()

    if (socket) {
      socket.close()
      socket = null
    }

    currentConnectionKey = ''
    status.value = 'idle'
  }

  function scheduleReconnect() {
    clearReconnectTimer()

    if (!auth.isAuthenticated || !auth.activeTenantId || !auth.accessToken) {
      return
    }

    const delayMs = Math.min(10000, 1000 * Math.max(1, 2 ** reconnectAttempt))
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = null
      void connect()
    }, delayMs)
  }

  async function connect() {
    if (import.meta.server) {
      return
    }

    const tenantId = String(auth.activeTenantId || auth.tenantContext?.[0]?.id || '').trim()
    const accessToken = String(auth.accessToken || '').trim()

    if (!auth.isAuthenticated || !tenantId || !accessToken) {
      disconnect()
      return
    }

    const nextConnectionKey = `${tenantId}:${accessToken}`
    if (
      socket &&
      currentConnectionKey === nextConnectionKey &&
      socket.readyState <= WebSocket.OPEN
    ) {
      return
    }

    intentionallyClosed = false
    clearReconnectTimer()

    if (socket) {
      socket.close()
      socket = null
    }

    currentConnectionKey = nextConnectionKey
    status.value = 'connecting'
    const requestGeneration = (connectionGeneration += 1)

    let socketURL = ''
    try {
      socketURL = await buildRealtimeSocketURL(
        runtimeConfig,
        '/v1/realtime/context',
        { tenantId },
        accessToken,
      )
    } catch (error) {
      if (
        requestGeneration === connectionGeneration &&
        currentConnectionKey === nextConnectionKey
      ) {
        currentConnectionKey = ''
        status.value = 'error'
      }
      if (import.meta.client) {
        console.warn('[context-ws] ticket request failed; socket not opened', error)
      }
      return
    }

    if (
      requestGeneration !== connectionGeneration ||
      currentConnectionKey !== nextConnectionKey ||
      !auth.isAuthenticated ||
      String(auth.accessToken || '').trim() !== accessToken
    ) {
      return
    }

    const nextSocket = new WebSocket(socketURL)
    socket = nextSocket

    nextSocket.addEventListener('open', () => {
      reconnectAttempt = 0
      status.value = 'connected'
    })

    nextSocket.addEventListener('message', async (message) => {
      try {
        const payload = JSON.parse(String(message.data || '{}'))
        lastEvent.value = payload

        if (payload?.type !== 'context.updated') {
          return
        }

        if (String(payload?.tenantId || '').trim() !== String(auth.activeTenantId || '').trim()) {
          return
        }

        if (String(payload?.resource || '').trim() === 'settings') {
          const activeStoreId = String(
            auth.activeStoreId || runtime.state.activeStoreId || '',
          ).trim()
          const payloadTenantId = String(payload?.resourceId || payload?.tenantId || '').trim()

          if (!payloadTenantId || payloadTenantId === String(auth.activeTenantId || '').trim()) {
            await refreshActiveStoreSettings(activeStoreId)
          }

          return
        }

        const resource = String(payload?.resource || '').trim()

        if (resource === 'operationgoal') {
          // Meta criada/editada/excluida em qualquer sessao do tenant.
          // Recarrega lista de metas e overview do CRM (que cruza com operation_goal_targets).
          // Nao depende de `ready`: se a store ja tem overview carregado, refresca.
          const action = String(payload?.action || '').trim()
          const resourceId = String(payload?.resourceId || '').trim()
          const shouldSkipGoalsRefresh = operationGoalsStore.shouldSkipRealtimeUpdate(
            action,
            resourceId,
          )
          const followUps = []
          if (
            !shouldSkipGoalsRefresh &&
            (operationGoalsStore.ready || operationGoalsStore.goals?.length)
          ) {
            followUps.push(
              operationGoalsStore.loadGoals(operationGoalsStore.lastFilters).catch(() => null),
            )
          }
          crmStore.invalidateOverview()
          if (crmStore.overview) {
            followUps.push(crmStore.refreshOverview().catch(() => null))
          }
          await Promise.allSettled(followUps)
          return
        }

        await refreshContextState()

        // Loja foi criada/atualizada/arquivada/excluida: o cruzamento do CRM por storeCode
        // pode ter mudado (rename, archive, delete). Refresca tambem o CRM se ja carregado.
        if (resource === 'store' && crmStore.overview) {
          await crmStore.refreshOverview().catch(() => null)
        }

        if (['access', 'user'].includes(resource)) {
          await accessControl.refreshRealtimeState()
        }

        if (resource === 'alerts') {
          await alertsStore.refreshRealtimeState()

          const currentUserId = String(auth.principal?.userId || '').trim()
          const isConsultant = auth.role === 'consultant'

          for (const alert of alertsStore.items) {
            if (
              alert.status === 'active' &&
              alert.displayKind === 'toast' &&
              !toastedAlertIds.has(alert.id)
            ) {
              if (!isConsultant || !currentUserId || alert.consultantId === currentUserId) {
                toastedAlertIds.add(alert.id)
                ui.notify({
                  type: 'info',
                  title: alert.headline || 'Atendimento longo',
                  message: alert.body || '',
                  duration: 6000,
                })
              }
            }
          }
        }
      } catch {
        return
      }
    })

    nextSocket.addEventListener('close', () => {
      if (socket === nextSocket) {
        socket = null
      }

      if (intentionallyClosed) {
        status.value = 'idle'
        return
      }

      reconnectAttempt += 1
      status.value = 'reconnecting'
      scheduleReconnect()
    })

    nextSocket.addEventListener('error', () => {
      status.value = 'error'
    })
  }

  onMounted(() => {
    stopWatcher = watch(
      [() => auth.isAuthenticated, () => auth.activeTenantId, () => auth.accessToken],
      () => {
        void connect()
      },
      {
        immediate: true,
      },
    )
  })

  onBeforeUnmount(() => {
    if (typeof stopWatcher === 'function') {
      stopWatcher()
      stopWatcher = null
    }

    disconnect()
  })

  return {
    status,
    lastEvent,
  }
}
