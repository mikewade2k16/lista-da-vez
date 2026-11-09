import { expect, it } from 'vitest'
import { emptyFiscalProfile, emptyFiscalTemplate } from '../types/fiscal'
import type { FinanceDocumentReview } from '../types/documents'
import { documentPreparation } from './document-preparation'
it('exports only preparation data, never unknown secrets or execution permission', () => {
  const review: FinanceDocumentReview = {
    hash: 'hash',
    draftVersion: 'v1',
    profileVersion: 'p1',
    issuer: Object.assign(emptyFiscalProfile(), {
      password: 'secret-password',
      ciphertext: 'secret-ciphertext',
    }),
    fields: {
      documentType: 'comprovante',
      amount: 12,
      paymentDate: '2026-10-31',
      payerDocument: '',
      payeeDocument: '',
      payerName: 'Receipt party',
      payeeName: 'Receipt party',
      customerName: 'Confirmed customer',
      customerDocument: '12345678901',
      competenceDate: '2026-10-31',
      service: emptyFiscalTemplate().fields,
      templateId: '',
    },
    missingFields: [],
    ready: true,
    approvalId: 'review-id',
    approvedAt: 'date',
    emissionAvailable: false,
  }
  const payload = documentPreparation(review)
  const json = JSON.stringify(payload)
  expect(payload.status).toBe('reviewed')
  expect(payload.emissionAuthorized).toBe(false)
  expect(payload.customer.name).toBe('Confirmed customer')
  expect(payload.service.issRate).toBeNull()
  expect(payload.service.issWithheld).toBeNull()
  expect(json).not.toContain('secret-')
  expect(json).not.toContain('Receipt party')
  expect(json).not.toContain('portalUrl')
})
