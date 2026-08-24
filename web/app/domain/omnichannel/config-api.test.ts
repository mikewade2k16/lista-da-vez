import { describe, expect, it, vi } from 'vitest'

import {
  ASSISTANT_AI_CREDENTIALS_PATH,
  createAICredential,
  deleteAICredential,
  fetchAICredentialModels,
  fetchAICredentials,
  importLegacyAICredentials,
  normalizeAICredentials,
  updateAICredential,
} from './config-api'
import type { ApiRequest } from './config-api'

describe('assistant AI credential endpoint', () => {
  it('normalizes ownership metadata and keeps legacy own credentials mutable', () => {
    expect(
      normalizeAICredentials([
        {
          id: 'shared-id',
          name: 'Agencia OpenAI',
          provider: 'OPENAI',
          last4: '1234',
          ownedByAccount: false,
          ownerName: 'Crow Visuals',
          readOnly: true,
        },
        { id: 'own-id', name: 'Gemini', provider: 'gemini', last4: '9876' },
        { id: 'claude-id', name: 'Claude principal', provider: 'ANTHROPIC', last4: 'abcd' },
        { id: '', provider: 'openai' },
      ]),
    ).toEqual([
      expect.objectContaining({
        id: 'shared-id',
        provider: 'openai',
        ownedByAccount: false,
        ownerName: 'Crow Visuals',
        readOnly: true,
      }),
      expect.objectContaining({
        id: 'own-id',
        ownedByAccount: true,
        ownerName: '',
        readOnly: false,
      }),
      expect.objectContaining({
        id: 'claude-id',
        provider: 'anthropic',
        readOnly: false,
      }),
    ])
  })

  it('routes list, CRUD, import and models through the neutral assistant path', async () => {
    const request = vi.fn().mockResolvedValue([])
    const api = request as unknown as ApiRequest
    const options = {
      basePath: ASSISTANT_AI_CREDENTIALS_PATH,
      headers: { 'X-Account-Id': 'account-b' },
    }

    await fetchAICredentials(api, options)
    await createAICredential(
      api,
      { name: 'Principal', provider: 'openai', apiKey: 'secret' },
      options,
    )
    await updateAICredential(api, 'credential/1', { name: 'Nova' }, options)
    await deleteAICredential(api, 'credential/1', options)
    await importLegacyAICredentials(api, options)
    await fetchAICredentialModels(api, 'credential/1', 'response', options)

    expect(request.mock.calls.map(([path]) => path)).toEqual([
      '/v1/assistant/ai-credentials',
      '/v1/assistant/ai-credentials',
      '/v1/assistant/ai-credentials/credential%2F1',
      '/v1/assistant/ai-credentials/credential%2F1',
      '/v1/assistant/ai-credentials/import-legacy',
      '/v1/assistant/ai-credentials/credential%2F1/models?capability=response',
    ])
    expect(request.mock.calls.every(([, requestOptions]) => !('basePath' in requestOptions))).toBe(
      true,
    )
    expect(request.mock.calls[0]?.[1]?.headers).toEqual({ 'X-Account-Id': 'account-b' })
  })
})
