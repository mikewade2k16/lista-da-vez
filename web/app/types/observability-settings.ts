// Tipos das configuracoes de alertas, e-mail e relatorio semanal (obs.settings).

export type ObsSettingsKey = 'alerts' | 'email' | 'weeklyReport'

export interface ObsRuleSetting {
  enabled: boolean
  severity: string
  threshold: number | null
}

export interface ObsQuietHours {
  enabled: boolean
  start: string
  end: string
  timezone: string
  allowCritical: boolean
}

export interface ObsAlertSettings {
  recipients: string[]
  renotifyMinutes: number
  quietHours: ObsQuietHours
  rules: Record<string, ObsRuleSetting>
}

export interface ObsEmailSettings {
  subjectPrefix: string
  group: boolean
  sections: string[]
  includeResolved: boolean
  footer: string
}

export interface ObsWeeklySettings {
  enabled: boolean
  weekday: number
  hour: number
  timezone: string
  recipients: string[]
}

export interface ObsRuleCatalogItem {
  rule: string
  label: string
  area: string
  hasThreshold: boolean
  defaultThreshold: number
  unit: string
  min: number
  max: number
}

export interface ObsSettingsChannel {
  email: string
  whatsapp: string
  smtpConfigured: boolean
  envRecipientCount: number
  alertRecipientCount: number
  weeklyRecipientCount: number
  cacheSeconds: number
}

export interface ObsSettings {
  alerts: ObsAlertSettings
  email: ObsEmailSettings
  weeklyReport: ObsWeeklySettings
  rulesCatalog: ObsRuleCatalogItem[]
  channel: ObsSettingsChannel
  weeklyState: { lastSentAt: string | null; lastError: string }
}

export interface ObsTextPreview {
  subject: string
  text: string
}

export type ObsFieldErrors = Record<string, string>
