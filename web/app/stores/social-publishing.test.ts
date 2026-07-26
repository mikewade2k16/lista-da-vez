import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useCoreAccountStore } from '../../layers/core/stores/account'
import { setApiAccountIdProvider } from '~/utils/api-client'
import { useAuthStore } from './auth'
import { SOCIAL_PUBLISHING_PAGE_SIZE, useSocialPublishingStore } from './social-publishing'
import type { SocialPublishingPostInput } from '~/domain/social-publishing/model'

function fetchMock() {
  return (globalThis as typeof globalThis & { $fetch: ReturnType<typeof vi.fn> }).$fetch
}

function queryValue(path: string, key: string): string {
  return new URL(path, 'http://localhost').searchParams.get(key) || ''
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

  it('blocks individual reads and mutations while the portfolio mode is active', async () => {
    const store = useSocialPublishingStore()

    store.setPortfolioMode(true)

    await expect(store.initialize({ includeAnalytics: true })).resolves.toBe(false)
    await expect(store.connect('technical-token')).resolves.toBe(false)
    await expect(store.savePost(postInput)).resolves.toBeNull()
    expect(fetchMock()).not.toHaveBeenCalled()
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
      if (path.startsWith('/v1/social-publishing/analytics/posts?')) {
        return Promise.resolve([
          {
            id: 'published-1',
            status: 'published',
            analytics: { views: 72, reach: 40, saved: 5 },
          },
        ])
      }
      if (path.startsWith('/v1/social-publishing/posts?')) {
        if (queryValue(path, 'statuses').startsWith('scheduled')) {
          return Promise.resolve([
            {
              id: 'post-1',
              status: 'scheduled',
              mediaUrl: 'https://cdn.example.com/post.jpg',
              version: 2,
            },
          ])
        }
        return Promise.resolve([
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
          analytics: { views: 50, reach: 30, totalInteractions: 4 },
        })
      }
      if (path.endsWith('/summary')) {
        return Promise.resolve({ counts: { scheduled: 1 }, upcoming: [] })
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
      fetchMock().mock.calls.some(
        ([path]) =>
          path ===
          '/v1/social-publishing/posts?statuses=scheduled%2Cpublishing%2Cfailed&limit=25&offset=0&order=scheduled',
      ),
    ).toBe(true)
    expect(
      fetchMock().mock.calls.some(
        ([path]) =>
          path ===
          '/v1/social-publishing/posts?statuses=draft%2Cpublished%2Ccancelled&limit=25&offset=0',
      ),
    ).toBe(true)
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

  it('paginates queue and content independently with a look-ahead item', async () => {
    fetchMock().mockImplementation((path: string) => {
      if (path.endsWith('/connection')) {
        return Promise.resolve({ status: 'disconnected', secret: { set: false } })
      }
      if (path.startsWith('/v1/social-publishing/posts?')) {
        const statuses = queryValue(path, 'statuses')
        const offset = Number(queryValue(path, 'offset'))
        if (statuses.startsWith('scheduled') && offset === 0) {
          return Promise.resolve(
            Array.from({ length: 25 }, (_, index) => ({
              id: `queue-${index + 1}`,
              status: 'scheduled',
            })),
          )
        }
        if (statuses.startsWith('scheduled') && offset === 24) {
          return Promise.resolve([
            { id: 'queue-25', status: 'scheduled' },
            { id: 'queue-26', status: 'failed' },
          ])
        }
        return Promise.resolve([{ id: 'draft-1', status: 'draft' }])
      }
      if (path.endsWith('/summary')) {
        return Promise.resolve({ counts: { scheduled: 26, draft: 1 }, upcoming: [] })
      }
      return Promise.reject(new Error(`Unexpected path: ${path}`))
    })
    const store = useSocialPublishingStore()
    await store.initialize()

    expect(store.queuePosts).toHaveLength(SOCIAL_PUBLISHING_PAGE_SIZE)
    expect(store.queueHasNext).toBe(true)
    expect(store.contentPosts.map((post) => post.id)).toEqual(['draft-1'])

    await expect(store.loadPage('queue', 1)).resolves.toBe(true)

    expect(store.queuePage).toBe(1)
    expect(store.queuePosts.map((post) => post.id)).toEqual(['queue-25', 'queue-26'])
    expect(store.queueHasNext).toBe(false)
    expect(store.contentPage).toBe(0)
    expect(store.contentPosts.map((post) => post.id)).toEqual(['draft-1'])
  })

  it('keeps analytics pending through progressive polling until captures stabilize', async () => {
    let overviewCalls = 0
    let analyticsViews = 10
    fetchMock().mockImplementation((path: string, options: { method?: string } = {}) => {
      if (path.endsWith('/connection')) {
        return Promise.resolve({ status: 'active', secret: { set: true, last4: '1234' } })
      }
      if (path.endsWith('/analytics/sync') && options.method === 'POST') {
        analyticsViews = 20
        return Promise.resolve({ queued: 1 })
      }
      if (path.startsWith('/v1/social-publishing/analytics/posts?')) {
        return Promise.resolve([{ postId: 'published-1', views: analyticsViews }])
      }
      if (path.endsWith('/overview')) {
        overviewCalls += 1
        return Promise.resolve({
          counts: {},
          analytics: {
            views: analyticsViews,
            capturedAt: overviewCalls > 1 ? '2026-07-23T16:30:00Z' : '2026-07-23T16:00:00Z',
          },
          upcoming: [],
        })
      }
      if (path.endsWith('/summary')) {
        return Promise.resolve({ counts: { published: 1 }, upcoming: [] })
      }
      if (path.startsWith('/v1/social-publishing/posts?')) {
        return queryValue(path, 'statuses').startsWith('draft')
          ? Promise.resolve([{ id: 'published-1', status: 'published' }])
          : Promise.resolve([])
      }
      return Promise.reject(new Error(`Unexpected path: ${path}`))
    })
    const store = useSocialPublishingStore()
    await store.initialize({ includeAnalytics: true })

    await expect(store.refreshAnalytics()).resolves.toBe(1)
    expect(store.analyticsSyncPending).toBe(true)
    expect(store.hasPollingWork).toBe(true)

    await expect(store.poll()).resolves.toBe(true)

    expect(store.analyticsSyncPending).toBe(true)
    expect(store.overview?.capturedAt).toBe('2026-07-23T16:30:00Z')
    expect(store.contentPosts[0]?.analytics?.views).toBe(20)

    for (let pollIndex = 0; pollIndex < 3; pollIndex += 1) {
      await expect(store.poll()).resolves.toBe(true)
    }
    expect(store.analyticsSyncPending).toBe(true)

    await expect(store.poll()).resolves.toBe(true)
    expect(store.analyticsSyncPending).toBe(false)
  })

  it('sleeps until a distant schedule enters the five-minute monitoring window', async () => {
    const scheduledFor = new Date(Date.now() + 24 * 60 * 60_000).toISOString()
    fetchMock().mockImplementation((path: string) => {
      if (path.endsWith('/connection')) {
        return Promise.resolve({ status: 'active', secret: { set: true, last4: '1234' } })
      }
      if (path.startsWith('/v1/social-publishing/posts?')) {
        return queryValue(path, 'statuses').startsWith('scheduled')
          ? Promise.resolve([{ id: 'future-1', status: 'scheduled', scheduledFor }])
          : Promise.resolve([])
      }
      if (path.endsWith('/summary')) {
        return Promise.resolve({
          counts: { scheduled: 1 },
          upcoming: [{ id: 'future-1', status: 'scheduled', scheduledFor }],
        })
      }
      return Promise.reject(new Error(`Unexpected path: ${path}`))
    })
    const store = useSocialPublishingStore()

    await expect(store.initialize()).resolves.toBe(true)

    expect(store.hasPollingWork).toBe(false)
    expect(store.nextPollingWakeAt).toBe(Date.parse(scheduledFor) - 5 * 60_000)
  })

  it('uses a failure-free monitor query so old failures cannot hide active publishing', async () => {
    fetchMock().mockImplementation((path: string) => {
      if (path.endsWith('/connection')) {
        return Promise.resolve({ status: 'active', secret: { set: true, last4: '1234' } })
      }
      if (path.endsWith('/summary')) {
        return Promise.resolve({ counts: { failed: 30, publishing: 1 }, upcoming: [] })
      }
      if (path.startsWith('/v1/social-publishing/posts?')) {
        const statuses = queryValue(path, 'statuses')
        if (statuses === 'scheduled,publishing') {
          return Promise.resolve([{ id: 'active-1', status: 'publishing' }])
        }
        if (statuses === 'scheduled,publishing,failed') {
          return Promise.resolve(
            Array.from({ length: 25 }, (_, index) => ({
              id: `failed-${index + 1}`,
              status: 'failed',
            })),
          )
        }
        return Promise.resolve([])
      }
      return Promise.reject(new Error(`Unexpected path: ${path}`))
    })
    const store = useSocialPublishingStore()

    await expect(store.initialize()).resolves.toBe(true)

    expect(store.queuePosts.every((post) => post.status === 'failed')).toBe(true)
    expect(store.hasActivePublishing).toBe(true)
    expect(store.hasPollingWork).toBe(true)
  })

  it('requests analytics only for published posts on the selected content page', async () => {
    fetchMock().mockImplementation((path: string) => {
      if (path.endsWith('/connection')) {
        return Promise.resolve({ status: 'active', secret: { set: true, last4: '1234' } })
      }
      if (path.endsWith('/summary')) {
        return Promise.resolve({ counts: { published: 2 }, upcoming: [] })
      }
      if (path.endsWith('/overview')) {
        return Promise.resolve({ analytics: { views: 40 } })
      }
      if (path.startsWith('/v1/social-publishing/analytics/posts?')) {
        const postIds = queryValue(path, 'postIds')
        return Promise.resolve([{ postId: postIds, views: postIds === 'published-2' ? 22 : 18 }])
      }
      if (path.startsWith('/v1/social-publishing/posts?')) {
        const statuses = queryValue(path, 'statuses')
        const offset = Number(queryValue(path, 'offset'))
        if (statuses === 'draft,published,cancelled') {
          return Promise.resolve([
            {
              id: offset === SOCIAL_PUBLISHING_PAGE_SIZE ? 'published-2' : 'published-1',
              status: 'published',
            },
          ])
        }
        return Promise.resolve([])
      }
      return Promise.reject(new Error(`Unexpected path: ${path}`))
    })
    const store = useSocialPublishingStore()
    await store.initialize({ includeAnalytics: true })

    await expect(store.loadPage('content', 1)).resolves.toBe(true)

    expect(store.contentPosts[0]?.id).toBe('published-2')
    expect(store.contentPosts[0]?.analytics?.views).toBe(22)
    expect(
      fetchMock().mock.calls.some(
        ([path]) => path === '/v1/social-publishing/analytics/posts?postIds=published-2',
      ),
    ).toBe(true)
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
      if (path.startsWith('/v1/social-publishing/analytics/posts')) return Promise.resolve([])
      if (path.startsWith('/v1/social-publishing/posts?')) return Promise.resolve([])
      if (path.endsWith('/summary')) {
        return Promise.resolve({ counts: {}, upcoming: [] })
      }
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
