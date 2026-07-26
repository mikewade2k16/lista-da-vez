import { describe, expect, it, vi } from 'vitest'
import {
  configureIntelligenceCredential,
  configureIntelligenceModel,
  createIntelligenceAgent,
  createIntelligenceAgentVersion,
  fetchIntelligenceAgents,
  fetchIntelligenceCredentials,
  publishIntelligenceAgentVersion,
  revokeIntelligenceCredential,
  updateIntelligenceAgent,
} from './agent-admin-api'

describe('customer intelligence agent admin HTTP contracts', () => {
  it('keeps credentials write-only and exposes only the secret status on reads', async () => {
    const api = vi
      .fn()
      .mockResolvedValueOnce({
        id: 'credential-1',
        provider: 'openai',
        label: 'Producao',
        apiKey: 'unexpected-leak',
        secret: { set: true, last4: '1234', ciphertext: 'unexpected' },
        updatedAt: '2026-07-23T12:00:00Z',
      })
      .mockResolvedValueOnce([
        {
          id: 'credential-1',
          provider: 'openai',
          label: 'Producao',
          apiKey: 'unexpected-leak',
          secret: { set: true, last4: '1234', ciphertext: 'unexpected' },
        },
      ])

    const written = await configureIntelligenceCredential(api, {
      provider: 'openai',
      label: 'Producao',
      apiKey: 'sk-write-only',
    })
    const listed = await fetchIntelligenceCredentials(api)

    expect(api.mock.calls[0]).toEqual([
      '/v1/customer-intelligence/credentials',
      {
        method: 'PUT',
        body: {
          provider: 'openai',
          label: 'Producao',
          apiKey: 'sk-write-only',
        },
      },
    ])
    expect(written).not.toHaveProperty('apiKey')
    expect(written.secret).toEqual({ set: true, last4: '1234' })
    expect(listed[0]).not.toHaveProperty('apiKey')
    expect(listed[0]?.secret).toEqual({ set: true, last4: '1234' })
  })

  it('uses the registered model PUT and preserves revision and opaque config', async () => {
    const api = vi.fn().mockResolvedValue({
      id: 'model-1',
      provider: 'gemini',
      model: 'gemini-2.5-flash',
      baseUrl: 'https://example.invalid',
      status: 'disabled',
      config: { capabilities: ['text'] },
      revision: 4,
    })

    await configureIntelligenceModel(api, {
      id: 'model-1',
      provider: 'gemini',
      model: 'gemini-2.5-flash',
      baseUrl: 'https://example.invalid',
      status: 'disabled',
      config: { capabilities: ['text'] },
      revision: 3,
    })

    expect(api).toHaveBeenCalledWith(
      '/v1/customer-intelligence/models',
      expect.objectContaining({
        method: 'PUT',
        body: expect.objectContaining({
          revision: 3,
          config: { capabilities: ['text'] },
        }),
      }),
    )
  })

  it('sends explicit client scope and exact agent create and patch contracts', async () => {
    const api = vi
      .fn()
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({
        id: 'agent-1',
        name: 'Resumo',
        purpose: 'resumo_cliente',
        status: 'disabled',
        revision: 1,
      })
      .mockResolvedValueOnce({
        id: 'agent-1',
        name: 'Resumo atualizado',
        purpose: 'resumo_cliente',
        status: 'disabled',
        revision: 2,
      })

    await fetchIntelligenceAgents(api, 'client-1')
    await createIntelligenceAgent(api, {
      clientAccountId: 'client-1',
      slug: 'resumo_cliente',
      name: 'Resumo',
    })
    await updateIntelligenceAgent(api, 'agent-1', {
      name: 'Resumo atualizado',
      enabled: false,
      expectedRevision: 1,
    })

    expect(api.mock.calls[0]?.[0]).toBe('/v1/customer-intelligence/agents?clientAccountId=client-1')
    expect(api.mock.calls[1]).toEqual([
      '/v1/customer-intelligence/agents',
      {
        method: 'POST',
        body: {
          clientAccountId: 'client-1',
          slug: 'resumo_cliente',
          name: 'Resumo',
        },
      },
    ])
    expect(api.mock.calls[2]).toEqual([
      '/v1/customer-intelligence/agents/agent-1',
      {
        method: 'PATCH',
        body: {
          name: 'Resumo atualizado',
          enabled: false,
          expectedRevision: 1,
        },
      },
    ])
  })

  it('creates and publishes versions only through registered POST routes', async () => {
    const version = {
      id: 'version-1',
      agentId: 'agent-1',
      version: 1,
      status: 'draft',
      modelId: 'model-1',
      credentialId: 'credential-1',
      temperature: 0.2,
      maxOutputTokens: 2000,
      timeoutMs: 30000,
      promptOverride: '',
      config: {},
    }
    const api = vi
      .fn()
      .mockResolvedValueOnce(version)
      .mockResolvedValueOnce({ ...version, status: 'published' })
      .mockResolvedValueOnce(undefined)

    await createIntelligenceAgentVersion(api, 'agent-1', {
      modelId: 'model-1',
      credentialId: 'credential-1',
      temperature: 0.2,
      maxOutputTokens: 2000,
      timeoutMs: 30000,
      promptOverride: '',
      config: {},
    })
    await publishIntelligenceAgentVersion(api, 'version-1')
    await revokeIntelligenceCredential(api, 'credential-1')

    expect(api.mock.calls[0]?.[0]).toBe('/v1/customer-intelligence/agents/agent-1/versions')
    expect(api.mock.calls[1]).toEqual([
      '/v1/customer-intelligence/agent-versions/version-1/publish',
      { method: 'POST' },
    ])
    expect(api.mock.calls[2]).toEqual([
      '/v1/customer-intelligence/credentials/credential-1',
      { method: 'DELETE' },
    ])
  })
})
