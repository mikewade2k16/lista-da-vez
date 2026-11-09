import { defineStore } from 'pinia'
import { useAuthStore } from './auth'
import { createApiRequest } from '~/utils/api-client'
import type {
  FinanceDocumentDraft,
  FinanceDocumentPreparation,
  FinanceDocumentFields,
  FinanceDocumentReview,
} from '../../layers/finance/types/documents'
export const useFinanceDocumentsStore = defineStore('finance-documents', () => {
  const auth = useAuthStore()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
  return {
    async get(accountId: string, id: string): Promise<FinanceDocumentDraft> {
      const response = (await api(`/v1/finance/documents/${encodeURIComponent(id)}`, {
        headers: { 'X-Account-Id': accountId },
        dedupe: false,
        skipLoadingIndicator: true,
      })) as { data: FinanceDocumentDraft }
      return response.data
    },
    async prepare(
      accountId: string,
      draft: FinanceDocumentDraft,
    ): Promise<FinanceDocumentPreparation> {
      const response = (await api(`/v1/finance/documents/${encodeURIComponent(draft.id)}/prepare`, {
        method: 'POST',
        headers: { 'X-Account-Id': accountId },
        body: { expectedUpdatedAt: draft.updatedAt },
        timeout: 90000,
        retry: 0,
        skipLoadingIndicator: true,
      })) as { data: FinanceDocumentPreparation }
      return response.data
    },
    async review(accountId: string, id: string): Promise<FinanceDocumentReview> {
      const response = (await api(`/v1/finance/documents/${encodeURIComponent(id)}/review`, {
        headers: { 'X-Account-Id': accountId },
        skipLoadingIndicator: true,
        dedupe: false,
      })) as { data: FinanceDocumentReview }
      return response.data
    },
    async approveReview(
      accountId: string,
      id: string,
      review: FinanceDocumentReview,
      approve: boolean,
    ): Promise<FinanceDocumentReview> {
      const response = (await api(
        `/v1/finance/documents/${encodeURIComponent(id)}/review/approval`,
        {
          method: approve ? 'POST' : 'DELETE',
          headers: { 'X-Account-Id': accountId },
          skipLoadingIndicator: true,
          body: {
            reviewHash: review.hash,
            expectedUpdatedAt: review.draftVersion,
            expectedProfileUpdatedAt: review.profileVersion,
            confirmed: approve,
          },
        },
      )) as { data: FinanceDocumentReview }
      return response.data
    },
    async reidentifyParties(
      accountId: string,
      draft: FinanceDocumentDraft,
    ): Promise<FinanceDocumentDraft> {
      const response = (await api(`/v1/finance/documents/${encodeURIComponent(draft.id)}/parties`, {
        method: 'POST',
        headers: { 'X-Account-Id': accountId },
        body: { expectedUpdatedAt: draft.updatedAt },
        skipLoadingIndicator: true,
      })) as { data: FinanceDocumentDraft }
      return response.data
    },
    async list(accountId: string) {
      const response = (await api('/v1/finance/documents', {
        headers: { 'X-Account-Id': accountId },
        skipLoadingIndicator: true,
      })) as { data: FinanceDocumentDraft[] }
      return response.data
    },
    async extract(accountId: string, file: File) {
      const body = new FormData()
      body.append('file', file)
      const response = (await api('/v1/finance/documents/extract', {
        method: 'POST',
        body,
        headers: { 'X-Account-Id': accountId },
        timeout: 100000,
        retry: 0,
        skipLoadingIndicator: true,
      })) as { data: FinanceDocumentDraft }
      return response.data
    },
    async save(accountId: string, draft: FinanceDocumentDraft, fields: FinanceDocumentFields) {
      const response = (await api(`/v1/finance/documents/${encodeURIComponent(draft.id)}`, {
        method: 'PUT',
        headers: { 'X-Account-Id': accountId },
        body: { fields, expectedUpdatedAt: draft.updatedAt },
        skipLoadingIndicator: true,
      })) as { data: FinanceDocumentDraft }
      return response.data
    },
  }
})
