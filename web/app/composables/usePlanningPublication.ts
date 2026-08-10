import type { ComputedRef, Ref } from 'vue'

import { suppressPlanningRealtimeEcho } from '~/composables/usePlanningRealtimeEvents'
import type { PlanningShift, ScheduleStatus } from '~/domain/planning/types'
import { usePlanningPersistenceStore } from '~/stores/planning-persistence'
import { usePlanningStore } from '~/stores/planning'
import { useUiStore } from '~/stores/ui'

export function usePlanningPublication(
  canEdit: ComputedRef<boolean>,
  scheduleStatus: Ref<ScheduleStatus>,
  hardIssueCount: ComputedRef<number>,
  activeShifts: ComputedRef<PlanningShift[]>,
  activeStoreId: Ref<string>,
  weekStart: Ref<string>,
  scheduleVersion: Ref<number | undefined>,
  lastSavedAt: Ref<string>,
  saveError: Ref<string>,
  persistSchedule: (status: 'saved' | 'published') => Promise<boolean>,
) {
  const planning = usePlanningStore()
  const persistence = usePlanningPersistenceStore()
  const ui = useUiStore()

  async function publishSchedule() {
    if (!canEdit.value || scheduleStatus.value === 'published') return
    if (hardIssueCount.value > 0 || activeShifts.value.length === 0) {
      ui.error(
        hardIssueCount.value > 0
          ? 'Resolva as restrições obrigatórias antes de publicar.'
          : 'Gere ao menos um turno antes de publicar.',
        'Publicação bloqueada',
      )
      return
    }
    if (await persistSchedule('published')) {
      ui.success('Escala publicada e salva no banco de dados.', 'Escala aprovada')
    }
  }

  async function reopenSchedule() {
    const storeId = activeStoreId.value
    const version = scheduleVersion.value
    if (!canEdit.value || !storeId || !version || scheduleStatus.value !== 'published') return
    const { confirmed } = (await ui.confirm({
      title: 'Reabrir escala publicada?',
      message: 'A semana voltará a aceitar alterações. Esta ação ficará registrada no histórico.',
      confirmLabel: 'Reabrir escala',
    })) as { confirmed: boolean }
    if (!confirmed) return
    planning.markScheduleSaving()
    saveError.value = ''
    try {
      suppressPlanningRealtimeEcho(storeId)
      const saved = await persistence.reopenSchedule(storeId, weekStart.value, version)
      planning.markScheduleSaved(saved)
      lastSavedAt.value = saved.updatedAt || new Date().toISOString()
      ui.success('Escala reaberta para edição.', 'Escala reaberta')
    } catch (error) {
      saveError.value =
        error instanceof Error ? error.message : 'Não foi possível reabrir a escala.'
      planning.applyPersistedSchedule(
        await persistence
          .load(storeId, weekStart.value)
          .then((result) => result.schedule)
          .catch(() => null),
      )
      ui.error(
        'Não foi possível reabrir a escala. Recarregue e tente novamente.',
        'Falha ao reabrir',
      )
    }
  }

  return { publishSchedule, reopenSchedule }
}
