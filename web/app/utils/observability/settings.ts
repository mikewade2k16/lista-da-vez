// Logica pura das configuracoes de observabilidade (aba Alertas > Configuracoes).

import type {
  ObsAlertSettings,
  ObsEmailSettings,
  ObsFieldErrors,
  ObsRuleCatalogItem,
  ObsWeeklySettings,
} from '~/types/observability-settings'

export const WEEKDAY_OPTIONS = [
  { value: 1, label: 'Segunda-feira' },
  { value: 2, label: 'Terça-feira' },
  { value: 3, label: 'Quarta-feira' },
  { value: 4, label: 'Quinta-feira' },
  { value: 5, label: 'Sexta-feira' },
  { value: 6, label: 'Sábado' },
  { value: 0, label: 'Domingo' },
] as const

export const RULE_SEVERITY_OPTIONS = [
  { value: 'auto', label: 'Automática' },
  { value: 'critical', label: 'Crítica' },
  { value: 'high', label: 'Alta' },
  { value: 'medium', label: 'Média' },
  { value: 'low', label: 'Baixa' },
] as const

export const EMAIL_SECTION_OPTIONS = [
  { value: 'summary', label: 'Resumo (frase de abertura)' },
  { value: 'details', label: 'Detalhes de cada alerta (onde, desde quando, medição)' },
  { value: 'link', label: 'Link para o painel' },
] as const

const EMAIL_PATTERN = /^[^\s@<>"]+@[^\s@<>"]+\.[^\s@<>"]+$/

export function isValidEmail(value: string): boolean {
  return EMAIL_PATTERN.test(value.trim())
}

// Aceita um ou varios e-mails separados por virgula, ponto e virgula, espaco ou
// quebra de linha. Devolve os validos (minusculos, sem repeticao) e os invalidos.
export function parseRecipients(
  text: string,
  existing: string[] = [],
): { valid: string[]; invalid: string[] } {
  const valid: string[] = []
  const invalid: string[] = []
  const seen = new Set(existing.map((item) => item.toLowerCase()))
  for (const raw of text.split(/[\s,;]+/)) {
    const item = raw.trim().toLowerCase()
    if (!item) continue
    if (!isValidEmail(item)) {
      invalid.push(item)
    } else if (!seen.has(item)) {
      seen.add(item)
      valid.push(item)
    }
  }
  return { valid, invalid }
}

// Erros por campo do 400 `invalid_settings` ({ error: { details: { fields } } }).
export function fieldErrorsFrom(cause: unknown): ObsFieldErrors {
  const fields = (cause as { data?: { error?: { details?: { fields?: unknown } } } })?.data?.error
    ?.details?.fields
  if (!fields || typeof fields !== 'object') return {}
  const out: ObsFieldErrors = {}
  for (const [key, value] of Object.entries(fields as Record<string, unknown>)) {
    if (typeof value === 'string' && value) out[key] = value
  }
  return out
}

export function cloneJson<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

export function sameJson(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b)
}

export function formatThreshold(item: ObsRuleCatalogItem): string {
  return `${item.defaultThreshold} ${item.unit}`
}

export function thresholdHint(item: ObsRuleCatalogItem): string {
  return `Padrão ${item.defaultThreshold}; de ${item.min} a ${item.max} (${item.unit}).`
}

// Quantas regras estao desligadas ou com limite/severidade diferentes do padrao.
export function customizedRules(
  rules: ObsAlertSettings['rules'],
  catalog: ObsRuleCatalogItem[],
): number {
  let count = 0
  for (const item of catalog) {
    const rule = rules[item.rule]
    if (!rule) continue
    const changedLimit =
      item.hasThreshold && rule.threshold !== null && rule.threshold !== item.defaultThreshold
    if (!rule.enabled || rule.severity !== 'auto' || changedLimit) count += 1
  }
  return count
}

// Corpo do PUT de regras: so o que a API aceita (regra sem limite nao leva threshold).
export function rulesPayload(
  rules: ObsAlertSettings['rules'],
  catalog: ObsRuleCatalogItem[],
): ObsAlertSettings['rules'] {
  const out: ObsAlertSettings['rules'] = {}
  for (const item of catalog) {
    const rule = rules[item.rule]
    if (!rule) continue
    out[item.rule] = {
      enabled: rule.enabled,
      severity: rule.severity || 'auto',
      threshold: item.hasThreshold ? rule.threshold : null,
    }
  }
  return out
}

export function minutesLabel(minutes: number): string {
  if (minutes >= 60 && minutes % 60 === 0) return `${minutes / 60} h`
  return `${minutes} min`
}

export function alertsGeneralSummary(
  settings: ObsAlertSettings,
  envRecipientCount: number,
): string {
  const to = settings.recipients.length
    ? `${settings.recipients.length} destinatário(s)`
    : `padrão do ambiente (${envRecipientCount})`
  const quiet = settings.quietHours.enabled
    ? `silêncio ${settings.quietHours.start}–${settings.quietHours.end}`
    : 'sem horário de silêncio'
  return `${to}; reaviso a cada ${minutesLabel(settings.renotifyMinutes)}; ${quiet}`
}

export function rulesSummary(
  rules: ObsAlertSettings['rules'],
  catalog: ObsRuleCatalogItem[],
): string {
  const custom = customizedRules(rules, catalog)
  const off = catalog.filter((item) => rules[item.rule] && !rules[item.rule]!.enabled).length
  return `${catalog.length} regras; ${custom} personalizada(s); ${off} desligada(s)`
}

export function emailSummary(settings: ObsEmailSettings): string {
  const mode = settings.group ? 'um e-mail por rodada' : 'um e-mail por alerta'
  return `Prefixo ${settings.subjectPrefix || '(nenhum)'}; ${mode}; ${settings.sections.length} seção(ões)`
}

export function weeklySummary(settings: ObsWeeklySettings): string {
  if (!settings.enabled) return 'Desligado'
  const day = WEEKDAY_OPTIONS.find((item) => item.value === settings.weekday)?.label ?? '?'
  const hour = String(settings.hour).padStart(2, '0')
  return `${day}, ${hour}:00`
}
