import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import {
  createSocialPublishingMutations,
  type SocialPublishingRequestContext,
} from '~/stores/social-publishing-mutations'
import {
  fetchSocialPublishingMonitor,
  fetchSocialPublishingPageAnalytics,
  mergeSocialPublishingSummary,
} from '~/stores/social-publishing-queries'
import * as publishingApi from '~/domain/social-publishing/social-publishing-api'
import {
  mergeSocialPostAnalytics,
  type SocialPostAnalyticsRecord,
  type SocialPostStatus,
  type SocialPublishingConnection,
  type SocialPublishingOverview,
  type SocialPublishingPost,
} from '~/domain/social-publishing/model'

export const SOCIAL_PUBLISHING_PAGE_SIZE = 24
const ANALYTICS_POLL_LIMIT = 12
const ANALYTICS_STABLE_POLLS = 4
const SCHEDULE_POLL_LEAD_MS = 5 * 60_000

export type SocialPublishingCollection = 'queue' | 'content'

interface InitializeOptions {
  includeAnalytics?: boolean
}

interface RefreshOptions {
  includeConnection?: boolean
  refreshQueueMonitor?: boolean
  silent?: boolean
}

const COLLECTION_STATUSES: Record<SocialPublishingCollection, SocialPostStatus[]> = {
  queue: ['scheduled', 'publishing', 'failed'],
  content: ['draft', 'published', 'cancelled'],
}

export const useSocialPublishingStore = defineStore('social-publishing', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const connection = ref<SocialPublishingConnection | null>(null)
  const queuePosts = ref<SocialPublishingPost[]>([])
  const queueMonitorPosts = ref<SocialPublishingPost[]>([])
  const contentPosts = ref<SocialPublishingPost[]>([])
  const overview = ref<SocialPublishingOverview | null>(null)
  const analyticsRecords = ref<SocialPostAnalyticsRecord[]>([])
  const queuePage = ref(0)
  const contentPage = ref(0)
  const queueHasNext = ref(false)
  const contentHasNext = ref(false)
  const initialized = ref(false)
  const loading = ref(false)
  const refreshing = ref(false)
  const queueLoading = ref(false)
  const contentLoading = ref(false)
  const polling = ref(false)
  const error = ref('')
  const savingPost = ref(false)
  const connectionBusy = ref(false)
  const analyticsSyncing = ref(false)
  const analyticsSyncPending = ref(false)
  const busyPostIds = ref<string[]>([])
  const portfolioMode = ref(false)

  let generation = 0
  let snapshotVersion = 0
  let queueRequestVersion = 0
  let contentRequestVersion = 0
  let loadRequested = false
  let includeAnalyticsRequested = false
  let analyticsBaselineCapturedAt: string | null = null
  let analyticsLastCapturedAt: string | null = null
  let analyticsPollsRemaining = 0
  let analyticsStablePolls = 0

  const posts = computed(() => {
    const byId = new Map<string, SocialPublishingPost>()
    for (const post of [...queuePosts.value, ...contentPosts.value]) byId.set(post.id, post)
    return [...byId.values()]
  })
  const scheduledPosts = computed(() => queuePosts.value)
  const publishedPosts = computed(() =>
    contentPosts.value.filter((post) => post.status === 'published'),
  )
  const scheduleCandidates = computed(() => [
    ...queueMonitorPosts.value,
    ...(overview.value?.upcoming ?? []),
  ])
  const hasActivePublishing = computed(() => {
    const now = Date.now()
    return (
      queueMonitorPosts.value.some((post) => post.status === 'publishing') ||
      (overview.value?.publishing ?? 0) > 0 ||
      scheduleCandidates.value.some(
        (post) =>
          post.status === 'scheduled' &&
          Number.isFinite(Date.parse(post.scheduledFor || '')) &&
          Date.parse(post.scheduledFor || '') <= now + SCHEDULE_POLL_LEAD_MS,
      )
    )
  })
  const nextPollingWakeAt = computed(() => {
    const now = Date.now()
    const wakeTimes = scheduleCandidates.value
      .filter((post) => post.status === 'scheduled')
      .map((post) => Date.parse(post.scheduledFor || '') - SCHEDULE_POLL_LEAD_MS)
      .filter((timestamp) => Number.isFinite(timestamp) && timestamp > now)
    return wakeTimes.length ? Math.min(...wakeTimes) : null
  })
  const hasPollingWork = computed(() => hasActivePublishing.value || analyticsSyncPending.value)

  function currentAccountId(): string {
    return String(accountStore.activeAccountId || '').trim()
  }

  function captureContext(): SocialPublishingRequestContext {
    return {
      accountId: portfolioMode.value ? '' : currentAccountId(),
      generation,
    }
  }

  function isCurrent(context: SocialPublishingRequestContext): boolean {
    return (
      context.generation === generation &&
      Boolean(context.accountId) &&
      context.accountId === currentAccountId()
    )
  }

  function resetData(): void {
    connection.value = null
    queuePosts.value = []
    queueMonitorPosts.value = []
    contentPosts.value = []
    overview.value = null
    analyticsRecords.value = []
    queuePage.value = 0
    contentPage.value = 0
    queueHasNext.value = false
    contentHasNext.value = false
    initialized.value = false
    loading.value = false
    refreshing.value = false
    queueLoading.value = false
    contentLoading.value = false
    polling.value = false
    error.value = ''
    savingPost.value = false
    connectionBusy.value = false
    analyticsSyncing.value = false
    analyticsSyncPending.value = false
    busyPostIds.value = []
    analyticsBaselineCapturedAt = null
    analyticsLastCapturedAt = null
    analyticsPollsRemaining = 0
    analyticsStablePolls = 0
  }

  function mergeKnownAnalytics(items: SocialPublishingPost[]): SocialPublishingPost[] {
    return mergeSocialPostAnalytics(items, analyticsRecords.value)
  }

  function setPage(
    collection: SocialPublishingCollection,
    pageIndex: number,
    page: publishingApi.SocialPublishingPostPage,
  ): void {
    const items = mergeKnownAnalytics(page.items)
    if (collection === 'queue') {
      queuePage.value = pageIndex
      queuePosts.value = items
      queueHasNext.value = page.hasNext
      return
    }
    contentPage.value = pageIndex
    contentPosts.value = items
    contentHasNext.value = page.hasNext
  }

  function fetchPage(collection: SocialPublishingCollection, pageIndex: number) {
    return publishingApi.fetchPosts(apiRequest, {
      statuses: COLLECTION_STATUSES[collection],
      pageSize: SOCIAL_PUBLISHING_PAGE_SIZE,
      offset: pageIndex * SOCIAL_PUBLISHING_PAGE_SIZE,
      order: collection === 'queue' ? 'scheduled' : undefined,
    })
  }

  function fetchQueueMonitor() {
    return fetchSocialPublishingMonitor(apiRequest, SOCIAL_PUBLISHING_PAGE_SIZE)
  }

  function fetchPageAnalytics(page: publishingApi.SocialPublishingPostPage) {
    return fetchSocialPublishingPageAnalytics(apiRequest, page, includeAnalyticsRequested)
  }

  function applyAnalytics(records: SocialPostAnalyticsRecord[]): void {
    analyticsRecords.value = records
    queuePosts.value = mergeKnownAnalytics(queuePosts.value)
    queueMonitorPosts.value = mergeKnownAnalytics(queueMonitorPosts.value)
    contentPosts.value = mergeKnownAnalytics(contentPosts.value)
  }

  function updateAnalyticsPending(nextOverview: SocialPublishingOverview | null): void {
    if (!analyticsSyncPending.value) return
    const capturedAt = nextOverview?.capturedAt ?? null
    analyticsPollsRemaining -= 1
    if (capturedAt && capturedAt !== analyticsLastCapturedAt) {
      analyticsLastCapturedAt = capturedAt
      analyticsStablePolls = 0
    } else if (capturedAt && capturedAt !== analyticsBaselineCapturedAt) {
      analyticsStablePolls += 1
    }
    if (analyticsStablePolls >= ANALYTICS_STABLE_POLLS || analyticsPollsRemaining <= 0) {
      analyticsSyncPending.value = false
      analyticsPollsRemaining = 0
    }
  }

  async function refreshSnapshot(options: RefreshOptions = {}): Promise<boolean> {
    const context = captureContext()
    if (!auth.isAuthenticated || !context.accountId) return false
    const requestVersion = ++snapshotVersion
    const queueVersion = queueRequestVersion
    const contentVersion = contentRequestVersion
    const queueIndex = queuePage.value
    const contentIndex = contentPage.value
    if (!options.silent) error.value = ''
    try {
      const nextContentPromise = fetchPage('content', contentIndex)
      const nextAnalyticsPromise = nextContentPromise.then(fetchPageAnalytics)
      const [
        nextConnection,
        nextQueue,
        nextContent,
        nextSummary,
        nextAnalyticsOverview,
        nextAnalytics,
        nextMonitor,
      ] = await Promise.all([
        options.includeConnection
          ? publishingApi.fetchConnection(apiRequest)
          : Promise.resolve(connection.value),
        fetchPage('queue', queueIndex),
        nextContentPromise,
        publishingApi.fetchSummary(apiRequest),
        includeAnalyticsRequested
          ? publishingApi.fetchOverview(apiRequest)
          : Promise.resolve<SocialPublishingOverview | null>(null),
        nextAnalyticsPromise,
        options.refreshQueueMonitor
          ? fetchQueueMonitor()
          : Promise.resolve<publishingApi.SocialPublishingPostPage | null>(null),
      ])
      if (!isCurrent(context) || requestVersion !== snapshotVersion) return false
      connection.value = nextConnection
      const nextOverview = mergeSocialPublishingSummary(nextSummary, nextAnalyticsOverview)
      overview.value = nextOverview
      applyAnalytics(nextAnalytics)
      if (queueVersion === queueRequestVersion && queueIndex === queuePage.value) {
        setPage('queue', queueIndex, nextQueue)
      }
      if (contentVersion === contentRequestVersion && contentIndex === contentPage.value) {
        setPage('content', contentIndex, nextContent)
      }
      if (nextMonitor) queueMonitorPosts.value = mergeKnownAnalytics(nextMonitor.items)
      updateAnalyticsPending(nextOverview)
      return true
    } catch (caught) {
      if (isCurrent(context) && requestVersion === snapshotVersion && !options.silent) {
        error.value = getApiErrorMessage(caught, 'Não foi possível atualizar as publicações.')
      }
      return false
    }
  }

  async function initialize(options: InitializeOptions = {}): Promise<boolean> {
    loadRequested = true
    includeAnalyticsRequested = options.includeAnalytics === true
    queuePage.value = 0
    contentPage.value = 0
    const context = captureContext()
    if (!auth.isAuthenticated || !context.accountId) return false
    loading.value = true
    const result = await refreshSnapshot({
      includeConnection: true,
      refreshQueueMonitor: true,
    })
    if (isCurrent(context)) {
      initialized.value = result
      loading.value = false
    }
    return result
  }

  async function loadPage(
    collection: SocialPublishingCollection,
    requestedPage: number,
  ): Promise<boolean> {
    const pageIndex = Math.max(0, Math.round(requestedPage))
    const context = captureContext()
    if (!context.accountId) return false
    const requestVersion = collection === 'queue' ? ++queueRequestVersion : ++contentRequestVersion
    if (collection === 'queue') queueLoading.value = true
    else contentLoading.value = true
    error.value = ''
    try {
      const page = await fetchPage(collection, pageIndex)
      const nextAnalytics = collection === 'content' ? await fetchPageAnalytics(page) : null
      const currentVersion = collection === 'queue' ? queueRequestVersion : contentRequestVersion
      if (!isCurrent(context) || requestVersion !== currentVersion) return false
      if (nextAnalytics) applyAnalytics(nextAnalytics)
      setPage(collection, pageIndex, page)
      return true
    } catch (caught) {
      if (isCurrent(context)) {
        error.value = getApiErrorMessage(caught, 'Não foi possível carregar esta página.')
      }
      return false
    } finally {
      if (isCurrent(context)) {
        if (collection === 'queue' && requestVersion === queueRequestVersion) {
          queueLoading.value = false
        }
        if (collection === 'content' && requestVersion === contentRequestVersion) {
          contentLoading.value = false
        }
      }
    }
  }

  async function refreshWorkspace(): Promise<boolean> {
    if (refreshing.value) return false
    refreshing.value = true
    const context = captureContext()
    const result = await refreshSnapshot({
      includeConnection: true,
      refreshQueueMonitor: true,
    })
    if (isCurrent(context)) refreshing.value = false
    return result
  }

  async function poll(force = false): Promise<boolean> {
    if (polling.value || (!force && !hasPollingWork.value)) return false
    polling.value = true
    const context = captureContext()
    const result = await refreshSnapshot({ refreshQueueMonitor: true, silent: true })
    if (isCurrent(context)) polling.value = false
    return result
  }

  function updateCollectionPost(
    items: SocialPublishingPost[],
    pageIndex: number,
    belongs: boolean,
    post: SocialPublishingPost,
  ): SocialPublishingPost[] {
    const index = items.findIndex((entry) => entry.id === post.id)
    if (!belongs) return index < 0 ? items : items.filter((entry) => entry.id !== post.id)
    if (index >= 0) return items.map((entry) => (entry.id === post.id ? post : entry))
    return pageIndex === 0 ? [post, ...items].slice(0, SOCIAL_PUBLISHING_PAGE_SIZE) : items
  }

  function replacePost(post: SocialPublishingPost): void {
    if (!post.id) return
    const previous = posts.value.find((entry) => entry.id === post.id)
    const nextPost =
      !post.analytics && previous?.analytics ? { ...post, analytics: previous.analytics } : post
    const inQueue = COLLECTION_STATUSES.queue.includes(nextPost.status)
    const inContent = COLLECTION_STATUSES.content.includes(nextPost.status)
    queuePosts.value = updateCollectionPost(queuePosts.value, queuePage.value, inQueue, nextPost)
    queueMonitorPosts.value = updateCollectionPost(queueMonitorPosts.value, 0, inQueue, nextPost)
    contentPosts.value = updateCollectionPost(
      contentPosts.value,
      contentPage.value,
      inContent,
      nextPost,
    )
  }

  const mutations = createSocialPublishingMutations({
    apiRequest,
    connection,
    error,
    savingPost,
    connectionBusy,
    busyPostIds,
    captureContext,
    isCurrent,
    replacePost,
  })

  async function refreshAnalytics(): Promise<number | null> {
    const context = captureContext()
    if (!context.accountId || analyticsSyncing.value) return null
    analyticsSyncing.value = true
    error.value = ''
    try {
      const queued = await publishingApi.syncAnalytics(apiRequest)
      if (!isCurrent(context)) return null
      analyticsSyncPending.value = queued > 0
      analyticsBaselineCapturedAt = overview.value?.capturedAt ?? null
      analyticsLastCapturedAt = analyticsBaselineCapturedAt
      analyticsPollsRemaining = queued > 0 ? ANALYTICS_POLL_LIMIT : 0
      analyticsStablePolls = 0
      return queued
    } catch (caught) {
      if (isCurrent(context)) {
        error.value = getApiErrorMessage(caught, 'Não foi possível enfileirar os analytics.')
      }
      return null
    } finally {
      if (isCurrent(context)) analyticsSyncing.value = false
    }
  }

  function clearError(): void {
    error.value = ''
  }

  function suspend(): void {
    loadRequested = false
    includeAnalyticsRequested = false
    generation += 1
    snapshotVersion += 1
    queueRequestVersion += 1
    contentRequestVersion += 1
    resetData()
  }

  function setPortfolioMode(enabled: boolean): void {
    const next = enabled === true
    if (next === portfolioMode.value) return
    portfolioMode.value = next
    if (next) suspend()
  }

  watch(
    () => [accountStore.activeAccountId, auth.isAuthenticated] as const,
    ([accountId, authenticated], [previousAccountId, previousAuthenticated]) => {
      if (accountId === previousAccountId && authenticated === previousAuthenticated) return
      generation += 1
      snapshotVersion += 1
      queueRequestVersion += 1
      contentRequestVersion += 1
      resetData()
      if (loadRequested && authenticated && accountId) {
        void initialize({ includeAnalytics: includeAnalyticsRequested })
      }
    },
  )

  return {
    connection,
    posts,
    queuePosts,
    contentPosts,
    overview,
    initialized,
    loading,
    refreshing,
    queueLoading,
    contentLoading,
    polling,
    error,
    savingPost,
    connectionBusy,
    analyticsSyncing,
    analyticsSyncPending,
    busyPostIds,
    queuePage,
    contentPage,
    queueHasNext,
    contentHasNext,
    scheduledPosts,
    publishedPosts,
    hasActivePublishing,
    hasPollingWork,
    nextPollingWakeAt,
    portfolioMode,
    initialize,
    loadPage,
    refreshWorkspace,
    poll,
    ...mutations,
    refreshAnalytics,
    clearError,
    suspend,
    setPortfolioMode,
  }
})
