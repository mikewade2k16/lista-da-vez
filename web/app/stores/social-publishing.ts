import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import * as publishingApi from '~/domain/social-publishing/social-publishing-api'
import type {
  SocialPublishingConnection,
  SocialPublishingOverview,
  SocialPublishingPost,
  SocialPublishingPostInput,
} from '~/domain/social-publishing/model'
import { mergeSocialPostAnalytics } from '~/domain/social-publishing/model'

interface RequestContext {
  accountId: string
  generation: number
}

interface InitializeOptions {
  includeAnalytics?: boolean
}

type PostAction = () => Promise<SocialPublishingPost>

export const useSocialPublishingStore = defineStore('social-publishing', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const connection = ref<SocialPublishingConnection | null>(null)
  const posts = ref<SocialPublishingPost[]>([])
  const overview = ref<SocialPublishingOverview | null>(null)
  const initialized = ref(false)
  const loading = ref(false)
  const error = ref('')
  const savingPost = ref(false)
  const connectionBusy = ref(false)
  const analyticsSyncing = ref(false)
  const busyPostIds = ref<string[]>([])

  let generation = 0
  let loadVersion = 0
  let loadRequested = false
  let includeAnalyticsRequested = false

  const scheduledPosts = computed(() =>
    posts.value.filter((post) => ['scheduled', 'publishing', 'failed'].includes(post.status)),
  )
  const contentPosts = computed(() =>
    posts.value.filter((post) => !['scheduled', 'publishing', 'failed'].includes(post.status)),
  )
  const publishedPosts = computed(() => posts.value.filter((post) => post.status === 'published'))

  function currentAccountId(): string {
    return String(accountStore.activeAccountId || '').trim()
  }

  function captureContext(): RequestContext {
    return { accountId: currentAccountId(), generation }
  }

  function isCurrent(context: RequestContext): boolean {
    return (
      context.generation === generation &&
      Boolean(context.accountId) &&
      context.accountId === currentAccountId()
    )
  }

  function resetData(): void {
    connection.value = null
    posts.value = []
    overview.value = null
    initialized.value = false
    loading.value = false
    error.value = ''
    savingPost.value = false
    connectionBusy.value = false
    analyticsSyncing.value = false
    busyPostIds.value = []
  }

  function replacePost(post: SocialPublishingPost): void {
    if (!post.id) return
    const index = posts.value.findIndex((entry) => entry.id === post.id)
    if (index < 0) {
      posts.value = [post, ...posts.value]
      return
    }
    posts.value = posts.value.map((entry) => (entry.id === post.id ? post : entry))
  }

  function markPostBusy(postId: string, busy: boolean): void {
    const id = String(postId || '').trim()
    if (!id) return
    busyPostIds.value = busy
      ? [...new Set([...busyPostIds.value, id])]
      : busyPostIds.value.filter((entry) => entry !== id)
  }

  async function initialize(options: InitializeOptions = {}): Promise<boolean> {
    loadRequested = true
    const includeAnalytics = options.includeAnalytics === true
    includeAnalyticsRequested = includeAnalytics
    const context = captureContext()
    if (!auth.isAuthenticated || !context.accountId) return false

    const requestVersion = ++loadVersion
    loading.value = true
    error.value = ''
    try {
      const [nextConnection, nextPosts, nextOverview, nextAnalytics] = await Promise.all([
        publishingApi.fetchConnection(apiRequest),
        publishingApi.fetchPosts(apiRequest),
        includeAnalytics
          ? publishingApi.fetchOverview(apiRequest)
          : Promise.resolve<SocialPublishingOverview | null>(null),
        includeAnalytics ? publishingApi.fetchAnalyticsPosts(apiRequest) : Promise.resolve([]),
      ])
      if (!isCurrent(context) || requestVersion !== loadVersion) return false
      connection.value = nextConnection
      posts.value = mergeSocialPostAnalytics(nextPosts, nextAnalytics)
      overview.value = nextOverview
      initialized.value = true
      return true
    } catch (caught) {
      if (!isCurrent(context) || requestVersion !== loadVersion) return false
      error.value = getApiErrorMessage(
        caught,
        'Não foi possível carregar o agendamento de postagens.',
      )
      return false
    } finally {
      if (isCurrent(context) && requestVersion === loadVersion) {
        loading.value = false
      }
    }
  }

  async function refreshPosts(): Promise<boolean> {
    const context = captureContext()
    if (!context.accountId) return false
    try {
      const nextPosts = await publishingApi.fetchPosts(apiRequest)
      if (!isCurrent(context)) return false
      posts.value = nextPosts
      return true
    } catch (caught) {
      if (isCurrent(context)) {
        error.value = getApiErrorMessage(caught, 'Não foi possível atualizar as publicações.')
      }
      return false
    }
  }

  async function connect(accessToken: string): Promise<boolean> {
    const token = String(accessToken || '').trim()
    const context = captureContext()
    if (!token || !context.accountId || connectionBusy.value) return false
    connectionBusy.value = true
    error.value = ''
    try {
      const nextConnection = await publishingApi.beginConnection(apiRequest, token)
      if (!isCurrent(context)) return false
      connection.value = nextConnection
      return true
    } catch (caught) {
      if (isCurrent(context)) {
        error.value = getApiErrorMessage(
          caught,
          'Não foi possível validar a conexão técnica com o Instagram.',
        )
      }
      return false
    } finally {
      if (isCurrent(context)) connectionBusy.value = false
    }
  }

  async function disconnect(): Promise<boolean> {
    const context = captureContext()
    if (!context.accountId || connectionBusy.value) return false
    connectionBusy.value = true
    error.value = ''
    try {
      const nextConnection = await publishingApi.removeConnection(apiRequest)
      if (!isCurrent(context)) return false
      connection.value = nextConnection
      return true
    } catch (caught) {
      if (isCurrent(context)) {
        error.value = getApiErrorMessage(caught, 'Não foi possível remover a conexão.')
      }
      return false
    } finally {
      if (isCurrent(context)) connectionBusy.value = false
    }
  }

  async function savePost(
    input: SocialPublishingPostInput,
    postId = '',
  ): Promise<SocialPublishingPost | null> {
    const context = captureContext()
    if (!context.accountId || savingPost.value) return null
    savingPost.value = true
    error.value = ''
    try {
      const saved = postId
        ? await publishingApi.updatePost(apiRequest, postId, input)
        : await publishingApi.createPost(apiRequest, input)
      if (!isCurrent(context)) return null
      if (!saved.id) {
        error.value = 'A API não devolveu a publicação salva.'
        return null
      }
      replacePost(saved)
      return saved
    } catch (caught) {
      if (isCurrent(context)) {
        error.value = getApiErrorMessage(caught, 'Não foi possível salvar a publicação.')
      }
      return null
    } finally {
      if (isCurrent(context)) savingPost.value = false
    }
  }

  async function saveAndSchedule(
    input: SocialPublishingPostInput,
    postId = '',
  ): Promise<SocialPublishingPost | null> {
    const scheduledFor = String(input.scheduledFor || '').trim()
    if (!scheduledFor || !input.timezone) return null
    const saved = await savePost(input, postId)
    if (!saved || !postId) return saved
    return runPostAction(
      saved.id,
      () =>
        publishingApi.schedulePost(
          apiRequest,
          saved.id,
          scheduledFor,
          input.timezone,
          saved.version,
        ),
      'Não foi possível reagendar a publicação.',
    )
  }

  async function runPostAction(
    postId: string,
    action: PostAction,
    fallbackMessage: string,
  ): Promise<SocialPublishingPost | null> {
    const context = captureContext()
    if (!context.accountId || busyPostIds.value.includes(postId)) return null
    markPostBusy(postId, true)
    error.value = ''
    try {
      const post = await action()
      if (!isCurrent(context)) return null
      replacePost(post)
      return post
    } catch (caught) {
      if (isCurrent(context)) {
        error.value = getApiErrorMessage(caught, fallbackMessage)
      }
      return null
    } finally {
      if (isCurrent(context)) markPostBusy(postId, false)
    }
  }

  function cancel(post: SocialPublishingPost): Promise<SocialPublishingPost | null> {
    return runPostAction(
      post.id,
      () => publishingApi.cancelPost(apiRequest, post.id, post.version),
      'Não foi possível cancelar a publicação.',
    )
  }

  function retry(post: SocialPublishingPost): Promise<SocialPublishingPost | null> {
    return runPostAction(
      post.id,
      () => publishingApi.retryPost(apiRequest, post.id, post.version),
      'Não foi possível reenviar a publicação.',
    )
  }

  async function refreshAnalytics(): Promise<boolean> {
    const context = captureContext()
    if (!context.accountId || analyticsSyncing.value) return false
    analyticsSyncing.value = true
    error.value = ''
    try {
      await publishingApi.syncAnalytics(apiRequest)
      const [nextOverview, nextPosts, nextAnalytics] = await Promise.all([
        publishingApi.fetchOverview(apiRequest),
        publishingApi.fetchPosts(apiRequest),
        publishingApi.fetchAnalyticsPosts(apiRequest),
      ])
      if (!isCurrent(context)) return false
      overview.value = nextOverview
      posts.value = mergeSocialPostAnalytics(nextPosts, nextAnalytics)
      return true
    } catch (caught) {
      if (isCurrent(context)) {
        error.value = getApiErrorMessage(caught, 'Não foi possível sincronizar os analytics.')
      }
      return false
    } finally {
      if (isCurrent(context)) analyticsSyncing.value = false
    }
  }

  function clearError(): void {
    error.value = ''
  }

  watch(
    () => [accountStore.activeAccountId, auth.isAuthenticated] as const,
    ([accountId, authenticated], [previousAccountId, previousAuthenticated]) => {
      if (accountId === previousAccountId && authenticated === previousAuthenticated) return
      generation += 1
      loadVersion += 1
      resetData()
      if (loadRequested && authenticated && accountId) {
        void initialize({ includeAnalytics: includeAnalyticsRequested })
      }
    },
  )

  return {
    connection,
    posts,
    overview,
    initialized,
    loading,
    error,
    savingPost,
    connectionBusy,
    analyticsSyncing,
    busyPostIds,
    scheduledPosts,
    contentPosts,
    publishedPosts,
    initialize,
    refreshPosts,
    connect,
    disconnect,
    savePost,
    saveAndSchedule,
    cancel,
    retry,
    refreshAnalytics,
    clearError,
  }
})
