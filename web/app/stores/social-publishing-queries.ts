import * as publishingApi from '~/domain/social-publishing/social-publishing-api'
import type {
  SocialPostAnalyticsRecord,
  SocialPostStatus,
  SocialPublishingOverview,
} from '~/domain/social-publishing/model'

const MONITOR_STATUSES: SocialPostStatus[] = ['scheduled', 'publishing']

export function mergeSocialPublishingSummary(
  summary: SocialPublishingOverview,
  analytics: SocialPublishingOverview | null,
): SocialPublishingOverview {
  return {
    ...summary,
    views: analytics?.views ?? 0,
    reach: analytics?.reach ?? 0,
    totalInteractions: analytics?.totalInteractions ?? 0,
    likes: analytics?.likes ?? 0,
    comments: analytics?.comments ?? 0,
    saved: analytics?.saved ?? 0,
    shares: analytics?.shares ?? 0,
    capturedAt: analytics?.capturedAt ?? null,
  }
}

export function fetchSocialPublishingMonitor(
  api: publishingApi.SocialPublishingApiRequest,
  pageSize: number,
): Promise<publishingApi.SocialPublishingPostPage> {
  return publishingApi.fetchPosts(api, {
    statuses: MONITOR_STATUSES,
    pageSize,
    offset: 0,
    order: 'scheduled',
  })
}

export function fetchSocialPublishingPageAnalytics(
  api: publishingApi.SocialPublishingApiRequest,
  page: publishingApi.SocialPublishingPostPage,
  enabled: boolean,
): Promise<SocialPostAnalyticsRecord[]> {
  if (!enabled) return Promise.resolve([])
  const postIds = page.items
    .filter((post) => post.status === 'published')
    .map((post) => post.id)
    .filter(Boolean)
  return postIds.length ? publishingApi.fetchAnalyticsPosts(api, postIds) : Promise.resolve([])
}
