// Formatacao e faixas de cor do painel de observabilidade (funcoes puras).

export type Band = 'good' | 'warn' | 'bad' | 'none'

const numberFormatter = new Intl.NumberFormat('pt-BR', { maximumFractionDigits: 1 })
const intFormatter = new Intl.NumberFormat('pt-BR', { maximumFractionDigits: 0 })

export function formatInt(value: number | null | undefined): string {
  return Number.isFinite(value) ? intFormatter.format(value as number) : '-'
}

export function formatMs(value: number | null | undefined): string {
  if (!Number.isFinite(value)) return '-'
  const ms = value as number
  return ms >= 1000 ? `${numberFormatter.format(ms / 1000)} s` : `${intFormatter.format(ms)} ms`
}

// Aceita fracao (0.034) ou percentual ja pronto quando > 1.
export function formatRate(value: number | null | undefined): string {
  if (!Number.isFinite(value)) return '-'
  const rate = value as number
  const pct = rate <= 1 ? rate * 100 : rate
  return `${numberFormatter.format(pct)}%`
}

export function formatDate(value: string | null | undefined): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' })
}

// Nota de modulo (0-100): >= 80 boa, >= 60 atencao, abaixo ruim.
export function scoreBand(score: number | null | undefined): Band {
  if (!Number.isFinite(score)) return 'none'
  if ((score as number) >= 80) return 'good'
  if ((score as number) >= 60) return 'warn'
  return 'bad'
}

// Limites oficiais (web.dev) de bom / precisa melhorar para cada vital.
const VITAL_LIMITS: Record<string, [number, number]> = {
  LCP: [2500, 4000],
  INP: [200, 500],
  CLS: [0.1, 0.25],
  TTFB: [800, 1800],
  LOAD: [3000, 6000],
}

export function vitalBand(name: string, value: number | null | undefined): Band {
  const limits = VITAL_LIMITS[name.toUpperCase()]
  if (!limits || !Number.isFinite(value)) return 'none'
  if ((value as number) <= limits[0]) return 'good'
  if ((value as number) <= limits[1]) return 'warn'
  return 'bad'
}

export function formatVital(name: string, value: number | null | undefined): string {
  if (!Number.isFinite(value)) return '-'
  return name.toUpperCase() === 'CLS' ? (value as number).toFixed(3) : formatMs(value)
}

export function latencyBand(p95Ms: number | null | undefined): Band {
  if (!Number.isFinite(p95Ms)) return 'none'
  if ((p95Ms as number) <= 300) return 'good'
  if ((p95Ms as number) <= 1000) return 'warn'
  return 'bad'
}

export function errorRateBand(rate: number | null | undefined): Band {
  if (!Number.isFinite(rate)) return 'none'
  const pct = (rate as number) <= 1 ? (rate as number) * 100 : (rate as number)
  if (pct < 1) return 'good'
  if (pct < 5) return 'warn'
  return 'bad'
}

// Valor ja em percentual (0-100), como a cobertura do runner.
export function formatPercentValue(value: number | null | undefined): string {
  return Number.isFinite(value) ? `${numberFormatter.format(value as number)}%` : '-'
}
