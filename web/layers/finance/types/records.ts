// Contratos de /v1/finance/receipts, /documents/{id}/original, /retention e
// /fiscal/customers (back/internal/modules/finance). Espelha o JSON 1:1.
import type { FinanceDocumentDraft } from './documents'

export type ReceiptLinkStatus = 'linked' | 'line_removed' | 'amount_changed'

export interface FinanceReceiptLink {
  documentId: string
  sheetId: string
  lineId: string
  kind: 'entrada' | 'saida'
  scope: 'pessoal' | 'empresarial'
  amount: string
  createdLine: boolean
  status: ReceiptLinkStatus
}

export type DocumentOriginalStatus = 'stored' | 'not_stored' | 'purged' | 'removed'

export interface FinanceDocumentOriginal {
  documentId: string
  status: DocumentOriginalStatus
  notStoredReason?: string
  fileName: string
  contentType: string
  sizeBytes: number
  storedAt?: string
  purgedAt?: string
  expiresAt?: string
}

export interface FinanceReceiptItem {
  document: FinanceDocumentDraft
  link?: FinanceReceiptLink
  sheetTitle?: string
  original?: FinanceDocumentOriginal
}

export interface FinanceReceiptPage {
  items: FinanceReceiptItem[]
  page: number
  more: boolean
}

export interface FinanceRetentionSettings {
  receiptOriginalDays: number
  version: number
  isDefault: boolean
  updatedAt?: string
  updatedBy?: string
  stored: number
  storedBytes: number
  purged: number
  notStored: number
  nextExpiry?: string
  storageAvailable: boolean
}

export interface FinanceRetentionEvent {
  version: number
  previousDays?: number
  receiptOriginalDays: number
  actorId: string
  createdAt: string
}

export interface FinanceFiscalCustomer {
  id: string
  name: string
  document: string
  serviceDescription: string
  templateId?: string
  amount: number | null
  version: number
  lastUsedAt: string
  updatedAt: string
}

export interface FinanceFiscalCustomerVersion {
  version: number
  data: {
    name?: string
    document?: string
    serviceDescription?: string
    amount?: number | null
    editedManually?: boolean
  }
  sourceDraftId?: string
  createdAt: string
}

export interface FinanceFiscalCustomerInput {
  name: string
  serviceDescription: string
  amount: number | null
  expectedVersion: number
}
