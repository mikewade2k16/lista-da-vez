import type {
  PlanningGoalPeriod,
  PlanningGoalReference,
  PlanningLaborPolicy,
  PlanningPersistedConfiguration,
  PlanningStaffMember,
  PlanningStaffContract,
  PlanningStore,
} from './types'
import { goalWeekCount, goalWeekPeriods } from '~/utils/goal-periods'

export function syncStoreGoalReferences(
  store: PlanningStore,
  month: string,
  references: PlanningGoalReference[],
): void {
  const targets = { month: 0, p1: 0, p2: 0, p3: 0, p4: 0, p5: 0 }
  for (const reference of references) {
    if (
      reference.scope !== 'store' ||
      reference.storeId !== store.id ||
      reference.month !== month
    ) {
      continue
    }
    const period: PlanningGoalPeriod =
      reference.week >= 1 && reference.week <= goalWeekCount(month)
        ? (`p${reference.week}` as PlanningGoalPeriod)
        : 'month'
    targets[period] = Math.max(0, Number(reference.monthlyGoal) || 0)
  }
  if (targets.month > 0) {
    const distributedWeeklyTarget = targets.month / goalWeekCount(month)
    for (const period of goalWeekPeriods(month)) {
      if (targets[period] <= 0) targets[period] = distributedWeeklyTarget
    }
  }
  store.goalsByMonth[month] = targets
}

export function mergeStaffContracts(
  currentStaff: PlanningStaffMember[],
  storeId: string,
  contracts: PlanningStaffContract[],
): PlanningStaffMember[] {
  const contractByID = new Map(contracts.map((contract) => [contract.consultantId, contract]))
  return currentStaff.map((member) => {
    const contract = member.storeId === storeId ? contractByID.get(member.id) : undefined
    return contract
      ? {
          ...member,
          weeklyHours: contract.weeklyHours,
          maxDailyHours: contract.maxDailyHours,
          targetWeight: contract.targetWeight,
          availableDays: [...contract.availableWeekdays],
        }
      : member
  })
}

export function buildPersistedConfiguration(
  store: PlanningStore,
  activePolicyId: string,
  policies: PlanningLaborPolicy[],
  staff: PlanningStaffMember[],
  showWorkspaceHero: boolean,
): PlanningPersistedConfiguration {
  return {
    uiPreferences: { showWorkspaceHero },
    activePolicyId,
    operatingHoursByLocationType: {
      street: store.operatingHoursByLocationType.street.map((day) => ({ ...day })),
      shopping: store.operatingHoursByLocationType.shopping.map((day) => ({ ...day })),
    },
    shiftTemplatesByLocationType: {
      street: store.shiftTemplatesByLocationType.street.map((template) => ({ ...template })),
      shopping: store.shiftTemplatesByLocationType.shopping.map((template) => ({ ...template })),
    },
    coverageByLocationType: {
      street: { ...store.coverageByLocationType.street },
      shopping: { ...store.coverageByLocationType.shopping },
    },
    holidays: store.holidays.map((holiday) => ({ ...holiday })),
    exceptions: store.exceptions.map((exception) => ({ ...exception })),
    policies: policies.map((policy) => ({ ...policy })),
    staff: staff.map((member) => ({ ...member, availableDays: [...member.availableDays] })),
  }
}

export function mergePersistedStaff(
  currentStaff: PlanningStaffMember[],
  storeId: string,
  persistedStaff: PlanningStaffMember[],
): PlanningStaffMember[] {
  const persistedByID = new Map(persistedStaff.map((member) => [member.id, member]))
  return currentStaff.map((member) => {
    const persisted = member.storeId === storeId ? persistedByID.get(member.id) : undefined
    if (!persisted) return member
    return {
      ...member,
      ...structuredClone(persisted),
      id: member.id,
      storeId: member.storeId,
      name: member.name,
      nick: member.nick,
      employeeCode: member.employeeCode,
      jobRole: member.jobRole,
    }
  })
}
