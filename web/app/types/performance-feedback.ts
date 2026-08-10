export interface PerformanceFeedbackPeriod {
  month: string
  week: number
  dateFrom: string
  dateTo: string
  label: string
}

export interface PerformanceFeedbackConsultant {
  id: string
  name: string
  initials: string
  color: string
}

export interface PerformanceFeedbackMetrics {
  soldValue: number
  attendances: number
  conversions: number
  nonConversions: number
  conversionRate: number
  ticketAverage: number
  paScore: number
  qualityScore: number
  avgDurationMs: number
  nonClientConversions: number
  queueJumpRate?: number
  queueJumpServices?: number
  cancellationRate?: number
  erpOrders?: number
  soldValueSource?: string
  ticketAverageSource?: string
  paScoreSource?: string
  salesGoal: number
  ticketGoal: number
  conversionGoal: number
  paGoal: number
  transcriptionScore?: number
  transcriptionSamples: number
}

export interface PerformanceFeedbackSection {
  id: string
  title: string
  contentHtml: string
}

export interface PerformanceFeedbackSettings {
  tenantId: string
  cadence: 'monthly' | 'weekly'
  defaultSections: PerformanceFeedbackSection[]
  configured: boolean
  updatedByUserId?: string
  createdAt?: string
  updatedAt?: string
  version: number
}

export interface PerformanceFeedbackTarget {
  storeId: string
  storeName?: string
  consultantId: string
  consultantName: string
  month?: string
  week?: number
  metrics: PerformanceFeedbackMetrics
}

export interface PerformanceFeedbackReview {
  id: string
  tenantId: string
  storeId: string
  storeName: string
  consultantId: string
  consultantName: string
  period: PerformanceFeedbackPeriod
  status: 'draft' | 'shared' | 'acknowledged'
  feedbackSections: PerformanceFeedbackSection[]
  consultantNotesHtml: string
  metrics: PerformanceFeedbackMetrics
  createdByUserId: string
  updatedByUserId: string
  sharedAt?: string
  acknowledgedAt?: string
  createdAt: string
  updatedAt: string
  version: number
}

export interface PerformanceFeedbackHistoryItem {
  id: string
  period: PerformanceFeedbackPeriod
  status: PerformanceFeedbackReview['status']
  metrics: PerformanceFeedbackMetrics
  updatedAt: string
  version: number
}

export interface PerformanceFeedbackStore {
  id: string
  name: string
  code: string
  tenantId: string
}

export interface PerformanceFeedbackContext {
  store: PerformanceFeedbackStore
  consultants: PerformanceFeedbackConsultant[]
  selectedConsultant?: PerformanceFeedbackConsultant
  period: PerformanceFeedbackPeriod
  metrics?: PerformanceFeedbackMetrics
  review?: PerformanceFeedbackReview
  history: PerformanceFeedbackHistoryItem[]
  canManage: boolean
  canRespond: boolean
  settings: PerformanceFeedbackSettings
}
