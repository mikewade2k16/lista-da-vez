import { computed, onMounted, ref, watch, type ComputedRef } from 'vue'
import { storeToRefs } from 'pinia'

import { usePlanningWorkspacePersistence } from '~/composables/usePlanningWorkspacePersistence'
import {
  canEditPlanning as canEditPlanningPermission,
  canViewPlanning as canViewPlanningPermission,
} from '~/domain/planning/permissions'
import type { PlanningSectionId, PlanningStoreReference } from '~/domain/planning/types'
import { useAuthStore } from '~/stores/auth'
import { useMultiStoreStore } from '~/stores/multistore'
import { usePlanningStore } from '~/stores/planning'
import { isGoalPeriodForMonth } from '~/utils/goal-periods'
import { useCoreAccountStore } from '../../layers/core/stores/account'

export function usePlanningWorkspaceContext(activeSection: ComputedRef<PlanningSectionId>) {
  const planning = usePlanningStore()
  const multiStore = useMultiStoreStore()
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()
  const { managedStores } = storeToRefs(multiStore)
  const { activeStoreId, selectedMonth, selectedPeriod } = storeToRefs(planning)
  const bootPending = ref(true)
  const clientReady = ref(false)
  const bootError = ref('')
  const selectedGoalsMonth = ref('')
  const selectedGoalsStoreId = ref('')
  const selectedGoalsPeriod = ref('month')

  const activeManagedStores = computed(() =>
    managedStores.value.filter((store) => store.isActive !== false),
  )
  const planningStoreReferences = computed<PlanningStoreReference[]>(() =>
    activeManagedStores.value.map((store) => ({
      id: store.id,
      name: store.name,
      city: store.city,
      storeType: store.storeType === 'shopping' ? 'shopping' : 'bairro',
    })),
  )
  const storeOptions = computed(() =>
    planning.stores.map((store) => ({
      value: store.id,
      label: store.name,
      meta: `${store.city} · ${store.locationType === 'shopping' ? 'Shopping' : 'Loja de rua'}`,
    })),
  )
  const allowAllGoalStores = computed(
    () => new Set(activeManagedStores.value.map((store) => store.id)).size > 1,
  )
  const canViewPlanning = computed(() =>
    canViewPlanningPermission(auth.role, auth.permissionKeys, auth.permissionsResolved),
  )
  const canEditPlanning = computed(() =>
    canEditPlanningPermission(auth.role, auth.permissionKeys, auth.permissionsResolved),
  )
  const activeAuthStoreId = computed(() => auth.activeStoreId)
  const {
    contextPending: planningContextPending,
    contextError: planningContextError,
    history: scheduleHistory,
    saveError,
    lastSavedAt,
    refresh: refreshPlanningContext,
    persistSchedule,
    generate: generateSchedule,
  } = usePlanningWorkspacePersistence(canEditPlanning)

  function updateGoalsStore(value: string): void {
    selectedGoalsStoreId.value = value
    if (value && planning.stores.some((store) => store.id === value)) {
      planning.setActiveStore(value)
    }
  }

  function updateGoalsMonth(value: string): void {
    selectedGoalsMonth.value = value
    if (isGoalPeriodForMonth(selectedGoalsPeriod.value, value)) {
      planning.setGoalReference(value, selectedGoalsPeriod.value)
    }
  }

  function updateGoalsPeriod(value: string): void {
    selectedGoalsPeriod.value = value
    if (isGoalPeriodForMonth(value, selectedGoalsMonth.value)) {
      planning.setGoalReference(selectedGoalsMonth.value, value)
    }
  }

  async function loadPlanningStores(): Promise<void> {
    if (!canViewPlanning.value) {
      planning.syncStoreReferences([])
      bootPending.value = false
      return
    }
    bootPending.value = true
    bootError.value = ''
    try {
      if (!accountStore.accountsLoaded) await accountStore.fetchAccounts()
      if (!accountStore.activeAccountId) {
        planning.syncStoreReferences([])
        throw new Error('Nenhuma conta ativa está disponível para carregar o planejamento.')
      }
      await multiStore.ensureLoaded({ force: true, includeOverview: false })
      planning.syncStoreReferences(planningStoreReferences.value)
    } catch (error) {
      bootError.value =
        error instanceof Error && error.message.trim()
          ? error.message
          : 'Não foi possível carregar as lojas do planejamento.'
    } finally {
      bootPending.value = false
    }
  }

  onMounted(() => {
    clientReady.value = true
    void loadPlanningStores()
  })

  watch(planningStoreReferences, (references) => planning.syncStoreReferences(references), {
    deep: true,
    immediate: true,
  })

  watch(
    () => accountStore.activeAccountId,
    (nextAccountId, previousAccountId) => {
      if (
        clientReady.value &&
        accountStore.accountsLoaded &&
        nextAccountId &&
        previousAccountId &&
        nextAccountId !== previousAccountId
      ) {
        planning.syncStoreReferences([])
        void loadPlanningStores()
      }
    },
    { flush: 'sync' },
  )

  watch(
    () => [
      clientReady.value,
      bootPending.value,
      accountStore.accountsLoaded,
      accountStore.activeAccountId,
      activeSection.value,
      activeStoreId.value,
      selectedMonth.value,
      selectedPeriod.value,
      auth.activeTenantId,
    ],
    () => {
      if (
        !clientReady.value ||
        bootPending.value ||
        !accountStore.accountsLoaded ||
        !accountStore.activeAccountId ||
        !planningStoreReferences.value.some((store) => store.id === activeStoreId.value)
      ) {
        return
      }
      if (
        activeSection.value === 'goals' &&
        (selectedGoalsPeriod.value === 'month' || !selectedGoalsStoreId.value)
      ) {
        return
      }
      void refreshPlanningContext()
    },
    { immediate: true },
  )

  return {
    bootPending,
    clientReady,
    bootError,
    selectedGoalsMonth,
    selectedGoalsStoreId,
    selectedGoalsPeriod,
    activeManagedStores,
    storeOptions,
    allowAllGoalStores,
    canViewPlanning,
    canEditPlanning,
    activeAuthStoreId,
    planningContextPending,
    planningContextError,
    scheduleHistory,
    saveError,
    lastSavedAt,
    refreshPlanningContext,
    persistSchedule,
    generateSchedule,
    updateGoalsStore,
    updateGoalsMonth,
    updateGoalsPeriod,
    loadPlanningStores,
  }
}
