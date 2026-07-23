import type { TaskItem } from '../../layers/tasks/types/tasks'

// Barras multi-dia (WAVE 11): tasks do board configurado com INICIO (dueDate) e FIM
// (dueEndDate) atravessando 2+ dias viram uma BARRA CONTINUA na grade do mes (estilo
// Google Calendar). Helpers puros: derivam os spans das tasks e calculam, por SEMANA,
// os segmentos (coluna inicial/final + "lane" para empilhar sobreposicoes).

export interface CalendarTaskSpan {
  id: string
  title: string
  clientId: string
  startKey: string // YYYY-MM-DD (inclusive)
  endKey: string // YYYY-MM-DD (inclusive)
}

export interface CalendarSpanSegment {
  span: CalendarTaskSpan
  colStart: number // 1..7 (coluna da grade)
  colEnd: number // 1..7 (inclusive)
  lane: number // 0.. (empilhamento na semana)
  startsHere: boolean // o span COMECA nesta semana (borda esquerda arredondada)
  endsHere: boolean // o span TERMINA nesta semana (borda direita arredondada)
}

// Teto de lanes por semana: acima disso os spans excedentes nao renderizam (a grade nao
// vira um "gantt"; o detalhe completo vive no board).
export const MAX_SPAN_LANES = 3

function dateOnly(value: string): string {
  const raw = String(value || '').trim()
  return /^\d{4}-\d{2}-\d{2}/.test(raw) ? raw.slice(0, 10) : ''
}

/**
 * Deriva os spans multi-dia das tasks: precisa de dueDate E dueEndDate validos com
 * fim > inicio (mesmo dia nao vira barra — ja aparece como task normal). Tasks
 * arquivadas ficam de fora.
 */
export function taskSpansFrom(tasks: TaskItem[]): CalendarTaskSpan[] {
  const out: CalendarTaskSpan[] = []
  for (const task of tasks || []) {
    if (task.archived) continue
    const start = dateOnly(task.dueDate)
    const end = dateOnly(task.dueEndDate)
    if (!start || !end || end <= start) continue
    out.push({
      id: task.id,
      title: task.title || '(sem título)',
      clientId: task.clientId || '',
      startKey: start,
      endKey: end,
    })
  }
  // Mais longos primeiro (lanes mais estaveis), depois por inicio.
  return out.sort((a, b) =>
    a.startKey === b.startKey ? (a.endKey < b.endKey ? 1 : -1) : a.startKey < b.startKey ? -1 : 1,
  )
}

export function hasTaskSpanInMonth(spans: CalendarTaskSpan[], monthKey: string): boolean {
  if (!/^\d{4}-\d{2}$/.test(monthKey)) return false
  const monthStart = `${monthKey}-01`
  const monthEnd = `${monthKey}-31`
  return spans.some((span) => span.endKey >= monthStart && span.startKey <= monthEnd)
}

/**
 * Segmentos de uma SEMANA (7 dateKeys em ordem): para cada span que intersecta a semana,
 * a coluna inicial/final + a lane (greedy: primeira lane livre; acima de MAX_SPAN_LANES
 * o span nao renderiza nesta semana).
 */
export function weekSpanSegments(
  weekDays: string[],
  spans: CalendarTaskSpan[],
): CalendarSpanSegment[] {
  if (weekDays.length === 0) return []
  const weekStart = weekDays[0]!
  const weekEnd = weekDays[weekDays.length - 1]!
  const laneEnds: string[] = [] // por lane: ultima coluna ocupada (dateKey final)
  const out: CalendarSpanSegment[] = []
  for (const span of spans) {
    if (span.endKey < weekStart || span.startKey > weekEnd) continue
    const startIdx = span.startKey <= weekStart ? 0 : weekDays.indexOf(span.startKey)
    const endIdx = span.endKey >= weekEnd ? weekDays.length - 1 : weekDays.indexOf(span.endKey)
    if (startIdx < 0 || endIdx < 0 || endIdx < startIdx) continue
    let lane = laneEnds.findIndex((end) => end < span.startKey)
    if (lane < 0) {
      if (laneEnds.length >= MAX_SPAN_LANES) continue
      lane = laneEnds.length
      laneEnds.push(span.endKey)
    } else {
      laneEnds[lane] = span.endKey
    }
    out.push({
      span,
      colStart: startIdx + 1,
      colEnd: endIdx + 1,
      lane,
      startsHere: span.startKey >= weekStart,
      endsHere: span.endKey <= weekEnd,
    })
  }
  return out
}
