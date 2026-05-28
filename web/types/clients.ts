export type ClientStatus = 'active' | 'inactive'
export type ClientBillingMode = 'single' | 'per_store'

export type ClientFieldKey =
  | 'name'
  | 'status'
  | 'userCount'
  | 'userNicks'
  | 'projectCount'
  | 'projectSegments'
  | 'billingMode'
  | 'monthlyPaymentAmount'
  | 'paymentDueDay'
  | 'logo'
  | 'webhookEnabled'
  | 'contactPhone'
  | 'contactSite'
  | 'contactAddress'
  | 'requireUserStoreLink'
  | 'requireUserRegistration'
  | 'modules'

export interface ClientStoreCharge {
  id: string
  name: string
  amount: number
}

export interface ClientModuleAccess {
  code: string
  name: string
  status: string
}

export interface ClientItem {
  id: number
  coreTenantId: string
  name: string
  status: ClientStatus
  userCount: number
  userNicks: string
  projectCount: number
  projectSegments: string
  billingMode: ClientBillingMode
  monthlyPaymentAmount: number
  paymentDueDay: string
  stores: ClientStoreCharge[]
  storesCount: number
  modules: ClientModuleAccess[]
  moduleCodes: string[]
  logo: string
  webhookEnabled: boolean
  webhookKey: string
  contactPhone: string
  contactSite: string
  contactAddress: string
  requireUserStoreLink: boolean
  requireUserRegistration: boolean
}

export interface ClientsListMeta {
  page: number
  limit: number
  total: number
  totalPages: number
  hasMore: boolean
}

export interface ClientsListResponse {
  status: 'success'
  data: ClientItem[]
  meta: ClientsListMeta
}

export interface ClientMutationResponse {
  status: 'success'
  data: ClientItem
}
