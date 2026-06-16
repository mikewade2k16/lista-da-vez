export type AccountStatus = 'active' | 'inactive'
export type AccountBillingMode = 'single' | 'per_store'

// Campos editáveis pelo UI — TODOS persistem em core.accounts via PATCH.
// Os campos read-only (userCount, userNicks, projectCount, projectSegments) são
// agregados pelo backend e expostos no GET; o UI mostra mas não envia patch.
export type AccountFieldKey =
  | 'name'
  | 'slug'
  | 'status'
  | 'organizationId'
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

export interface AccountModule {
  moduleId: string
  label: string
  isCore: boolean
  enabled: boolean
}

// Shape antigo (code/name/status) usado pelo UI da multiselect inline.
// Derivado em runtime a partir de AccountModule (backend) no normalizer.
export interface AccountModuleAccess {
  code: string
  name: string
  status: string
}

export interface AccountStore {
  id: string
  code: string
  name: string
  city: string
  active: boolean
  amount: number
}

export interface AccountItem {
  id: string
  slug: string
  name: string
  status: AccountStatus
  planCode: string
  // Conta-workspace da agência (core.accounts.is_agency). O backend já EXCLUI
  // contas is_agency=true da listagem /v1/admin/accounts; este campo espelha o
  // contrato e permite filtro defensivo no front. Não é um cliente.
  isAgency: boolean
  billingMode: AccountBillingMode
  monthlyPaymentAmount: number
  paymentDueDay: number | null
  webhookEnabled: boolean
  webhookKey: string
  contactPhone: string
  contactSite: string
  contactAddress: string
  logo: string
  organizationId: string
  requireUserStoreLink: boolean
  requireUserRegistration: boolean
  // Agregados read-only vindos do backend (computed em ListAccounts).
  userCount: number
  userNicks: string
  projectCount: number
  projectSegments: string
  // Listas vindas do backend (loadModulesByAccount / loadStoresByAccount).
  modules: AccountModuleAccess[]
  moduleCodes: string[]
  stores: AccountStore[]
  createdAt: string
  updatedAt: string
}

export interface AccountCreateInput {
  name: string
  slug: string
  planCode: string
  adminEmail: string
}
