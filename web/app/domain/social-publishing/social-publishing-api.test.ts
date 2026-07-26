import { describe, expect, it, vi } from 'vitest'

import {
  beginConnection,
  cancelPost,
  createPost,
  fetchAnalyticsPosts,
  fetchPublishingPortfolio,
  fetchPublishingScope,
  fetchPosts,
  fetchSummary,
  retryPost,
  schedulePost,
  syncAnalytics,
  updatePost,
  type SocialPublishingApiRequest,
} from './social-publishing-api'
import type { SocialPublishingPostInput } from './model'

function apiMock(response: unknown = { id: 'post-1', status: 'draft' }) {
  return vi.fn().mockResolvedValue(response)
}

const input: SocialPublishingPostInput = {
  idempotencyKey: 'create-post-key',
  mediaType: 'image',
  caption: 'Legenda',
  mediaUrl: 'https://cdn.example.com/post.jpg',
  altText: 'Descrição',
  status: 'scheduled',
  scheduledFor: '2026-07-24T15:00:00.000Z',
  timezone: 'America/Sao_Paulo',
  version: 7,
}

describe('social publishing API contract', () => {
  it('sends a connection token only in the technical POST body', async () => {
    const request = apiMock({ status: 'active', secret: { set: true, last4: '1234' } })

    await beginConnection(request as unknown as SocialPublishingApiRequest, 'technical-token')

    expect(request).toHaveBeenCalledWith('/v1/social-publishing/connection', {
      method: 'POST',
      body: { accessToken: 'technical-token' },
    })
  })

  it('creates a post with only fields accepted by the strict backend decoder', async () => {
    const request = apiMock()

    await createPost(request as unknown as SocialPublishingApiRequest, input)

    expect(request).toHaveBeenCalledWith('/v1/social-publishing/posts', {
      method: 'POST',
      body: {
        idempotencyKey: input.idempotencyKey,
        caption: input.caption,
        mediaUrl: input.mediaUrl,
        altText: input.altText,
        status: input.status,
        scheduledFor: input.scheduledFor,
        timezone: input.timezone,
      },
    })
  })

  it('patches editable fields and optimistic version, then schedules explicitly', async () => {
    const request = apiMock()
    const api = request as unknown as SocialPublishingApiRequest

    await updatePost(api, 'post/1', input)
    await schedulePost(api, 'post/1', input.scheduledFor || '', input.timezone, 8)

    expect(request.mock.calls[0]).toEqual([
      '/v1/social-publishing/posts/post%2F1',
      {
        method: 'PATCH',
        body: {
          caption: input.caption,
          mediaUrl: input.mediaUrl,
          altText: input.altText,
          timezone: input.timezone,
          version: 7,
        },
      },
    ])
    expect(request.mock.calls[1]).toEqual([
      '/v1/social-publishing/posts/post%2F1/schedule',
      {
        method: 'POST',
        body: {
          scheduledFor: input.scheduledFor,
          timezone: input.timezone,
          version: 8,
        },
      },
    ])
  })

  it('sends the current version for cancel and retry actions', async () => {
    const request = apiMock()
    const api = request as unknown as SocialPublishingApiRequest

    await cancelPost(api, 'post-1', 9)
    await retryPost(api, 'post-1', 10)

    expect(request.mock.calls).toEqual([
      ['/v1/social-publishing/posts/post-1/cancel', { method: 'POST', body: { version: 9 } }],
      ['/v1/social-publishing/posts/post-1/retry', { method: 'POST', body: { version: 10 } }],
    ])
  })

  it('loads per-post analytics from its permission-scoped endpoint', async () => {
    const request = apiMock([
      { id: 'post-1', status: 'published', analytics: { views: 42, saved: 3 } },
    ])

    await expect(
      fetchAnalyticsPosts(request as unknown as SocialPublishingApiRequest, ['post-1', 'post-1']),
    ).resolves.toMatchObject([{ postId: 'post-1', views: 42, saved: 3 }])
    expect(request).toHaveBeenCalledWith('/v1/social-publishing/analytics/posts?postIds=post-1')
  })

  it('loads tenant summary from the view-scoped endpoint', async () => {
    const request = apiMock({ counts: { scheduled: 3 }, upcoming: [] })

    await expect(
      fetchSummary(request as unknown as SocialPublishingApiRequest),
    ).resolves.toMatchObject({ scheduled: 3, upcoming: [] })
    expect(request).toHaveBeenCalledWith('/v1/social-publishing/summary')
  })

  it('loads the selector scope using an explicit host account header', async () => {
    const request = apiMock({
      canSelect: true,
      clients: [{ id: ' client-1 ', name: ' Cliente um ' }],
    })

    await expect(
      fetchPublishingScope(request as unknown as SocialPublishingApiRequest, ' agency-account '),
    ).resolves.toEqual({
      canSelect: true,
      lockedClientId: '',
      clients: [{ id: 'client-1', name: 'Cliente um' }],
    })
    expect(request).toHaveBeenCalledWith('/v1/social-publishing/scope', {
      headers: { 'X-Account-Id': 'agency-account' },
    })
  })

  it('loads the consolidated portfolio without putting account scope in the URL', async () => {
    const request = apiMock({ clientCount: 1, clients: [{ accountId: 'client-1' }] })

    await expect(
      fetchPublishingPortfolio(request as unknown as SocialPublishingApiRequest, 'agency-account'),
    ).resolves.toMatchObject({
      clientCount: 1,
      clients: [{ accountId: 'client-1' }],
    })
    expect(request).toHaveBeenCalledWith('/v1/social-publishing/portfolio', {
      headers: { 'X-Account-Id': 'agency-account' },
    })
  })

  it('requests one look-ahead item for a chronological queue page', async () => {
    const response = Array.from({ length: 25 }, (_, index) => ({
      id: `post-${index + 1}`,
      status: 'scheduled',
    }))
    const request = apiMock(response)

    const page = await fetchPosts(request as unknown as SocialPublishingApiRequest, {
      statuses: ['scheduled', 'publishing', 'failed'],
      pageSize: 24,
      offset: 48,
      order: 'scheduled',
    })

    expect(request).toHaveBeenCalledWith(
      '/v1/social-publishing/posts?statuses=scheduled%2Cpublishing%2Cfailed&limit=25&offset=48&order=scheduled',
    )
    expect(page.items).toHaveLength(24)
    expect(page.hasNext).toBe(true)
    expect(page.offset).toBe(48)
  })

  it('returns the queued analytics job count', async () => {
    const request = apiMock({ queued: 3 })

    await expect(syncAnalytics(request as unknown as SocialPublishingApiRequest)).resolves.toBe(3)
  })
})
