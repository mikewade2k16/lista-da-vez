import type { ContentOperationsBrief } from '~/domain/content-operations/content-operations-api'

export type SystemNotificationSeverity = 'critical' | 'warning' | 'info'
export type SystemNotificationBucket = 'system' | 'feedback' | 'content' | 'operations'

export interface SystemNotificationItem {
  id: string
  sourceId: string
  bucket: SystemNotificationBucket
  sourceLabel: string
  title: string
  body: string
  meta: string
  linkPath: string
  occurredAt: string
  severity: SystemNotificationSeverity
  dismissible: boolean
}

export interface InAppNotificationInput {
  id?: string
  sourceModule?: string
  title?: string
  body?: string
  linkPath?: string
  createdAt?: string
  readAt?: string | null
  payload?: Record<string, unknown>
}

export interface FeedbackNotificationInput {
  id?: string
  subject?: string
  body?: string
  status?: string
  user_name?: string
  store_id?: string
  unread_count?: number
  last_message_body?: string
  last_message_at?: string | null
  updated_at?: string
}

export interface OperationalAlertInput {
  id?: string
  status?: string
  severity?: string
  sourceModule?: string
  headline?: string
  body?: string
  consultantName?: string
  createdAt?: string
  lastTriggeredAt?: string
}

export interface BuildSystemNotificationsInput {
  inApp?: InAppNotificationInput[]
  feedback?: FeedbackNotificationInput[]
  feedbackPath?: string
  feedbackManager?: boolean
  storeLabels?: Record<string, string>
  contentBrief?: ContentOperationsBrief | null
  operationalAlerts?: OperationalAlertInput[]
}

const sourceLabels: Record<string, string> = {
  tasks: 'Tasks',
  calendar: 'Calendário',
  feedback: 'Chamados',
  notifications: 'Sistema',
  content_operations: 'Operação de conteúdo',
  queue: 'Operação',
  operations: 'Operação',
  alerts: 'Alertas operacionais',
}

const severityRank: Record<SystemNotificationSeverity, number> = {
  critical: 0,
  warning: 1,
  info: 2,
}

function text(value: unknown): string {
  return String(value ?? '').trim()
}

function sourceLabel(sourceModule: unknown): string {
  const normalized = text(sourceModule).toLowerCase()
  if (sourceLabels[normalized]) return sourceLabels[normalized]
  if (!normalized) return 'Sistema'
  return normalized
    .replaceAll('_', ' ')
    .replace(/\b\p{L}/gu, (character) => character.toUpperCase())
}

function normalizeSeverity(value: unknown): SystemNotificationSeverity {
  const normalized = text(value).toLowerCase()
  if (['critical', 'danger', 'high', 'error'].includes(normalized)) return 'critical'
  if (['warning', 'attention', 'medium', 'warn'].includes(normalized)) return 'warning'
  return 'info'
}

function timestamp(value: string): number {
  const parsed = new Date(value || 0).getTime()
  return Number.isFinite(parsed) ? parsed : 0
}

function mapInApp(items: InAppNotificationInput[]): SystemNotificationItem[] {
  return items
    .filter((item) => text(item.id) && !text(item.readAt))
    .map((item) => ({
      id: `system:${text(item.id)}`,
      sourceId: text(item.id),
      bucket: 'system' as const,
      sourceLabel: sourceLabel(item.sourceModule),
      title: text(item.title) || 'Nova notificação',
      body: text(item.body),
      meta: '',
      linkPath: text(item.linkPath) || '/',
      occurredAt: text(item.createdAt),
      severity: normalizeSeverity(item.payload?.severity),
      dismissible: true,
    }))
}

function mapFeedback(
  items: FeedbackNotificationInput[],
  path: string,
  manager: boolean,
  storeLabels: Record<string, string>,
): SystemNotificationItem[] {
  return items
    .filter((item) => Number(item.unread_count || 0) > 0 && text(item.status) !== 'closed')
    .map((item) => {
      const id = text(item.id)
      const occurredAt = text(item.last_message_at || item.updated_at)
      const meta = manager
        ? [
            text(item.user_name) || 'Usuário',
            storeLabels[text(item.store_id)] || 'Loja não informada',
          ].join(' · ')
        : text(item.status) || 'Chamado'
      return {
        id: `feedback:${id}:${occurredAt}`,
        sourceId: id,
        bucket: 'feedback' as const,
        sourceLabel: 'Chamados',
        title: text(item.subject) || 'Chamado sem assunto',
        body: text(item.last_message_body || item.body),
        meta,
        linkPath: `${path}?id=${encodeURIComponent(id)}`,
        occurredAt,
        severity: 'info' as const,
        dismissible: true,
      }
    })
}

function mapContent(brief: ContentOperationsBrief | null | undefined): SystemNotificationItem[] {
  if (!brief) return []
  return brief.alerts.map((alert) => ({
    id: `content:${alert.id}`,
    sourceId: alert.id,
    bucket: 'content' as const,
    sourceLabel: 'Operação de conteúdo',
    title: alert.title,
    body: alert.body,
    meta: alert.clientName,
    linkPath: alert.linkPath || '/operacao-conteudo',
    occurredAt: alert.occurredOn || brief.generatedAt,
    severity: normalizeSeverity(alert.severity),
    dismissible: false,
  }))
}

function mapOperational(items: OperationalAlertInput[]): SystemNotificationItem[] {
  return items
    .filter((item) => text(item.id) && text(item.status).toLowerCase() === 'active')
    .map((item) => ({
      id: `operations:${text(item.id)}`,
      sourceId: text(item.id),
      bucket: 'operations' as const,
      sourceLabel: sourceLabel(item.sourceModule || 'alerts'),
      title: text(item.headline) || 'Alerta operacional',
      body: text(item.body),
      meta: text(item.consultantName),
      linkPath: '/alertas',
      occurredAt: text(item.lastTriggeredAt || item.createdAt),
      severity: normalizeSeverity(item.severity),
      dismissible: false,
    }))
}

export function buildSystemNotifications(
  input: BuildSystemNotificationsInput,
): SystemNotificationItem[] {
  const unique = new Map<string, SystemNotificationItem>()
  const combined = [
    ...mapInApp(input.inApp ?? []),
    ...mapFeedback(
      input.feedback ?? [],
      input.feedbackPath || '/meus-chamados',
      Boolean(input.feedbackManager),
      input.storeLabels ?? {},
    ),
    ...mapContent(input.contentBrief),
    ...mapOperational(input.operationalAlerts ?? []),
  ]

  for (const item of combined) unique.set(item.id, item)

  return [...unique.values()].sort((left, right) => {
    const severityDifference = severityRank[left.severity] - severityRank[right.severity]
    if (severityDifference !== 0) return severityDifference
    return timestamp(right.occurredAt) - timestamp(left.occurredAt)
  })
}
