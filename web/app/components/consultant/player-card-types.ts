// Tipos compartilhados entre ConsultantPlayerCard.vue e ConsultantPlayerCardMetrics.vue.
// Centralizados aqui para o card e o componente de métricas usarem o MESMO contrato.

export type LiveStatusCode = 'available' | 'service' | 'queue' | 'paused' | 'assignment'

export interface PlayerCardStats {
  monthlyGoal: number
  soldValue: number
  remainingToGoal: number
  ticketAverage: number
  paScore: number
  conversionRate: number
  // Campos exclusivos do modo FULL — o grid (mini) não os envia. Opcionais para o
  // grid type-checar sem precisar preencher KPIs que o mini não mostra.
  estimatedCommission?: number
  commissionRate?: number
  conversions?: number
  nonConversions?: number
  nonClientConversions?: number
  queueJumpServices?: number
  averageDurationMs?: number
  erpOrders?: number
  soldValueSource?: string
  ticketAverageSource?: string
  paScoreSource?: string
  avgTicketGoal?: number
  paGoal?: number
  conversionGoal?: number
  cancellationRate?: number
}

export interface PlayerCardConsultant {
  id: string
  name: string
  role?: string
  storeName?: string
  // string (não a union) porque o roster/grid integrado tipa o status como string.
  liveStatusCode?: string
  liveStatusLabel?: string
}
