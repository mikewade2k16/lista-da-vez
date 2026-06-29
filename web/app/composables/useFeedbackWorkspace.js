import {
  computed,
  inject,
  nextTick,
  onBeforeUnmount,
  onMounted,
  provide,
  reactive,
  ref,
  watch,
} from 'vue'
import { storeToRefs } from 'pinia'

import {
  feedbackDetailStatusOptions,
  feedbackKindLabel,
  feedbackKindOptions,
  feedbackStatusLabel,
  feedbackStatusOptions,
  formatFeedbackDate,
} from '~/domain/utils/feedback-display'
import { hasPermission, normalizeAppRole } from '~/domain/utils/permissions'
import { useAuthStore } from '~/stores/auth'
import { useFeedbackStore } from '~/stores/feedback'
import { useUiStore } from '~/stores/ui'
import { compressFeedbackImage } from '~/utils/feedback-image'

export const feedbackWorkspaceContextKey = Symbol('feedbackWorkspace')

export function provideFeedbackWorkspace(ctx) {
  provide(feedbackWorkspaceContextKey, ctx)
}

export function useFeedbackWorkspaceContext() {
  const ctx = inject(feedbackWorkspaceContextKey)
  if (!ctx) throw new Error('Feedback workspace context not provided')
  return ctx
}

export function useFeedbackWorkspace() {
  const feedbackStore = useFeedbackStore()
  const ui = useUiStore()
  const auth = useAuthStore()
  const route = useRoute()
  const router = useRouter()
  const { storeContext, user } = storeToRefs(auth)

  const selectedFeedbackId = ref('')
  const replyMessage = ref('')
  const editingStatus = ref('')
  const selectedKindFilter = ref('')
  const selectedStatusFilter = ref('')
  const searchValue = ref('')
  const saving = ref(false)
  const messagesViewport = ref(null)
  const replyTextarea = ref(null)
  const replyImage = ref(null)
  const replyImagePreviewUrl = ref('')
  const syncingStatusFromFeedback = ref(false)
  const feedbackSyncCursor = ref('')
  let feedbackPollingTimer = null
  let messagesPollingTimer = null

  const canEditFeedback = computed(() => {
    if (auth.permissionsResolved) {
      return hasPermission(auth.permissionKeys, 'workspace.feedback.edit')
    }

    const normalizedRole = normalizeAppRole(auth.role)
    return ['platform_admin', 'owner', 'manager'].includes(normalizedRole)
  })

  const storeLookup = computed(
    () =>
      new Map((storeContext.value || []).map((store) => [String(store?.id || '').trim(), store])),
  )

  function getStoreLabel(storeId) {
    const store = storeLookup.value.get(String(storeId || '').trim())
    if (!store) return 'Loja nao informada'
    return String(store.name || store.code || store.city || 'Loja nao informada').trim()
  }

  const filteredFeedbacks = computed(() => {
    const normalizedSearch = String(searchValue.value || '')
      .trim()
      .toLowerCase()

    return feedbackStore.feedbacks.filter((feedback) => {
      if (selectedKindFilter.value && feedback.kind !== selectedKindFilter.value) return false
      if (selectedStatusFilter.value && feedback.status !== selectedStatusFilter.value) return false
      if (!normalizedSearch) return true

      const haystack = [
        feedback.subject,
        feedback.user_name,
        feedback.body,
        getStoreLabel(feedback.store_id),
        feedbackKindLabel(feedback.kind),
        feedbackStatusLabel(feedback.status),
      ]
        .join(' ')
        .toLowerCase()

      return haystack.includes(normalizedSearch)
    })
  })

  const selectedFeedback = computed(
    () =>
      feedbackStore.feedbacks.find((feedback) => feedback.id === selectedFeedbackId.value) || null,
  )

  const selectedMessages = computed(() => {
    if (!selectedFeedback.value?.id) return []
    return feedbackStore.messagesByFeedbackId[selectedFeedback.value.id] || []
  })

  const lastSelectedMessageCreatedAt = computed(() => {
    const timestamps = selectedMessages.value
      .map((message) => new Date(message.created_at).getTime())
      .filter((value) => Number.isFinite(value))

    return timestamps.length ? new Date(Math.max(...timestamps)).toISOString() : ''
  })

  const isSelectedFeedbackClosed = computed(
    () => String(editingStatus.value || selectedFeedback.value?.status || '').trim() === 'closed',
  )

  function isDocumentVisible() {
    return !import.meta.client || document.visibilityState === 'visible'
  }

  function getFeedbackMessages(feedbackId) {
    return feedbackStore.messagesByFeedbackId[String(feedbackId || '').trim()] || []
  }

  function getFeedbackPreview(feedback) {
    // O chamado aberto tem mensagens reais carregadas (loadSelectedMessages); os
    // demais usam o preview que o list trouxe (last_message_body).
    const localLatest = getFeedbackMessages(feedback.id).at(-1)
    if (localLatest) {
      return localLatest.body || (localLatest.image_url ? 'Imagem anexada' : feedback.body || '')
    }
    return feedback.last_message_body || feedback.body || ''
  }

  function getUnreadCount(feedback) {
    // unread_count vem do backend (GET /v1/feedback), ja pela perspectiva do
    // viewer. Zera localmente ao marcar como lido (applyLocalReadState).
    return Number(feedback?.unread_count || 0)
  }

  function syncEditingStatus(status) {
    syncingStatusFromFeedback.value = true
    editingStatus.value = String(status || '').trim()
    void nextTick(() => {
      syncingStatusFromFeedback.value = false
    })
  }

  function setMessagesViewport(element) {
    messagesViewport.value = element
  }

  function setReplyTextarea(element) {
    replyTextarea.value = element
  }

  function setReplyImage(file) {
    if (import.meta.client && replyImagePreviewUrl.value) {
      URL.revokeObjectURL(replyImagePreviewUrl.value)
    }

    replyImage.value = file
    replyImagePreviewUrl.value = file && import.meta.client ? URL.createObjectURL(file) : ''
  }

  function clearReplyImage() {
    setReplyImage(null)
  }

  function syncReplyTextareaHeight(reset = false) {
    const textarea = replyTextarea.value
    if (!textarea) return

    if (reset) {
      textarea.style.height = ''
      textarea.style.overflowY = 'hidden'
      return
    }

    textarea.style.height = '0px'
    const nextHeight = Math.min(textarea.scrollHeight, 176)
    textarea.style.height = `${Math.max(nextHeight, 44)}px`
    textarea.style.overflowY = textarea.scrollHeight > 176 ? 'auto' : 'hidden'
  }

  function handleReplyKeydown(event) {
    if (event.key !== 'Enter' || event.isComposing) return
    if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) {
      void nextTick(() => syncReplyTextareaHeight())
      return
    }

    event.preventDefault()
    if (saving.value || !canEditFeedback.value || isSelectedFeedbackClosed.value) return
    if (!replyMessage.value.trim() && !replyImage.value) return
    void sendReply()
  }

  async function handleReplyImageChange(event) {
    const target = event.target
    const file = target.files?.[0] || null
    if (!file) return

    try {
      const compressedImage = await compressFeedbackImage(file)
      setReplyImage(compressedImage)
    } catch (err) {
      ui.error(err instanceof Error ? err.message : 'Nao foi possivel preparar a imagem.')
    } finally {
      target.value = ''
    }
  }

  function syncRouteWithFeedbackId(feedbackId) {
    if (!import.meta.client) return
    const normalizedId = String(feedbackId || '').trim()
    const currentId = String(route.query.id || '').trim()
    if (normalizedId === currentId) return

    const nextQuery = { ...route.query }
    if (normalizedId) nextQuery.id = normalizedId
    else delete nextQuery.id
    void router.replace({ query: nextQuery })
  }

  function syncSelectionFromRoute() {
    const routeId = String(route.query.id || '').trim()
    if (routeId && filteredFeedbacks.value.some((feedback) => feedback.id === routeId)) {
      selectedFeedbackId.value = routeId
      return
    }
    if (!filteredFeedbacks.value.length) {
      selectedFeedbackId.value = ''
      return
    }
    if (!filteredFeedbacks.value.some((feedback) => feedback.id === selectedFeedbackId.value)) {
      selectedFeedbackId.value = filteredFeedbacks.value[0].id
    }
  }

  async function loadFeedbacks(options = {}) {
    if (!isDocumentVisible()) return
    const nextSince = Object.prototype.hasOwnProperty.call(options, 'since')
      ? options.since
      : feedbackSyncCursor.value
    const result = await feedbackStore.fetchFeedbacks({
      kind: selectedKindFilter.value,
      status: selectedStatusFilter.value,
      ...(nextSince ? { since: nextSince } : {}),
    })

    if (!result.ok) {
      ui.error(result.message || 'Erro ao carregar feedbacks')
      return
    }
    if (result.cursor) feedbackSyncCursor.value = result.cursor
    syncSelectionFromRoute()
  }

  async function loadFeedbackUpdates() {
    await loadFeedbacks()
  }

  function hasUnreadMessages(feedback) {
    if (!feedback?.id) return false
    const feedbackOwnerId = String(feedback.user_id || '').trim()
    const readAt = new Date(feedback.user_last_read_at || feedback.created_at).getTime()

    return selectedMessages.value.some((message) => {
      const authorUserId = String(message.author_user_id || '').trim()
      const createdAt = new Date(message.created_at).getTime()
      return authorUserId === feedbackOwnerId && createdAt > readAt
    })
  }

  async function markSelectedFeedbackAsRead() {
    if (
      !selectedFeedback.value?.id ||
      !isDocumentVisible() ||
      !hasUnreadMessages(selectedFeedback.value)
    ) {
      return
    }

    const result = await feedbackStore.markFeedbackAsRead(selectedFeedback.value.id)
    if (!result.ok) ui.error(result.message || 'Erro ao marcar chamado como lido')
  }

  async function loadSelectedMessages(options = {}) {
    if (!selectedFeedback.value?.id || !isDocumentVisible()) return
    const result = await feedbackStore.fetchMessages(selectedFeedback.value.id, {
      after: lastSelectedMessageCreatedAt.value,
    })

    if (!result.ok) {
      ui.error(result.message || 'Erro ao carregar mensagens')
      return
    }
    if (options.markRead) await markSelectedFeedbackAsRead()
    await scrollMessagesToBottom()
  }

  async function scrollMessagesToBottom() {
    await nextTick()
    if (messagesViewport.value) {
      messagesViewport.value.scrollTop = messagesViewport.value.scrollHeight
    }
  }

  function selectFeedback(feedbackId) {
    selectedFeedbackId.value = String(feedbackId || '').trim()
  }

  async function persistStatusIfNeeded() {
    if (!selectedFeedback.value?.id || !canEditFeedback.value) return { ok: true, changed: false }
    if (editingStatus.value === selectedFeedback.value.status) return { ok: true, changed: false }

    const result = await feedbackStore.updateFeedback(selectedFeedback.value.id, {
      status: editingStatus.value,
    })
    if (!result.ok) {
      ui.error(result.message || 'Erro ao atualizar status')
      return { ok: false, changed: false }
    }
    return { ok: true, changed: true }
  }

  async function saveStatus() {
    if (!selectedFeedback.value?.id || !canEditFeedback.value) return
    saving.value = true
    try {
      const result = await persistStatusIfNeeded()
      if (!result.ok) {
        syncEditingStatus(String(selectedFeedback.value?.status || ''))
        return
      }
      if (result.changed) ui.success('Status atualizado.')
    } finally {
      saving.value = false
    }
  }

  async function sendReply() {
    if (!selectedFeedback.value?.id) return
    if (isSelectedFeedbackClosed.value) {
      ui.error('Chamado encerrado. Nao e mais possivel enviar mensagens.')
      return
    }
    if (!canEditFeedback.value) {
      ui.error('Seu acesso ao feedback esta em modo somente leitura.')
      return
    }

    const body = String(replyMessage.value || '').trim()
    const image = replyImage.value
    saving.value = true

    try {
      const statusResult = await persistStatusIfNeeded()
      if (!statusResult.ok) return
      if (!body && !image) {
        if (statusResult.changed) ui.success('Status atualizado.')
        return
      }

      const result = await feedbackStore.sendMessage(selectedFeedback.value.id, { body, image })
      if (!result.ok) {
        ui.error(result.message || 'Erro ao enviar resposta')
        return
      }

      replyMessage.value = ''
      clearReplyImage()
      syncReplyTextareaHeight(true)
      await scrollMessagesToBottom()
      ui.success(
        statusResult.changed ? 'Status atualizado e resposta enviada.' : 'Resposta enviada.',
      )
    } finally {
      saving.value = false
    }
  }

  function startPolling() {
    stopPolling()
    feedbackPollingTimer = window.setInterval(loadFeedbackUpdates, 30000)
    // 15s: ritmo de chat sem martelar a API. As mensagens novas tambem chegam
    // ao abrir/trocar de chamado e ao refocar a aba (handleVisibilityChange).
    messagesPollingTimer = window.setInterval(loadSelectedMessages, 15000)
  }

  function stopPolling() {
    if (feedbackPollingTimer) {
      window.clearInterval(feedbackPollingTimer)
      feedbackPollingTimer = null
    }
    if (messagesPollingTimer) {
      window.clearInterval(messagesPollingTimer)
      messagesPollingTimer = null
    }
  }

  function handleVisibilityChange() {
    if (isDocumentVisible()) {
      void loadFeedbackUpdates()
      void loadSelectedMessages()
    }
  }

  watch(filteredFeedbacks, syncSelectionFromRoute, { immediate: true })
  watch(selectedFeedbackId, (feedbackId) => {
    syncRouteWithFeedbackId(feedbackId)
    replyMessage.value = ''
    clearReplyImage()
    syncReplyTextareaHeight(true)
    feedbackStore.applyLocalReadState(feedbackId)
    void loadSelectedMessages({ markRead: true })
  })
  watch(
    () => route.query.id,
    () => syncSelectionFromRoute(),
  )
  watch(
    () => selectedFeedback.value?.status,
    (status) => syncEditingStatus(String(status || '')),
    { immediate: true },
  )
  watch(editingStatus, (status) => {
    const shouldSave =
      !syncingStatusFromFeedback.value &&
      selectedFeedback.value?.id &&
      canEditFeedback.value &&
      status &&
      status !== selectedFeedback.value.status
    if (shouldSave) void saveStatus()
  })
  watch(replyMessage, () => void nextTick(() => syncReplyTextareaHeight()))
  watch(selectedMessages, () => void scrollMessagesToBottom())
  watch([selectedKindFilter, selectedStatusFilter], () => {
    feedbackSyncCursor.value = ''
    void loadFeedbacks()
  })

  onMounted(async () => {
    await loadFeedbacks()
    await loadSelectedMessages({ markRead: true })
    syncReplyTextareaHeight(true)
    startPolling()
    document.addEventListener('visibilitychange', handleVisibilityChange)
  })

  onBeforeUnmount(() => {
    stopPolling()
    document.removeEventListener('visibilitychange', handleVisibilityChange)
    clearReplyImage()
  })

  return reactive({
    canEditFeedback,
    clearReplyImage,
    detailStatusOptions: feedbackDetailStatusOptions,
    editingStatus,
    filteredFeedbacks,
    formatDate: formatFeedbackDate,
    getFeedbackPreview,
    getStoreLabel,
    getUnreadCount,
    handleReplyImageChange,
    handleReplyKeydown,
    isSelectedFeedbackClosed,
    kindLabel: feedbackKindLabel,
    kindOptions: feedbackKindOptions,
    replyImage,
    replyImagePreviewUrl,
    replyMessage,
    saving,
    searchValue,
    selectFeedback,
    selectedFeedback,
    selectedKindFilter,
    selectedMessages,
    selectedStatusFilter,
    sendReply,
    setMessagesViewport,
    setReplyTextarea,
    statusLabel: feedbackStatusLabel,
    statusOptions: feedbackStatusOptions,
    syncReplyTextareaHeight,
    user,
  })
}
