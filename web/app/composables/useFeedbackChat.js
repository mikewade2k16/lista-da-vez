import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'

import { useFeedbackStore } from '~/stores/feedback'
import { useUiStore } from '~/stores/ui'
import { compressFeedbackImage } from '~/utils/feedback-image'

// Nucleo de chat compartilhado entre o workspace admin (useFeedbackWorkspace)
// e o do usuario (UserFeedbackWorkspace). Concentra so a parte COMUM e solida:
// autosize do textarea, ciclo da imagem anexada, scroll, leitura/polling das
// mensagens do chamado aberto. As regras que divergem entre admin e usuario
// (envio/reply, status, perspectiva de nao-lido, colecao da lista) ficam fora
// daqui, parametrizadas pelo chamador.
//
// Parametros:
// - selectedFeedback: ComputedRef do chamado aberto (ou null).
// - selectedMessages: ComputedRef das mensagens do chamado aberto.
// - isReadFromOwnerPerspective: fn(authorUserId, feedback) -> boolean. Decide se
//   uma mensagem conta como "nao lida" pela perspectiva do viewer. No admin,
//   nao-lidas sao as mensagens do dono do chamado; no usuario, as que nao sao
//   dele. E a unica peca da leitura que muda entre as duas telas.
// - loadFeedbackUpdates: fn() -> Promise. Recarrega a colecao da lista (admin usa
//   loadFeedbackUpdates; usuario usa loadMyFeedbackUpdates).
// - messagesLoadErrorMessage: string usada no toast quando o fetch de mensagens
//   falha (admin: "Erro ao carregar mensagens"; usuario: "Erro ao carregar
//   conversa").
export function useFeedbackChat({
  selectedFeedback,
  selectedMessages,
  isReadFromOwnerPerspective,
  loadFeedbackUpdates,
  messagesLoadErrorMessage = 'Erro ao carregar mensagens',
}) {
  const feedbackStore = useFeedbackStore()
  const ui = useUiStore()

  const replyMessage = ref('')
  const replyImage = ref(null)
  const replyImagePreviewUrl = ref('')
  const replyTextarea = ref(null)
  const messagesViewport = ref(null)
  let feedbackPollingTimer = null
  let messagesPollingTimer = null

  const lastSelectedMessageCreatedAt = computed(() => {
    const timestamps = selectedMessages.value
      .map((message) => new Date(message.created_at).getTime())
      .filter((value) => Number.isFinite(value))

    return timestamps.length ? new Date(Math.max(...timestamps)).toISOString() : ''
  })

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
    // unread_count vem do backend (GET /v1/feedback(/me)), ja pela perspectiva do
    // viewer. Zera localmente ao marcar como lido (applyLocalReadState).
    return Number(feedback?.unread_count || 0)
  }

  function hasUnreadMessages(feedback) {
    if (!feedback?.id) return false
    const readAt = new Date(feedback.user_last_read_at || feedback.created_at).getTime()

    return selectedMessages.value.some((message) => {
      const authorUserId = String(message.author_user_id || '').trim()
      const createdAt = new Date(message.created_at).getTime()
      return isReadFromOwnerPerspective(authorUserId, feedback) && createdAt > readAt
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
      ui.error(result.message || messagesLoadErrorMessage)
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

  function startVisibilityTracking() {
    document.addEventListener('visibilitychange', handleVisibilityChange)
  }

  function stopVisibilityTracking() {
    document.removeEventListener('visibilitychange', handleVisibilityChange)
  }

  onMounted(() => {
    // O chamador faz os loads iniciais na sua propria ordem; aqui so registramos
    // o tracking de visibilidade e garantimos a limpeza no unmount.
    startVisibilityTracking()
  })

  onBeforeUnmount(() => {
    stopPolling()
    stopVisibilityTracking()
    clearReplyImage()
  })

  return {
    // estado do compositor
    replyMessage,
    replyImage,
    replyImagePreviewUrl,
    replyTextarea,
    messagesViewport,
    // derivados/helpers de leitura
    lastSelectedMessageCreatedAt,
    isDocumentVisible,
    getFeedbackMessages,
    getFeedbackPreview,
    getUnreadCount,
    hasUnreadMessages,
    // acoes de mensagens/leitura
    markSelectedFeedbackAsRead,
    loadSelectedMessages,
    scrollMessagesToBottom,
    // compositor (imagem + textarea)
    setReplyImage,
    clearReplyImage,
    syncReplyTextareaHeight,
    handleReplyImageChange,
    // polling
    startPolling,
    stopPolling,
  }
}
