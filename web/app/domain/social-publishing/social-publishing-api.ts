import type { createApiRequest } from '~/utils/api-client'
import {
  normalizeConnection,
  normalizeOverview,
  normalizePostAnalyticsList,
  normalizePublishingPortfolio,
  normalizePublishingScope,
  normalizeSocialPost,
  normalizeSocialPostList,
  type SocialPostAnalyticsRecord,
  type SocialPostStatus,
  type SocialPublishingConnection,
  type SocialPublishingOverview,
  type SocialPublishingPortfolio,
  type SocialPublishingPost,
  type SocialPublishingPostInput,
  type SocialPublishingScope,
} from './model'

export type SocialPublishingApiRequest = ReturnType<typeof createApiRequest>

export interface SocialPublishingPostPage {
  items: SocialPublishingPost[]
  offset: number
  pageSize: number
  hasNext: boolean
}

export interface SocialPublishingPostQuery {
  statuses: SocialPostStatus[]
  pageSize: number
  offset: number
  order?: 'scheduled'
}

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

export async function fetchPublishingScope(
  api: SocialPublishingApiRequest,
  accountId = '',
): Promise<SocialPublishingScope> {
  const normalizedAccountId = String(accountId || '').trim()
  if (!normalizedAccountId) {
    return normalizePublishingScope(await api('/v1/social-publishing/scope'))
  }
  return normalizePublishingScope(
    await api('/v1/social-publishing/scope', {
      headers: { 'X-Account-Id': normalizedAccountId },
    }),
  )
}

export async function fetchPublishingPortfolio(
  api: SocialPublishingApiRequest,
  accountId = '',
): Promise<SocialPublishingPortfolio> {
  const normalizedAccountId = String(accountId || '').trim()
  if (!normalizedAccountId) {
    return normalizePublishingPortfolio(await api('/v1/social-publishing/portfolio'))
  }
  return normalizePublishingPortfolio(
    await api('/v1/social-publishing/portfolio', {
      headers: { 'X-Account-Id': normalizedAccountId },
    }),
  )
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

export async function fetchPosts(
  api: SocialPublishingApiRequest,
  query: SocialPublishingPostQuery,
): Promise<SocialPublishingPostPage> {
  const pageSize = Math.max(1, Math.round(query.pageSize))
  const offset = Math.max(0, Math.round(query.offset))
  const requestLimit = pageSize + 1
  const params = new URLSearchParams({
    statuses: query.statuses.join(','),
    limit: String(requestLimit),
    offset: String(offset),
  })
  if (query.order) params.set('order', query.order)
  const items = normalizeSocialPostList(
    await api(`/v1/social-publishing/posts?${params.toString()}`),
  )
  return {
    items: items.slice(0, pageSize),
    offset,
    pageSize,
    hasNext: items.length > pageSize,
  }
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

export async function fetchSummary(
  api: SocialPublishingApiRequest,
): Promise<SocialPublishingOverview> {
  return normalizeOverview(await api('/v1/social-publishing/summary'))
}

export async function fetchAnalyticsPosts(
  api: SocialPublishingApiRequest,
  postIds: string[] = [],
): Promise<SocialPostAnalyticsRecord[]> {
  const uniquePostIds = [...new Set(postIds.map((id) => String(id || '').trim()).filter(Boolean))]
  const params = new URLSearchParams()
  if (uniquePostIds.length) params.set('postIds', uniquePostIds.join(','))
  const query = params.toString()
  const path = `/v1/social-publishing/analytics/posts${query ? `?${query}` : ''}`
  return normalizePostAnalyticsList(await api(path))
}

export async function syncAnalytics(api: SocialPublishingApiRequest): Promise<number> {
  const response = asRecord(await api('/v1/social-publishing/analytics/sync', { method: 'POST' }))
  const queued = Number(response.queued)
  return Number.isFinite(queued) ? Math.max(0, Math.round(queued)) : 0
}
