export type LeadStatus = 'new' | 'contacted' | 'qualified' | 'lost'

export type LeadFieldKey = 'nome' | 'email' | 'telefone' | 'status' | 'notes'

export interface LeadItem {
  id: string
  accountId: string
  sourceId: string
  sourceLabel: string
  nome: string
  email: string
  telefone: string
  page: string
  cupom: string
  consent: boolean
  consentLabel: string
  trackingData: string
  payloadRaw: string
  status: LeadStatus
  notes: string
  createdAt: string
  updatedAt: string
}

export interface LeadCreateInput {
  nome: string
  email: string
  telefone: string
  page: string
  cupom: string
  consent: boolean
  consentLabel: string
  sourceLabel: string
  notes: string
}

export interface LeadsListResponse {
  leads: LeadItem[]
  total: number
  page: number
  perPage: number
}
