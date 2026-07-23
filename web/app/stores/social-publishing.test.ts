import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useCoreAccountStore } from '../../layers/core/stores/account'
import { setApiAccountIdProvider } from '~/utils/api-client'
import { useAuthStore } from './auth'
import { useSocialPublishingStore } from './social-publishing'
import type { SocialPublishingPostInput } from '~/domain/social-publishing/model'

function fetchMock() {
  return (globalThis as typeof globalThis & { $fetch: ReturnType<typeof vi.fn> }).$fetch
}

function authenticate(accountId = 'account-a') {
  const auth = useAuthStore()
  auth.accessToken = 'test-token'
  auth.user = { id: 'user-1', name: 'Operador' }
  auth.principal = {
    role: 'owner',
    permissions: ['social_publishing.view', 'social_publishing.manage'],
    permissionsResolved: true,
  }
  auth.hydrated = true

  const account = useCoreAccountStore()
  account.activeAccountId = accountId
  setApiAccountIdProvider(() => account.activeAccountId)
  return account
}

const postInput: SocialPublishingPostInput = {
  idempotencyKey: 'store-create-key',
  mediaType: 'image',
  caption: 'Legenda',
  mediaUrl: 'https://cdn.example.com/post.jpg',
  altText: 'Descrição',
  status: 'draft',
  scheduledFor: null,
  timezone: 'America/Sao_Paulo',
}

describe('useSocialPublishingStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    authenticate()
  })

  afterEach(() => {
    setApiAccountIdProvider(null)
  })

  it('loads connection, posts and overview for the active account', async () => {
    fetchMock().mockImplementation((path: string) => {
      if (path.endsWith('/connection')) {
        return Promise.resolve({
          status: 'active',
          igUserId: 'ig-1',
          username: 'cliente',
          secret: { set: true, last4: '9x8y' },
        })
      }
      if (path.endsWith('/analytics/posts')) {
        return Promise.resolve([
          {
            id: 'published-1',
            status: 'published',
            analytics: { views: 72, reach: 40, saved: 5 },
          },
        ])
      }
      if (path.endsWith('/posts')) {
        return Promise.resolve([
          {
            id: 'post-1',
            status: 'scheduled',
            mediaUrl: 'https://cdn.example.com/post.jpg',
            version: 2,
          },
          {
            id: 'published-1',
            status: 'published',
            mediaUrl: 'https://cdn.example.com/published.jpg',
            version: 3,
          },
        ])
      }
      if (path.endsWith('/overview')) {
        return Promise.resolve({
          counts: { scheduled: 1 },
          analytics: { views: 50, reach: 30, totalInteractions: 4 },
          upcoming: [],
        })
      }
      return Promise.reject(new Error(`Unexpected path: ${path}`))
    })
    const store = useSocialPublishingStore()

    await expect(store.initialize({ includeAnalytics: true })).resolves.toBe(true)

    expect(store.initialized).toBe(true)
    expect(store.connection?.igUserId).toBe('ig-1')
    expect(store.scheduledPosts.map((post) => post.id)).toEqual(['post-1'])
    expect(store.posts.find((post) => post.id === 'published-1')?.analytics?.views).toBe(72)
    expect(store.overview?.views).toBe(50)
    expect(
      fetchMock().mock.calls.every(
        ([, options]) => options?.headers?.['X-Account-Id'] === 'account-a',
      ),
    ).toBe(true)
  })

  it('never persists the technical token in store state', async () => {
    fetchMock().mockResolvedValue({
      status: 'active',
      igUserId: 'ig-1',
      secret: { set: true, last4: 'cret' },
    })
    const store = useSocialPublishingStore()

    await expect(store.connect('super-secret')).resolves.toBe(true)

    expect(JSON.stringify(store.$state)).not.toContain('super-secret')
    expect(store.connection?.secretLast4).toBe('cret')
    const request = fetchMock().mock.calls[0]?.[1] as { body?: string }
    expect(JSON.parse(request.body || '{}')).toEqual({ accessToken: 'super-secret' })
  })

  it('discards an old mutation response after the active account changes', async () => {
    let resolveOldCreate: (value: unknown) => void = () => undefined
    const oldCreate = new Promise<unknown>((resolve) => {
      resolveOldCreate = resolve
    })
    fetchMock().mockImplementation((path: string, options: { method?: string } = {}) => {
      if (path.endsWith('/posts') && options.method === 'POST') return oldCreate
      if (path.endsWith('/connection')) {
        return Promise.resolve({ status: 'disconnected', secret: { set: false } })
      }
      if (path.endsWith('/analytics/posts')) return Promise.resolve([])
      if (path.endsWith('/posts')) return Promise.resolve([])
      if (path.endsWith('/overview')) {
        return Promise.resolve({ counts: {}, analytics: {}, upcoming: [] })
      }
      return Promise.reject(new Error(`Unexpected path: ${path}`))
    })
    const account = useCoreAccountStore()
    const store = useSocialPublishingStore()
    await store.initialize({ includeAnalytics: true })

    const pendingSave = store.savePost(postInput)
    await vi.waitFor(() =>
      expect(
        fetchMock().mock.calls.some(
          ([path, options]) => path.endsWith('/posts') && options?.method === 'POST',
        ),
      ).toBe(true),
    )
    account.activeAccountId = 'account-b'
    await vi.waitFor(() =>
      expect(
        fetchMock().mock.calls.some(
          ([, options]) => options?.headers?.['X-Account-Id'] === 'account-b',
        ),
      ).toBe(true),
    )

    resolveOldCreate({
      id: 'post-from-account-a',
      status: 'draft',
      mediaUrl: 'https://cdn.example.com/old.jpg',
      version: 1,
    })

    await expect(pendingSave).resolves.toBeNull()
    await vi.waitFor(() => expect(store.initialized).toBe(true))
    expect(store.posts).toEqual([])
    expect(store.savingPost).toBe(false)
  })
})
