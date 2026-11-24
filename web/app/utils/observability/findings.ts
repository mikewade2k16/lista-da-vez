import type { ObsFindingStatus, ObsSecurityFinding, ObsSeverity } from '~/types/observability'

// Logica pura da fila de achados (aba Seguranca): vencimento, filtros e contadores.

export type FindingFilter = 'all' | 'open' | 'overdue' | 'mine'

export const FINDING_FILTERS: { key: FindingFilter; label: string }[] = [
  { key: 'all', label: 'Todos' },
  { key: 'open', label: 'Abertos' },
  { key: 'overdue', label: 'Vencidos' },
  { key: 'mine', label: 'Meus' },
]

export const FINDING_STATUS_OPTIONS: { value: ObsFindingStatus; label: string }[] = [
  { value: 'open', label: 'Aberto' },
  { value: 'fixed', label: 'Corrigido' },
  { value: 'accepted', label: 'Risco aceito' },
]

export const NOTE_MAX = 2000
export const OWNER_MAX = 120

// Data local de hoje em YYYY-MM-DD (o prazo e uma data sem hora).
export function todayISO(now: Date = new Date()): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
}

// Vencido: aberto com prazo anterior a hoje.
export function isFindingOverdue(
  finding: Pick<ObsSecurityFinding, 'status' | 'dueDate'>,
  today: string = todayISO(),
): boolean {
  return finding.status === 'open' && Boolean(finding.dueDate) && finding.dueDate! < today
}

function sameName(a: string, b: string): boolean {
  const left = a.trim().toLocaleLowerCase('pt-BR')
  return left !== '' && left === b.trim().toLocaleLowerCase('pt-BR')
}

export function filterFindings(
  list: ObsSecurityFinding[],
  filter: FindingFilter,
  me: string,
  today: string = todayISO(),
): ObsSecurityFinding[] {
  switch (filter) {
    case 'open':
      return list.filter((f) => f.status === 'open')
    case 'overdue':
      return list.filter((f) => isFindingOverdue(f, today))
    case 'mine':
      return list.filter((f) => sameName(f.owner, me))
    default:
      return list
  }
}

export interface FindingCounters {
  open: Record<ObsSeverity, number>
  overdue: number
}

export function countFindings(
  list: ObsSecurityFinding[],
  today: string = todayISO(),
): FindingCounters {
  const open = { critical: 0, high: 0, medium: 0, low: 0 } as Record<ObsSeverity, number>
  let overdue = 0
  for (const finding of list) {
    if (finding.status !== 'open') continue
    if (finding.severity in open) open[finding.severity] += 1
    if (isFindingOverdue(finding, today)) overdue += 1
  }
  return { open, overdue }
}

// "2026-11-24" -> "24/11/2026"
export function formatDueDate(value: string | null | undefined): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value || '')
  return match ? `${match[3]}/${match[2]}/${match[1]}` : '-'
}
