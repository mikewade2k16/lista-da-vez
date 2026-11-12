import type { ComputedRef, Ref } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import { normalizeText } from '../../layers/tasks/utils/text'
import {
  resolveRealtimeAccountId,
  sourceValue,
  useRealtimeSocket,
} from '../../layers/tasks/composables/useRealtimeSocket'

// Realtime dos roteiros: canal por conta (scripts:account:{id}) via /v1/realtime/scripts.
// O evento só avisa "algo mudou" (id + updatedAt); quem consome refaz o fetch pela API,
// que reaplica o escopo de leitura. Versão simples, sem edição colaborativa — ver
// back/internal/modules/tools/AGENT.md ("Implementações futuras").

type Source<T> = T | Ref<T> | ComputedRef<T> | (() => T)

interface ScriptsRealtimeEvent {
  type?: string
  accountId?: string
  resourceId?: string
  userId?: string
  payload?: Record<string, unknown>
}

interface ScriptsRealtimeOptions {
  enabled: Source<boolean>
  accountId: Source<string>
  onSaved: (id: string, updatedAt: string) => void
  onDeleted: (id: string) => void
  /** Ordem da lateral mudou em outra aba do MESMO usuário (a ordem é por usuário). */
  onReordered: () => void
}

export function useScriptsRealtime(options: ScriptsRealtimeOptions) {
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()

  function applyEvent(event: ScriptsRealtimeEvent) {
    const type = normalizeText(event.type, 40)
    if (!type.startsWith('script.')) return
    // Defesa em profundidade: o tópico já é por conta; descarta evento de outra conta.
    const eventAccount = normalizeText(event.accountId, 80)
    const current = resolveRealtimeAccountId(auth, accountStore, sourceValue(options.accountId, ''))
    if (eventAccount && current && eventAccount !== current) return
    const id = normalizeText(event.resourceId, 80)
    if (type === 'script.saved' && id)
      options.onSaved(id, normalizeText(event.payload?.updatedAt, 64))
    else if (type === 'script.deleted' && id) options.onDeleted(id)
    else if (type === 'script.reordered') {
      const me = auth.principal?.userId || auth.user?.id || ''
      if (me && normalizeText(event.userId, 80) === me) options.onReordered()
    }
  }

  return useRealtimeSocket({
    enabled: options.enabled,
    scope: 'account',
    accountId: options.accountId,
    path: '/v1/realtime/scripts',
    scopeDefault: 'account',
    normalizeScope: () => 'account',
    isValid: ({ accountId }) => Boolean(accountId),
    watchSources: [
      () => sourceValue(options.enabled, false),
      () => sourceValue(options.accountId, ''),
      () => auth.isAuthenticated,
      () => auth.accessToken,
      () => accountStore.activeAccountId,
    ],
    onMessage: (payload) => applyEvent(payload as ScriptsRealtimeEvent),
    logTicketError: (_desired, error) => {
      if (import.meta.client) console.error('[scripts-ws] ticket ERROR', error)
    },
    logClose: () => {
      if (import.meta.client) console.warn('[scripts-ws] socket CLOSED — reconectando')
    },
    onError: () => {
      if (import.meta.client) console.error('[scripts-ws] socket ERROR')
    },
  })
}
