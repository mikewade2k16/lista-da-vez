import { defineStore } from 'pinia'

import type {
  PlanningPersistedConfiguration,
  PlanningPersistedSchedule,
  PlanningStaffContract,
  PlanningScheduleRevision,
  PlanningShift,
} from '~/domain/planning/types'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest } from '~/utils/api-client'

interface ConfigurationRecord {
  configuration: PlanningPersistedConfiguration
}

interface PlanningSnapshot {
  configuration: ConfigurationRecord | null
  schedule: PlanningPersistedSchedule | null
  contracts: PlanningStaffContract[]
  history: PlanningScheduleRevision[]
}

export const usePlanningPersistenceStore = defineStore('planning-persistence', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  async function load(storeId: string, weekStart: string): Promise<PlanningSnapshot> {
    const params = new URLSearchParams({ storeId, weekStart })
    return (await apiRequest(`/v1/operations/planning?${params.toString()}`)) as PlanningSnapshot
  }

  async function saveConfiguration(
    storeId: string,
    configuration: PlanningPersistedConfiguration,
  ): Promise<void> {
    await apiRequest('/v1/operations/planning/configuration', {
      method: 'PUT',
      body: { storeId, configuration },
    })
  }

  async function saveSchedule(
    storeId: string,
    weekStart: string,
    shifts: PlanningShift[],
    targetMonth: string,
    goalWeek: number,
    status: 'saved' | 'published' = 'saved',
    expectedVersion?: number,
  ): Promise<PlanningPersistedSchedule> {
    const response = (await apiRequest('/v1/operations/planning/schedule', {
      method: 'PUT',
      body: { storeId, weekStart, shifts, targetMonth, goalWeek, status, expectedVersion },
    })) as { schedule: PlanningPersistedSchedule }
    return response.schedule
  }

  async function reopenSchedule(
    storeId: string,
    weekStart: string,
    expectedVersion: number,
  ): Promise<PlanningPersistedSchedule> {
    const response = (await apiRequest('/v1/operations/planning/schedule/reopen', {
      method: 'POST',
      body: { storeId, weekStart, expectedVersion },
    })) as { schedule: PlanningPersistedSchedule }
    return response.schedule
  }

  async function generateSchedule(
    storeId: string,
    weekStart: string,
    targetMonth: string,
    goalWeek: number,
    expectedVersion?: number,
  ): Promise<PlanningPersistedSchedule> {
    const response = (await apiRequest('/v1/operations/planning/schedule/generate', {
      method: 'POST',
      body: { storeId, weekStart, targetMonth, goalWeek, expectedVersion },
    })) as { schedule: PlanningPersistedSchedule }
    return response.schedule
  }

  return {
    load,
    saveConfiguration,
    saveSchedule,
    generateSchedule,
    reopenSchedule,
  }
})
