import * as calendarApi from '~/domain/calendar/calendar-api'
import type { ApiRequest } from '~/domain/calendar/calendar-api'
import type { useUiStore } from '~/stores/ui'
import type { CalendarEventInput } from '~/utils/calendar'

// CRUD de eventos do calendario, extraido do store (stores/calendar.ts) na SPEC-F9 para
// manter o arquivo < 450 linhas e concentrar a logica do optimistic locking (contrato
// C12). Recebe as dependencias do store (apiRequest, o refetch da janela e o store de UI
// para o aviso de task).

export type UpdateEventOutcome = 'ok' | 'conflict' | 'error'

interface EventCrudDeps {
  apiRequest: ApiRequest
  refetch: () => Promise<void>
  ui: ReturnType<typeof useUiStore>
}

// Detecta o 409 version_conflict (C12) no erro do $fetch: o back responde
// { error: { code: 'version_conflict' } } com status HTTP 409.
function isVersionConflict(err: unknown): boolean {
  const e = err as {
    status?: number
    statusCode?: number
    response?: { status?: number }
    data?: { error?: { code?: string } }
  }
  if (e?.data?.error?.code === 'version_conflict') return true
  const status = e?.response?.status ?? e?.status ?? e?.statusCode
  return status === 409
}

export function useCalendarEventCrud(deps: EventCrudDeps) {
  const { apiRequest, refetch, ui } = deps

  async function createEvent(input: CalendarEventInput): Promise<boolean> {
    try {
      // C10: com createTask=true o back tenta criar/vincular a task no board da config;
      // se a task falhar, o evento AINDA salva (201) e volta taskWarning. Avisamos sem
      // derrubar o sucesso da criacao do evento.
      const result = await calendarApi.postEvent(apiRequest, input)
      await refetch()
      if (result.taskWarning) ui.info(result.taskWarning, 'Task nao criada')
      return true
    } catch {
      return false
    }
  }

  // version = a version que o FORM carregou (editingEvent), nao a atual do store: o
  // realtime pode ter refetchado o evento para uma version mais nova enquanto o usuario
  // editava; usar a do store mascararia o conflito (lost update). Sem version = sem If-Match.
  async function updateEvent(
    id: string,
    input: CalendarEventInput,
    version?: number,
  ): Promise<UpdateEventOutcome> {
    try {
      await calendarApi.putEvent(apiRequest, id, input, version)
      await refetch()
      return 'ok'
    } catch (err) {
      if (isVersionConflict(err)) {
        // Traz a versao nova para o store; a UI decide se re-hidrata o form. O draft do
        // usuario NAO e descartado sem o "recarregar" explicito (principio 1).
        await refetch()
        return 'conflict'
      }
      return 'error'
    }
  }

  async function deleteEvent(id: string, archiveTask = false): Promise<boolean> {
    try {
      await calendarApi.deleteEvent(apiRequest, id, archiveTask)
      await refetch()
      return true
    } catch {
      return false
    }
  }

  return { createEvent, updateEvent, deleteEvent }
}
