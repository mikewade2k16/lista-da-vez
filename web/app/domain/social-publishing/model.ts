export type SocialPublishingPermission =
  | 'social_publishing.view'
  | 'social_publishing.manage'
  | 'social_publishing.connect'
  | 'social_publishing.analytics'

export type SocialPostStatus =
  | 'draft'
  | 'scheduled'
  | 'publishing'
  | 'published'
  | 'failed'
  | 'cancelled'
  | 'unknown'

export type SocialPostMediaType = 'image' | 'unknown'

export type SocialConnectionStatus = 'disconnected' | 'pending' | 'connected' | 'error'

export interface SocialPostAnalytics {
  views: number
  reach: number
  totalInteractions: number
  likes: number
  comments: number
  saved: number
  shares: number
  capturedAt: string | null
}

export interface SocialPostAnalyticsRecord extends SocialPostAnalytics {
  postId: string
}

export interface SocialPublishingPost {
  id: string
  mediaType: SocialPostMediaType
  caption: string
  mediaUrl: string
  altText: string
  status: SocialPostStatus
  scheduledFor: string | null
  timezone: string
  publishedAt: string | null
  version: number
  externalMediaId: string
  permalink: string
  lastErrorCode: string
  lastErrorMessage: string
  createdAt: string | null
  updatedAt: string | null
  analytics: SocialPostAnalytics | null
}

export interface SocialPublishingPostInput {
  idempotencyKey: string
  mediaType: 'image'
  caption: string
  mediaUrl: string
  altText: string
  status: 'draft' | 'scheduled'
  scheduledFor: string | null
  timezone: string
  version?: number
}

export interface SocialPublishingConnection {
  status: SocialConnectionStatus
  connected: boolean
  igUserId: string
  username: string
  accountType: string
  mediaCount: number
  secretSet: boolean
  secretLast4: string
  connectedAt: string | null
  updatedAt: string | null
}

export interface SocialPublishingOverview {
  draft: number
  scheduled: number
  publishing: number
  published: number
  failed: number
  cancelled: number
  views: number
  reach: number
  totalInteractions: number
  likes: number
  comments: number
  saved: number
  shares: number
  capturedAt: string | null
  upcoming: SocialPublishingPost[]
}

type UnknownRecord = Record<string, unknown>

const POST_STATUSES = new Set<SocialPostStatus>([
  'draft',
  'scheduled',
  'publishing',
  'published',
  'failed',
  'cancelled',
])

const CONNECTION_STATUSES = new Set<SocialConnectionStatus>([
  'disconnected',
  'pending',
  'connected',
  'error',
])

function asRecord(value: unknown): UnknownRecord {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? (value as UnknownRecord)
    : {}
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function nullableDate(value: unknown): string | null {
  const normalized = stringValue(value)
  if (!normalized) return null
  const timestamp = Date.parse(normalized)
  return Number.isFinite(timestamp) ? normalized : null
}

function nonNegativeInteger(value: unknown): number {
  const normalized = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(normalized) ? Math.max(0, Math.round(normalized)) : 0
}

function normalizePostStatus(value: unknown): SocialPostStatus {
  const normalized = stringValue(value).toLowerCase() as SocialPostStatus
  return POST_STATUSES.has(normalized) ? normalized : 'unknown'
}

function normalizePostMediaType(value: unknown): SocialPostMediaType {
  const normalized = stringValue(value).toLowerCase()
  return !normalized || normalized === 'image' ? 'image' : 'unknown'
}

function normalizeConnectionStatus(value: unknown, connected: boolean): SocialConnectionStatus {
  const normalized = stringValue(value).toLowerCase()
  if (normalized === 'active') return 'connected'
  if (normalized === 'connecting') return 'pending'
  if (CONNECTION_STATUSES.has(normalized as SocialConnectionStatus)) {
    return normalized as SocialConnectionStatus
  }
  return connected ? 'connected' : 'disconnected'
}

export function isHttpsMediaUrl(value: string): boolean {
  try {
    return new URL(value).protocol === 'https:'
  } catch {
    return false
  }
}

function normalizeHttpsUrl(value: unknown): string {
  const normalized = stringValue(value)
  if (!normalized) return ''
  try {
    const url = new URL(normalized)
    return url.protocol === 'https:' ? url.toString() : ''
  } catch {
    return ''
  }
}

type SecureCrypto = Pick<Crypto, 'getRandomValues'> & Partial<Pick<Crypto, 'randomUUID'>>

export function createSocialPublishingIdempotencyKey(
  source: SecureCrypto | undefined = globalThis.crypto,
): string {
  if (typeof source?.randomUUID === 'function') return source.randomUUID()
  if (!source?.getRandomValues) {
    throw new Error('Gerador seguro indisponível para criar a chave de idempotência.')
  }
  const bytes = source.getRandomValues(new Uint8Array(16))
  bytes[6] = ((bytes[6] ?? 0) & 15) | 64
  bytes[8] = ((bytes[8] ?? 0) & 63) | 128
  const encoded = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
  return [
    encoded.slice(0, 8),
    encoded.slice(8, 12),
    encoded.slice(12, 16),
    encoded.slice(16, 20),
    encoded.slice(20),
  ].join('-')
}

export function normalizePostAnalytics(value: unknown): SocialPostAnalytics | null {
  if (value === null || value === undefined) return null
  const raw = asRecord(value)
  if (Object.keys(raw).length === 0) return null
  return {
    views: nonNegativeInteger(raw.views ?? raw.impressions),
    reach: nonNegativeInteger(raw.reach),
    totalInteractions: nonNegativeInteger(raw.totalInteractions),
    likes: nonNegativeInteger(raw.likes),
    comments: nonNegativeInteger(raw.comments),
    saved: nonNegativeInteger(raw.saved ?? raw.saves),
    shares: nonNegativeInteger(raw.shares),
    capturedAt: nullableDate(raw.capturedAt ?? raw.syncedAt),
  }
}

export function normalizePostAnalyticsList(value: unknown): SocialPostAnalyticsRecord[] {
  const raw = asRecord(value)
  const entries = Array.isArray(value)
    ? value
    : Array.isArray(raw.analytics)
      ? raw.analytics
      : Array.isArray(raw.items)
        ? raw.items
        : []
  return entries.flatMap((entry) => {
    const record = asRecord(entry)
    const postId = stringValue(record.postId ?? record.id)
    const analytics = normalizePostAnalytics(record.analytics ?? record)
    return postId && analytics ? [{ postId, ...analytics }] : []
  })
}

export function normalizeSocialPost(value: unknown): SocialPublishingPost {
  const raw = asRecord(value)
  return {
    id: stringValue(raw.id),
    mediaType: normalizePostMediaType(raw.mediaType),
    caption: stringValue(raw.caption),
    mediaUrl: stringValue(raw.mediaUrl),
    altText: stringValue(raw.altText),
    status: normalizePostStatus(raw.status),
    scheduledFor: nullableDate(raw.scheduledFor),
    timezone: stringValue(raw.timezone),
    publishedAt: nullableDate(raw.publishedAt),
    version: nonNegativeInteger(raw.version),
    externalMediaId: stringValue(raw.externalMediaId ?? raw.instagramMediaId),
    permalink: normalizeHttpsUrl(raw.permalink ?? raw.instagramPermalink),
    lastErrorCode: stringValue(raw.lastErrorCode),
    lastErrorMessage: stringValue(raw.lastErrorMessage ?? raw.errorMessage),
    createdAt: nullableDate(raw.createdAt),
    updatedAt: nullableDate(raw.updatedAt),
    analytics: normalizePostAnalytics(raw.analytics),
  }
}

export function normalizeSocialPostList(value: unknown): SocialPublishingPost[] {
  const raw = asRecord(value)
  const entries = Array.isArray(value)
    ? value
    : Array.isArray(raw.posts)
      ? raw.posts
      : Array.isArray(raw.items)
        ? raw.items
        : []
  return entries.map(normalizeSocialPost).filter((post) => post.id)
}

export function mergeSocialPostAnalytics(
  posts: SocialPublishingPost[],
  records: SocialPostAnalyticsRecord[],
): SocialPublishingPost[] {
  const analyticsByPostId = new Map(records.map((record) => [record.postId, record]))
  return posts.map((post) => {
    const analytics = post.status === 'published' ? analyticsByPostId.get(post.id) : undefined
    if (!analytics) return post
    const { postId: _postId, ...metrics } = analytics
    return { ...post, analytics: metrics }
  })
}

export function normalizeConnection(value: unknown): SocialPublishingConnection {
  const outer = asRecord(value)
  const raw = Object.keys(asRecord(outer.connection)).length ? asRecord(outer.connection) : outer
  const secret = Object.keys(asRecord(raw.secret)).length
    ? asRecord(raw.secret)
    : asRecord(raw.token)
  const secretSet = raw.secretSet === true || raw.tokenSet === true || secret.set === true
  const status = normalizeConnectionStatus(raw.status, raw.connected === true || secretSet)
  return {
    status,
    connected: raw.connected === true || status === 'connected',
    igUserId: stringValue(raw.igUserId ?? raw.instagramAccountId),
    username: stringValue(raw.username),
    accountType: stringValue(raw.accountType),
    mediaCount: nonNegativeInteger(raw.mediaCount),
    secretSet,
    secretLast4: stringValue(raw.secretLast4 ?? raw.tokenLast4 ?? secret.last4).slice(-4),
    connectedAt: nullableDate(raw.connectedAt),
    updatedAt: nullableDate(raw.updatedAt),
  }
}

export function normalizeOverview(value: unknown): SocialPublishingOverview {
  const outer = asRecord(value)
  const counts = Object.keys(asRecord(outer.counts)).length
    ? asRecord(outer.counts)
    : Object.keys(asRecord(outer.summary)).length
      ? asRecord(outer.summary)
      : outer
  const analytics = Object.keys(asRecord(outer.analytics)).length
    ? asRecord(outer.analytics)
    : outer
  return {
    draft: nonNegativeInteger(counts.draft ?? counts.drafts),
    scheduled: nonNegativeInteger(counts.scheduled),
    publishing: nonNegativeInteger(counts.publishing),
    published: nonNegativeInteger(counts.published),
    failed: nonNegativeInteger(counts.failed),
    cancelled: nonNegativeInteger(counts.cancelled),
    views: nonNegativeInteger(analytics.views ?? analytics.impressions),
    reach: nonNegativeInteger(analytics.reach),
    totalInteractions: nonNegativeInteger(analytics.totalInteractions),
    likes: nonNegativeInteger(analytics.likes),
    comments: nonNegativeInteger(analytics.comments),
    saved: nonNegativeInteger(analytics.saved ?? analytics.saves),
    shares: nonNegativeInteger(analytics.shares),
    capturedAt: nullableDate(analytics.capturedAt ?? analytics.syncedAt),
    upcoming: normalizeSocialPostList(outer.upcoming),
  }
}

export function socialPostStatusLabel(status: SocialPostStatus): string {
  const labels: Record<SocialPostStatus, string> = {
    draft: 'Rascunho',
    scheduled: 'Agendada',
    publishing: 'Publicando',
    published: 'Publicada',
    failed: 'Falhou',
    cancelled: 'Cancelada',
    unknown: 'Status desconhecido',
  }
  return labels[status]
}

export function socialPostMediaTypeLabel(mediaType: SocialPostMediaType): string {
  const labels: Record<SocialPostMediaType, string> = {
    image: 'Imagem',
    unknown: 'Formato desconhecido',
  }
  return labels[mediaType]
}
