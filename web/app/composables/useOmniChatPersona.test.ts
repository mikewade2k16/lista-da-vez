import { createPinia, setActivePinia } from 'pinia'
import { effectScope, ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  normalizeOmniAssistantSurfaceModules,
  OMNI_ASSISTANT_SURFACE_MODULE_DEFAULTS,
  useOmniChatPersona,
} from './useOmniChatPersona'
import { useAuthStore } from '~/stores/auth'

function getFetchMock() {
  return (globalThis as typeof globalThis & { $fetch: ReturnType<typeof vi.fn> }).$fetch
}

function authenticateSession(): void {
  const auth = useAuthStore()
  auth.accessToken = 'test-token'
  auth.user = { id: 'user-1', name: 'Agency Owner' }
  auth.principal = { role: 'owner', permissions: [], permissionsResolved: true }
  auth.hydrated = true
}

describe('normalizeOmniAssistantSurfaceModules', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    authenticateSession()
  })

  it('returns the complete safe defaults for legacy responses', () => {
    const normalized = normalizeOmniAssistantSurfaceModules(undefined)

    expect(normalized).toEqual(OMNI_ASSISTANT_SURFACE_MODULE_DEFAULTS)
    expect(Object.keys(normalized)).toEqual(['calendar', 'meta_ads', 'global'])
    expect(Object.keys(normalized.calendar)).toEqual(['calendar', 'tasks', 'meta_ads', 'users'])
  })

  it('normalizes valid modes and falls back cell by cell for partial invalid data', () => {
    const normalized = normalizeOmniAssistantSurfaceModules({
      calendar: {
        calendar: ' READ ',
        tasks: 'invalid',
        meta_ads: 'WRITE',
        unknown: 'write',
      },
      meta_ads: null,
      global: {
        users: 'off',
      },
      unknown: {
        calendar: 'write',
      },
    })

    expect(normalized).toEqual({
      calendar: { calendar: 'read', tasks: 'write', meta_ads: 'write', users: 'read' },
      meta_ads: { calendar: 'off', tasks: 'off', meta_ads: 'write', users: 'off' },
      global: { calendar: 'read', tasks: 'read', meta_ads: 'read', users: 'off' },
    })
  })

  it('creates independent maps instead of exposing the shared defaults', () => {
    const first = normalizeOmniAssistantSurfaceModules(null)
    first.calendar.calendar = 'off'

    const second = normalizeOmniAssistantSurfaceModules(null)
    expect(second.calendar.calendar).toBe('write')
    expect(OMNI_ASSISTANT_SURFACE_MODULE_DEFAULTS.calendar.calendar).toBe('write')
  })

  it('hydrates an Anthropic credential without falling back to OpenAI', async () => {
    getFetchMock().mockResolvedValueOnce({
      enabled: true,
      systemPrompt: 'Assistente Calendar',
      credentialId: 'claude-id',
      provider: 'anthropic',
      model: 'claude-sonnet-4-6',
    })
    const scope = effectScope()
    const persona = scope.run(() => useOmniChatPersona(() => 'account-a'))!

    await persona.fetchPersona()

    expect(persona.credentialId.value).toBe('claude-id')
    expect(persona.provider.value).toBe('anthropic')
    expect(persona.model.value).toBe('claude-sonnet-4-6')
    scope.stop()
  })

  it('clears and aborts account-bound load/save without applying stale responses', async () => {
    interface PendingRequest {
      options: {
        body?: string
        headers?: Record<string, string>
        method?: string
        signal?: AbortSignal
      }
      resolve: (value: unknown) => void
    }

    const pending: PendingRequest[] = []
    getFetchMock().mockImplementation(
      (_path: string, options: PendingRequest['options']) =>
        new Promise((resolve) => pending.push({ options, resolve })),
    )
    const accountId = ref('account-a')
    const scope = effectScope()
    const persona = scope.run(() => useOmniChatPersona(() => accountId.value))!

    const accountALoad = persona.fetchPersona()
    expect(pending[0]?.options.headers?.['X-Account-Id']).toBe('account-a')

    accountId.value = 'account-b'
    expect(pending[0]?.options.signal?.aborted).toBe(true)
    expect(persona.draft.value).toBe('')
    expect(persona.credentialId.value).toBe('')
    expect(persona.ready.value).toBe(false)

    const accountBLoad = persona.fetchPersona()
    expect(pending[1]?.options.headers?.['X-Account-Id']).toBe('account-b')
    pending[1]?.resolve({
      enabled: true,
      systemPrompt: 'Prompt da conta B',
      inherited: true,
      inheritedFrom: 'Agência Canônica',
      credentialId: 'credential-b',
      provider: 'openai',
      model: 'gpt-4.1-mini',
    })
    await accountBLoad
    expect(persona.draft.value).toBe('Prompt da conta B')
    expect(persona.inherited.value).toBe(true)
    expect(persona.inheritedFrom.value).toBe('Agência Canônica')
    expect(persona.ready.value).toBe(true)

    pending[0]?.resolve({ systemPrompt: 'Prompt antigo da conta A' })
    await accountALoad
    expect(persona.draft.value).toBe('Prompt da conta B')

    persona.draft.value = 'Prompt editado da conta B'
    const accountBSave = persona.savePersona(persona.draft.value)
    expect(pending[2]?.options.method).toBe('PUT')
    expect(pending[2]?.options.headers?.['X-Account-Id']).toBe('account-b')
    expect(JSON.parse(pending[2]?.options.body || '{}').systemPrompt).toBe(
      'Prompt editado da conta B',
    )

    accountId.value = 'account-c'
    expect(pending[2]?.options.signal?.aborted).toBe(true)
    expect(persona.draft.value).toBe('')
    expect(persona.inherited.value).toBe(false)
    expect(persona.inheritedFrom.value).toBe('')
    expect(persona.ready.value).toBe(false)

    pending[2]?.resolve({ systemPrompt: 'Resposta tardia da conta B' })
    await accountBSave
    expect(persona.draft.value).toBe('')
    expect(persona.successMessage.value).toBe('')

    scope.stop()
  })
})
