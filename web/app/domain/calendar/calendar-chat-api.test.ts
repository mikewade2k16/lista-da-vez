import { describe, expect, it, vi } from 'vitest'
import type { ApiRequest } from './calendar-api'
import { getConversation } from './calendar-chat-api'

describe('calendar chat proposal kinds', () => {
  it('preserva taskItem ao normalizar uma mensagem persistida', async () => {
    const api = vi.fn().mockResolvedValue({
      id: 'conversation-1',
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
  })
})
