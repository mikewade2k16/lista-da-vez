import { computed, reactive, ref, watch, type ComputedRef, type Ref } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

type RelationsSource<T> = T | Ref<T> | ComputedRef<T> | (() => T)

export type TaskRelationStatus = 'idle' | 'loading' | 'ready' | 'error'

// Espelho do shape devolvido por GET /v1/tasks/{taskId}/relations:expand. A `metadataCache` e'
// generica (depende do modulo resolvido); para campos conhecidos como `status` e `url`,
// extraimos com checagem leve no consumer.
export interface TaskRelation {
  id: string
  taskId: string
  module: string
  resourceType: string
  resourceId: string
  labelCache: string
  metadataCache: Record<string, unknown>
  refreshedAt: string
}

interface TaskRelationsOptions {
  enabled: RelationsSource<boolean>
  taskId: RelationsSource<string>
  // Eventos task.* do canal realtime — passar `lastEvent` de useTasksRealtime para invalidar o
  // cache automaticamente quando `task.relation_added`/`task.relation_removed` chegarem.
  realtimeEvent?: RelationsSource<{ type?: string; taskId?: string } | null>
}

function sourceValue<T>(source: RelationsSource<T> | undefined, fallback: T): T {
  if (typeof source === 'function') {
    const value = (source as () => T)()
    return value == null ? fallback : value
  }
  if (source && typeof source === 'object' && 'value' in source) {
    const value = (source as Ref<T>).value
    return value == null ? fallback : value
  }
  return source == null ? fallback : source
}

function normalizeText(value: unknown, max = 240) {
  return String(value ?? '').trim().slice(0, max)
}

function normalizeRelation(raw: Record<string, unknown>): TaskRelation {
  return {
    id: normalizeText(raw.id, 120),
    taskId: normalizeText(raw.taskId, 120),
    module: normalizeText(raw.module, 80),
    resourceType: normalizeText(raw.resourceType, 80),
    resourceId: normalizeText(raw.resourceId, 200),
    labelCache: normalizeText(raw.labelCache, 240),
    metadataCache: (raw.metadataCache && typeof raw.metadataCache === 'object' && !Array.isArray(raw.metadataCache))
      ? raw.metadataCache as Record<string, unknown>
      : {},
    refreshedAt: normalizeText(raw.refreshedAt, 40)
  }
}

// useTaskRelations carrega vinculos cross-module (crm/erp/operations) de uma task via o endpoint
// T4 `GET /v1/tasks/{taskId}/relations:expand`. Cache local por taskId; eventos realtime
// `task.relation_added`/`task.relation_removed` invalidam a entrada e disparam refetch.
export function useTaskRelations(options: TaskRelationsOptions) {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const request = createApiRequest(runtimeConfig, () => auth.accessToken)

  const cache = reactive<Record<string, TaskRelation[]>>({})
  const status = ref<TaskRelationStatus>('idle')
  const errorMessage = ref('')
  const activeTaskId = ref('')

  const relations = computed<TaskRelation[]>(() => {
    const id = activeTaskId.value
    if (!id) return []
    return cache[id] || []
  })

  async function fetchRelations(taskId: string, { force = false }: { force?: boolean } = {}) {
    const id = normalizeText(taskId, 120)
    if (!id) return []
    if (!force && Array.isArray(cache[id])) {
      return cache[id]
    }
    try {
      status.value = 'loading'
      errorMessage.value = ''
      const response = await request(`/v1/tasks/${encodeURIComponent(id)}/relations:expand`)
      const list = Array.isArray(response?.relations)
        ? (response.relations as Record<string, unknown>[]).map(normalizeRelation)
        : []
      cache[id] = list
      status.value = 'ready'
      return list
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Nao foi possivel carregar os vinculos.')
      status.value = 'error'
      return cache[id] || []
    }
  }

  function invalidate(taskId: string) {
    const id = normalizeText(taskId, 120)
    if (!id) return
    delete cache[id]
  }

  async function refresh() {
    const id = activeTaskId.value
    if (!id) return
    await fetchRelations(id, { force: true })
  }

  // Recarrega quando o consumer abrir um novo modal/task.
  watch(
    () => ({
      enabled: Boolean(sourceValue(options.enabled, false)),
      taskId: normalizeText(sourceValue(options.taskId, ''), 120)
    }),
    ({ enabled, taskId }) => {
      if (!enabled || !taskId) {
        activeTaskId.value = ''
        status.value = 'idle'
        return
      }
      activeTaskId.value = taskId
      void fetchRelations(taskId)
    },
    { immediate: true }
  )

  // Invalidacao via realtime: quando o canal `tasks` publica `task.relation_added` ou
  // `task.relation_removed` para a task ativa, descarta cache e refetcha.
  if (options.realtimeEvent !== undefined) {
    watch(
      () => sourceValue(options.realtimeEvent, null),
      (event) => {
        if (!event || typeof event !== 'object') return
        const eventType = normalizeText((event as { type?: unknown }).type, 80)
        if (eventType !== 'task.relation_added' && eventType !== 'task.relation_removed') return
        const eventTaskId = normalizeText((event as { taskId?: unknown }).taskId, 120)
        if (!eventTaskId) return
        invalidate(eventTaskId)
        if (eventTaskId === activeTaskId.value) {
          void fetchRelations(eventTaskId, { force: true })
        }
      }
    )
  }

  return {
    status,
    errorMessage,
    relations,
    fetchRelations,
    refresh,
    invalidate
  }
}
