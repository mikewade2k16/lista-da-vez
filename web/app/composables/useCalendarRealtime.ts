import { computed, onBeforeUnmount, ref, type ComputedRef, type Ref } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import { normalizeText } from '../../layers/tasks/utils/text'
import {
  resolveRealtimeAccountId,
  sourceValue,
  useRealtimeSocket,
} from '../../layers/tasks/composables/useRealtimeSocket'

// Realtime do calendario (contrato C11). Canal por conta (topico calendar:account:{id})
// via /v1/realtime/calendar. Aplica os eventos por INVALIDACAO: o WS so avisa "algo
// mudou", o consumidor (a pagina) refaz o fetch da fonte real (banco -> back -> front).
// A maquina de socket (ticket + reconnect + isolamento por conta) vive na base generica
// useRealtimeSocket (mesma de tasks); aqui ficam so o scope e o roteamento de eventos.

type RealtimeSource<T> = T | Ref<T> | ComputedRef<T> | (() => T)

export type CalendarRealtimeStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'error'

export interface CalendarRealtimeEvent {
  type?: string
  accountId?: string
  resourceId?: string
  payload?: Record<string, unknown>
  [key: string]: unknown
}

interface CalendarRealtimeOptions {
  enabled: RealtimeSource<boolean>
  accountId?: RealtimeSource<string>
  // Eventos de evento (create/update/delete) + midia do dia: refetch da janela (debounced).
  onWindowInvalidate?: () => void
  // Nota de um mes mudou: recarrega a nota SO se ja carregada e sem edicao pendente (a pagina decide).
  onNoteUpdated?: (monthKey: string) => void
  // Config do calendario mudou: refetch da config.
  onConfigUpdated?: () => void
  // Plano de IA mudou de status: notifica o modal de IA (encerra polling se terminou).
  onPlanUpdated?: (planId: string, status: string) => void
  // Perfil do cliente mudou (WAVE 10): a aba Clientes refaz o fetch do indice/perfil (sem reload).
  onClientProfileUpdated?: (clientId: string) => void
}

// Rajada de eventos (ex.: aplicar um plano de IA cria varios eventos) vira UM refetch.
const WINDOW_REFRESH_DEBOUNCE_MS = 250

export function useCalendarRealtime(options: CalendarRealtimeOptions) {
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()

  const lastEvent = ref<CalendarRealtimeEvent | null>(null)

  let windowRefreshTimer: ReturnType<typeof setTimeout> | null = null
  function clearWindowRefreshTimer() {
    if (!windowRefreshTimer) return
    clearTimeout(windowRefreshTimer)
    windowRefreshTimer = null
  }
  function scheduleWindowRefresh() {
    clearWindowRefreshTimer()
    windowRefreshTimer = setTimeout(() => {
      windowRefreshTimer = null
      options.onWindowInvalidate?.()
    }, WINDOW_REFRESH_DEBOUNCE_MS)
  }

  function applyEvent(payload: CalendarRealtimeEvent) {
    const type = normalizeText(payload.type, 80)
    if (!type || type === 'realtime.connected') return
    if (!type.startsWith('calendar.')) return

    // Isolamento por conta (defesa em profundidade): o topico ja e' account-scoped no back,
    // mas descartamos qualquer evento de outra conta usando a MESMA fonte de conta do WS.
    const eventAccountId = normalizeText(payload.accountId, 80)
    const currentAccountId = resolveRealtimeAccountId(
      auth,
      accountStore,
      sourceValue(options.accountId, ''),
    )
    if (eventAccountId && currentAccountId && eventAccountId !== currentAccountId) return

    lastEvent.value = payload
    const data =
      payload.payload && typeof payload.payload === 'object'
        ? (payload.payload as Record<string, unknown>)
        : {}

    if (type.startsWith('calendar.event_')) {
      scheduleWindowRefresh()
      return
    }
    if (type === 'calendar.note_updated') {
      options.onNoteUpdated?.(normalizeText(data.monthKey, 7))
      return
    }
    if (type === 'calendar.config_updated') {
      options.onConfigUpdated?.()
      return
    }
    if (type === 'calendar.client_profile_updated') {
      options.onClientProfileUpdated?.(normalizeText(payload.resourceId, 80))
      return
    }
    if (type === 'calendar.plan_updated') {
      options.onPlanUpdated?.(
        normalizeText(payload.resourceId, 120),
        normalizeText(data.status, 40),
      )
    }
  }

  const socket = useRealtimeSocket({
    enabled: options.enabled,
    scope: 'account',
    accountId: options.accountId,
    path: '/v1/realtime/calendar',
    scopeDefault: 'account',
    normalizeScope: () => 'account',
    isValid: ({ accountId }) => Boolean(accountId),
    watchSources: [
      () => sourceValue(options.enabled, false),
      () => sourceValue(options.accountId, ''),
      () => auth.isAuthenticated,
      () => auth.accessToken,
      () => auth.activeTenantId,
      () => accountStore.activeAccountId,
    ],
    onMessage: (payload) => applyEvent(payload as CalendarRealtimeEvent),
    logTicketError: (_desired, error) => {
      if (import.meta.client) console.error('[calendar-ws] ticket ERROR', error)
    },
    logClose: () => {
      if (import.meta.client) console.warn('[calendar-ws] socket CLOSED — reconectando')
    },
    onError: () => {
      if (import.meta.client) console.error('[calendar-ws] socket ERROR')
    },
  })

  onBeforeUnmount(clearWindowRefreshTimer)

  const status = socket.status as Ref<CalendarRealtimeStatus>
  const isConnected = computed(() => status.value === 'connected')

  return {
    status,
    isConnected,
    lastEvent,
    disconnect: socket.disconnect,
  }
}
