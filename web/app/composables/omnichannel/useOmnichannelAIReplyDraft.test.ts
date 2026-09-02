import { computed, effectScope, ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { useOmnichannelAIReplyDraft } from './useOmnichannelAIReplyDraft'

async function flushPromises(): Promise<void> {
  await Promise.resolve()
  await Promise.resolve()
}

const suggestion = {
  id: 'draft-1',
  conversationId: 'conversation-1',
  generation: 4,
  content: 'Resposta sugerida',
  status: 'pending' as const,
  edited: false,
  createdAt: '2026-08-28T12:00:00Z',
  decidedAt: null,
}

describe('useOmnichannelAIReplyDraft', () => {
  it('carrega por conversa, preenche sem sobrescrever e limpa apos envio', async () => {
    const activeConversationId = ref<string | null>('conversation-1')
    const composerDraft = ref('')
    const apiFetch = vi.fn(async () => ({ draft: suggestion }))
    const scope = effectScope()
    const state = scope.run(() => useOmnichannelAIReplyDraft({
      activeConversationId,
      canManageConversation: computed(() => true),
      composerDraft,
      apiFetch: apiFetch as never,
    }))
    if (!state) throw new Error('state unavailable')

    await flushPromises()
    expect(apiFetch).toHaveBeenCalledWith('/conversations/conversation-1/ai-reply-draft')
    expect(state.aiReplyDraft.value?.id).toBe('draft-1')
    expect(state.useAIReplyDraft()).toBe(true)
    expect(composerDraft.value).toBe('Resposta sugerida')
    expect(state.selectedAIReplyDraftId.value).toBe('draft-1')

    state.markAIReplyDraftSent('draft-1')
    expect(state.aiReplyDraft.value).toBeNull()
    expect(state.selectedAIReplyDraftId.value).toBeNull()
    scope.stop()
  })

  it('nunca sobrescreve o texto atual do operador', async () => {
    const composerDraft = ref('Texto humano em andamento')
    const scope = effectScope()
    const state = scope.run(() => useOmnichannelAIReplyDraft({
      activeConversationId: ref('conversation-1'),
      canManageConversation: computed(() => true),
      composerDraft,
      apiFetch: vi.fn(async () => ({ draft: suggestion })) as never,
    }))
    if (!state) throw new Error('state unavailable')

    await flushPromises()
    expect(state.useAIReplyDraft()).toBe(false)
    expect(composerDraft.value).toBe('Texto humano em andamento')
    expect(state.selectedAIReplyDraftId.value).toBeNull()
    expect(state.aiReplyDraftError.value).toContain('Limpe')
    scope.stop()
  })

  it('descarta somente o draft e a conversa selecionados', async () => {
    const apiFetch = vi.fn(async (path: string) => {
      if (path.endsWith('/ai-reply-draft')) return { draft: suggestion }
      return undefined
    })
    const scope = effectScope()
    const state = scope.run(() => useOmnichannelAIReplyDraft({
      activeConversationId: ref('conversation-1'),
      canManageConversation: computed(() => true),
      composerDraft: ref(''),
      apiFetch: apiFetch as never,
    }))
    if (!state) throw new Error('state unavailable')

    await flushPromises()
    await state.dismissAIReplyDraft('resposta inadequada')
    expect(apiFetch).toHaveBeenLastCalledWith(
      '/conversations/conversation-1/ai-reply-drafts/draft-1/dismiss',
      { method: 'POST', body: { reason: 'resposta inadequada' } },
    )
    expect(state.aiReplyDraft.value).toBeNull()
    scope.stop()
  })

  it('nao consulta nem expoe draft sem permissao de resposta', async () => {
    const apiFetch = vi.fn()
    const scope = effectScope()
    const state = scope.run(() => useOmnichannelAIReplyDraft({
      activeConversationId: ref('conversation-1'),
      canManageConversation: computed(() => false),
      composerDraft: ref(''),
      apiFetch: apiFetch as never,
    }))
    if (!state) throw new Error('state unavailable')

    await flushPromises()
    expect(apiFetch).not.toHaveBeenCalled()
    expect(state.aiReplyDraft.value).toBeNull()
    scope.stop()
  })
})
