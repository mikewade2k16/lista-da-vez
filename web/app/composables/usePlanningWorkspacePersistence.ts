import { ref, type ComputedRef } from 'vue'
import { storeToRefs } from 'pinia'

import { usePlanningConflictSync } from '~/composables/usePlanningConflictSync'
import { suppressPlanningRealtimeEcho } from '~/composables/usePlanningRealtimeEvents'
import type { PlanningScheduleRevision } from '~/domain/planning/types'
import { useAuthStore } from '~/stores/auth'
import { useOperationGoalsStore } from '~/stores/operation-goals'
import { usePlanningStore } from '~/stores/planning'
import { usePlanningPersistenceStore } from '~/stores/planning-persistence'
import { useUiStore } from '~/stores/ui'

export function usePlanningWorkspacePersistence(canEditPlanning: ComputedRef<boolean>) {
  const planning = usePlanningStore()
  const persistence = usePlanningPersistenceStore()
  const operationGoals = useOperationGoalsStore()
  const auth = useAuthStore()
  const ui = useUiStore()
  const { activeStoreId, weekStart, selectedMonth, selectedPeriod, scheduleVersion, activeShifts } =
    storeToRefs(planning)
  const contextPending = ref(false)
  const contextError = ref('')
  const history = ref<PlanningScheduleRevision[]>([])
  const saveError = ref('')
  const lastSavedAt = ref('')
  let requestId = 0

  const { handlePersistenceError } = usePlanningConflictSync(
    activeStoreId,
    weekStart,
    history,
    refresh,
  )

  function readableError(error: unknown, fallback: string): string {
    if (error instanceof Error && error.message.trim()) return error.message
    return fallback
  }

  async function refresh(): Promise<void> {
    const storeId = activeStoreId.value
    if (!auth.isAuthenticated || !storeId) return
    const currentRequest = ++requestId
    contextPending.value = true
    contextError.value = ''

    try {
      const [goalReferences, consultants, persisted] = await Promise.all([
        operationGoals.loadGoals({
          tenantId: auth.activeTenantId,
          storeId,
          month: selectedMonth.value,
        }),
        operationGoals.loadConsultants(storeId),
        persistence.load(storeId, weekStart.value),
      ])
      if (currentRequest !== requestId || storeId !== activeStoreId.value) return
      planning.syncGoalReferences(storeId, selectedMonth.value, goalReferences)
      planning.syncStaffReferences(
        storeId,
        consultants.map((consultant) => ({
          id: consultant.id,
          storeId,
          name: consultant.name,
          nick: consultant.nick,
          employeeCode: consultant.employeeCode,
          role: consultant.role,
        })),
      )
      planning.resetActiveConfiguration()
      if (persisted.configuration?.configuration) {
        planning.applyPersistedConfiguration(persisted.configuration.configuration)
      }
      planning.applyStaffContracts(persisted.contracts || [])
      planning.applyPersistedSchedule(persisted.schedule)
      history.value = persisted.history || []
      lastSavedAt.value = persisted.schedule?.updatedAt || ''
      saveError.value = ''
    } catch (error) {
      if (currentRequest === requestId) {
        contextError.value = readableError(
          error,
          'Confira sua conexão e tente carregar a escala novamente.',
        )
      }
    } finally {
      if (currentRequest === requestId) contextPending.value = false
    }
  }

  async function persistConfiguration(): Promise<boolean> {
    const storeId = activeStoreId.value
    const configuration = planning.persistedConfiguration()
    if (!canEditPlanning.value || !storeId || !configuration) return false
    try {
      suppressPlanningRealtimeEcho(storeId)
      await persistence.saveConfiguration(storeId, configuration)
      return true
    } catch (error) {
      saveError.value = readableError(error, 'Não foi possível salvar as configurações no banco.')
      ui.error('Não foi possível salvar as configurações no banco.', 'Falha ao salvar')
      return false
    }
  }

  async function persistSchedule(
    status: 'saved' | 'published' = 'saved',
    persistConfigurationFirst = false,
  ): Promise<boolean> {
    const storeId = activeStoreId.value
    if (
      !canEditPlanning.value ||
      !storeId ||
      (persistConfigurationFirst && !(await persistConfiguration()))
    )
      return false
    saveError.value = ''
    planning.markScheduleSaving()
    try {
      suppressPlanningRealtimeEcho(storeId)
      const saved = await persistence.saveSchedule(
        storeId,
        weekStart.value,
        activeShifts.value,
        selectedMonth.value,
        Number(selectedPeriod.value.slice(1)),
        status,
        scheduleVersion.value,
      )
      planning.markScheduleSaved(saved)
      lastSavedAt.value = saved.updatedAt || new Date().toISOString()
      return true
    } catch (error) {
      saveError.value = readableError(error, 'Não foi possível salvar a escala.')
      if (await handlePersistenceError(error)) saveError.value = ''
      return false
    }
  }

  async function generate(): Promise<void> {
    const storeId = activeStoreId.value
    if (!storeId || !(await persistConfiguration())) return
    saveError.value = ''
    planning.markScheduleSaving()
    try {
      suppressPlanningRealtimeEcho(storeId)
      const saved = await persistence.generateSchedule(
        storeId,
        weekStart.value,
        selectedMonth.value,
        Number(selectedPeriod.value.slice(1)),
        scheduleVersion.value,
      )
      planning.applyPersistedSchedule(saved)
      lastSavedAt.value = saved.updatedAt || new Date().toISOString()
      ui.success('Escala gerada e salva no banco de dados.', 'Escala pronta')
    } catch (error) {
      saveError.value = readableError(error, 'Não foi possível gerar e salvar a escala.')
      if (await handlePersistenceError(error)) saveError.value = ''
    }
  }

  return {
    contextPending,
    contextError,
    history,
    saveError,
    lastSavedAt,
    refresh,
    persistSchedule,
    generate,
  }
}
