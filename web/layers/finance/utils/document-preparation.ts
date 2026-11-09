import type { FinanceDocumentReview } from '../types/documents'
// Explicit projection: an export never becomes an execution ticket or copies unknown secret fields.
export function documentPreparation(review: FinanceDocumentReview) {
  const p = review.issuer
  const f = review.fields
  return {
    format: 'omni.finance.preparation.v1',
    purpose: 'review-only',
    emissionAuthorized: false,
    status: review.approvalId ? 'reviewed' : review.ready ? 'ready-for-review' : 'incomplete',
    reviewHash: review.hash,
    draftVersion: review.draftVersion,
    profileVersion: review.profileVersion,
    reviewedAt: review.approvedAt || null,
    missingFields: [...review.missingFields],
    issuer: {
      legalName: p.legalName,
      document: p.document,
      issuerType: p.issuerType,
      taxRegime: p.taxRegime,
      municipalRegistration: p.municipalRegistration,
      municipality: p.municipality,
      municipalityCode: p.municipalityCode,
      state: p.state,
      email: p.email,
      address: {
        postalCode: p.postalCode,
        street: p.street,
        number: p.number,
        complement: p.complement,
        district: p.district,
      },
    },
    customer: { name: f.customerName, document: f.customerDocument },
    service: {
      amount: f.amount,
      currency: 'BRL',
      competenceDate: f.competenceDate,
      description: f.service.serviceDescription,
      serviceCode: f.service.serviceCode,
      cnae: f.service.cnae,
      nbs: f.service.nbs,
      issRate: f.service.issRate,
      issWithheld: f.service.issWithheld,
      notes: f.service.notes,
    },
  }
}
