// Tipos e helpers da CONFIG do calendario (contrato C2 do CALENDARIO_SPECS).
// Separado de utils/calendar.ts para manter cada arquivo < 450 linhas; o
// calendar.ts re-exporta tudo daqui, entao os imports existentes nao mudam.
// SEM estado, SEM fetch — so tipos + funcoes puras.
import type { RgbTriplet, WeekStart } from '~/utils/calendar'

/** Provedor de IA suportado (contrato C2/C6; +gemini na wave 2; +openai na wave 3). */
export type CalendarAiProvider =
  | 'claude'
  | 'deepseek'
  | 'qwen'
  | 'kimi'
  | 'glm'
  | 'gemini'
  | 'openai'
  | 'custom'

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

/** Provider de transcricao de audio (contrato CFG v4). 'local' = Whisper self-hosted
 * (aceita o audio webm do navegador, sem key); 'openai' = Whisper hospedado; 'gemini'
 * NAO transcreve o webm do navegador (fica so como opcao). */
export type CalendarTranscribeProvider = 'openai' | 'gemini' | 'local'

/** Provider que tem chave de API secreta (contrato SEC): so estes tres. */
export type CalendarAiSecretProvider = 'gemini' | 'glm' | 'openai'

/** Escopo da IA (WAVE 3.1): uma config geral p/ todos ou individual por cliente. */
export type CalendarAiScopeMode = 'general' | 'perClient'

/** Status MASCARADO de uma chave (contrato SEC): nunca a chave crua, so set+last4. */
export interface CalendarAiKeyStatus {
  set: boolean
  last4: string
}

/**
 * Config de IA do calendario (contrato CFG v4). As CHAVES de API NUNCA moram aqui:
 * vivem em secrets server-side (calendar.ai_secrets / core.platform_settings) e o
 * front so recebe status mascarado. `enabled` = kill switch da IA; `useGlobalKeys`
 * = usar as chaves GLOBAIS da plataforma (true) ou as DESTA conta (false).
 */
export interface CalendarAiConfig {
  enabled: boolean
  useGlobalKeys: boolean
  provider: CalendarAiProvider
  model: string
  baseUrl: string
  systemPrompt: string
  temperature: number
  transcribeProvider: CalendarTranscribeProvider
  transcribeModel: string
  /** WAVE 3.1: 'general' (uma config p/ todos) | 'perClient' (config por cliente). */
  scopeMode: CalendarAiScopeMode
  /** WAVE 3.1 (modo general): clientes com a IA DESLIGADA (excecoes por id). */
  disabledClientIds: string[]
}

/**
 * Override de IA POR CLIENTE (WAVE 3.1, SEC+). So COMPORTAMENTO — as chaves de API
 * seguem no nivel conta/global (contrato SEC). Cada campo em null/'' = HERDAR a config
 * geral da conta; preenchido = vence no merge por campo (resolver EffectiveAIConfig no
 * back). Override vazio (normalizado de {}) = o cliente usa 100% a config geral.
 */
export interface CalendarClientAiOverride {
  enabled: boolean | null
  provider: CalendarAiProvider | ''
  model: string
  baseUrl: string
  systemPrompt: string
  temperature: number | null
}

/** Posicao da janela de chat (contrato CFG v4): espelha o modo do modal. */
export type CalendarChatPosition = 'center' | 'left' | 'right' | 'fullscreen'

/** Layout da janela de chat por conta (contrato CFG v4). width/height 0 = default. */
export interface CalendarChatConfig {
  position: CalendarChatPosition
  width: number
  height: number
}

/** WAVE 5 (E5): mapeia UM status de evento a UMA coluna do board (nos dois sentidos). */
export interface CalendarStatusColumnMapEntry {
  eventStatus: string
  columnId: string
}

/**
 * Integracao calendario <-> tasks (contrato C6 + WAVE 5). Vazio = integracao DESLIGADA.
 * boardId/defaultColumnId sao UUID (ou vazio); o back sanitiza no PUT. WAVE 5: mirrorTasks
 * liga o espelho task->evento (default true); defaultEventType = tipo do evento-espelho;
 * statusColumnMap = mapa status<->coluna (E5). Sem board, mirror/statusMap nao tem efeito.
 */
export interface CalendarTasksConfig {
  boardId: string
  defaultColumnId: string
  mirrorTasks: boolean
  defaultEventType: string
  statusColumnMap: CalendarStatusColumnMapEntry[]
}

/** Config do calendario por conta (contrato C2/C6, jsonb calendar.config). */
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
  /** Board/coluna de destino ao criar task pelo evento (contrato C6). */
  tasks: CalendarTasksConfig
  /** Layout da janela de chat por conta (contrato CFG v4). */
  chat: CalendarChatConfig
  /** Atalhos de teclado (WAVE 11): { acao: tecla }. Vazio = atalho desligado. */
  shortcuts: Record<string, string>
}

/** Teclas default dos atalhos (WAVE 11) — espelho de shortcutDefaults() do back. */
export const CALENDAR_SHORTCUT_DEFAULTS: Record<string, string> = {
  chatOpen: 'c',
  chatRecordStart: 'a',
  chatRecordStop: 'enter',
  chatClose: 'escape',
  calToday: 't',
  calMonthView: 'm',
  calWeekView: 'w',
  calNewItem: 'n',
  calNotesSidebar: 's',
  calSpans: 'b',
  calPrev: 'arrowleft',
  calNext: 'arrowright',
}

/** Acoes de atalho para a UI de config (label + grupo), na ordem de exibicao. */
export const CALENDAR_SHORTCUT_ACTIONS: { key: string; label: string; group: 'chat' | 'cal' }[] = [
  { key: 'chatOpen', label: 'Abrir/fechar o assistente', group: 'chat' },
  { key: 'chatRecordStart', label: 'Iniciar gravação de voz', group: 'chat' },
  { key: 'chatRecordStop', label: 'Parar gravação', group: 'chat' },
  { key: 'chatClose', label: 'Fechar a janela da IA', group: 'chat' },
  { key: 'calToday', label: 'Ir para hoje', group: 'cal' },
  { key: 'calMonthView', label: 'Visão mensal', group: 'cal' },
  { key: 'calWeekView', label: 'Visão semanal', group: 'cal' },
  { key: 'calNewItem', label: 'Novo item', group: 'cal' },
  { key: 'calNotesSidebar', label: 'Recolher/mostrar as anotações', group: 'cal' },
  { key: 'calSpans', label: 'Mostrar/ocultar as barras multi-dia', group: 'cal' },
  { key: 'calPrev', label: 'Mês/semana anterior', group: 'cal' },
  { key: 'calNext', label: 'Próximo mês/semana', group: 'cal' },
]

/** Teclas especiais aceitas no vocabulario de atalhos (espelho do back sanitizeShortcuts). */
export const CALENDAR_SHORTCUT_SPECIAL_KEYS = [
  'enter',
  'escape',
  'space',
  'arrowleft',
  'arrowright',
  'arrowup',
  'arrowdown',
]

/** Modificadores aceitos, na ORDEM CANONICA do combo (front e back gravam nesta ordem). */
export const CALENDAR_SHORTCUT_MODIFIERS = ['ctrl', 'alt', 'shift', 'meta']

// baseKeyFromCode deriva a tecla-base a partir do event.code (posicao FISICA — estavel
// independente de layout e de Shift/Alt; Shift+2 vira 'shift+2', nao 'shift+@'). Vazio =
// modificador sozinho ou tecla nao suportada.
function baseKeyFromCode(code: string): string {
  if (/^Key[A-Z]$/.test(code)) return code.slice(3).toLowerCase()
  if (/^Digit[0-9]$/.test(code)) return code.slice(5)
  if (/^Numpad[0-9]$/.test(code)) return code.slice(6)
  switch (code) {
    case 'ArrowLeft':
      return 'arrowleft'
    case 'ArrowRight':
      return 'arrowright'
    case 'ArrowUp':
      return 'arrowup'
    case 'ArrowDown':
      return 'arrowdown'
    case 'Enter':
    case 'NumpadEnter':
      return 'enter'
    case 'Escape':
      return 'escape'
    case 'Space':
      return 'space'
    default:
      return ''
  }
}

/**
 * Deriva o COMBO de atalho de um KeyboardEvent: modificadores (ctrl/alt/shift/meta, ordem
 * canonica) + tecla-base, ex.: 'shift+t', 'ctrl+shift+k', 'alt+arrowleft', 't'. Modificador
 * sozinho / tecla nao suportada => '' (o chamador ignora). Mesma regra no runtime e no back.
 */
export function shortcutComboFromEvent(event: KeyboardEvent): string {
  const base = baseKeyFromCode(event.code)
  if (!base) return ''
  const parts: string[] = []
  if (event.ctrlKey) parts.push('ctrl')
  if (event.altKey) parts.push('alt')
  if (event.shiftKey) parts.push('shift')
  if (event.metaKey) parts.push('meta')
  parts.push(base)
  return parts.join('+')
}

/** Rotulo amigavel de um combo para exibir no painel ('' => travessao). Ex.: 'Ctrl+Shift+K'. */
export function shortcutKeyLabel(combo: string): string {
  if (!combo) return '—'
  const map: Record<string, string> = {
    ctrl: 'Ctrl',
    alt: 'Alt',
    shift: 'Shift',
    meta: 'Meta',
    enter: 'Enter',
    escape: 'Esc',
    space: 'Espaço',
    arrowleft: '←',
    arrowright: '→',
    arrowup: '↑',
    arrowdown: '↓',
  }
  return combo
    .split('+')
    .map((p) => map[p] || p.toUpperCase())
    .join('+')
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
      enabled: true,
      useGlobalKeys: true,
      provider: 'gemini',
      model: 'gemini-2.5-flash',
      baseUrl: '',
      systemPrompt: '',
      temperature: 0.7,
      transcribeProvider: 'local',
      transcribeModel: '',
      scopeMode: 'general',
      disabledClientIds: [],
    },
    tasks: {
      boardId: '',
      defaultColumnId: '',
      mirrorTasks: true, // WAVE 5: espelho task->evento ligado por padrao (decisao do dono)
      defaultEventType: '',
      statusColumnMap: [],
    },
    chat: { position: 'center', width: 0, height: 0 },
    shortcuts: { ...CALENDAR_SHORTCUT_DEFAULTS },
  }
}

// Base URLs default por provider (placeholder no front; o n8n usa o mesmo mapa).
// Contrato C2 — chave vazia no config = usar o default do provider.
export const AI_PROVIDER_BASE_URL: Record<CalendarAiProvider, string> = {
  claude: 'https://api.anthropic.com',
  deepseek: 'https://api.deepseek.com',
  qwen: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
  kimi: 'https://api.moonshot.cn/v1',
  glm: 'https://api.z.ai/api/paas/v4',
  // Camada OpenAI-compatible do Google AI Studio (free tier); mesmo mapa no n8n.
  gemini: 'https://generativelanguage.googleapis.com/v1beta/openai',
  openai: 'https://api.openai.com/v1',
  custom: '',
}

export const AI_PROVIDER_LABEL: Record<CalendarAiProvider, string> = {
  claude: 'Claude (Anthropic)',
  deepseek: 'DeepSeek',
  qwen: 'Qwen (Alibaba)',
  kimi: 'Kimi (Moonshot)',
  glm: 'GLM (z.ai)',
  gemini: 'Gemini (Google, free tier)',
  openai: 'OpenAI',
  custom: 'Personalizado',
}

// Providers oferecidos no dropdown do painel. So os que tem slot de chave (SEC) e
// workflow ligado na Wave 3 — claude/deepseek/qwen/kimi/custom ficam de fora (nao
// resolvem chave, cairiam em ai_key_missing). O type CalendarAiProvider mantem os
// demais para compat de dados antigos.
export const AI_PROVIDERS: CalendarAiProvider[] = ['gemini', 'glm', 'openai']

// Providers com chave de API secreta gerenciada pelo painel (contrato SEC). O front
// so manipula o status mascarado destes; os demais nao usam chave propria por aqui.
export const AI_SECRET_PROVIDERS: CalendarAiSecretProvider[] = ['gemini', 'glm', 'openai']

// --- Override de IA por cliente (WAVE 3.1, SEC+) --------------------------------

/** Override "vazio" = todos os campos herdam a config geral (null/''). */
export function defaultClientAiOverride(): CalendarClientAiOverride {
  return {
    enabled: null,
    provider: '',
    model: '',
    baseUrl: '',
    systemPrompt: '',
    temperature: null,
  }
}

// Coincide com o back (jsonb {} = sem override). null/'' = herda a config geral; o
// provider e' validado contra o enum conhecido (fora dele -> '' = herda); temperature
// clampa 0..1 (invalida/fora de faixa -> null = herda). NUNCA carrega chave de API.
export function normalizeClientAiOverride(res: unknown): CalendarClientAiOverride {
  const raw = (res && typeof res === 'object' ? res : {}) as Partial<CalendarClientAiOverride>
  const provider =
    typeof raw.provider === 'string' && raw.provider in AI_PROVIDER_LABEL
      ? (raw.provider as CalendarAiProvider)
      : ''
  let temperature: number | null = null
  if (typeof raw.temperature === 'number' && Number.isFinite(raw.temperature)) {
    temperature = Math.min(1, Math.max(0, raw.temperature))
  }
  return {
    enabled: typeof raw.enabled === 'boolean' ? raw.enabled : null,
    provider,
    model: typeof raw.model === 'string' ? raw.model : '',
    baseUrl: typeof raw.baseUrl === 'string' ? raw.baseUrl : '',
    systemPrompt: typeof raw.systemPrompt === 'string' ? raw.systemPrompt : '',
    temperature,
  }
}

// Override sem nada preenchido: usado pro badge "usa config geral" e pra saber se ha
// algo persistido para o cliente.
export function isEmptyClientAiOverride(o: CalendarClientAiOverride): boolean {
  return (
    o.enabled === null &&
    o.provider === '' &&
    o.model === '' &&
    o.baseUrl === '' &&
    o.systemPrompt === '' &&
    o.temperature === null
  )
}

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
