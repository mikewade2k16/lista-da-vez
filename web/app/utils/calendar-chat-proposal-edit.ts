import type {
  CalendarChatProposalFields,
  CalendarChatStoredProposal,
} from '~/domain/calendar/calendar-chat-api'

// Edit inline dos campos propostos no cartao do chat (WAVE 9). Alcance: ajustar os campos QUE a IA
// propos (nao adiciona campos novos). Helpers PUROS; o estado (quais propostas estao em edicao + o
// rascunho editavel) vive no CalendarChatMessage.

export type EditFieldKind = 'text' | 'textarea' | 'mode'
export interface EditField {
  key: string
  label: string
  kind: EditFieldKind
  path: string
}

const PROFILE_LABELS: Record<string, string> = {
  segment: 'Segmento',
  positioning: 'Posicionamento',
  description: 'Descrição',
  history: 'História',
  siteUrl: 'Site',
  instagram: 'Instagram',
  address: 'Endereço',
  objectives: 'Objetivos',
  brandVoice: 'Tom de voz',
  audience: 'Público-alvo',
  offer: 'Oferta',
  pillars: 'Pilares',
  cadence: 'Cadência',
  restrictions: 'Restrições',
  performance: 'Performance',
  assets: 'Assets',
}
const PROFILE_STABLE = [
  'segment',
  'positioning',
  'description',
  'history',
  'siteUrl',
  'instagram',
  'address',
  'objectives',
  'brandVoice',
]
const PROFILE_EXTRA = [
  'audience',
  'offer',
  'pillars',
  'cadence',
  'restrictions',
  'performance',
  'assets',
]
const PROFILE_MULTILINE = new Set([
  'description',
  'history',
  'objectives',
  'brandVoice',
  'positioning',
  'audience',
  'offer',
  'pillars',
  'restrictions',
  'performance',
  'assets',
])
const EVENT_FIELDS: { key: string; label: string; multiline?: boolean }[] = [
  { key: 'title', label: 'Título' },
  { key: 'date', label: 'Data' },
  { key: 'time', label: 'Horário' },
  { key: 'dueDate', label: 'Prazo' },
  { key: 'type', label: 'Tipo' },
  { key: 'status', label: 'Status' },
  { key: 'priority', label: 'Prioridade' },
  { key: 'description', label: 'Descrição', multiline: true },
]

function has(obj: Record<string, unknown>, key: string): boolean {
  return String(obj[key] ?? '').trim() !== ''
}

// editableFields lista os campos QUE a proposta setou (o que a IA propos), para editar inline.
export function editableFields(proposal: CalendarChatStoredProposal): EditField[] {
  const f = (proposal.fields || {}) as Record<string, unknown>
  const out: EditField[] = []
  if (proposal.kind === 'clientProfile') {
    const prof = (f.profile || {}) as Record<string, unknown>
    for (const key of PROFILE_STABLE) {
      if (has(prof, key)) {
        out.push({
          key,
          label: PROFILE_LABELS[key] || key,
          kind: PROFILE_MULTILINE.has(key) ? 'textarea' : 'text',
          path: `profile.${key}`,
        })
      }
    }
    const extra = (prof.extra || {}) as Record<string, unknown>
    for (const key of PROFILE_EXTRA) {
      if (has(extra, key)) {
        out.push({
          key,
          label: PROFILE_LABELS[key] || key,
          kind: 'textarea',
          path: `profile.extra.${key}`,
        })
      }
    }
    return out
  }
  if (proposal.kind === 'note') {
    out.push({ key: 'content', label: 'Conteúdo', kind: 'textarea', path: 'note.content' })
    if (proposal.action !== 'delete') {
      out.push({ key: 'mode', label: 'Modo', kind: 'mode', path: 'note.mode' })
    }
    return out
  }
  for (const def of EVENT_FIELDS) {
    if (has(f, def.key)) {
      out.push({
        key: def.key,
        label: def.label,
        kind: def.multiline ? 'textarea' : 'text',
        path: def.key,
      })
    }
  }
  return out
}

// getFieldByPath le um valor aninhado por caminho ("profile.extra.offer"); ausente => ''.
export function getFieldByPath(
  fields: CalendarChatProposalFields | undefined,
  path: string,
): string {
  let cur: unknown = fields
  for (const seg of path.split('.')) {
    cur = cur && typeof cur === 'object' ? (cur as Record<string, unknown>)[seg] : undefined
  }
  return String(cur ?? '')
}

// setFieldByPath grava um valor aninhado por caminho (cria os objetos intermediarios).
export function setFieldByPath(
  fields: CalendarChatProposalFields | undefined,
  path: string,
  value: string,
): void {
  if (!fields) return
  const segs = path.split('.')
  let cur = fields as unknown as Record<string, unknown>
  for (let i = 0; i < segs.length - 1; i++) {
    const key = segs[i]!
    if (!cur[key] || typeof cur[key] !== 'object') cur[key] = {}
    cur = cur[key] as Record<string, unknown>
  }
  cur[segs[segs.length - 1]!] = value
}
