import { computed, ref, type Ref } from 'vue'

import * as calendarApi from '~/domain/calendar/calendar-api'
import type { ApiRequest } from '~/domain/calendar/calendar-api'
import type { useAuthStore } from '~/stores/auth'
import {
  clientColorFor,
  resolveClientColor,
  type CalendarClient,
  type CalendarConfig,
  type CalendarEventInput,
} from '~/utils/calendar'

interface CalendarClientScopeDeps {
  apiRequest: ApiRequest
  auth: ReturnType<typeof useAuthStore>
  accountId: () => string
  config: Ref<CalendarConfig>
}

export function useCalendarClientScope(deps: CalendarClientScopeDeps) {
  const selectedClientId = ref('')
  const canSelectClient = ref(false)
  const lockedClientId = ref('')
  const scopeClients = ref<calendarApi.CalendarScopeClient[]>([])
  const scopeLoaded = ref(false)
  let fetchVersion = 0

  const clients = computed<CalendarClient[]>(() =>
    scopeClients.value.map((client, index) => ({
      id: client.id,
      name: client.name || 'Cliente',
      color: resolveClientColor(
        deps.config.value.clientColors?.[client.id],
        clientColorFor(client.id, index),
      ),
    })),
  )

  const effectiveClientId = computed(() =>
    canSelectClient.value ? selectedClientId.value : lockedClientId.value,
  )

  async function fetchScope(): Promise<boolean> {
    const requestVersion = ++fetchVersion
    const accountId = deps.accountId()
    await deps.auth.ensureSession()
    if (!deps.auth.isAuthenticated) return false
    try {
      const next = await calendarApi.fetchScope(deps.apiRequest)
      if (requestVersion !== fetchVersion || accountId !== deps.accountId()) return false

      // Sem select, o contrato precisa trazer o cliente travado tambem na lista nomeada.
      // Falhar fechado evita transformar uma resposta parcial em visao "todos".
      if (
        !next.canSelect &&
        (!next.lockedClientId || !next.clients.some((client) => client.id === next.lockedClientId))
      ) {
        return false
      }

      scopeClients.value = next.clients
      canSelectClient.value = next.canSelect
      lockedClientId.value = next.canSelect ? '' : next.lockedClientId
      if (!next.canSelect) {
        selectedClientId.value = next.lockedClientId
      } else if (
        selectedClientId.value &&
        !next.clients.some((client) => client.id === selectedClientId.value)
      ) {
        selectedClientId.value = ''
      }
      scopeLoaded.value = true
      return true
    } catch {
      return false
    }
  }

  function resetScope(): void {
    fetchVersion += 1
    selectedClientId.value = ''
    canSelectClient.value = false
    lockedClientId.value = ''
    scopeClients.value = []
    scopeLoaded.value = false
  }

  function setClientFilter(clientId: string): boolean {
    const previous = selectedClientId.value
    if (!canSelectClient.value) {
      selectedClientId.value = lockedClientId.value
      return previous !== selectedClientId.value
    }
    const normalized = String(clientId || '').trim()
    selectedClientId.value =
      !normalized || scopeClients.value.some((client) => client.id === normalized) ? normalized : ''
    return previous !== selectedClientId.value
  }

  function scopeEventInput(input: CalendarEventInput): CalendarEventInput {
    if (canSelectClient.value) return input
    return { ...input, clientId: effectiveClientId.value }
  }

  return {
    selectedClientId,
    canSelectClient,
    lockedClientId,
    scopeLoaded,
    clients,
    effectiveClientId,
    fetchScope,
    resetScope,
    setClientFilter,
    scopeEventInput,
  }
}
