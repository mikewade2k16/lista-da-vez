import type { Ref } from 'vue'
import * as publishingApi from '~/domain/social-publishing/social-publishing-api'
import type { SocialPublishingApiRequest } from '~/domain/social-publishing/social-publishing-api'
import type {
  SocialPublishingConnection,
  SocialPublishingPost,
  SocialPublishingPostInput,
} from '~/domain/social-publishing/model'
import { getApiErrorMessage } from '~/utils/api-client'

export interface SocialPublishingRequestContext {
  accountId: string
  generation: number
}

interface MutationDependencies {
  apiRequest: SocialPublishingApiRequest
  connection: Ref<SocialPublishingConnection | null>
  error: Ref<string>
  savingPost: Ref<boolean>
  connectionBusy: Ref<boolean>
  busyPostIds: Ref<string[]>
  captureContext: () => SocialPublishingRequestContext
  isCurrent: (context: SocialPublishingRequestContext) => boolean
  replacePost: (post: SocialPublishingPost) => void
}

type PostAction = () => Promise<SocialPublishingPost>

export function createSocialPublishingMutations(dependencies: MutationDependencies) {
  const {
    apiRequest,
    connection,
    error,
    savingPost,
    connectionBusy,
    busyPostIds,
    captureContext,
    isCurrent,
    replacePost,
  } = dependencies

  function markPostBusy(postId: string, busy: boolean): void {
    const id = String(postId || '').trim()
    if (!id) return
    busyPostIds.value = busy
      ? [...new Set([...busyPostIds.value, id])]
      : busyPostIds.value.filter((entry) => entry !== id)
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
      if (isCurrent(context)) error.value = getApiErrorMessage(caught, fallbackMessage)
      return null
    } finally {
      if (isCurrent(context)) markPostBusy(postId, false)
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

  return { connect, disconnect, savePost, saveAndSchedule, cancel, retry }
}
