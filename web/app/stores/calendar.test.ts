import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useAuthStore } from './auth'
import { useCalendarStore } from './calendar'

function getFetchMock() {
  return (globalThis as typeof globalThis & { $fetch: ReturnType<typeof vi.fn> }).$fetch
}

function authenticateSession() {
  const auth = useAuthStore()
  auth.accessToken = 'test-token'
  auth.user = { id: 'user-1', name: 'Dr Lucas' }
  auth.principal = { role: 'owner', permissions: [], permissionsResolved: true }
  auth.hydrated = true
}

describe('useCalendarStore client scope', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    authenticateSession()
  })

  it('locks a client user to the server-provided client and rejects local changes', async () => {
    getFetchMock().mockResolvedValue({
      canSelect: false,
      lockedClientId: 'client-lucas',
      clients: [{ id: 'client-lucas', name: 'Dr Lucas Martins' }],
    })
    const store = useCalendarStore()

    await expect(store.fetchScope()).resolves.toBe(true)

    expect(store.canSelectClient).toBe(false)
    expect(store.lockedClientId).toBe('client-lucas')
    expect(store.selectedClientId).toBe('client-lucas')
    expect(store.effectiveClientId).toBe('client-lucas')
    expect(store.clients.map((client) => client.id)).toEqual(['client-lucas'])

    store.setClientFilter('outro-cliente')
    expect(store.selectedClientId).toBe('client-lucas')
    expect(store.effectiveClientId).toBe('client-lucas')
  })

  it('keeps agency selection editable but only accepts clients from scope', async () => {
    getFetchMock().mockResolvedValue({
      canSelect: true,
      lockedClientId: '',
      clients: [
        { id: 'client-1', name: 'Cliente 1' },
        { id: 'client-2', name: 'Cliente 2' },
      ],
    })
    const store = useCalendarStore()

    await store.fetchScope()
    store.setClientFilter('client-2')

    expect(store.canSelectClient).toBe(true)
    expect(store.selectedClientId).toBe('client-2')
    expect(store.effectiveClientId).toBe('client-2')

    store.setClientFilter('fora-do-escopo')
    expect(store.selectedClientId).toBe('')
    expect(store.effectiveClientId).toBe('')
  })

  it('forces the locked client in event write payloads', async () => {
    const fetchMock = getFetchMock()
    fetchMock.mockResolvedValue({
      canSelect: false,
      lockedClientId: 'client-lucas',
      clients: [{ id: 'client-lucas', name: 'Dr Lucas Martins' }],
    })
    const store = useCalendarStore()
    await store.fetchScope()
    fetchMock.mockImplementation((path: string) =>
      path.startsWith('/v1/calendar/events?')
        ? Promise.resolve({ events: [] })
        : Promise.resolve({}),
    )

    await store.createEvent({
      date: '2026-07-22',
      time: '10:00',
      clientId: 'forjado',
      type: 'post',
      title: 'Conteudo',
      status: 'planejado',
      priority: 'media',
      responsibleId: '',
      involvedIds: [],
      media: [],
      description: '',
    })

    const postCall = fetchMock.mock.calls.find(
      ([path, options]) => path === '/v1/calendar/events' && options?.method === 'POST',
    )
    expect(postCall).toBeTruthy()
    expect(JSON.parse(postCall?.[1]?.body as string).clientId).toBe('client-lucas')
  })
})
