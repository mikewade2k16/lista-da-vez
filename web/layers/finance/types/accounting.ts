export interface AccountingContext {
  accountId: string
  actorId: string
  modules: readonly string[]
  permissions: readonly string[]
}
export interface IssuerProfile {
  legalName: string
  document: string | null
  issuerType: string | null
  taxRegime: string | null
  municipality: string | null
  municipalityCode: string | null
  state: string | null
  municipalRegistration: string | null
  stateRegistration: string | null
}
export interface Issuer {
  id: string
  version: number
  profileVersion: number
  profile: IssuerProfile
  emissionAvailable: boolean
}
export interface AccountingAnswer {
  value: string | null
  confirmation: 'unknown' | 'declared' | 'confirmed'
  origin: string | null
  evidenceRef: string | null
}
export interface AccountingPending {
  field: string
  code: string
  blocks: string[]
}
export interface Diagnosis {
  id: string
  version: number
  profileVersion: number
  intent: 'service' | 'product' | 'mixed'
  competenceDate: string | null
  answers: Record<string, AccountingAnswer>
  pending: AccountingPending[]
  nextQuestionId?: string
}
export interface AccountingAddress {
  street: string
  number: string
  district: string
  postalCode: string
  municipalityCode: string
  state: string
}
export interface AccountingParty {
  name: string | null
  document: string | null
  address: AccountingAddress | null
}
export interface AccountingFields {
  competenceDate: string | null
  amount: string | null
  service?: {
    description: string | null
    serviceCode: string | null
    cnae: string | null
    nbs: string | null
    issRate: string | null
    issWithheld: boolean | null
    location: string | null
  }
  customer?: AccountingParty
  items?: Array<{
    description: string
    unit: string
    quantity: string
    unitPrice: string
    ncm: string | null
    cfop: string | null
    cst: string | null
    csosn: string | null
    origin: string | null
  }>
  recipient?: AccountingParty
  operation?: { nature: string; purpose: string; originState: string; destinationState: string }
}
export interface AccountingDocument {
  id: string
  issuerId: string
  kind: 'nfse' | 'nfe'
  diagnosisId?: string
  diagnosisVersion?: number
  state: string
  version: number
  profileVersion: number
  fields: AccountingFields
}
export interface AccountingReview {
  document: AccountingDocument
  profile: IssuerProfile
  snapshotHash: string
  pending: string[]
  emissionAvailable: boolean
  connectorId: string
  environment: string
}
export type AccountingTransport = <T>(
  accountId: string,
  path: string,
  options: {
    method?: 'GET' | 'POST' | 'PUT' | 'PATCH'
    body?: unknown
    signal: AbortSignal
    key?: string
  },
) => Promise<T>
export function emptyIssuerProfile(): IssuerProfile {
  return {
    legalName: '',
    document: null,
    issuerType: null,
    taxRegime: null,
    municipality: null,
    municipalityCode: null,
    state: null,
    municipalRegistration: null,
    stateRegistration: null,
  }
}
export function emptyAccountingFields(kind: 'nfse' | 'nfe'): AccountingFields {
  const common = { competenceDate: null, amount: null }
  if (kind === 'nfe')
    return {
      ...common,
      items: [
        {
          description: '',
          unit: '',
          quantity: '',
          unitPrice: '',
          ncm: null,
          cfop: null,
          cst: null,
          csosn: null,
          origin: null,
        },
      ],
      recipient: { name: null, document: null, address: null },
      operation: { nature: '', purpose: '', originState: '', destinationState: '' },
    }
  return {
    ...common,
    service: {
      description: null,
      serviceCode: null,
      cnae: null,
      nbs: null,
      issRate: null,
      issWithheld: null,
      location: null,
    },
    customer: { name: null, document: null, address: null },
  }
}
