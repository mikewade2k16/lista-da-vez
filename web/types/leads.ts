export interface LeadClientOption {
  id: number
  name: string
}

export interface LeadItem {
  id: number
  clientId: number
  sourceLeadId: number | null
  source: string
  nome: string
  email: string
  telefone: string
  page: string
  cupom: string
  consent: boolean
  consentLabel: string
  trackingData: string
  payloadJson: string
  formattedDate: string
  createdAt: string
  clientName: string
}

export interface LeadsListMeta {
  page: number
  limit: number
  total: number
  totalPages: number
  hasMore: boolean
}

export interface LeadsListFilters {
  clients: LeadClientOption[]
}

export interface LeadsListResponse {
  status: 'success'
  data: LeadItem[]
  meta: LeadsListMeta
  filters: LeadsListFilters
}
