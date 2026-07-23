import { describe, expect, it } from 'vitest'

import {
  createSocialPublishingIdempotencyKey,
  isHttpsMediaUrl,
  mergeSocialPostAnalytics,
  normalizeConnection,
  normalizeOverview,
  normalizePostAnalyticsList,
  normalizeSocialPost,
} from './model'

describe('social publishing normalizers', () => {
  it('normalizes the current Instagram post contract and analytics aliases', () => {
    const post = normalizeSocialPost({
      id: ' post-1 ',
      caption: ' Conteúdo ',
      mediaUrl: 'https://cdn.example.com/post.jpg',
      status: 'published',
      externalMediaId: 'ig-media-1',
      permalink: 'https://www.instagram.com/p/example/',
      lastErrorCode: 'provider_error',
      lastErrorMessage: ' provider error ',
      analytics: {
        views: 120,
        reach: 95,
        totalInteractions: 21,
        likes: 14,
        comments: 3,
        saves: 2,
        shares: 2,
        capturedAt: '2026-07-23T14:00:00Z',
      },
    })

    expect(post).toMatchObject({
      id: 'post-1',
      mediaType: 'image',
      externalMediaId: 'ig-media-1',
      permalink: 'https://www.instagram.com/p/example/',
      lastErrorCode: 'provider_error',
      lastErrorMessage: 'provider error',
    })
    expect(post.analytics).toEqual({
      views: 120,
      reach: 95,
      totalInteractions: 21,
      likes: 14,
      comments: 3,
      saved: 2,
      shares: 2,
      capturedAt: '2026-07-23T14:00:00Z',
    })
  })

  it('normalizes connection secret status without exposing a token', () => {
    expect(
      normalizeConnection({
        status: 'active',
        igUserId: 'ig-user-1',
        username: 'cliente',
        accountType: 'BUSINESS',
        mediaCount: 18,
        secret: { set: true, last4: 'a1b2' },
        connectedAt: '2026-07-23T10:00:00Z',
      }),
    ).toMatchObject({
      status: 'connected',
      connected: true,
      igUserId: 'ig-user-1',
      username: 'cliente',
      accountType: 'BUSINESS',
      mediaCount: 18,
      secretSet: true,
      secretLast4: 'a1b2',
    })
  })

  it('normalizes overview counts, aggregate analytics and upcoming posts', () => {
    const overview = normalizeOverview({
      counts: {
        draft: 2,
        scheduled: 3,
        publishing: 1,
        published: 8,
        failed: 1,
        cancelled: 4,
      },
      analytics: {
        views: 900,
        reach: 700,
        totalInteractions: 75,
        likes: 50,
        comments: 10,
        saved: 8,
        shares: 7,
        capturedAt: '2026-07-23T15:00:00Z',
      },
      upcoming: [
        {
          id: 'post-next',
          mediaUrl: 'https://cdn.example.com/next.jpg',
          status: 'scheduled',
        },
      ],
    })

    expect(overview).toMatchObject({
      draft: 2,
      scheduled: 3,
      publishing: 1,
      published: 8,
      failed: 1,
      cancelled: 4,
      views: 900,
      reach: 700,
      totalInteractions: 75,
      saved: 8,
      capturedAt: '2026-07-23T15:00:00Z',
    })
    expect(overview.upcoming.map((post) => post.id)).toEqual(['post-next'])
  })

  it('accepts only HTTPS media URLs', () => {
    expect(isHttpsMediaUrl('https://cdn.example.com/image.jpg')).toBe(true)
    expect(isHttpsMediaUrl('http://cdn.example.com/image.jpg')).toBe(false)
    expect(isHttpsMediaUrl('not-a-url')).toBe(false)
  })

  it('creates a UUID from secure randomness and merges analytics only by published post id', () => {
    const source = {
      getRandomValues(target: Uint8Array) {
        target.set(Uint8Array.from({ length: 16 }, (_, index) => index))
        return target
      },
    } as unknown as Crypto
    const key = createSocialPublishingIdempotencyKey(source)
    const posts = [
      normalizeSocialPost({ id: 'published-1', status: 'published' }),
      normalizeSocialPost({ id: 'draft-1', status: 'draft' }),
    ]
    const records = normalizePostAnalyticsList([
      { postId: 'published-1', views: 80, saved: 4 },
      { postId: 'draft-1', views: 999 },
    ])

    expect(key).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/)
    const merged = mergeSocialPostAnalytics(posts, records)
    expect(merged[0]?.analytics?.views).toBe(80)
    expect(merged[0]?.analytics?.saved).toBe(4)
    expect(merged[1]?.analytics).toBeNull()
  })
})
