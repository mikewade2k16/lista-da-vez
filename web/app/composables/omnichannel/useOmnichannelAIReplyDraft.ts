import { ref, watch, type ComputedRef, type Ref } from 'vue'
import { formatSendError } from '~/composables/omnichannel/useOmnichannelInboxShared'

export interface OmnichannelAIReplyDraft {
  id: string
  conversationId: string
  generation: number
  content: string
  status: 'pending'
  edited: boolean
  createdAt: string
  decidedAt: string | null
}

interface AIReplyDraftEnvelope {
  draft: OmnichannelAIReplyDraft | null
}

export function useOmnichannelAIReplyDraft(options: {
  activeConversationId: Ref<string | null>
  canManageConversation: ComputedRef<boolean>
  composerDraft: Ref<string>
  apiFetch: <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>
}) {
  const aiReplyDraft = ref<OmnichannelAIReplyDraft | null>(null)
  const selectedAIReplyDraftId = ref<string | null>(null)
  const loadingAIReplyDraft = ref(false)
  const dismissingAIReplyDraft = ref(false)
  const aiReplyDraftError = ref('')
  let requestSequence = 0

  function clearAIReplyDraft(): void {
    requestSequence += 1
    aiReplyDraft.value = null
    selectedAIReplyDraftId.value = null
    loadingAIReplyDraft.value = false
    dismissingAIReplyDraft.value = false
    aiReplyDraftError.value = ''
  }

  async function loadAIReplyDraft(conversationId = options.activeConversationId.value): Promise<void> {
    const normalizedId = String(conversationId ?? '').trim()
    const sequence = ++requestSequence
    if (!normalizedId || !options.canManageConversation.value) {
      aiReplyDraft.value = null
      selectedAIReplyDraftId.value = null
      aiReplyDraftError.value = ''
      loadingAIReplyDraft.value = false
      return
    }

    loadingAIReplyDraft.value = true
    aiReplyDraftError.value = ''
    try {
      const response = await options.apiFetch<AIReplyDraftEnvelope>(
        `/conversations/${encodeURIComponent(normalizedId)}/ai-reply-draft`,
      )
      if (sequence !== requestSequence || options.activeConversationId.value !== normalizedId) return
      aiReplyDraft.value = response.draft
      if (!response.draft || selectedAIReplyDraftId.value !== response.draft.id) {
        selectedAIReplyDraftId.value = null
      }
    } catch (cause) {
      if (sequence !== requestSequence) return
      aiReplyDraft.value = null
      selectedAIReplyDraftId.value = null
      aiReplyDraftError.value = formatSendError(
        cause,
        'Nao foi possivel carregar a sugestao da IA.',
      )
    } finally {
      if (sequence === requestSequence) loadingAIReplyDraft.value = false
    }
  }

  function useAIReplyDraft(): boolean {
    const suggestion = aiReplyDraft.value
    if (!suggestion) return false
    if (options.composerDraft.value.trim()) {
      aiReplyDraftError.value = 'Limpe a mensagem atual antes de usar a sugestao da IA.'
      return false
    }
    options.composerDraft.value = suggestion.content
    selectedAIReplyDraftId.value = suggestion.id
    aiReplyDraftError.value = ''
    return true
  }

  async function dismissAIReplyDraft(reason = 'operator_dismissed'): Promise<void> {
    const conversationId = options.activeConversationId.value
    const suggestion = aiReplyDraft.value
    if (!conversationId || !suggestion || dismissingAIReplyDraft.value) return

    dismissingAIReplyDraft.value = true
    aiReplyDraftError.value = ''
    try {
      await options.apiFetch(
        `/conversations/${encodeURIComponent(conversationId)}/ai-reply-drafts/${encodeURIComponent(suggestion.id)}/dismiss`,
        { method: 'POST', body: { reason } },
      )
      if (options.activeConversationId.value === conversationId) {
        aiReplyDraft.value = null
        selectedAIReplyDraftId.value = null
      }
    } catch (cause) {
      aiReplyDraftError.value = formatSendError(
        cause,
        'Nao foi possivel descartar a sugestao da IA.',
      )
    } finally {
      dismissingAIReplyDraft.value = false
    }
  }

  function markAIReplyDraftSent(draftId: string | null): void {
    if (draftId && selectedAIReplyDraftId.value !== draftId) return
    selectedAIReplyDraftId.value = null
    aiReplyDraft.value = null
    aiReplyDraftError.value = ''
  }

  watch(
    [options.activeConversationId, options.canManageConversation],
    () => {
      selectedAIReplyDraftId.value = null
      void loadAIReplyDraft()
    },
    { immediate: true },
  )

  return {
    aiReplyDraft,
    selectedAIReplyDraftId,
    loadingAIReplyDraft,
    dismissingAIReplyDraft,
    aiReplyDraftError,
    loadAIReplyDraft,
    useAIReplyDraft,
    dismissAIReplyDraft,
    markAIReplyDraftSent,
    clearAIReplyDraft,
  }
}
