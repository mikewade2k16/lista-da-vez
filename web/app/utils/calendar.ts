// Utilitarios puros do Calendario: tipos de dominio, constantes de apresentacao
// (tipo/status/prioridade), paleta de cor por cliente e helpers de grade de
// mes/semana + formatacao pt-BR. SEM estado, SEM fetch — so funcoes puras.
//
// As cores de cliente sao DADO (triplet RGB que no futuro vem da API), nao token
// de tema; por isso vivem aqui como paleta-semente e sao aplicadas via
// `rgb(r g b / alpha)` no ponto de uso, igual a filosofia dos tokens.

export type RgbTriplet = readonly [number, number, number]

export type CalendarEventType = 'post' | 'story' | 'reels' | 'reuniao' | 'gravacao' | 'evento'

export type CalendarEventStatus =
  | 'planejado'
  | 'producao'
  | 'revisao'
  | 'aprovada'
  | 'standby'
  | 'publicado'

export type CalendarPriority = 'alta' | 'media' | 'baixa'

export type StatusTone = 'info' | 'success' | 'warning' | 'danger' | 'neutral'

export type CalendarView = 'month' | 'week'

export type WeekStart = 'sunday' | 'monday'

export interface CalendarClient {
  id: string
  name: string
  color: RgbTriplet
}

export interface CalendarPerson {
  id: string
  name: string
}

export interface CalendarEvent {
  id: string
  /** Chave de data local no formato `YYYY-MM-DD`. */
  date: string
  /** Horario `HH:mm` (vazio = dia inteiro). */
  time: string
  clientId: string
  type: CalendarEventType
  title: string
  status: CalendarEventStatus
  priority: CalendarPriority
  responsibleId: string
  involvedIds: string[]
  /** Anexos (imagem/video) do post. */
  media: CalendarMediaItem[]
  description: string
}

/** Anexo (imagem ou video) de um evento ou dia. `url` = /uploads/calendar/... */
export interface CalendarMediaItem {
  id: string
  url: string
  name: string
  type: 'image' | 'video'
  contentType: string
  sizeBytes: number
}

/** Tetos de upload definidos NA PLATAFORMA (globais). */
export interface CalendarMediaLimits {
  imageMaxBytes: number
  videoMaxBytes: number
}

export function defaultCalendarMediaLimits(): CalendarMediaLimits {
  return { imageMaxBytes: 10 * 1024 * 1024, videoMaxBytes: 300 * 1024 * 1024 }
}

/** Tamanho legivel (ex.: 300 MB, 1.4 GB). Base 1024. */
export function formatBytes(bytes: number): string {
  if (!bytes || bytes < 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const exponent = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  const value = bytes / 1024 ** exponent
  const rounded = exponent === 0 ? value : Math.round(value * 10) / 10
  return `${rounded} ${units[exponent]}`
}

export interface DayCellModel {
  dateKey: string
  day: number
  inMonth: boolean
}

/** Payload de criar/atualizar um evento (tudo menos o id, que o back gera). */
export type CalendarEventInput = Omit<CalendarEvent, 'id'>

/** Usuario da conta (candidato/atual a responsavel). */
export interface CalendarMember {
  id: string
  name: string
}

/** Config do calendario por conta (responsaveis + feriados). */
export interface CalendarConfig {
  responsibleUserIds: string[]
  holidays: {
    brNational: boolean
    sergipe: boolean
    aracaju: boolean
    luxuryIntl: boolean
  }
}

export function defaultCalendarConfig(): CalendarConfig {
  return {
    responsibleUserIds: [],
    holidays: { brNational: true, sergipe: true, aracaju: true, luxuryIntl: true },
  }
}

/** Feriado/data comemorativa (vem do back, calculado). */
export interface CalendarHoliday {
  date: string
  name: string
  set: string
}

// --- Constantes de apresentacao -------------------------------------------------

export const EVENT_TYPE_META: Record<CalendarEventType, { label: string; icon: string }> = {
  post: { label: 'Post', icon: 'i-lucide-image' },
  story: { label: 'Story', icon: 'i-lucide-circle-play' },
  reels: { label: 'Reels', icon: 'i-lucide-film' },
  reuniao: { label: 'Reuniao', icon: 'i-lucide-users' },
  gravacao: { label: 'Gravacao', icon: 'i-lucide-video' },
  evento: { label: 'Evento', icon: 'i-lucide-calendar' },
}

export const STATUS_META: Record<CalendarEventStatus, { label: string; tone: StatusTone }> = {
  planejado: { label: 'Planejado', tone: 'neutral' },
  producao: { label: 'Producao', tone: 'info' },
  revisao: { label: 'Em revisao', tone: 'warning' },
  aprovada: { label: 'Aprovada', tone: 'success' },
  standby: { label: 'Standby', tone: 'warning' },
  publicado: { label: 'Publicado', tone: 'success' },
}

export const PRIORITY_META: Record<CalendarPriority, { label: string; tone: StatusTone }> = {
  alta: { label: 'Alta', tone: 'danger' },
  media: { label: 'Media', tone: 'warning' },
  baixa: { label: 'Baixa', tone: 'success' },
}

// Paleta-semente de cores por cliente. Triplets RGB (sem `rgb()`), aplicadas com
// alpha no ponto de uso. Distribuidas de forma estavel por indice/ hash do id.
export const CLIENT_COLOR_PALETTE: RgbTriplet[] = [
  [236, 72, 153], // rosa
  [139, 92, 246], // roxo
  [14, 165, 233], // azul
  [34, 197, 94], // verde
  [249, 115, 22], // laranja
  [234, 179, 8], // ambar
  [20, 184, 166], // teal
  [244, 63, 94], // vermelho-rosa
  [99, 102, 241], // indigo
  [217, 70, 239], // fucsia
  [6, 182, 212], // ciano
  [132, 204, 22], // lima
]

/** Cor estavel para um cliente, derivada do indice ou de um hash simples do id. */
export function clientColorFor(id: string, index: number): RgbTriplet {
  if (index >= 0) {
    return CLIENT_COLOR_PALETTE[index % CLIENT_COLOR_PALETTE.length]!
  }
  let hash = 0
  for (let i = 0; i < id.length; i += 1) {
    hash = (hash * 31 + id.charCodeAt(i)) >>> 0
  }
  return CLIENT_COLOR_PALETTE[hash % CLIENT_COLOR_PALETTE.length]!
}

/** Monta `rgb(r g b / alpha)` a partir de um triplet. */
export function rgba(color: RgbTriplet, alpha = 1): string {
  return `rgb(${color[0]} ${color[1]} ${color[2]} / ${alpha})`
}

// --- Helpers de data ------------------------------------------------------------

function pad2(value: number): string {
  return String(value).padStart(2, '0')
}

function keyOfDate(date: Date): string {
  return `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}`
}

function parseDateKey(dateKey: string): Date {
  const [year, month, day] = dateKey.split('-').map((part) => Number(part))
  return new Date(year || 1970, (month || 1) - 1, day || 1)
}

function parseMonthKey(monthKey: string): { year: number; month0: number } {
  const [year, month] = monthKey.split('-').map((part) => Number(part))
  return { year: year || 1970, month0: (month || 1) - 1 }
}

export function todayKey(): string {
  return keyOfDate(new Date())
}

export function monthKeyOf(dateKey: string): string {
  return dateKey.slice(0, 7)
}

export function addDaysToKey(dateKey: string, delta: number): string {
  const date = parseDateKey(dateKey)
  date.setDate(date.getDate() + delta)
  return keyOfDate(date)
}

export function addMonthsToKey(monthKey: string, delta: number): string {
  const { year, month0 } = parseMonthKey(monthKey)
  const date = new Date(year, month0 + delta, 1)
  return `${date.getFullYear()}-${pad2(date.getMonth() + 1)}`
}

export function monthKeyWindow(centerKey: string, before: number, after: number): string[] {
  const keys: string[] = []
  for (let offset = -before; offset <= after; offset += 1) {
    keys.push(addMonthsToKey(centerKey, offset))
  }
  return keys
}

export function startOfWeekKey(dateKey: string, weekStartsOn: WeekStart): string {
  const date = parseDateKey(dateKey)
  const weekday = date.getDay()
  const offset = weekStartsOn === 'monday' ? (weekday + 6) % 7 : weekday
  date.setDate(date.getDate() - offset)
  return keyOfDate(date)
}

export function addWeeksToKey(weekStartKey: string, delta: number): string {
  return addDaysToKey(weekStartKey, delta * 7)
}

export function weekStartWindow(centerWeekKey: string, before: number, after: number): string[] {
  const keys: string[] = []
  for (let offset = -before; offset <= after; offset += 1) {
    keys.push(addWeeksToKey(centerWeekKey, offset))
  }
  return keys
}

/** Matriz de semanas (cada uma com 7 celulas) que cobre o mes inteiro. */
export function buildMonthMatrix(monthKey: string, weekStartsOn: WeekStart): DayCellModel[][] {
  const { year, month0 } = parseMonthKey(monthKey)
  const firstWeekday = new Date(year, month0, 1).getDay()
  const startOffset = weekStartsOn === 'monday' ? (firstWeekday + 6) % 7 : firstWeekday
  const gridStart = new Date(year, month0, 1 - startOffset)
  const daysInMonth = new Date(year, month0 + 1, 0).getDate()
  const weekCount = Math.ceil((startOffset + daysInMonth) / 7)

  const weeks: DayCellModel[][] = []
  for (let w = 0; w < weekCount; w += 1) {
    const week: DayCellModel[] = []
    for (let d = 0; d < 7; d += 1) {
      const cur = new Date(gridStart)
      cur.setDate(gridStart.getDate() + w * 7 + d)
      week.push({ dateKey: keyOfDate(cur), day: cur.getDate(), inMonth: cur.getMonth() === month0 })
    }
    weeks.push(week)
  }
  return weeks
}

/** 7 celulas da semana a partir do inicio (todas marcadas como `inMonth`). */
export function buildWeekDays(weekStartKey: string): DayCellModel[] {
  const start = parseDateKey(weekStartKey)
  const days: DayCellModel[] = []
  for (let d = 0; d < 7; d += 1) {
    const cur = new Date(start)
    cur.setDate(start.getDate() + d)
    days.push({ dateKey: keyOfDate(cur), day: cur.getDate(), inMonth: true })
  }
  return days
}

const WEEKDAY_LABELS_SUN = ['Dom', 'Seg', 'Ter', 'Qua', 'Qui', 'Sex', 'Sab']

export function weekdayLabels(weekStartsOn: WeekStart): string[] {
  if (weekStartsOn === 'monday') {
    return [...WEEKDAY_LABELS_SUN.slice(1), WEEKDAY_LABELS_SUN[0]!]
  }
  return [...WEEKDAY_LABELS_SUN]
}

// --- Formatacao pt-BR -----------------------------------------------------------

const MONTH_YEAR_FMT = new Intl.DateTimeFormat('pt-BR', { month: 'long', year: 'numeric' })
const MONTH_DAY_FMT = new Intl.DateTimeFormat('pt-BR', { day: 'numeric', month: 'long' })
const WEEKDAY_DAY_MONTH_FMT = new Intl.DateTimeFormat('pt-BR', {
  weekday: 'long',
  day: 'numeric',
  month: 'long',
})

function capitalize(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1)
}

/** `2026-07` -> `Julho 2026`. */
export function formatMonthTitle(monthKey: string): string {
  const { year, month0 } = parseMonthKey(monthKey)
  return capitalize(MONTH_YEAR_FMT.format(new Date(year, month0, 1)))
}

/** `2026-07-14` -> `Quarta, 14 de Julho`. */
export function formatDayTitle(dateKey: string): string {
  const parts = WEEKDAY_DAY_MONTH_FMT.formatToParts(parseDateKey(dateKey))
  const weekday = parts.find((part) => part.type === 'weekday')?.value || ''
  const day = parts.find((part) => part.type === 'day')?.value || ''
  const month = parts.find((part) => part.type === 'month')?.value || ''
  return `${capitalize(weekday)}, ${day} de ${capitalize(month)}`
}

/** Intervalo curto da semana, ex. `7 – 13 de Julho`. */
export function formatWeekRangeTitle(weekStartKey: string): string {
  const start = parseDateKey(weekStartKey)
  const end = parseDateKey(addDaysToKey(weekStartKey, 6))
  if (start.getMonth() === end.getMonth()) {
    return `${start.getDate()} – ${capitalize(MONTH_DAY_FMT.format(end))}`
  }
  return `${capitalize(MONTH_DAY_FMT.format(start))} – ${capitalize(MONTH_DAY_FMT.format(end))}`
}
