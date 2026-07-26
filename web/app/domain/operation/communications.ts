export interface QueueCommunication {
  id: string
  accountId: string
  title: string
  excerpt: string
  body: string
  startsAt: string | null
  endsAt: string | null
  isPublished: boolean
  displayOrder: number
  targetsAllStores: boolean
  storeIds: string[]
  createdAt: string
  updatedAt: string
}

export interface QueueCommunicationInput {
  title: string
  excerpt: string
  body: string
  startsAt: string | null
  endsAt: string | null
  isPublished: boolean
  displayOrder: number
  targetsAllStores: boolean
  storeIds: string[]
}

export type QueueCommunicationStatus = 'active' | 'scheduled' | 'expired' | 'draft'

export function communicationStatus(
  item: Pick<QueueCommunication, 'isPublished' | 'startsAt' | 'endsAt'>,
  now = Date.now(),
): QueueCommunicationStatus {
  if (!item.isPublished) return 'draft'
  if (item.startsAt && new Date(item.startsAt).getTime() > now) return 'scheduled'
  if (item.endsAt && new Date(item.endsAt).getTime() <= now) return 'expired'
  return 'active'
}

export function communicationStatusLabel(status: QueueCommunicationStatus): string {
  return {
    active: 'Em exibição',
    scheduled: 'Agendado',
    expired: 'Encerrado',
    draft: 'Rascunho',
  }[status]
}

export function formatCommunicationDate(value: string | null | undefined): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat('pt-BR', {
    day: '2-digit',
    month: '2-digit',
    year: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

export function communicationPeriod(item: Pick<QueueCommunication, 'startsAt' | 'endsAt'>): string {
  const startsAt = formatCommunicationDate(item.startsAt)
  const endsAt = formatCommunicationDate(item.endsAt)
  if (startsAt && endsAt) return `De ${startsAt} até ${endsAt}`
  if (startsAt) return `A partir de ${startsAt}`
  if (endsAt) return `Vigência até ${endsAt}`
  return 'Sem prazo de exibição'
}
