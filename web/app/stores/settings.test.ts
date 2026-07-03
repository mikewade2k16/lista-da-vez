import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useAuthStore } from './auth'
import { useAppRuntimeStore } from './app-runtime'
import { useSettingsStore } from './settings'

function getFetchMock() {
  return (globalThis as any).$fetch as ReturnType<typeof vi.fn>
}

function authenticateSession(partial: Record<string, unknown> = {}) {
  const auth = useAuthStore()
  auth.accessToken = 'test-token'
  auth.user = { id: 'user-1', name: 'Teste' } as any
  auth.principal = { role: 'owner', permissions: [], permissionsResolved: true } as any
  auth.hydrated = true // ensureSession() curto-circuita (auth.ts)
  auth.activeTenantId = 'tenant-1'
  Object.assign(auth, partial)
  return auth
}

const httpError = (status: number, message: string) =>
  Object.assign(new Error(message), { statusCode: status, data: { error: { message } } })

describe('useSettingsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('fails fast when trying to persist settings without an authenticated session', async () => {
    const store = useSettingsStore()

    await expect(store.updateSetting('queueLimit', 5)).resolves.toEqual({
      ok: false,
      message: 'Sessao indisponivel.',
    })
  })

  it('fails fast when the session has no active tenant', async () => {
    authenticateSession({ activeTenantId: '', tenantContext: [] })
    const store = useSettingsStore()

    await expect(store.updateSetting('queueLimit', 5)).resolves.toEqual({
      ok: false,
      message: 'Tenant ativo nao identificado para a sessao.',
    })
    expect(getFetchMock()).not.toHaveBeenCalled()
  })

  it('persists an operation setting through the canonical endpoint', async () => {
    authenticateSession()
    const fetchMock = getFetchMock()
    fetchMock.mockResolvedValue({})
    const store = useSettingsStore()

    const result = await store.updateSetting('queueLimit', 5)

    expect(result.ok).toBe(true)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [path, options] = fetchMock.mock.calls[0]
    expect(path).toContain('/v1/settings/operation')
    expect(path).toContain('tenantId=tenant-1')
    expect(options.method).toBe('PATCH')
    // O api-client serializa o body para JSON antes do $fetch.
    expect(JSON.parse(options.body).settings.queueLimit).toBe(5)
  })

  it('rolls back the runtime state when the persistence fails', async () => {
    authenticateSession()
    const fetchMock = getFetchMock()
    fetchMock.mockRejectedValue(httpError(500, 'boom'))
    const store = useSettingsStore()

    // O runtime so ganha o shape canonico depois de hidratar (ensure()). O rollback
    // restaura para esse estado hidratado, entao o baseline tem que ser capturado
    // apos ensure(), nao sobre o mockQueueState cru — senao comparamos formas diferentes.
    await store.ensure()

    const before = JSON.parse(JSON.stringify(useAppRuntimeStore().state))
    const result = await store.updateSetting('queueLimit', 5)

    expect(result).toEqual({ ok: false, message: 'boom' })
    expect(JSON.parse(JSON.stringify(useAppRuntimeStore().state))).toEqual(before)
  })
})
