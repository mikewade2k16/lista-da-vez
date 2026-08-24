import { describe, expect, it, vi } from 'vitest'
import type { ApiRequest } from './calendar-api'
import {
  assistantTranscribePath,
  assistantResourceInstruction,
  calendarChatProposalConfirmationKey,
  confirmCalendarChatProposal,
  createConversation,
  fetchConversations,
  getConversation,
  normalizeAssistantResources,
} from './calendar-chat-api'

describe('calendar chat proposal kinds', () => {
  it('preserva taskItem ao normalizar uma mensagem persistida', async () => {
    const api = vi.fn().mockResolvedValue({
      id: 'conversation-1',
      entrySurface: 'meta_ads',
      messages: [
        {
          id: 'message-1',
          role: 'assistant',
          content: 'Revise o cartão.',
          proposals: [
            {
              id: '0',
              action: 'update',
              kind: 'taskItem',
              status: 'pending',
              fields: {
                targetId: 'task-1',
                taskItem: { id: 'item-1', status: 'posted', statusDate: '2026-08-13' },
              },
            },
          ],
        },
      ],
    })

    const conversation = await getConversation(api as ApiRequest, 'conversation-1')

    expect(conversation.messages[0]?.proposals[0]).toMatchObject({
      kind: 'taskItem',
      fields: {
        targetId: 'task-1',
        taskItem: { id: 'item-1', status: 'posted', statusDate: '2026-08-13' },
      },
    })
    expect(conversation.surface).toBe('meta_ads')
    expect(api).toHaveBeenCalledWith('/v1/assistant/chat/conversations/conversation-1')
  })

  it('preserva somente o schema fechado do card metaAction', async () => {
    const api = vi.fn().mockResolvedValue({
      id: 'conversation-meta',
      surface: 'meta_ads',
      messages: [
        {
          id: 'message-meta',
          role: 'assistant',
          content: 'Revise a ação.',
          proposals: [
            {
              id: '0',
              action: 'create',
              kind: 'metaAction',
              status: 'pending',
              fields: {
                clientId: 'forged',
                metaAction: {
                  action: 'update_campaign',
                  adAccountId: 'account-1',
                  adAccountName: 'Conta principal',
                  campaignId: 'campaign-1',
                  campaignName: 'Campanha atual',
                  currency: 'brl',
                  budget: { type: 'daily', amount: 125.5 },
                  actionProposalId: 'action-1',
                  summary: 'Atualizar orçamento.',
                  actionStatus: 'pending',
                  executionAvailable: true,
                  canConfirm: true,
                  requiresSpendAcknowledgement: true,
                },
              },
            },
          ],
        },
      ],
    })

    const conversation = await getConversation(api as ApiRequest, 'conversation-meta')

    expect(conversation.messages[0]?.proposals[0]).toEqual({
      id: '0',
      action: 'create',
      kind: 'metaAction',
      status: 'pending',
      fields: {
        metaAction: {
          action: 'update_campaign',
          adAccountId: 'account-1',
          adAccountName: 'Conta principal',
          campaignId: 'campaign-1',
          campaignName: 'Campanha atual',
          currency: 'BRL',
          name: '',
          objective: '',
          specialAdCategories: [],
          budget: { type: 'daily', amount: 125.5 },
          instagramPostId: '',
          instagramPostTitle: '',
          adSetName: '',
          adName: '',
          countries: [],
          ageMin: 0,
          ageMax: 0,
          actionProposalId: 'action-1',
          summary: 'Atualizar orçamento.',
          actionStatus: 'pending',
          executionAvailable: true,
          canConfirm: true,
          requiresSpendAcknowledgement: true,
          expiresAt: '',
          errorCode: '',
          errorMessage: '',
        },
      },
    })
  })

  it('preserva somente os campos visuais seguros da promoção de post', async () => {
    const api = vi.fn().mockResolvedValue({
      id: 'conversation-post',
      surface: 'meta_ads',
      messages: [
        {
          id: 'message-post',
          role: 'assistant',
          content: 'Revise.',
          proposals: [
            {
              id: '0',
              kind: 'metaAction',
              fields: {
                metaAction: {
                  action: 'promote_instagram_post',
                  adAccountId: 'account-1',
                  instagramPostId: '77889900',
                  instagramPostTitle: 'Post real',
                  adSetName: 'Conjunto',
                  adName: 'Anúncio',
                  countries: ['br', 'INVALID'],
                  ageMin: 18,
                  ageMax: 65,
                  igUserId: 'secret-source-id',
                  pageId: 'secret-page-id',
                  actionStatus: 'pending',
                },
              },
            },
          ],
        },
      ],
    })

    const conversation = await getConversation(api as ApiRequest, 'conversation-post')
    const meta = conversation.messages[0]?.proposals[0]?.fields.metaAction

    expect(meta).toMatchObject({
      action: 'promote_instagram_post',
      instagramPostId: '77889900',
      instagramPostTitle: 'Post real',
      countries: ['BR'],
      ageMin: 18,
      ageMax: 65,
    })
    expect(meta).not.toHaveProperty('igUserId')
    expect(meta).not.toHaveProperty('pageId')
  })

  it('envia a surface ao criar uma conversa no endpoint compartilhado', async () => {
    const api = vi.fn().mockResolvedValue({ id: 'conversation-2' })

    const id = await createConversation(
      api as ApiRequest,
      'meta_ads',
      'client',
      'client-1',
      'Campanhas de agosto',
    )

    expect(id).toBe('conversation-2')
    expect(api).toHaveBeenCalledWith('/v1/assistant/chat/conversations', {
      method: 'POST',
      body: {
        surface: 'meta_ads',
        scopeMode: 'client',
        scopeClientId: 'client-1',
        title: 'Campanhas de agosto',
      },
    })
  })

  it('confirma card local somente pelo endpoint autoritativo com chave estavel', async () => {
    const api = vi.fn().mockResolvedValue({
      message: {
        id: 'message-1',
        role: 'assistant',
        content: 'Revise.',
        proposals: [
          {
            id: 'proposal-1',
            action: 'create',
            kind: 'event',
            status: 'accepted',
            fields: { title: 'Post aprovado' },
            execution: { status: 'succeeded', canConfirm: false },
          },
        ],
      },
    })
    const key = calendarChatProposalConfirmationKey('message-1', 'proposal-1')

    const message = await confirmCalendarChatProposal(
      api as ApiRequest,
      'conversation-1',
      'message-1',
      'proposal-1',
      key,
      { title: 'Post aprovado' },
      'client-1',
    )

    expect(message.proposals[0]).toMatchObject({
      id: 'proposal-1',
      status: 'accepted',
      execution: { status: 'succeeded', canConfirm: false },
    })
    expect(api).toHaveBeenCalledWith(
      '/v1/assistant/chat/conversations/conversation-1/messages/message-1/proposals/proposal-1/confirm',
      {
        method: 'POST',
        body: { fields: { title: 'Post aprovado' }, clientId: 'client-1' },
        headers: { 'Idempotency-Key': key },
      },
    )
  })

  it('normaliza conversas legadas sem surface como calendar', async () => {
    const api = vi.fn().mockResolvedValue({
      conversations: [{ id: 'legacy', title: 'Legada', scopeMode: 'all' }],
    })

    const conversations = await fetchConversations(api as ApiRequest)

    expect(conversations[0]?.surface).toBe('calendar')
    expect(api).toHaveBeenCalledWith('/v1/assistant/chat/conversations')
  })

  it('amarra a transcricao a surface ativa', () => {
    expect(assistantTranscribePath('meta_ads')).toBe(
      '/v1/assistant/chat/transcribe?surface=meta_ads',
    )
    expect(assistantTranscribePath('calendar')).toBe(
      '/v1/assistant/chat/transcribe?surface=calendar',
    )
  })

  it('normaliza resources autoritativos, remove URLs inseguras e limita a 20', () => {
    const resources = normalizeAssistantResources([
      {
        id: 'instagram_post:post-1',
        kind: 'instagram_post',
        title: '  Post   real  ',
        imageUrl: 'http://cdn.example/post.jpg',
        permalink: 'https://instagram.com/p/post-1',
        metadata: { username: '@cliente', 'unsafe key': 'drop' },
      },
      { id: 'instagram_post:post-1', kind: 'instagram_post', title: 'Duplicado' },
      { id: 'meta_campaign:forged', kind: 'meta_ad_account', title: 'Prefixo divergente' },
      ...Array.from({ length: 25 }, (_, index) => ({
        id: `meta_campaign:campaign-${index}`,
        kind: 'meta_campaign',
        title: `Campanha ${index}`,
      })),
    ])

    expect(resources).toHaveLength(20)
    expect(resources[0]).toMatchObject({
      id: 'instagram_post:post-1',
      title: 'Post real',
      imageUrl: '',
      permalink: 'https://instagram.com/p/post-1',
      metadata: { username: '@cliente' },
    })
    expect(resources.filter((resource) => resource.id === 'instagram_post:post-1')).toHaveLength(1)
    expect(resources.some((resource) => resource.id === 'meta_campaign:forged')).toBe(false)
  })

  it('selecionar um resource produz somente um draft revisavel', () => {
    const [resource] = normalizeAssistantResources([
      { id: 'instagram_post:post-1', kind: 'instagram_post', title: 'Lancamento' },
    ])

    expect(assistantResourceInstruction(resource!)).toBe(
      'Use o post "Lancamento" (instagram_post:post-1) como criativo para uma campanha. Prepare a proposta para eu revisar; nao execute nada ainda.',
    )
  })
})
