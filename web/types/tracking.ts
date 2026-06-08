export interface SiteTrackingEventItem {
  id: string
  accountId: string
  sourceId: string
  sourceLabel: string
  source: string
  batchId: string
  sourceEventId: string
  visitorId: string
  sessionId: string
  eventType: string
  eventName: string
  pageUrl: string
  pagePath: string
  pageTitle: string
  pageGroup: string
  pageName: string
  referrer: string
  elementTag: string
  elementText: string
  elementHref: string
  elementId: string
  elementClasses: string
  elementRole: string
  productCode: string
  activeSeconds: number | null
  scrollDepth: number | null
  screenWidth: number | null
  screenHeight: number | null
  viewportWidth: number | null
  viewportHeight: number | null
  deviceType: string
  browserLang: string
  timezone: string
  utmSource: string
  utmMedium: string
  utmCampaign: string
  utmTerm: string
  utmContent: string
  eventData: string
  rawPayload: string
  ip: string
  userAgent: string
  sentAt: string
  receivedAt: string
}

export interface SiteTrackingEventsListResponse {
  events: SiteTrackingEventItem[]
  total: number
  page: number
  perPage: number
}

export interface SiteTrackingTotals {
  totalEvents: number
  totalSessions: number
  totalVisitors: number
  pageViews: number
  today: number
  last7Days: number
}

export interface SiteTrackingCountItem {
  label: string
  count: number
}

export interface SiteTrackingConversion {
  key: string
  label: string
  count: number
  percentOfVisitors: number
}

export interface SiteTrackingDailyCount {
  date: string
  count: number
}

export interface SiteTrackingRecentVisit {
  receivedAt: string
  deviceType: string
  ip: string
  referrer: string
  pagePath: string
}

export interface SiteTrackingAnalytics {
  rangeDays: number
  totals: SiteTrackingTotals
  devices: SiteTrackingCountItem[]
  eventsByType: SiteTrackingCountItem[]
  conversions: SiteTrackingConversion[]
  accessByDay: SiteTrackingDailyCount[]
  topReferrers: SiteTrackingCountItem[]
  recentVisits: SiteTrackingRecentVisit[]
}
