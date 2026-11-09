import type { FiscalTemplateFields } from './fiscal'
export interface FinanceDocumentFields {
  documentType: 'documento' | 'comprovante'
  amount: number | null
  paymentDate: string
  payerDocument: string
  payeeDocument: string
  payerName: string
  payeeName: string
  customerName: string
  customerDocument: string
  competenceDate: string
  service: FiscalTemplateFields
  templateId: string
}
export interface FinanceDocumentDraft {
  id: string
  sourceName: string
  extractedText: string
  fields: FinanceDocumentFields
  missingFields: string[]
  warnings: string[]
  updatedAt: string
}
export interface FinanceDocumentReview {
  hash: string
  draftVersion: string
  profileVersion: string
  issuer: import('./fiscal').FiscalProfile
  fields: FinanceDocumentFields
  missingFields: string[]
  ready: boolean
  approvalId: string
  approvedAt: string
  emissionAvailable: boolean
}

export interface FinanceDocumentPreparation {
  draft: FinanceDocumentDraft
  review: FinanceDocumentReview
  session: import('./fiscal').FiscalPortalSession
}
