import type { ComputedRef, Ref } from 'vue'

import { createPlanningStaffMember, createPlanningStore } from '~/domain/planning/fixtures'
import { deriveShiftTemplatesFromOperatingHours } from '~/domain/planning/scheduler'
import {
  buildPersistedConfiguration,
  mergePersistedStaff,
  mergeStaffContracts,
  syncStoreGoalReferences,
} from '~/domain/planning/store-sync'
import type {
  PlanningGoalReference,
  PlanningLaborPolicy,
  PlanningPersistedConfiguration,
  PlanningShift,
  PlanningStaffContract,
  PlanningStaffMember,
  PlanningStaffReference,
  PlanningStore,
  PlanningStoreReference,
} from '~/domain/planning/types'

interface PlanningReferenceState {
  stores: Ref<PlanningStore[]>
  staff: Ref<PlanningStaffMember[]>
  policies: Ref<PlanningLaborPolicy[]>
  shifts: Ref<PlanningShift[]>
  activeStoreId: Ref<string>
  activePolicyId: Ref<string>
  activeStore: ComputedRef<PlanningStore | undefined>
  activeStaff: ComputedRef<PlanningStaffMember[]>
  showWorkspaceHero: Ref<boolean>
  resetScheduleLoading: () => void
}

export function usePlanningReferenceState(state: PlanningReferenceState) {
  const defaultPolicies = state.policies.value.map((policy) => ({ ...policy }))

  function resetReferenceContext(): void {
    state.stores.value = []
    state.staff.value = []
    state.shifts.value = []
    state.policies.value = defaultPolicies.map((policy) => ({ ...policy }))
    state.activeStoreId.value = ''
    state.activePolicyId.value = state.policies.value[0]?.id || ''
    state.showWorkspaceHero.value = true
    state.resetScheduleLoading()
  }

  function syncStoreReferences(references: PlanningStoreReference[]): void {
    const normalized = references.filter((reference) => reference.id && reference.name)
    const currentStoreIDs = new Set(state.stores.value.map((store) => store.id))
    const sharesCurrentContext = normalized.some((reference) => currentStoreIDs.has(reference.id))

    if (!normalized.length) {
      resetReferenceContext()
      return
    }
    if (currentStoreIDs.size > 0 && !sharesCurrentContext) resetReferenceContext()

    const currentByID = new Map(state.stores.value.map((store) => [store.id, store]))
    state.stores.value = normalized.map((reference) => {
      const current = currentByID.get(reference.id)
      if (!current) return createPlanningStore(reference)
      current.name = reference.name
      current.city = reference.city
      current.locationType = reference.storeType === 'shopping' ? 'shopping' : 'street'
      return current
    })
    const validStoreIDs = new Set(state.stores.value.map((store) => store.id))
    state.staff.value = state.staff.value.filter((member) => validStoreIDs.has(member.storeId))
    state.shifts.value = state.shifts.value.filter((shift) =>
      state.staff.value.some((member) => member.id === shift.staffId),
    )
    if (!validStoreIDs.has(state.activeStoreId.value)) {
      state.activeStoreId.value = state.stores.value[0]?.id || ''
      state.resetScheduleLoading()
    }
  }

  function syncStaffReferences(storeId: string, references: PlanningStaffReference[]): void {
    if (!state.stores.value.some((store) => store.id === storeId)) return

    const normalized = references.filter(
      (reference) => reference.id && reference.name && reference.storeId === storeId,
    )
    const currentByID = new Map(state.staff.value.map((member) => [member.id, member]))
    const otherStores = state.staff.value.filter((member) => member.storeId !== storeId)
    const nextStoreStaff = normalized.map((reference) => {
      const current = currentByID.get(reference.id)
      return current
        ? {
            ...current,
            name: reference.name,
            nick: reference.nick || reference.name,
            employeeCode: reference.employeeCode || '',
            jobRole: reference.role,
          }
        : createPlanningStaffMember(reference)
    })
    state.staff.value = [...otherStores, ...nextStoreStaff]
    const validStaffIDs = new Set(state.staff.value.map((member) => member.id))
    state.shifts.value = state.shifts.value.filter((shift) => validStaffIDs.has(shift.staffId))
  }

  function applyStaffContracts(contracts: PlanningStaffContract[]): void {
    state.staff.value = mergeStaffContracts(state.staff.value, state.activeStoreId.value, contracts)
  }

  function syncGoalReferences(
    storeId: string,
    month: string,
    references: PlanningGoalReference[],
  ): void {
    const store = state.stores.value.find((item) => item.id === storeId)
    if (!store || !/^\d{4}-\d{2}$/.test(month)) return
    syncStoreGoalReferences(store, month, references)
  }

  function persistedConfiguration(): PlanningPersistedConfiguration | undefined {
    const store = state.activeStore.value
    if (!store) return undefined
    return buildPersistedConfiguration(
      store,
      state.activePolicyId.value,
      state.policies.value,
      state.activeStaff.value,
      state.showWorkspaceHero.value,
    )
  }

  function resetActiveConfiguration(): void {
    const store = state.activeStore.value
    if (!store) return
    const defaults = createPlanningStore({
      id: store.id,
      name: store.name,
      city: store.city,
      storeType: store.locationType === 'shopping' ? 'shopping' : 'bairro',
    })
    store.operatingHoursByLocationType = defaults.operatingHoursByLocationType
    store.shiftTemplatesByLocationType = defaults.shiftTemplatesByLocationType
    store.coverageByLocationType = defaults.coverageByLocationType
    store.holidays = []
    store.exceptions = []
    state.policies.value = defaultPolicies.map((policy) => ({ ...policy }))
    state.activePolicyId.value = state.policies.value[0]?.id || ''
    state.showWorkspaceHero.value = true
    state.staff.value = state.staff.value.map((member) =>
      member.storeId === store.id
        ? createPlanningStaffMember({
            id: member.id,
            storeId: member.storeId,
            name: member.name,
            nick: member.nick,
            employeeCode: member.employeeCode,
            role: member.jobRole,
          })
        : member,
    )
  }

  function applyPersistedConfiguration(configuration: PlanningPersistedConfiguration): void {
    const store = state.activeStore.value
    if (!store) return
    state.showWorkspaceHero.value = configuration.uiPreferences?.showWorkspaceHero !== false
    store.operatingHoursByLocationType = structuredClone(configuration.operatingHoursByLocationType)
    store.shiftTemplatesByLocationType = {
      street: deriveShiftTemplatesFromOperatingHours(
        store.operatingHoursByLocationType.street,
        configuration.shiftTemplatesByLocationType.street,
      ),
      shopping: deriveShiftTemplatesFromOperatingHours(
        store.operatingHoursByLocationType.shopping,
        configuration.shiftTemplatesByLocationType.shopping,
      ),
    }
    if (configuration.coverageByLocationType) {
      store.coverageByLocationType = structuredClone(configuration.coverageByLocationType)
    }
    store.holidays = structuredClone(configuration.holidays || [])
    store.exceptions = structuredClone(configuration.exceptions || [])
    if (Array.isArray(configuration.policies) && configuration.policies.length) {
      state.policies.value = structuredClone(configuration.policies)
    }
    if (state.policies.value.some((policy) => policy.id === configuration.activePolicyId)) {
      state.activePolicyId.value = configuration.activePolicyId
    }
    state.staff.value = mergePersistedStaff(state.staff.value, store.id, configuration.staff || [])
  }

  return {
    syncStoreReferences,
    syncStaffReferences,
    applyStaffContracts,
    syncGoalReferences,
    persistedConfiguration,
    resetActiveConfiguration,
    applyPersistedConfiguration,
  }
}
