import type { createApiRequest } from '~/utils/api-client'
import {
  normalizeConnection,
  normalizeOverview,
  normalizePostAnalyticsList,
  normalizeSocialPost,
  normalizeSocialPostList,
  type SocialPostAnalyticsRecord,
  type SocialPublishingConnection,
  type SocialPublishingOverview,
  type SocialPublishingPost,
  type SocialPublishingPostInput,
} from './model'

export type SocialPublishingApiRequest = ReturnType<typeof createApiRequest>

type UnknownRecord = Record<string, unknown>

function asRecord(value: unknown): UnknownRecord {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? (value as UnknownRecord)
    : {}
}

function unwrapPost(value: unknown): SocialPublishingPost {
  const raw = asRecord(value)
  return normalizeSocialPost(raw.post ?? value)
}

function postPath(postId: string, suffix = ''): string {
  const id = encodeURIComponent(String(postId || '').trim())
  return `/v1/social-publishing/posts/${id}${suffix}`
}

export async function fetchConnection(
  api: SocialPublishingApiRequest,
): Promise<SocialPublishingConnection> {
  return normalizeConnection(await api('/v1/social-publishing/connection'))
}

export async function beginConnection(
  api: SocialPublishingApiRequest,
  accessToken: string,
): Promise<SocialPublishingConnection> {
  return normalizeConnection(
    await api('/v1/social-publishing/connection', {
      method: 'POST',
      body: { accessToken },
    }),
  )
}

export async function removeConnection(
  api: SocialPublishingApiRequest,
): Promise<SocialPublishingConnection> {
  return normalizeConnection(
    await api('/v1/social-publishing/connection', {
      method: 'DELETE',
    }),
  )
}

export async function fetchPosts(api: SocialPublishingApiRequest): Promise<SocialPublishingPost[]> {
  return normalizeSocialPostList(await api('/v1/social-publishing/posts'))
}

export async function createPost(
  api: SocialPublishingApiRequest,
  input: SocialPublishingPostInput,
): Promise<SocialPublishingPost> {
  return unwrapPost(
    await api('/v1/social-publishing/posts', {
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
    }),
  )
}

export async function updatePost(
  api: SocialPublishingApiRequest,
  postId: string,
  input: SocialPublishingPostInput,
): Promise<SocialPublishingPost> {
  return unwrapPost(
    await api(postPath(postId), {
      method: 'PATCH',
      body: {
        caption: input.caption,
        mediaUrl: input.mediaUrl,
        altText: input.altText,
        timezone: input.timezone,
        version: input.version ?? 0,
      },
    }),
  )
}

export async function schedulePost(
  api: SocialPublishingApiRequest,
  postId: string,
  scheduledFor: string,
  timezone: string,
  version: number,
): Promise<SocialPublishingPost> {
  return unwrapPost(
    await api(postPath(postId, '/schedule'), {
      method: 'POST',
      body: { scheduledFor, timezone, version },
    }),
  )
}

export async function cancelPost(
  api: SocialPublishingApiRequest,
  postId: string,
  version: number,
): Promise<SocialPublishingPost> {
  return unwrapPost(
    await api(postPath(postId, '/cancel'), {
      method: 'POST',
      body: { version },
    }),
  )
}

export async function retryPost(
  api: SocialPublishingApiRequest,
  postId: string,
  version: number,
): Promise<SocialPublishingPost> {
  return unwrapPost(
    await api(postPath(postId, '/retry'), {
      method: 'POST',
      body: { version },
    }),
  )
}

export async function fetchOverview(
  api: SocialPublishingApiRequest,
): Promise<SocialPublishingOverview> {
  return normalizeOverview(await api('/v1/social-publishing/overview'))
}

export async function fetchAnalyticsPosts(
  api: SocialPublishingApiRequest,
): Promise<SocialPostAnalyticsRecord[]> {
  return normalizePostAnalyticsList(await api('/v1/social-publishing/analytics/posts'))
}

export async function syncAnalytics(api: SocialPublishingApiRequest): Promise<void> {
  await api('/v1/social-publishing/analytics/sync', { method: 'POST' })
}
