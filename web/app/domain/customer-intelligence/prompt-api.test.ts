import { describe, expect, it, vi } from 'vitest'
import {
  createPromptDraft,
  fetchPromptProcessView,
  publishPromptVersion,
  rollbackPromptBinding,
} from './prompt-api'

describe('customer intelligence prompt lifecycle contracts', () => {
  it('derives a safe publish and rollback descriptor from registered APIs', async () => {
    const api = vi
      .fn()
      .mockResolvedValueOnce([
        {
          id: 'definition-1',
          key: 'conversation.reply',
          label: 'Resposta',
          description: 'Responde',
          status: 'registered',
          schemaVersion: 'v1',
        },
      ])
      .mockResolvedValueOnce([
        {
          id: 'prompt-2',
          processKey: 'conversation.reply',
          layer: 'process_prompt',
          version: 2,
          status: 'validated',
          revision: 2,
        },
        {
          id: 'prompt-1',
          processKey: 'conversation.reply',
          layer: 'process_prompt',
          version: 1,
          status: 'published',
          revision: 3,
        },
        {
          id: 'prompt-0',
          processKey: 'conversation.reply',
          layer: 'process_prompt',
          version: 0,
          status: 'published',
          revision: 3,
        },
      ])
      .mockResolvedValueOnce([
        {
          id: 'binding-1',
          processKey: 'conversation.reply',
          processPromptVersionId: 'prompt-1',
          agentVersionId: 'agent-version-1',
          status: 'published',
          revision: 1,
        },
      ])
      .mockResolvedValueOnce([
        {
          id: 'agent-1',
          name: 'Atendimento',
          status: 'enabled',
          activeVersionId: 'agent-version-1',
        },
      ])
      .mockResolvedValueOnce([
        {
          id: 'evaluation-1',
          status: 'passed',
          scores: { structural: 1 },
          reasonCodes: [],
        },
      ])

    const view = await fetchPromptProcessView(api, 'conversation.reply', 'client-1')

    expect(view.canPublish).toBe(true)
    expect(view.canRollback).toBe(true)
    expect(view.effectiveBinding?.activeVersionId).toBe('prompt-1')
    expect(view.rollbackTargetVersionId).toBe('prompt-0')
    expect(view.publishAgents).toEqual([
      {
        agentId: 'agent-1',
        agentVersionId: 'agent-version-1',
        label: 'Atendimento',
      },
    ])
    expect(view.evaluations[0]?.schemaScore).toBe(1)
  })

  it('creates process prompts and publishes only closed policies', async () => {
    const api = vi.fn().mockResolvedValue({})

    await createPromptDraft(api, 'conversation.reply', 'client-1', 'Responda.')
    await publishPromptVersion(api, 'prompt-1', 'client-1', 'agent-version-1')
    await rollbackPromptBinding(api, 'binding-1', 'prompt-0')

    expect(api.mock.calls[0]).toEqual([
      '/v1/customer-intelligence/prompts/conversation.reply/drafts',
      {
        method: 'POST',
        body: {
          clientAccountId: 'client-1',
          layer: 'process_prompt',
          content: 'Responda.',
        },
      },
    ])
    expect(api.mock.calls[1]?.[1]).toEqual({
      method: 'POST',
      body: {
        clientAccountId: 'client-1',
        agentVersionId: 'agent-version-1',
        sourcePolicy: [],
        toolPolicy: [],
        knowledgePolicy: [],
        runtimePolicy: {},
      },
    })
    expect(api.mock.calls[2]).toEqual([
      '/v1/customer-intelligence/prompt-bindings/binding-1/rollback',
      {
        method: 'POST',
        body: {
          targetPromptVersionId: 'prompt-0',
          reasonCode: 'panel_rollback',
        },
      },
    ])
  })
})
