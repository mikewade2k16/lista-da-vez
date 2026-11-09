export interface FiscalProfile {
  legalName: string
  document: string
  issuerType: '' | 'mei' | 'company' | 'individual'
  taxRegime: string
  municipalRegistration: string
  municipality: string
  municipalityCode: string
  state: string
  email: string
  postalCode: string
  street: string
  number: string
  complement: string
  district: string
  portalLoginMethod: '' | 'portal' | 'govbr'
  portalUrl: string
}
export interface FiscalProfileView {
  profile: FiscalProfile
  updatedAt: string
  credentialsSet: boolean
  credentialsUpdatedAt: string
  missingFields: string[]
}
export interface FiscalTemplateFields {
  serviceDescription: string
  serviceCode: string
  cnae: string
  nbs: string
  issRate: number | null
  issWithheld: boolean | null
  notes: string
}
export interface FiscalTemplate {
  id: string
  name: string
  fields: FiscalTemplateFields
  isDefault: boolean
  updatedAt: string
}
export function emptyFiscalProfile(): FiscalProfile {
  return {
    legalName: '',
    document: '',
    issuerType: '',
    taxRegime: '',
    municipalRegistration: '',
    municipality: '',
    municipalityCode: '',
    state: '',
    email: '',
    postalCode: '',
    street: '',
    number: '',
    complement: '',
    district: '',
    portalUrl: '',
    portalLoginMethod: '',
  }
}
export function emptyFiscalTemplate() {
  return {
    name: '',
    isDefault: false,
    fields: {
      serviceDescription: '',
      serviceCode: '',
      cnae: '',
      nbs: '',
      issRate: null,
      issWithheld: null,
      notes: '',
    } as FiscalTemplateFields,
  }
}

export interface FiscalPortalCheck {
  status:
    | 'not_tested'
    | 'authenticated'
    | 'rejected'
    | 'manual_required'
    | 'unavailable'
    | 'unsupported'
  message: string
  checkedAt: string
  profileVersion: string
}

export interface FiscalPortalSession {
  status:
    | 'disconnected'
    | 'saved'
    | 'expired'
    | 'authenticated'
    | 'manual_required'
    | 'unsupported'
    | 'unavailable'
  message: string
  expiresAt: string
  verifiedAt: string
  reused: boolean
}
