import { computed, reactive, ref, watch, type ComputedRef, type Ref } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { useUsersStore } from '~/stores/users'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { compactUserLabel } from '../utils/user-label'

type CommentsSource<T> = T | Ref<T> | ComputedRef<T> | (() => T)

export type TaskCommentStatus = 'idle' | 'loading' | 'ready' | 'saving' | 'error'

export interface TaskCommentItem {
  id: string
  taskId: string
  authorUserId: string
  authorLabel: string
  bodyHtml: string
  createdAt: string
  updatedAt: string
}

interface TaskCommentsOptions {
  enabled: CommentsSource<boolean>
  taskId: CommentsSource<string>
  realtimeEvent?: CommentsSource<{ type?: string; taskId?: string } | null>
  remoteDraft?: CommentsSource<string | null>
  scheduleDraft?: (value: string) => void
}

function sourceValue<T>(source: CommentsSource<T> | undefined, fallback: T): T {
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
  return String(value ?? '')
    .trim()
    .slice(0, max)
}

function normalizeDraft(value: unknown, max = 4000) {
  // eslint-disable-next-line no-control-regex
  const text = String(value ?? '').replace(/\u0000/g, '')
  return text.length <= max ? text : text.slice(0, max)
}

function commentDraftStorageKey(taskId: string) {
  const id = normalizeText(taskId, 120)
  return id ? `omni.tasks.comment-draft.${id}` : ''
}

function readPersistedDraft(taskId: string) {
  if (!import.meta.client) return ''
  const storageKey = commentDraftStorageKey(taskId)
  if (!storageKey) return ''
  try {
    return normalizeDraft(localStorage.getItem(storageKey) || '')
  } catch {
    return ''
  }
}

function persistDraft(taskId: string, value: string) {
  if (!import.meta.client) return
  const storageKey = commentDraftStorageKey(taskId)
  if (!storageKey) return
  try {
    const nextValue = normalizeDraft(value)
    if (!nextValue) {
      localStorage.removeItem(storageKey)
      return
    }
    localStorage.setItem(storageKey, nextValue)
  } catch {
    // Draft local e' best-effort; falha de storage nao deve quebrar o modal.
  }
}

function currentUserLabel(auth: ReturnType<typeof useAuthStore>) {
  return (
    compactUserLabel(
      {
        nick: auth.user?.nick || auth.principal?.nick,
        displayName: auth.user?.displayName || auth.principal?.displayName,
        name: auth.user?.name,
        fullName: auth.user?.fullName,
        email: auth.user?.email || auth.principal?.email,
      },
      120,
    ) || 'Usuario'
  )
}

function escapeHtml(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

function commentBodyHtmlFromText(value: unknown) {
  const text = String(value ?? '').replace(/\r\n/g, '\n')
  const trimmed = text.trim()
  if (!trimmed) return ''
  return `<p>${escapeHtml(trimmed).replace(/\n/g, '<br>')}</p>`
}

export function useTaskComments(options: TaskCommentsOptions) {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const usersStore = useUsersStore()
  const request = createApiRequest(runtimeConfig, () => auth.accessToken)

  const status = ref<TaskCommentStatus>('idle')
  const errorMessage = ref('')
  const comments = ref<TaskCommentItem[]>([])
  const draft = ref('')
  const activeTaskId = ref('')
  let fetchSequence = 0
  const remoteDraft = computed(() => normalizeDraft(sourceValue(options.remoteDraft, ''), 4000))

  const accountId = computed(() =>
    normalizeText(auth.activeTenantId || auth.tenantContext?.[0]?.id, 80),
  )
  const userLabelsById = computed(() => {
    const map: Record<string, string> = {}
    const authUserId = normalizeText(auth.user?.id || auth.principal?.userId, 80)
    if (authUserId) map[authUserId] = currentUserLabel(auth)
    const users = Array.isArray(usersStore.users) ? usersStore.users : []
    users.forEach((user: Record<string, unknown>) => {
      const id = normalizeText(user.id, 80)
      if (!id) return
      const label = compactUserLabel(user, 120) || id
      map[id] = label
    })
    return map
  })

  function normalizeComment(raw: Record<string, unknown>): TaskCommentItem {
    const authorUserId = normalizeText(raw.authorUserId, 120)
    return {
      id: normalizeText(raw.id, 120),
      taskId: normalizeText(raw.taskId, 120),
      authorUserId,
      authorLabel: userLabelsById.value[authorUserId] || authorUserId || 'Usuario',
      bodyHtml: String(raw.bodyHtml ?? ''),
      createdAt: normalizeText(raw.createdAt, 80),
      updatedAt: normalizeText(raw.updatedAt, 80),
    }
  }

  function upsertComment(nextComment: TaskCommentItem) {
    const existingIndex = comments.value.findIndex((item) => item.id === nextComment.id)
    if (existingIndex === -1) {
      comments.value = [...comments.value, nextComment]
      return
    }
    const nextComments = [...comments.value]
    nextComments.splice(existingIndex, 1, nextComment)
    comments.value = nextComments
  }

  async function ensureUserLabels() {
    if (usersStore.ready || usersStore.pending) return
    await usersStore.ensureLoaded().catch(() => false)
  }

  async function fetchComments(taskId: string, { silent = false }: { silent?: boolean } = {}) {
    const id = normalizeText(taskId, 120)
    const resolvedAccountId = accountId.value
    const requestSequence = ++fetchSequence
    if (!id || !resolvedAccountId) {
      if (requestSequence === fetchSequence) comments.value = []
      return []
    }
    if (!silent) status.value = 'loading'
    errorMessage.value = ''
    await ensureUserLabels()
    try {
      const response = await request(`/v1/tasks/${encodeURIComponent(id)}/comments`, {
        headers: {
          'X-Account-Id': resolvedAccountId,
        },
      })
      const nextComments = Array.isArray(response?.comments)
        ? (response.comments as Record<string, unknown>[]).map(normalizeComment)
        : []
      if (requestSequence !== fetchSequence || activeTaskId.value !== id) return comments.value
      comments.value = nextComments
      status.value = 'ready'
      return comments.value
    } catch (error) {
      if (requestSequence !== fetchSequence || activeTaskId.value !== id) return comments.value
      errorMessage.value = getApiErrorMessage(error, 'Nao foi possivel carregar os comentarios.')
      status.value = 'error'
      return comments.value
    }
  }

  async function submitComment() {
    const taskId = activeTaskId.value
    const resolvedAccountId = accountId.value
    const bodyHtml = commentBodyHtmlFromText(draft.value)
    if (!taskId || !resolvedAccountId || !bodyHtml) return null
    status.value = 'saving'
    errorMessage.value = ''
    try {
      const response = await request(`/v1/tasks/${encodeURIComponent(taskId)}/comments`, {
        method: 'POST',
        headers: {
          'X-Account-Id': resolvedAccountId,
        },
        body: {
          bodyHtml,
          mentionedUserIds: [],
        },
      })
      const rawComment = response?.comment
      if (rawComment && typeof rawComment === 'object') {
        upsertComment(normalizeComment(rawComment as Record<string, unknown>))
      }
      draft.value = ''
      persistDraft(taskId, '')
      options.scheduleDraft?.('')
      status.value = 'ready'
      void fetchComments(taskId, { silent: true })
      return rawComment || null
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Nao foi possivel enviar o comentario.')
      status.value = 'error'
      return null
    }
  }

  function setDraft(value: unknown) {
    const nextDraft = normalizeDraft(value)
    draft.value = nextDraft
    persistDraft(activeTaskId.value, nextDraft)
    options.scheduleDraft?.(nextDraft)
  }

  const canSubmit = computed(
    () => Boolean(commentBodyHtmlFromText(draft.value)) && status.value !== 'saving',
  )

  watch(
    () => ({
      enabled: Boolean(sourceValue(options.enabled, false)),
      taskId: normalizeText(sourceValue(options.taskId, ''), 120),
    }),
    ({ enabled, taskId }) => {
      if (!enabled || !taskId) {
        fetchSequence += 1
        activeTaskId.value = ''
        comments.value = []
        draft.value = ''
        status.value = 'idle'
        errorMessage.value = ''
        return
      }
      activeTaskId.value = taskId
      draft.value = readPersistedDraft(taskId)
      void fetchComments(taskId)
    },
    { immediate: true },
  )

  if (options.realtimeEvent !== undefined) {
    watch(
      () => sourceValue(options.realtimeEvent, null),
      (event) => {
        if (!event || typeof event !== 'object') return
        const eventType = normalizeText((event as { type?: unknown }).type, 80)
        if (eventType !== 'task.comment_added') return
        const eventTaskId = normalizeText((event as { taskId?: unknown }).taskId, 120)
        if (!eventTaskId || eventTaskId !== activeTaskId.value) return
        void fetchComments(eventTaskId, { silent: true })
      },
    )
  }

  return reactive({
    status,
    errorMessage,
    comments,
    draft,
    remoteDraft,
    canSubmit,
    setDraft,
    fetchComments,
    submitComment,
  })
}
