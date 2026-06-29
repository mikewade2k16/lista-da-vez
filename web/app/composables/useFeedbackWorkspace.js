import { computed, inject, nextTick, onMounted, provide, reactive, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import { useFeedbackChat } from '~/composables/useFeedbackChat'
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
  const editingStatus = ref('')
  const selectedKindFilter = ref('')
  const selectedStatusFilter = ref('')
  const searchValue = ref('')
  const saving = ref(false)
  const syncingStatusFromFeedback = ref(false)
  const feedbackSyncCursor = ref('')

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

  // Nucleo de chat compartilhado com UserFeedbackWorkspace. No admin, nao-lido =
  // mensagem do DONO do chamado (user_id). loadFeedbackUpdates eh hoisted (func
  // declaration) e recarrega a lista filtrada.
  const chat = useFeedbackChat({
    selectedFeedback,
    selectedMessages,
    isReadFromOwnerPerspective: (authorUserId, feedback) =>
      authorUserId === String(feedback.user_id || '').trim(),
    loadFeedbackUpdates,
    messagesLoadErrorMessage: 'Erro ao carregar mensagens',
  })
  const {
    replyMessage,
    replyImage,
    replyImagePreviewUrl,
    replyTextarea,
    messagesViewport,
    isDocumentVisible,
    getFeedbackPreview,
    getUnreadCount,
    loadSelectedMessages,
    scrollMessagesToBottom,
    clearReplyImage,
    syncReplyTextareaHeight,
    handleReplyImageChange,
    startPolling,
  } = chat

  const isSelectedFeedbackClosed = computed(
    () => String(editingStatus.value || selectedFeedback.value?.status || '').trim() === 'closed',
  )

  function syncEditingStatus(status) {
    syncingStatusFromFeedback.value = true
    editingStatus.value = String(status || '').trim()
    void nextTick(() => {
      syncingStatusFromFeedback.value = false
    })
  }

  // O admin usa function-ref no template (:ref="ctx.setMessagesViewport"); estes
  // setters apenas escrevem nos refs do nucleo de chat, mantendo a API publica.
  function setMessagesViewport(element) {
    messagesViewport.value = element
  }

  function setReplyTextarea(element) {
    replyTextarea.value = element
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
    // O nucleo de chat (useFeedbackChat) cuida do tracking de visibilidade e da
    // limpeza (stopPolling/clearReplyImage) no onBeforeUnmount.
    await loadFeedbacks()
    await loadSelectedMessages({ markRead: true })
    syncReplyTextareaHeight(true)
    startPolling()
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
