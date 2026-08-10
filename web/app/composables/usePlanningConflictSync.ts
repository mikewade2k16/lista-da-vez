import { watch, type Ref } from 'vue'

import {
  isPlanningRealtimeEchoSuppressed,
  usePlanningRealtimeEvents,
} from '~/composables/usePlanningRealtimeEvents'
import type { PlanningScheduleRevision } from '~/domain/planning/types'
import { usePlanningPersistenceStore } from '~/stores/planning-persistence'
import { usePlanningStore } from '~/stores/planning'
import { useUiStore } from '~/stores/ui'

function planningErrorCode(error: unknown): string {
  if (!error || typeof error !== 'object') return ''
  const data = 'data' in error ? (error as { data?: unknown }).data : undefined
  if (!data || typeof data !== 'object') return ''
  const apiError = 'error' in data ? (data as { error?: unknown }).error : undefined
  return apiError && typeof apiError === 'object' && 'code' in apiError
    ? String((apiError as { code?: unknown }).code || '')
    : ''
}

export function usePlanningConflictSync(
  activeStoreId: Ref<string>,
  weekStart: Ref<string>,
  history: Ref<PlanningScheduleRevision[]>,
  refresh: () => Promise<void>,
) {
  const persistence = usePlanningPersistenceStore()
  const planning = usePlanningStore()
  const ui = useUiStore()
  const { event } = usePlanningRealtimeEvents()

  async function handlePersistenceError(error: unknown): Promise<boolean> {
    const code = planningErrorCode(error)
    const storeId = activeStoreId.value
    if (storeId && (code === 'version_conflict' || code === 'schedule_published')) {
      const latest = await persistence.load(storeId, weekStart.value).catch(() => null)
      if (!latest) {
        planning.markScheduleUnsaved()
        ui.error(
          'A escala mudou, mas não foi possível carregar a versão mais recente.',
          'Atualização necessária',
        )
        return false
      }
      planning.applyPersistedSchedule(latest.schedule)
      history.value = latest.history || []
      ui.error(
        code === 'version_conflict'
          ? 'Outro gestor alterou esta semana. A versão mais recente foi carregada.'
          : 'Esta semana foi publicada por outro gestor e agora está bloqueada.',
        'Escala atualizada',
      )
      return true
    }
    planning.markScheduleUnsaved()
    ui.error('Não foi possível salvar a escala no banco.', 'Falha ao salvar')
    return false
  }

  watch(event, (payload) => {
    if (!payload || payload.storeId !== activeStoreId.value) return
    if (isPlanningRealtimeEchoSuppressed(payload.storeId)) return
    window.setTimeout(() => void refresh(), 250)
  })

  return { handlePersistenceError }
}
