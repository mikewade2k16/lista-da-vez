import type { createApiRequest } from '~/utils/api-client'
import type {
  OmniInstagramAccount,
  OmniInstagramAction,
  OmniInstagramComment,
} from './config-types'

export type ApiRequest = ReturnType<typeof createApiRequest>

const BASE = '/v1/omnichannel/tenant/instagram'

function items<T>(value: unknown): T[] {
  if (Array.isArray(value)) return value as T[]
  const payload = value as { items?: T[] }
  return Array.isArray(payload.items) ? payload.items : []
}

export function saveInstagramAccount(
  api: ApiRequest,
  input: {
    igUserId: string
    username?: string
    displayName?: string
    pageId?: string
    graphVersion: string
    accessToken: string
    appSecret: string
    verifyToken: string
  },
): Promise<OmniInstagramAccount> {
  return api(`${BASE}/accounts`, { method: 'PUT', body: input }) as Promise<OmniInstagramAccount>
}

export function fetchInstagramAccounts(api: ApiRequest): Promise<OmniInstagramAccount[]> {
  return api(`${BASE}/accounts`).then((value: unknown) => items<OmniInstagramAccount>(value))
}

export function fetchInstagramComments(
  api: ApiRequest,
  instagramAccountId?: string,
): Promise<OmniInstagramComment[]> {
  const query = instagramAccountId
    ? `?${new URLSearchParams({ instagramAccountId }).toString()}`
    : ''
  return api(`${BASE}/comments${query}`, { dedupe: false }).then((value: unknown) =>
    items<OmniInstagramComment>(value),
  )
}

export function fetchInstagramActions(
  api: ApiRequest,
  commentId: string,
): Promise<OmniInstagramAction[]> {
  return api(`${BASE}/comments/${encodeURIComponent(commentId)}/actions`, {
    dedupe: false,
  }).then((value: unknown) => items<OmniInstagramAction>(value))
}

export function decideInstagramAction(
  api: ApiRequest,
  commentId: string,
  actionId: string,
  input: { actionKind: OmniInstagramAction['actionKind']; approvedText?: string },
): Promise<OmniInstagramAction> {
  return api(
    `${BASE}/comments/${encodeURIComponent(commentId)}/actions/${encodeURIComponent(actionId)}/decide`,
    { method: 'POST', body: input },
  ) as Promise<OmniInstagramAction>
}
