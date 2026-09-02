import { computed, ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import type { Conversation, Message } from '~/types'
import { useOmnichannelInboxStateMutators } from './useOmnichannelInboxStateMutators'

function conversation(
  id: string,
  instanceId: string | null,
  instanceScopeKey = instanceId ?? 'legacy-scope',
  channel: Conversation['channel'] = 'WHATSAPP',
): Conversation {
  return {
    id,
    instanceId,
    instanceScopeKey,
    instanceName: instanceScopeKey,
    instanceDisplayName: instanceScopeKey,
    channel,
    status: 'OPEN',
    externalId: `${id}@s.whatsapp.net`,
    contactId: null,
    contactName: id,
    contactAvatarUrl: null,
    contactPhone: '5579999999999',
    assignedToId: null,
    createdAt: '2026-08-27T10:00:00.000Z',
    updatedAt: '2026-08-27T10:00:00.000Z',
    lastMessageAt: '2026-08-27T10:00:00.000Z',
    lastMessage: null,
  }
}

describe('instance-scoped inbox projection reset', () => {
  it('clears only conversations from the selected instance', () => {
    const conversations = ref([
      conversation('conversation-a', 'instance-a'),
      conversation('conversation-b', 'instance-b'),
    ])
    const messages = ref<Message[]>([
      { id: 'message-a', conversationId: 'conversation-a' } as Message,
    ])
    const activeConversationId = ref<string | null>('conversation-a')
    const visibleMessagesConversationId = ref<string | null>('conversation-a')
    const hasMoreMessages = ref(true)
    const showLoadOlderMessagesButton = ref(true)
    const showScrollToLatestButton = ref(true)
    const replyTarget = ref<Message | null>(messages.value[0] ?? null)
    const notes = ref('nota')
    const mutators = useOmnichannelInboxStateMutators({
      replyTarget,
      conversations,
      messages,
      activeConversationId,
      visibleMessagesConversationId,
      hasMoreMessages,
      showLoadOlderMessagesButton,
      showScrollToLatestButton,
      mentionAlertState: ref({ 'conversation-a': 2, 'conversation-b': 1 }),
      draft: ref(''),
      search: ref(''),
      channel: ref('all'),
      status: ref('all'),
      instanceId: ref('all'),
      sidebarView: ref('conversations'),
      showFilters: ref(false),
      leftCollapsed: ref(false),
      rightCollapsed: ref(false),
      assigneeModel: ref(''),
      contactActionError: ref(''),
      internalNotes: computed({
        get: () => notes.value,
        set: (value) => {
          notes.value = value
        },
      }),
      setAttachmentFromFile: vi.fn(),
    })

    expect(mutators.clearInstanceConversationProjection('instance-a', 'instance-a')).toEqual([
      'conversation-a',
    ])
    expect(conversations.value.map((entry) => entry.id)).toEqual(['conversation-b'])
    expect(activeConversationId.value).toBeNull()
    expect(visibleMessagesConversationId.value).toBeNull()
    expect(messages.value).toEqual([])
    expect(hasMoreMessages.value).toBe(false)
    expect(showLoadOlderMessagesButton.value).toBe(false)
    expect(showScrollToLatestButton.value).toBe(false)
    expect(replyTarget.value).toBeNull()
  })

  it('keeps the active conversation when another instance is reset', () => {
    const conversations = ref([
      conversation('conversation-a', 'instance-a'),
      conversation('conversation-b', 'instance-b'),
    ])
    const messages = ref<Message[]>([
      { id: 'message-b', conversationId: 'conversation-b' } as Message,
    ])
    const activeConversationId = ref<string | null>('conversation-b')
    const visibleMessagesConversationId = ref<string | null>('conversation-b')
    const notes = ref('')
    const mutators = useOmnichannelInboxStateMutators({
      replyTarget: ref(null),
      conversations,
      messages,
      activeConversationId,
      visibleMessagesConversationId,
      hasMoreMessages: ref(false),
      showLoadOlderMessagesButton: ref(false),
      showScrollToLatestButton: ref(false),
      mentionAlertState: ref({}),
      draft: ref(''),
      search: ref(''),
      channel: ref('all'),
      status: ref('all'),
      instanceId: ref('all'),
      sidebarView: ref('conversations'),
      showFilters: ref(false),
      leftCollapsed: ref(false),
      rightCollapsed: ref(false),
      assigneeModel: ref(''),
      contactActionError: ref(''),
      internalNotes: computed({
        get: () => notes.value,
        set: (value) => {
          notes.value = value
        },
      }),
      setAttachmentFromFile: vi.fn(),
    })

    mutators.clearInstanceConversationProjection('instance-a', 'instance-a')

    expect(conversations.value.map((entry) => entry.id)).toEqual(['conversation-b'])
    expect(activeConversationId.value).toBe('conversation-b')
    expect(visibleMessagesConversationId.value).toBe('conversation-b')
    expect(messages.value.map((entry) => entry.id)).toEqual(['message-b'])
  })

  it('clears only WhatsApp with the exact id or legacy null id plus scope key', () => {
    const exact = conversation('exact-a', 'instance-a', 'scope-a')
    const legacy = conversation('legacy-a', null, 'scope-a')
    const other = conversation('instance-b', 'instance-b', 'scope-b')
    const instagram = conversation('instagram-a', 'instance-a', 'scope-a', 'INSTAGRAM')
    const conversations = ref([exact, legacy, other, instagram])
    const notes = ref('')
    const mutators = useOmnichannelInboxStateMutators({
      replyTarget: ref(null),
      conversations,
      messages: ref([]),
      activeConversationId: ref(null),
      visibleMessagesConversationId: ref(null),
      hasMoreMessages: ref(false),
      showLoadOlderMessagesButton: ref(false),
      showScrollToLatestButton: ref(false),
      mentionAlertState: ref({}),
      draft: ref(''),
      search: ref(''),
      channel: ref('all'),
      status: ref('all'),
      instanceId: ref('all'),
      sidebarView: ref('conversations'),
      showFilters: ref(false),
      leftCollapsed: ref(false),
      rightCollapsed: ref(false),
      assigneeModel: ref(''),
      contactActionError: ref(''),
      internalNotes: computed({
        get: () => notes.value,
        set: (value) => {
          notes.value = value
        },
      }),
      setAttachmentFromFile: vi.fn(),
    })

    expect(mutators.clearInstanceConversationProjection('instance-a', 'scope-a')).toEqual([
      'exact-a',
      'legacy-a',
    ])
    expect(conversations.value.map((entry) => entry.id)).toEqual([
      'instance-b',
      'instagram-a',
    ])
  })
})
