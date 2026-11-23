import type {
  ObsCheckStatus,
  ObsHealthStatus,
  ObsSecurityCheck,
  ObsSecuritySummary,
  ObsSeverity,
  ObsTabKey,
} from '~/types/observability'
import type { Band } from './format'

// Logica pura da aba "Saude" e da aba "Seguranca" do painel de observabilidade.

export const HEALTH_REFRESH_MS = 20_000
export const HEARTBEAT_INTERVAL_MS = 30_000

export function statusBand(status: ObsHealthStatus | 'ok' | 'warn' | 'critical'): Band {
  if (status === 'ok') return 'good'
  if (status === 'warn') return 'warn'
  if (status === 'critical') return 'bad'
  return 'none'
}

export const STATUS_LABEL: Record<ObsHealthStatus, string> = {
  ok: 'OK',
  warn: 'Atenção',
  critical: 'Crítico',
  unknown: 'Sem dados',
}

export function severityBand(severity: string): Band {
  if (severity === 'critical' || severity === 'high') return 'bad'
  if (severity === 'medium') return 'warn'
  return 'none'
}

export const SEVERITY_LABEL: Record<string, string> = {
  critical: 'Crítico',
  high: 'Alto',
  medium: 'Médio',
  low: 'Baixo',
}

export const SEVERITY_ORDER: ObsSeverity[] = ['critical', 'high', 'medium', 'low']

const TAB_KEYS: ObsTabKey[] = [
  'endpoints',
  'db',
  'pages',
  'errors',
  'security',
  'system',
  'deploys',
]

export function resolveLinkTab(link: { tab?: string } | null | undefined): ObsTabKey | null {
  const tab = link?.tab as ObsTabKey | undefined
  return tab && TAB_KEYS.includes(tab) ? tab : null
}

// "há 12 s", "há 3 min", "há 2 h", "há 4 d". Futuro/invalido vira "agora".
export function formatAge(iso: string | null | undefined, now: number = Date.now()): string {
  if (!iso) return '-'
  const time = new Date(iso).getTime()
  if (Number.isNaN(time)) return '-'
  const seconds = Math.max(0, Math.round((now - time) / 1000))
  if (seconds < 5) return 'agora'
  if (seconds < 60) return `há ${seconds} s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `há ${minutes} min`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `há ${hours} h`
  return `há ${Math.floor(hours / 24)} d`
}

export const CHECK_STATUS_LABEL: Record<ObsCheckStatus, string> = {
  pass: 'Passou',
  fail: 'Falhou',
  partial: 'Parcial',
  not_tested: 'Não testado',
}

// Nao testado e destacado (amarelo), nunca neutro: ausencia de teste nao e seguranca.
export function checkStatusBand(status: ObsCheckStatus): Band {
  if (status === 'pass') return 'good'
  if (status === 'fail') return 'bad'
  return 'warn'
}

export interface SecurityCoverage {
  total: number
  tested: number
  failed: number
  openFindings: number
  low: boolean
  text: string
  detail: string
}

// Cobertura honesta: "testado" = tudo que nao e not_tested. Abaixo de 50% = baixa.
export function securityCoverage(summary: ObsSecuritySummary | null | undefined): SecurityCoverage {
  const checks = summary?.checks ?? { pass: 0, fail: 0, partial: 0, not_tested: 0 }
  const total = checks.pass + checks.fail + checks.partial + checks.not_tested
  const tested = total - checks.not_tested
  const open = summary?.findings?.open ?? { critical: 0, high: 0, medium: 0, low: 0 }
  const openFindings = SEVERITY_ORDER.reduce((sum, key) => sum + (open[key] || 0), 0)
  const low = total === 0 || tested / total < 0.5
  const text = low
    ? 'Segurança: cobertura baixa. O sistema ainda NÃO foi testado contra a maioria dos ataques conhecidos.'
    : 'Segurança: parte dos ataques conhecidos ainda não foi testada.'
  const findings = SEVERITY_ORDER.filter((key) => open[key] > 0)
    .map((key) => `${open[key]} ${SEVERITY_LABEL[key]!.toLowerCase()}(s)`)
    .join(', ')
  const detail =
    `${tested} de ${total} ataques testados, ${checks.fail} falha(s). ` +
    (openFindings > 0 ? `Achados abertos: ${findings}.` : 'Nenhum achado aberto.')
  return { total, tested, failed: checks.fail, openFindings, low, text, detail }
}

export function groupChecksByCategory(
  checks: ObsSecurityCheck[],
): Array<{ category: string; items: ObsSecurityCheck[] }> {
  const map = new Map<string, ObsSecurityCheck[]>()
  for (const check of checks) {
    const key = check.category || 'Outros'
    const list = map.get(key) ?? []
    list.push(check)
    map.set(key, list)
  }
  return Array.from(map, ([category, items]) => ({ category, items })).sort((a, b) =>
    a.category.localeCompare(b.category, 'pt-BR'),
  )
}

export function alertChannelText(alerts: { configured: boolean; recipients: number }): string {
  return alerts.configured
    ? `Avisos por e-mail: ativo para ${alerts.recipients} destinatário(s)`
    : 'Avisos por e-mail: não configurado (defina SMTP e OBS_ALERT_EMAILS)'
}

export interface HeartbeatState {
  hidden: boolean
  loggedIn: boolean
  enabled: boolean
  lastSentAt: number
  now: number
  routeChanged: boolean
}

// Heartbeat: nunca com aba oculta, deslogado ou desligado; troca de rota envia ja,
// senao respeita o intervalo desde o ultimo envio.
export function shouldSendHeartbeat(state: HeartbeatState, intervalMs = HEARTBEAT_INTERVAL_MS) {
  if (state.hidden || !state.loggedIn || !state.enabled) return false
  if (state.routeChanged) return true
  return state.now - state.lastSentAt >= intervalMs
}

export function formatBytes(value: number | null | undefined): string {
  if (!Number.isFinite(value)) return '-'
  const bytes = value as number
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = bytes
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit += 1
  }
  return `${size.toLocaleString('pt-BR', { maximumFractionDigits: unit === 0 ? 0 : 1 })} ${units[unit]}`
}
