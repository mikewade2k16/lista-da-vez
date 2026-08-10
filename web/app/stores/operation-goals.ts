import { ref } from 'vue'
import { defineStore } from 'pinia'

import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

type LooseRecord = Record<string, unknown>

type GoalsResponse = {
  items?: LooseRecord[]
  summary?: LooseRecord
}

type ConsultantsResponse = {
  consultants?: LooseRecord[]
}

type GoalMutationResponse = {
  goal?: LooseRecord
}

export type OperationGoalTarget = {
  id: string
  tenantId: string
  month: string
  // 0 = meta mensal (mes inteiro); 1..4 = semana do mes (S1=1-7 ... S4=22-fim).
  week: number
  scope: 'store' | 'consultant'
  storeId: string
  storeCode: string
  storeName: string
  consultantId: string
  consultantName: string
  monthlyGoal: number
  avgTicketGoal: number
  conversionGoal: number
  paGoal: number
  createdAt: string
  updatedAt: string
}

export type OperationGoalSummary = {
  month: string
  totalRows: number
  storeRows: number
  consultantRows: number
  storesCovered: number
  consultantsCovered: number
  totalMonthlyGoal: number
}

export type OperationGoalConsultant = {
  id: string
  name: string
  nick: string
  role: string
  employeeCode: string
}

type UpdateGoalOptions = {
  reload?: boolean
  skipLoadingIndicator?: boolean
}

type CreateGoalOptions = {
  reload?: boolean
  skipLoadingIndicator?: boolean
}

function normalizeText(value: unknown) {
  return String(value || '').trim()
}

function parseLocaleNumber(value: unknown) {
  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : 0
  }

  const normalized = normalizeText(value)
  if (!normalized) {
    return 0
  }

  let sanitized = normalized
    .replace(/\s+/g, '')
    .replace(/[R$r$]/gi, '')
    .replace(/[^0-9,.-]/g, '')

  const hasComma = sanitized.includes(',')
  const hasDot = sanitized.includes('.')

  if (hasComma && hasDot) {
    if (sanitized.lastIndexOf(',') > sanitized.lastIndexOf('.')) {
      sanitized = sanitized.replace(/\./g, '').replace(',', '.')
    } else {
      sanitized = sanitized.replace(/,/g, '')
    }
  } else if (hasComma) {
    sanitized = sanitized.replace(/\./g, '').replace(',', '.')
  }

  const parsed = Number(sanitized)
  return Number.isFinite(parsed) ? parsed : 0
}

function normalizeNumber(value: unknown) {
  return Math.max(0, parseLocaleNumber(value))
}

function clampPercent(value: unknown) {
  return Math.min(100, normalizeNumber(value))
}

function currentMonth() {
  const now = new Date()
  return `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, '0')}`
}

function normalizeMonth(value: unknown) {
  const normalized = normalizeText(value)
  return /^\d{4}-\d{2}$/.test(normalized) ? normalized : currentMonth()
}

function emptySummary(month = currentMonth()): OperationGoalSummary {
  return {
    month,
    totalRows: 0,
    storeRows: 0,
    consultantRows: 0,
    storesCovered: 0,
    consultantsCovered: 0,
    totalMonthlyGoal: 0,
  }
}

function buildSummaryFromGoals(
  rows: OperationGoalTarget[] = [],
  month = currentMonth(),
): OperationGoalSummary {
  const storesCovered = new Set<string>()
  const consultantsCovered = new Set<string>()
  const summary = emptySummary(month)

  for (const row of rows) {
    summary.totalRows += 1
    summary.totalMonthlyGoal += normalizeNumber(row.monthlyGoal)
    if (row.storeId) {
      storesCovered.add(row.storeId)
    }

    if (row.scope === 'consultant') {
      summary.consultantRows += 1
      if (row.consultantId) {
        consultantsCovered.add(row.consultantId)
      }
    } else {
      summary.storeRows += 1
    }
  }

  summary.storesCovered = storesCovered.size
  summary.consultantsCovered = consultantsCovered.size
  return summary
}

function normalizeWeek(value: unknown): number {
  const parsed = Math.trunc(Number(value) || 0)
  if (parsed < 0) return 0
  if (parsed > 4) return 4
  return parsed
}

function normalizeGoal(goal: LooseRecord = {}): OperationGoalTarget {
  return {
    id: normalizeText(goal.id),
    tenantId: normalizeText(goal.tenantId),
    month: normalizeMonth(goal.month),
    week: normalizeWeek(goal.week),
    scope: normalizeText(goal.scope) === 'consultant' ? 'consultant' : 'store',
    storeId: normalizeText(goal.storeId),
    storeCode: normalizeText(goal.storeCode),
    storeName: normalizeText(goal.storeName),
    consultantId: normalizeText(goal.consultantId),
    consultantName: normalizeText(goal.consultantName),
    monthlyGoal: normalizeNumber(goal.monthlyGoal),
    avgTicketGoal: normalizeNumber(goal.avgTicketGoal),
    conversionGoal: clampPercent(goal.conversionGoal),
    paGoal: normalizeNumber(goal.paGoal),
    createdAt: normalizeText(goal.createdAt),
    updatedAt: normalizeText(goal.updatedAt),
  }
}

function normalizeSummary(summary: LooseRecord = {}, month = currentMonth()): OperationGoalSummary {
  return {
    month: normalizeMonth(summary.month || month),
    totalRows: Math.max(0, Number(summary.totalRows || 0) || 0),
    storeRows: Math.max(0, Number(summary.storeRows || 0) || 0),
    consultantRows: Math.max(0, Number(summary.consultantRows || 0) || 0),
    storesCovered: Math.max(0, Number(summary.storesCovered || 0) || 0),
    consultantsCovered: Math.max(0, Number(summary.consultantsCovered || 0) || 0),
    totalMonthlyGoal: normalizeNumber(summary.totalMonthlyGoal),
  }
}

function normalizeConsultant(consultant: LooseRecord = {}): OperationGoalConsultant {
  return {
    id: normalizeText(consultant.id),
    name: normalizeText(consultant.name),
    nick: normalizeText(consultant.nick) || normalizeText(consultant.name),
    role: normalizeText(consultant.role),
    employeeCode: normalizeText(consultant.employeeCode),
  }
}

function buildCreatePayload(payload: LooseRecord = {}) {
  return {
    storeId: normalizeText(payload.storeId),
    consultantId: normalizeText(payload.consultantId),
    month: normalizeMonth(payload.month),
    week: normalizeWeek(payload.week),
    monthlyGoal: normalizeNumber(payload.monthlyGoal),
    avgTicketGoal: normalizeNumber(payload.avgTicketGoal),
    conversionGoal: clampPercent(payload.conversionGoal),
    paGoal: normalizeNumber(payload.paGoal),
  }
}

function buildUpdatePayload(payload: LooseRecord = {}) {
  return {
    monthlyGoal: normalizeNumber(payload.monthlyGoal),
    avgTicketGoal: normalizeNumber(payload.avgTicketGoal),
    conversionGoal: clampPercent(payload.conversionGoal),
    paGoal: normalizeNumber(payload.paGoal),
  }
}

export const useOperationGoalsStore = defineStore('operation-goals', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const goals = ref<OperationGoalTarget[]>([])
  const summary = ref<OperationGoalSummary>(emptySummary())
  const consultantsByStore = ref<Record<string, OperationGoalConsultant[]>>({})
  const pending = ref(false)
  const consultantsPending = ref(false)
  const saving = ref(false)
  const ready = ref(false)
  const errorMessage = ref('')
  const lastFilters = ref({
    tenantId: '',
    storeId: '',
    month: currentMonth(),
  })
  const recentLocalMutations = ref<Record<string, number>>({})

  function mutationKey(action: string, resourceId: string) {
    const normalizedId = normalizeText(resourceId)
    if (!normalizedId) {
      return ''
    }
    return `${normalizeText(action) || 'updated'}:${normalizedId}`
  }

  function pruneRecentLocalMutations(now = Date.now()) {
    const next: Record<string, number> = {}
    for (const [key, timestamp] of Object.entries(recentLocalMutations.value)) {
      if (now - timestamp <= 5000) {
        next[key] = timestamp
      }
    }
    recentLocalMutations.value = next
  }

  function markRecentLocalMutation(action: string, resourceId: string) {
    const key = mutationKey(action, resourceId)
    if (!key) {
      return
    }

    pruneRecentLocalMutations()
    recentLocalMutations.value = {
      ...recentLocalMutations.value,
      [key]: Date.now(),
    }
  }

  function shouldSkipRealtimeUpdate(action: string, resourceId: string) {
    const now = Date.now()
    pruneRecentLocalMutations(now)

    const key = mutationKey(action, resourceId)
    if (!key) {
      return false
    }

    return Boolean(recentLocalMutations.value[key] && now - recentLocalMutations.value[key] <= 5000)
  }

  function goalMatchesLastFilters(goal: OperationGoalTarget) {
    const filters = lastFilters.value
    if (filters.tenantId && normalizeText(goal.tenantId) !== normalizeText(filters.tenantId)) {
      return false
    }
    if (filters.storeId && normalizeText(goal.storeId) !== normalizeText(filters.storeId)) {
      return false
    }
    return normalizeMonth(goal.month) === normalizeMonth(filters.month)
  }

  function replaceGoalLocally(goal: OperationGoalTarget) {
    if (!goal.id || !goalMatchesLastFilters(goal)) {
      return
    }

    const index = goals.value.findIndex((item) => item.id === goal.id)
    if (index === -1) {
      goals.value = [...goals.value, goal]
    } else {
      goals.value = goals.value.map((item) => (item.id === goal.id ? goal : item))
    }

    summary.value = buildSummaryFromGoals(goals.value, lastFilters.value.month)
    ready.value = true
  }

  function clearConsultantsCache(storeId = '') {
    const normalizedStoreId = normalizeText(storeId)
    if (!normalizedStoreId) {
      consultantsByStore.value = {}
      return
    }

    const { [normalizedStoreId]: _removed, ...rest } = consultantsByStore.value
    consultantsByStore.value = rest
  }

  async function loadGoals(filters: LooseRecord = {}) {
    if (!auth.isAuthenticated) {
      goals.value = []
      summary.value = emptySummary(lastFilters.value.month)
      ready.value = false
      errorMessage.value = ''
      return []
    }

    const tenantId = normalizeText(
      filters.tenantId || auth.activeTenantId || auth.tenantContext?.[0]?.id,
    )
    const storeId = normalizeText(filters.storeId)
    const month = normalizeMonth(filters.month || lastFilters.value.month)
    lastFilters.value = { tenantId, storeId, month }

    pending.value = true
    errorMessage.value = ''

    try {
      const params = new URLSearchParams()
      if (tenantId) {
        params.set('tenantId', tenantId)
      }
      if (storeId) {
        params.set('storeId', storeId)
      }
      params.set('month', month)

      const response = (await apiRequest(
        `/v1/operations/goals?${params.toString()}`,
      )) as GoalsResponse
      const loadedGoals = Array.isArray(response?.items) ? response.items.map(normalizeGoal) : []
      goals.value = loadedGoals
      summary.value = normalizeSummary(response?.summary, month)
      ready.value = true
      return loadedGoals
    } catch (error) {
      goals.value = []
      summary.value = emptySummary(month)
      errorMessage.value = getApiErrorMessage(error, 'Nao foi possivel carregar as metas.')
      throw error
    } finally {
      pending.value = false
    }
  }

  async function loadConsultants(storeId: string) {
    const normalizedStoreId = normalizeText(storeId)
    if (!auth.isAuthenticated || !normalizedStoreId) {
      return []
    }

    if (Array.isArray(consultantsByStore.value[normalizedStoreId])) {
      return consultantsByStore.value[normalizedStoreId]
    }

    consultantsPending.value = true
    try {
      const response = (await apiRequest(
        `/v1/consultants?storeId=${encodeURIComponent(normalizedStoreId)}`,
      )) as ConsultantsResponse
      const consultants = Array.isArray(response?.consultants)
        ? response.consultants.map(normalizeConsultant)
        : []
      consultantsByStore.value = {
        ...consultantsByStore.value,
        [normalizedStoreId]: consultants,
      }
      return consultants
    } finally {
      consultantsPending.value = false
    }
  }

  async function createGoal(payload: LooseRecord = {}, options: CreateGoalOptions = {}) {
    if (!auth.isAuthenticated) {
      return { ok: false, message: 'Sessao indisponivel.' }
    }

    saving.value = true
    try {
      const response = (await apiRequest('/v1/operations/goals', {
        method: 'POST',
        body: buildCreatePayload(payload),
        skipLoadingIndicator: options.skipLoadingIndicator ?? options.reload === false,
      })) as GoalMutationResponse

      const goal = normalizeGoal(response?.goal)
      if (options.reload === false) {
        replaceGoalLocally(goal)
      } else {
        await loadGoals(lastFilters.value)
      }
      return {
        ok: true,
        goal,
      }
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, 'Nao foi possivel cadastrar a meta.'),
      }
    } finally {
      saving.value = false
    }
  }

  async function updateGoal(
    goalId: string,
    payload: LooseRecord = {},
    options: UpdateGoalOptions = {},
  ) {
    if (!auth.isAuthenticated) {
      return { ok: false, message: 'Sessao indisponivel.' }
    }

    const normalizedGoalId = normalizeText(goalId)
    markRecentLocalMutation('updated', normalizedGoalId)

    saving.value = true
    try {
      const response = (await apiRequest(
        `/v1/operations/goals/${encodeURIComponent(normalizedGoalId)}`,
        {
          method: 'PATCH',
          body: buildUpdatePayload(payload),
          skipLoadingIndicator: options.skipLoadingIndicator ?? options.reload === false,
        },
      )) as GoalMutationResponse

      const goal = normalizeGoal(response?.goal)

      if (options.reload === false) {
        replaceGoalLocally(goal)
      } else {
        await loadGoals(lastFilters.value)
      }

      return {
        ok: true,
        goal,
      }
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, 'Nao foi possivel atualizar a meta.'),
      }
    } finally {
      saving.value = false
    }
  }

  async function deleteGoal(goalId: string) {
    if (!auth.isAuthenticated) {
      return { ok: false, message: 'Sessao indisponivel.' }
    }

    const normalizedGoalId = normalizeText(goalId)
    markRecentLocalMutation('deleted', normalizedGoalId)

    saving.value = true
    try {
      await apiRequest(`/v1/operations/goals/${encodeURIComponent(normalizedGoalId)}`, {
        method: 'DELETE',
      })

      await loadGoals(lastFilters.value)
      return { ok: true }
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, 'Nao foi possivel excluir a meta.'),
      }
    } finally {
      saving.value = false
    }
  }

  return {
    goals,
    summary,
    consultantsByStore,
    pending,
    consultantsPending,
    saving,
    ready,
    errorMessage,
    lastFilters,
    loadGoals,
    loadConsultants,
    clearConsultantsCache,
    shouldSkipRealtimeUpdate,
    createGoal,
    updateGoal,
    deleteGoal,
  }
})
