import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import type {
  ModulePriority,
  ModuleStatus,
  RoadmapModule,
  RoadmapRule,
  RuleCategory,
} from '~/components/roadmap/roadmap-data'
import { ROADMAP_MODULES, ROADMAP_RULES } from '~/components/roadmap/roadmap-data'

interface ApiModule {
  id: string
  sourceId: string
  accountId?: string | null
  isGlobal: boolean
  label: string
  route: string
  status: ModuleStatus
  priority: ModulePriority
  category?: string
  description: string
  scope: string[]
  dependsOn: string[]
  sortOrder: number
  createdAt: string
  updatedAt: string
}

interface ApiRule {
  id: string
  sourceId: string
  accountId?: string | null
  isGlobal: boolean
  category: RuleCategory
  title: string
  body: string
  why: string
  appliesWhen: string
  sortOrder: number
  createdAt: string
  updatedAt: string
}

export interface RoadmapModuleRow extends RoadmapModule {
  id: string
  sourceId: string
  isGlobal: boolean
}

export interface RoadmapRuleRow extends RoadmapRule {
  sourceId: string
  isGlobal: boolean
}

export interface DashboardTaskRow {
  id: string
  title: string
  status: string | null
  priority: string
  archived: boolean
  boardId: string
  columnId: string | null
  responsibleUserId: string | null
}

export interface DashboardCounts {
  total: number
  idea: number
  planning: number
  inProgress: number
  done: number
}

export interface DashboardModuleRow {
  module: RoadmapModuleRow
  tasks: DashboardTaskRow[]
  counts: DashboardCounts
}

export function normalizeDashboardTaskStatus(value: unknown) {
  return String(value ?? '')
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
}

export function isDashboardTaskDone(task: Pick<DashboardTaskRow, 'status'>) {
  const status = normalizeDashboardTaskStatus(task.status)
  return [
    'done',
    'concluido',
    'concluida',
    'finalizado',
    'finalizada',
    'finished',
    'complete',
    'completed',
  ].includes(status)
}

export function dashboardTaskProgress(task: Pick<DashboardTaskRow, 'status'>) {
  return isDashboardTaskDone(task) ? 100 : 0
}

export function dashboardTaskShare(totalTasks: number) {
  const total = Math.max(0, Number(totalTasks) || 0)
  return total > 0 ? 100 / total : 0
}

export function dashboardCountsFromTasks(tasks: DashboardTaskRow[]): DashboardCounts {
  const counts: DashboardCounts = {
    total: Array.isArray(tasks) ? tasks.length : 0,
    idea: 0,
    planning: 0,
    inProgress: 0,
    done: 0,
  }
  for (const task of tasks || []) {
    const status = normalizeDashboardTaskStatus(task.status)
    if (status === 'idea' || status === 'ideia') {
      counts.idea += 1
    } else if (
      [
        'in_progress',
        'doing',
        'em_andamento',
        'running',
        'execucao',
        'aguardando_aprovacao',
        'aprovada',
        'approved',
      ].includes(status)
    ) {
      counts.inProgress += 1
    } else if (isDashboardTaskDone(task)) {
      counts.done += 1
    } else {
      counts.planning += 1
    }
  }
  return counts
}

export function formatDashboardPercent(value: number) {
  const normalized = Math.max(0, Math.min(100, Number(value) || 0))
  return Number.isInteger(normalized) ? String(normalized) : normalized.toFixed(1)
}

export function dashboardModuleProgress(entry: Pick<DashboardModuleRow, 'tasks' | 'counts'>) {
  const taskCount = Array.isArray(entry.tasks) ? entry.tasks.length : 0
  const total = taskCount || Math.max(0, Number(entry.counts?.total) || 0)
  if (!total) return 0
  const done = taskCount
    ? entry.tasks.filter((task) => isDashboardTaskDone(task)).length
    : Math.max(0, Number(entry.counts?.done) || 0)
  return Math.round((done / total) * 100)
}

interface ApiDashboard {
  modules: Array<{
    module: ApiModule
    tasks: DashboardTaskRow[]
    counts: DashboardCounts
  }>
}

function moduleFromApi(m: ApiModule): RoadmapModuleRow {
  return {
    id: m.id,
    sourceId: m.sourceId,
    isGlobal: m.isGlobal,
    label: m.label,
    route: m.route,
    status: m.status,
    priority: m.priority,
    description: m.description,
    scope: Array.isArray(m.scope) ? m.scope : [],
    dependsOn: Array.isArray(m.dependsOn) ? m.dependsOn : [],
    category: (m.category || undefined) as RoadmapModule['category'],
  }
}

function ruleFromApi(r: ApiRule): RoadmapRuleRow {
  return {
    id: r.id,
    sourceId: r.sourceId,
    isGlobal: r.isGlobal,
    category: r.category,
    title: r.title,
    body: r.body,
    why: r.why,
    appliesWhen: r.appliesWhen,
  }
}

function staticModulesFallback(): RoadmapModuleRow[] {
  return ROADMAP_MODULES.map((m) => ({
    ...m,
    id: m.id,
    sourceId: m.id,
    isGlobal: true,
  }))
}

function staticRulesFallback(): RoadmapRuleRow[] {
  return ROADMAP_RULES.map((r) => ({
    ...r,
    sourceId: r.id,
    isGlobal: true,
  }))
}

export interface UpdateModulePayload {
  status?: ModuleStatus
  priority?: ModulePriority
  description?: string
  label?: string
  route?: string
  category?: string
  scope?: string[]
  dependsOn?: string[]
  sortOrder?: number
}

export interface UpdateRulePayload {
  title?: string
  body?: string
  why?: string
  appliesWhen?: string
  category?: RuleCategory
  sortOrder?: number
}

export interface CreateModulePayload {
  sourceId: string
  label: string
  route?: string
  status: ModuleStatus
  priority: ModulePriority
  category?: string
  description: string
  scope?: string[]
  dependsOn?: string[]
}

export interface CreateRulePayload {
  sourceId: string
  category: RuleCategory
  title: string
  body: string
  why?: string
  appliesWhen?: string
}

export const useRoadmapStore = defineStore('roadmap', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const rawApiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const accountId = computed(() => {
    const direct = String(auth.activeTenantId || '').trim()
    if (direct) return direct
    const ctx = auth.tenantContext as Array<{ id?: string }> | undefined
    return String(ctx?.[0]?.id || '').trim()
  })

  async function apiRequest<T = unknown>(
    path: string,
    options: Record<string, unknown> = {},
  ): Promise<T> {
    await auth.ensureSession()
    const resolvedAccountId = accountId.value
    if (!resolvedAccountId) {
      throw new Error('Conta ativa nao disponivel para Roadmap.')
    }
    return rawApiRequest(path, {
      ...options,
      headers: {
        ...((options.headers as Record<string, string>) || {}),
        'X-Account-Id': resolvedAccountId,
      },
    }) as Promise<T>
  }

  const modules = ref<RoadmapModuleRow[]>([])
  const rules = ref<RoadmapRuleRow[]>([])
  const dashboard = ref<DashboardModuleRow[]>([])
  const loading = ref(false)
  const error = ref('')
  const backendAvailable = ref(true)

  const hasData = computed(() => modules.value.length > 0 || rules.value.length > 0)

  function shouldUseStaticFallback(err: unknown) {
    const status =
      (err as { status?: number; statusCode?: number })?.status ??
      (err as { status?: number; statusCode?: number })?.statusCode ??
      0
    return status === 0 || status === 404 || status === 500 || status === 501 || status === 503
  }

  async function fetchAll() {
    loading.value = true
    error.value = ''
    try {
      const [modulesResult, rulesResult] = await Promise.allSettled([
        apiRequest<{ modules: ApiModule[] }>('/v1/roadmap/modules'),
        apiRequest<{ rules: ApiRule[] }>('/v1/roadmap/rules'),
      ])

      if (modulesResult.status === 'fulfilled') {
        modules.value = (modulesResult.value.modules || []).map(moduleFromApi)
      } else if (shouldUseStaticFallback(modulesResult.reason)) {
        modules.value = staticModulesFallback()
      } else {
        throw modulesResult.reason
      }

      if (rulesResult.status === 'fulfilled') {
        rules.value = (rulesResult.value.rules || []).map(ruleFromApi)
      } else if (shouldUseStaticFallback(rulesResult.reason)) {
        rules.value = staticRulesFallback()
      } else {
        throw rulesResult.reason
      }

      backendAvailable.value =
        modulesResult.status === 'fulfilled' && rulesResult.status === 'fulfilled'

      if (backendAvailable.value) {
        await fetchDashboard()
      } else {
        dashboard.value = []
      }
    } catch (err) {
      const message = getApiErrorMessage(err, 'Erro ao carregar roadmap')
      error.value = message || 'Erro ao carregar roadmap'
    } finally {
      loading.value = false
    }
  }

  async function updateModule(id: string, payload: UpdateModulePayload) {
    const current = modules.value.find((m) => m.id === id)
    if (!current) return
    const body = {
      sourceId: current.sourceId,
      label: payload.label ?? current.label,
      route: payload.route ?? current.route,
      status: payload.status ?? current.status,
      priority: payload.priority ?? current.priority,
      category: payload.category ?? current.category ?? '',
      description: payload.description ?? current.description,
      scope: payload.scope ?? current.scope ?? [],
      dependsOn: payload.dependsOn ?? current.dependsOn ?? [],
      sortOrder: payload.sortOrder ?? 100,
    }
    const response = await apiRequest<{ module: ApiModule }>(`/v1/roadmap/modules/${id}`, {
      method: 'PUT',
      body,
    })
    const updated = moduleFromApi(response.module)
    const idx = modules.value.findIndex((m) => m.sourceId === updated.sourceId)
    if (idx >= 0) modules.value.splice(idx, 1, updated)
    else modules.value.push(updated)
    return updated
  }

  async function updateRule(id: string, payload: UpdateRulePayload) {
    const current = rules.value.find((r) => r.id === id)
    if (!current) return
    const body = {
      sourceId: current.sourceId,
      category: payload.category ?? current.category,
      title: payload.title ?? current.title,
      body: payload.body ?? current.body,
      why: payload.why ?? current.why ?? '',
      appliesWhen: payload.appliesWhen ?? current.appliesWhen ?? '',
      sortOrder: payload.sortOrder ?? 100,
    }
    const response = await apiRequest<{ rule: ApiRule }>(`/v1/roadmap/rules/${id}`, {
      method: 'PUT',
      body,
    })
    const updated = ruleFromApi(response.rule)
    const idx = rules.value.findIndex((r) => r.sourceId === updated.sourceId)
    if (idx >= 0) rules.value.splice(idx, 1, updated)
    else rules.value.push(updated)
    return updated
  }

  async function createModule(payload: CreateModulePayload) {
    const body = {
      sourceId: payload.sourceId,
      label: payload.label,
      route: payload.route ?? '',
      status: payload.status,
      priority: payload.priority,
      category: payload.category ?? '',
      description: payload.description,
      scope: payload.scope ?? [],
      dependsOn: payload.dependsOn ?? [],
      sortOrder: 100,
    }
    const response = await apiRequest<{ module: ApiModule }>('/v1/roadmap/modules', {
      method: 'POST',
      body,
    })
    const created = moduleFromApi(response.module)
    const idx = modules.value.findIndex((m) => m.sourceId === created.sourceId)
    if (idx >= 0) modules.value.splice(idx, 1, created)
    else modules.value.push(created)
    return created
  }

  async function deleteModule(id: string) {
    await apiRequest(`/v1/roadmap/modules/${id}`, { method: 'DELETE' })
    await fetchAll()
  }

  async function createRule(payload: CreateRulePayload) {
    const body = {
      sourceId: payload.sourceId,
      category: payload.category,
      title: payload.title,
      body: payload.body,
      why: payload.why ?? '',
      appliesWhen: payload.appliesWhen ?? '',
      sortOrder: 100,
    }
    const response = await apiRequest<{ rule: ApiRule }>('/v1/roadmap/rules', {
      method: 'POST',
      body,
    })
    const created = ruleFromApi(response.rule)
    const idx = rules.value.findIndex((r) => r.sourceId === created.sourceId)
    if (idx >= 0) rules.value.splice(idx, 1, created)
    else rules.value.push(created)
    return created
  }

  async function deleteRule(id: string) {
    await apiRequest(`/v1/roadmap/rules/${id}`, { method: 'DELETE' })
    await fetchAll()
  }

  async function fetchDashboard() {
    if (!backendAvailable.value) return
    try {
      const response = await apiRequest<ApiDashboard>('/v1/roadmap/dashboard')
      dashboard.value = (response.modules || []).map((entry) => ({
        module: moduleFromApi(entry.module),
        tasks: Array.isArray(entry.tasks) ? entry.tasks : [],
        counts: dashboardCountsFromTasks(Array.isArray(entry.tasks) ? entry.tasks : []),
      }))
    } catch (err) {
      console.error('roadmap.dashboard fetch failed', err)
    }
  }

  function downloadMarkdownUrl() {
    return `/v1/roadmap/rules.md`
  }

  return {
    modules,
    rules,
    dashboard,
    loading,
    error,
    backendAvailable,
    hasData,
    fetchAll,
    fetchDashboard,
    createModule,
    updateModule,
    deleteModule,
    createRule,
    updateRule,
    deleteRule,
    downloadMarkdownUrl,
  }
})
