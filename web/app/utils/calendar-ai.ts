// Tipos e helpers do PLANO DE IA do calendario (contrato C4 do CALENDARIO_SPECS).
// Separado de calendar.ts/calendar-config.ts para manter cada arquivo < 450 linhas;
// o calendar.ts re-exporta daqui. SEM estado, SEM fetch — so tipos + funcoes puras.
import type { CalendarEventType } from '~/utils/calendar'

/** Status do plano (C4): pending -> done | error; done -> applied. */
export type CalendarAiPlanStatus = 'pending' | 'done' | 'error' | 'applied'

/** Um pilar de conteudo do plano (nome + proporcao + racional). */
export interface CalendarAiPlanPillar {
  name: string
  proportion: string
  rationale: string
}

/** Uma ideia de postagem para um dia (data + tipo + ideia + copy). */
export interface CalendarAiPlanDay {
  date: string
  /** post | story | reels (fora do enum vira post no back). */
  type: string
  idea: string
  copy: string
}

/** Estrategia + ideias de um cliente no plano. */
export interface CalendarAiPlanClient {
  clientId: string
  clientName: string
  strategy: string
  days: CalendarAiPlanDay[]
}

/** Conteudo gerado pela IA (contrato C4.content). */
export interface CalendarAiPlanContent {
  summary: string
  pillars: CalendarAiPlanPillar[]
  clients: CalendarAiPlanClient[]
}

/** Plano completo (GET /ai/plans/{id}). */
export interface CalendarAiPlan {
  id: string
  month: string
  clientIds: string[]
  status: CalendarAiPlanStatus
  provider: string
  model: string
  content: CalendarAiPlanContent
  error: string
  createdAt: string
  updatedAt: string
}

/** Linha lean da listagem de planos (GET /ai/plans?month=), sem o content. */
export interface CalendarAiPlanIndexItem {
  id: string
  month: string
  clientIds: string[]
  status: CalendarAiPlanStatus
  provider: string
  model: string
  createdAt: string
}

const PLAN_STATUSES: CalendarAiPlanStatus[] = ['pending', 'done', 'error', 'applied']

// Tipos de postagem que viram tipo de evento no "Criar eventos"; fora disso -> post.
const PLAN_TYPE_TO_EVENT: Record<string, CalendarEventType> = {
  post: 'post',
  story: 'story',
  reels: 'reels',
}

export function planTypeToEventType(type: string): CalendarEventType {
  return PLAN_TYPE_TO_EVENT[String(type || '').toLowerCase()] || 'post'
}

function planStatus(value: unknown): CalendarAiPlanStatus {
  const s = String(value || '')
  return (PLAN_STATUSES as string[]).includes(s) ? (s as CalendarAiPlanStatus) : 'pending'
}

function str(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function strList(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((v): v is string => typeof v === 'string') : []
}

export function defaultPlanContent(): CalendarAiPlanContent {
  return { summary: '', pillars: [], clients: [] }
}

// Normaliza o content da resposta (arrays nunca undefined; campos string sempre
// presentes). Espelha a normalizacao do back para o front nunca quebrar por shape.
export function normalizePlanContent(raw: unknown): CalendarAiPlanContent {
  const obj = (raw as Partial<CalendarAiPlanContent>) || {}
  const pillars = Array.isArray(obj.pillars) ? obj.pillars : []
  const clients = Array.isArray(obj.clients) ? obj.clients : []
  return {
    summary: str(obj.summary),
    pillars: pillars.map((p) => ({
      name: str(p?.name),
      proportion: str(p?.proportion),
      rationale: str(p?.rationale),
    })),
    clients: clients.map((c) => ({
      clientId: str(c?.clientId),
      clientName: str(c?.clientName),
      strategy: str(c?.strategy),
      days: (Array.isArray(c?.days) ? c.days : []).map((d) => ({
        date: str(d?.date),
        type: str(d?.type),
        idea: str(d?.idea),
        copy: str(d?.copy),
      })),
    })),
  }
}

/** Normaliza o plano completo (GET /ai/plans/{id}). */
export function normalizePlan(raw: unknown): CalendarAiPlan {
  const obj = (raw as Partial<CalendarAiPlan>) || {}
  return {
    id: str(obj.id),
    month: str(obj.month),
    clientIds: strList(obj.clientIds),
    status: planStatus(obj.status),
    provider: str(obj.provider),
    model: str(obj.model),
    content: normalizePlanContent(obj.content),
    error: str(obj.error),
    createdAt: str(obj.createdAt),
    updatedAt: str(obj.updatedAt),
  }
}

// Escapa texto para interpolar com seguranca no HTML da nota (v-html do editor).
function escapeHtml(value: string): string {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

/**
 * Monta o HTML do plano para anexar a nota do mes ("Aplicar nas notas"). Texto
 * escapado (o content vem da IA/n8n, tratado como nao-confiavel). Estrutura simples
 * de titulos/listas que o editor TipTap renderiza.
 */
export function planContentToNotesHtml(content: CalendarAiPlanContent): string {
  const parts: string[] = ['<h3>Plano de conteudo (IA)</h3>']
  if (content.summary.trim()) parts.push(`<p>${escapeHtml(content.summary)}</p>`)
  if (content.pillars.length) {
    parts.push('<p><strong>Pilares</strong></p><ul>')
    for (const pillar of content.pillars) {
      const proportion = pillar.proportion ? ` (${escapeHtml(pillar.proportion)})` : ''
      const rationale = pillar.rationale ? ` — ${escapeHtml(pillar.rationale)}` : ''
      parts.push(`<li><strong>${escapeHtml(pillar.name)}</strong>${proportion}${rationale}</li>`)
    }
    parts.push('</ul>')
  }
  for (const client of content.clients) {
    parts.push(`<p><strong>${escapeHtml(client.clientName || 'Cliente')}</strong></p>`)
    if (client.strategy.trim()) parts.push(`<p>${escapeHtml(client.strategy)}</p>`)
    if (client.days.length) {
      parts.push('<ul>')
      for (const day of client.days) {
        const idea = escapeHtml(day.idea)
        const copy = day.copy ? ` — ${escapeHtml(day.copy)}` : ''
        parts.push(`<li>${escapeHtml(day.date)} [${escapeHtml(day.type)}]: ${idea}${copy}</li>`)
      }
      parts.push('</ul>')
    }
  }
  return parts.join('')
}

/** Normaliza uma linha lean do index (GET /ai/plans). */
export function normalizePlanIndexItem(raw: unknown): CalendarAiPlanIndexItem {
  const obj = (raw as Partial<CalendarAiPlanIndexItem>) || {}
  return {
    id: str(obj.id),
    month: str(obj.month),
    clientIds: strList(obj.clientIds),
    status: planStatus(obj.status),
    provider: str(obj.provider),
    model: str(obj.model),
    createdAt: str(obj.createdAt),
  }
}
