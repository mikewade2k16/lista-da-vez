import type {
  CalendarChatCalendarItem,
  CalendarChatProposalTaskItem,
  CalendarChatScopeClient,
  CalendarChatStoredProposal,
} from '~/domain/calendar/calendar-chat-api'
import {
  eventTypeMeta,
  priorityMeta,
  statusMeta,
  type CalendarEvent,
  type CalendarPerson,
} from '~/utils/calendar'

type ProposalTarget = CalendarEvent | CalendarChatCalendarItem

export interface CalendarProposalChange {
  key: string
  label: string
  before: string
  after: string
}

export interface CalendarProposalPreviewContext {
  clients: CalendarChatScopeClient[]
  calendarItems: CalendarChatCalendarItem[]
  people: CalendarPerson[]
  getEventById: (id: string) => CalendarEvent | null
}

function dateLabel(value: string): string {
  const [year, month, day] = value.split('-').map(Number)
  if (!year || !month || !day) return value
  return new Intl.DateTimeFormat('pt-BR', { day: '2-digit', month: 'short' }).format(
    new Date(year, month - 1, day),
  )
}

function normalizeLabel(value: string): string {
  return String(value || '')
    .trim()
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
}

function clientLabel(id: string, fallback: string, clients: CalendarChatScopeClient[]): string {
  if (!id) return fallback || 'Sem cliente'
  return clients.find((client) => client.id === id)?.name || fallback || id
}

function resolvePersonId(value: string, people: CalendarPerson[]): string {
  const raw = String(value || '').trim()
  if (!raw) return ''
  if (people.some((person) => person.id === raw)) return raw
  const needle = normalizeLabel(raw)
  const matches = people.filter((person) => {
    const name = normalizeLabel(person.name)
    return name === needle || name.startsWith(`${needle} `)
  })
  return matches.length === 1 ? matches[0]!.id : raw
}

function personLabel(value: string, people: CalendarPerson[]): string {
  const raw = String(value || '').trim()
  if (!raw) return '-'
  const resolved = resolvePersonId(raw, people)
  const found = people.find((person) => person.id === resolved)
  return found?.name || raw
}

function peopleLabels(ids: string[] | undefined, people: CalendarPerson[]): string {
  const labels = (ids || []).map((id) => personLabel(id, people)).filter((label) => label !== '-')
  return labels.length ? labels.join(', ') : '-'
}

function shortText(value: string): string {
  const text = String(value || '')
    .replace(/\s+/g, ' ')
    .trim()
  return text.length > 90 ? `${text.slice(0, 87)}...` : text
}

function fieldHasValue(value: unknown): boolean {
  if (Array.isArray(value)) return value.length > 0
  if (typeof value === 'boolean') return true
  return String(value ?? '').trim() !== ''
}

function targetSnapshot(
  proposal: CalendarChatStoredProposal,
  ctx: CalendarProposalPreviewContext,
): ProposalTarget | null {
  const targetId = String(proposal.fields.targetId || '')
  if (!targetId) return null
  return (
    ctx.getEventById(targetId) ||
    ctx.calendarItems.find((item) => item.id === targetId || item.taskId === targetId) ||
    null
  )
}

function formattedField(
  key: keyof CalendarChatStoredProposal['fields'],
  value: unknown,
  fields: CalendarChatStoredProposal['fields'],
  ctx: CalendarProposalPreviewContext,
): string {
  const text = String(value ?? '').trim()
  if (key === 'type') return eventTypeMeta(text).label
  if (key === 'status') return statusMeta(text).label
  if (key === 'priority') return priorityMeta(text).label
  if (key === 'responsibleId') return personLabel(text, ctx.people)
  if (key === 'involvedIds') {
    return peopleLabels(Array.isArray(value) ? value.map(String) : [], ctx.people)
  }
  if (key === 'clientId')
    return clientLabel(text, String(fields.clientName || '').trim(), ctx.clients)
  if (key === 'clientName') return text
  if (key === 'description' || key === 'contentHtml') return shortText(text)
  if (key === 'archived') return value ? 'Arquivar' : 'Manter ativo'
  if (key === 'date' || key === 'dueDate' || key === 'startDate' || key === 'dueEndDate') {
    return text ? dateLabel(text) : '-'
  }
  return text || '-'
}

function snapshotValue(
  target: ProposalTarget | null,
  key: keyof CalendarChatStoredProposal['fields'],
) {
  if (!target) return ''
  if (key === 'dueDate') return 'dueDate' in target ? target.dueDate : target.date
  if (key === 'description' || key === 'contentHtml') return target.description || ''
  return (target as unknown as Record<string, unknown>)[key] ?? ''
}

const CHANGE_FIELDS: {
  key: keyof CalendarChatStoredProposal['fields']
  label: string
}[] = [
  { key: 'responsibleId', label: 'Responsável' },
  { key: 'date', label: 'Data' },
  { key: 'time', label: 'Horário' },
  { key: 'title', label: 'Título' },
  { key: 'type', label: 'Tipo' },
  { key: 'status', label: 'Status' },
  { key: 'priority', label: 'Prioridade' },
  { key: 'clientId', label: 'Cliente' },
  { key: 'involvedIds', label: 'Envolvidos' },
  { key: 'description', label: 'Descrição' },
  { key: 'contentHtml', label: 'Conteúdo' },
  { key: 'dueDate', label: 'Prazo' },
  { key: 'dueEndDate', label: 'Fim' },
  { key: 'archived', label: 'Arquivo' },
]

// Rotulos dos campos do perfil (WAVE 7) para o preview da proposta clientProfile. Ordem = contrato C3.
const PROFILE_FIELD_LABELS: { key: string; label: string; extra?: boolean }[] = [
  { key: 'segment', label: 'Segmento' },
  { key: 'positioning', label: 'Posicionamento' },
  { key: 'siteUrl', label: 'Site' },
  { key: 'instagram', label: 'Instagram' },
  { key: 'address', label: 'Endereço' },
  { key: 'description', label: 'Descrição' },
  { key: 'history', label: 'História' },
  { key: 'objectives', label: 'Objetivos' },
  { key: 'brandVoice', label: 'Tom de voz' },
  { key: 'audience', label: 'Público-alvo', extra: true },
  { key: 'offer', label: 'Oferta', extra: true },
  { key: 'pillars', label: 'Pilares', extra: true },
  { key: 'cadence', label: 'Cadência', extra: true },
  { key: 'restrictions', label: 'Restrições', extra: true },
  { key: 'performance', label: 'Performance', extra: true },
  { key: 'assets', label: 'Assets', extra: true },
]

// profileProposalChanges lista os campos do perfil na proposta (clientProfile, WAVE 7): preencher/
// editar mostra "campo -> valor"; delete mostra "Limpar" por campo ou "Zerar perfil inteiro".
function profileProposalChanges(proposal: CalendarChatStoredProposal): CalendarProposalChange[] {
  const prof = proposal.fields.profile || {}
  const out: CalendarProposalChange[] = []
  if (proposal.action === 'delete') {
    if (prof.clearAll) {
      out.push({ key: 'clearAll', label: 'Perfil', before: '', after: 'Zerar perfil inteiro' })
    }
    for (const key of prof.clearFields || []) {
      const def = PROFILE_FIELD_LABELS.find((d) => d.key === key)
      out.push({ key, label: def?.label || key, before: '', after: 'Limpar' })
    }
    return out
  }
  const extra = (prof.extra || {}) as Record<string, unknown>
  const stable = prof as unknown as Record<string, unknown>
  for (const def of PROFILE_FIELD_LABELS) {
    const value = String((def.extra ? extra[def.key] : stable[def.key]) ?? '').trim()
    if (value) out.push({ key: def.key, label: def.label, before: '', after: shortText(value) })
  }
  return out
}

// noteProposalChanges resume a proposta de anotacao (note, WAVE 7): acao (Acrescentar/Reescrever/
// Limpar), mes e um trecho do conteudo (sem tags HTML).
function noteProposalChanges(proposal: CalendarChatStoredProposal): CalendarProposalChange[] {
  const note = proposal.fields.note || {}
  const out: CalendarProposalChange[] = []
  const modeLabel =
    proposal.action === 'delete' ? 'Limpar' : note.mode === 'replace' ? 'Reescrever' : 'Acrescentar'
  out.push({ key: 'mode', label: 'Ação', before: '', after: modeLabel })
  if (note.month) out.push({ key: 'month', label: 'Mês', before: '', after: note.month })
  if (proposal.action !== 'delete') {
    const content = String(note.content || '')
      .replace(/<[^>]*>/g, ' ')
      .replace(/\s+/g, ' ')
      .trim()
    if (content) {
      out.push({ key: 'content', label: 'Conteúdo', before: '', after: shortText(content) })
    }
  }
  return out
}

const TASK_ITEM_STATUS_LABELS: Record<string, string> = {
  captured: 'Gravado',
  editing: 'Em edição',
  approval: 'Em aprovação',
  approved: 'Aprovado',
  scheduled: 'Agendado',
  posted: 'Postado',
}

function taskItemProposalChanges(proposal: CalendarChatStoredProposal): CalendarProposalChange[] {
  const item: CalendarChatProposalTaskItem = proposal.fields.taskItem || {}
  if (proposal.action === 'delete') return []

  const out: CalendarProposalChange[] = []
  const proposedTitle = String(item.title || '').trim()
  const currentTitle = String(item.itemTitle || '').trim()
  if (proposedTitle && (proposal.action === 'create' || proposedTitle !== currentTitle)) {
    out.push({
      key: 'taskItem.title',
      label: 'Item',
      before: proposal.action === 'update' ? currentTitle : '',
      after: proposedTitle,
    })
  }
  if (item.status) {
    out.push({
      key: 'taskItem.status',
      label: 'Status',
      before: '',
      after: TASK_ITEM_STATUS_LABELS[item.status] || item.status,
    })
  }
  if (item.statusDate) {
    out.push({
      key: 'taskItem.statusDate',
      label: 'Data do status',
      before: '',
      after: dateLabel(item.statusDate),
    })
  }
  if (typeof item.completed === 'boolean') {
    out.push({
      key: 'taskItem.completed',
      label: 'Finalizado',
      before: '',
      after: item.completed ? 'Sim' : 'Não',
    })
  }
  if (item.completedDate) {
    out.push({
      key: 'taskItem.completedDate',
      label: 'Finalizado em',
      before: '',
      after: dateLabel(item.completedDate),
    })
  }
  return out
}

export function calendarProposalTargetClientId(
  proposal: CalendarChatStoredProposal,
  ctx: CalendarProposalPreviewContext,
): string {
  if (proposal.kind === 'taskItem') return ''
  const target = targetSnapshot(proposal, ctx)
  return String(target?.clientId || '')
}

export function calendarProposalTargetTitle(
  proposal: CalendarChatStoredProposal,
  ctx: CalendarProposalPreviewContext,
): string {
  if (proposal.kind === 'taskItem') {
    return String(proposal.fields.taskItem?.taskTitle || '')
  }
  const target = targetSnapshot(proposal, ctx)
  return String(target?.title || '')
}

export function calendarProposalChanges(
  proposal: CalendarChatStoredProposal,
  ctx: CalendarProposalPreviewContext,
): CalendarProposalChange[] {
  if (proposal.kind === 'clientProfile') return profileProposalChanges(proposal)
  if (proposal.kind === 'note') return noteProposalChanges(proposal)
  if (proposal.kind === 'taskItem') return taskItemProposalChanges(proposal)
  const fields = proposal.fields || {}
  const target = targetSnapshot(proposal, ctx)
  const changes: CalendarProposalChange[] = []
  for (const def of CHANGE_FIELDS) {
    if (!Object.prototype.hasOwnProperty.call(fields, def.key)) continue
    const rawAfter = fields[def.key]
    if (!fieldHasValue(rawAfter)) continue
    const beforeRaw = snapshotValue(target, def.key)
    const before =
      proposal.action === 'update' ? formattedField(def.key, beforeRaw, fields, ctx) : ''
    const after = formattedField(def.key, rawAfter, fields, ctx)
    if (proposal.action === 'update' && normalizeLabel(before) === normalizeLabel(after)) continue
    changes.push({ key: def.key, label: def.label, before, after })
  }
  return changes
}
