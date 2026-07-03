// Tipos e helpers da CONFIG do calendario (contrato C2 do CALENDARIO_SPECS).
// Separado de utils/calendar.ts para manter cada arquivo < 450 linhas; o
// calendar.ts re-exporta tudo daqui, entao os imports existentes nao mudam.
// SEM estado, SEM fetch — so tipos + funcoes puras.
import type { RgbTriplet, WeekStart } from '~/utils/calendar'

/** Provedor de IA suportado no plano do mes (contrato C2). */
export type CalendarAiProvider = 'claude' | 'deepseek' | 'qwen' | 'kimi' | 'glm' | 'custom'

/** Feriados/datas comemorativas ligados na config. */
export interface CalendarHolidayFlags {
  brNational: boolean
  sergipe: boolean
  aracaju: boolean
  luxuryIntl: boolean
}

/** White-label do calendario (logo/titulo/cor primaria da agencia). */
export interface CalendarWhiteLabel {
  logoUrl: string
  title: string
  primaryColor: string
}

/** Config de IA do plano do mes. Chaves de API NUNCA aqui (vivem no n8n). */
export interface CalendarAiConfig {
  provider: CalendarAiProvider
  model: string
  baseUrl: string
  systemPrompt: string
  temperature: number
}

/** Config do calendario por conta (contrato C2, jsonb calendar.config). */
export interface CalendarConfig {
  responsibleUserIds: string[]
  holidays: CalendarHolidayFlags
  /** "sunday" | "monday" (default sunday = atual). */
  weekStartsOn: WeekStart
  /** { [clientId]: "#rrggbb" | "none" } — vazio = paleta-semente. */
  clientColors: Record<string, string>
  /** { [CalendarEventType]: "#rrggbb" } — vazio = cor do cliente. */
  typeColors: Record<string, string>
  whiteLabel: CalendarWhiteLabel
  ai: CalendarAiConfig
}

export function defaultCalendarConfig(): CalendarConfig {
  return {
    responsibleUserIds: [],
    holidays: { brNational: true, sergipe: true, aracaju: true, luxuryIntl: true },
    weekStartsOn: 'sunday',
    clientColors: {},
    typeColors: {},
    whiteLabel: { logoUrl: '', title: '', primaryColor: '' },
    ai: {
      provider: 'claude',
      model: 'claude-sonnet-5',
      baseUrl: '',
      systemPrompt: '',
      temperature: 0.7,
    },
  }
}

// Base URLs default por provider (placeholder no front; o n8n usa o mesmo mapa).
// Contrato C2 — chave vazia no config = usar o default do provider.
export const AI_PROVIDER_BASE_URL: Record<CalendarAiProvider, string> = {
  claude: 'https://api.anthropic.com',
  deepseek: 'https://api.deepseek.com',
  qwen: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
  kimi: 'https://api.moonshot.cn/v1',
  glm: 'https://open.bigmodel.cn/api/paas/v4',
  custom: '',
}

export const AI_PROVIDER_LABEL: Record<CalendarAiProvider, string> = {
  claude: 'Claude (Anthropic)',
  deepseek: 'DeepSeek',
  qwen: 'Qwen (Alibaba)',
  kimi: 'Kimi (Moonshot)',
  glm: 'GLM (Zhipu)',
  custom: 'Personalizado',
}

export const AI_PROVIDERS: CalendarAiProvider[] = [
  'claude',
  'deepseek',
  'qwen',
  'kimi',
  'glm',
  'custom',
]

// Cinza neutro para cliente com cor "none" (ou tipo sem override).
export const NEUTRAL_COLOR: RgbTriplet = [148, 163, 184]

// Validacao simples de cor #rrggbb (config guarda hex; o triplet e derivado).
const HEX_COLOR_RE = /^#[0-9a-fA-F]{6}$/

export function isHexColor(value: string): boolean {
  return HEX_COLOR_RE.test(value)
}

/** Converte "#rrggbb" em triplet RGB. Invalido -> cinza neutro. */
export function hexToTriplet(hex: string): RgbTriplet {
  if (!isHexColor(hex)) return NEUTRAL_COLOR
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return [r, g, b]
}

/** Converte um triplet RGB em "#rrggbb" (para o input color do config). */
export function tripletToHex(color: RgbTriplet): string {
  const hex = (n: number): string => Math.max(0, Math.min(255, n)).toString(16).padStart(2, '0')
  return `#${hex(color[0])}${hex(color[1])}${hex(color[2])}`
}

/**
 * Cor efetiva de um cliente: a override da config (`#rrggbb`) vence a semente;
 * `none` -> cinza neutro; sem override -> a cor-semente recebida.
 */
export function resolveClientColor(configColor: string | undefined, seed: RgbTriplet): RgbTriplet {
  if (configColor === 'none') return NEUTRAL_COLOR
  if (configColor && isHexColor(configColor)) return hexToTriplet(configColor)
  return seed
}
