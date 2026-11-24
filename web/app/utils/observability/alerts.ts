// Logica pura do reconhecimento e silencio de alertas (aba Alertas e lista da Saude).

export const SILENCE_OPTIONS = [
  { hours: 1, label: '1 h' },
  { hours: 8, label: '8 h' },
  { hours: 24, label: '24 h' },
  { hours: 168, label: '7 dias' },
] as const

export interface AlertControlFields {
  acknowledgedAt?: string | null
  acknowledgedBy?: string | null
  silencedUntil?: string | null
  silencedBy?: string | null
}

export function isSilenced(alert: AlertControlFields, now: number = Date.now()): boolean {
  if (!alert.silencedUntil) return false
  const until = new Date(alert.silencedUntil).getTime()
  return !Number.isNaN(until) && until > now
}

export function isAcknowledged(alert: AlertControlFields): boolean {
  return Boolean(alert.acknowledgedAt)
}

// "14:30" no mesmo dia; "25/11 14:30" em outro dia. Hora local do navegador.
export function formatSilenceUntil(
  iso: string | null | undefined,
  now: number = Date.now(),
): string {
  if (!iso) return '-'
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '-'
  const time = date.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })
  if (date.toDateString() === new Date(now).toDateString()) return time
  return `${date.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit' })} ${time}`
}

// Selo curto: silencio tem prioridade sobre reconhecimento (e o que muda o e-mail).
export function alertControlBadge(
  alert: AlertControlFields,
  now: number = Date.now(),
): { kind: 'silenced' | 'acknowledged'; text: string } | null {
  if (isSilenced(alert, now)) {
    return {
      kind: 'silenced',
      text: `silenciado até ${formatSilenceUntil(alert.silencedUntil, now)}`,
    }
  }
  if (isAcknowledged(alert)) return { kind: 'acknowledged', text: 'reconhecido' }
  return null
}

// Linha de detalhe: quem fez e quando. Vazia sem controle ativo.
export function alertControlDetail(alert: AlertControlFields, now: number = Date.now()): string {
  const parts: string[] = []
  if (isAcknowledged(alert)) {
    parts.push(`Reconhecido${alert.acknowledgedBy ? ` por ${alert.acknowledgedBy}` : ''}`)
  }
  if (isSilenced(alert, now)) {
    parts.push(
      `Silenciado${alert.silencedBy ? ` por ${alert.silencedBy}` : ''} até ${formatSilenceUntil(alert.silencedUntil, now)}`,
    )
  }
  return parts.join(' · ')
}
