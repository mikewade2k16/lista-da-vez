import { defineStore } from 'pinia'
import { useAuthStore } from './auth'
import { createApiRequest } from '~/utils/api-client'
import type {
  FinanceDocumentOriginal,
  FinanceFiscalCustomer,
  FinanceFiscalCustomerInput,
  FinanceFiscalCustomerVersion,
  FinanceReceiptPage,
  FinanceRetentionEvent,
  FinanceRetentionSettings,
} from '../../layers/finance/types/records'

// Comprovantes, originais, retenção e clientes. Sem cache local: cada tela relê do
// back depois de gravar (fonte única no banco). A conta vai explícita no header.
export const useFinanceRecordsStore = defineStore('finance-records', () => {
  const auth = useAuthStore()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
  const call = async <T>(
    accountId: string,
    path: string,
    options: Record<string, unknown> = {},
  ) => {
    const response = (await api(path, {
      ...options,
      headers: { 'X-Account-Id': accountId },
      dedupe: false,
      skipLoadingIndicator: true,
    })) as { data: T }
    return response.data
  }
  const id = (value: string) => encodeURIComponent(value)
  return {
    listReceipts: (accountId: string, page = 1, signal?: AbortSignal) =>
      call<FinanceReceiptPage>(accountId, `/v1/finance/receipts?page=${page}&limit=30`, { signal }),
    unlinkReceipt: (accountId: string, documentId: string) =>
      call<{ unlinked: boolean }>(
        accountId,
        `/v1/finance/documents/${id(documentId)}/receipt-link`,
        {
          method: 'DELETE',
        },
      ),
    removeOriginal: (accountId: string, documentId: string) =>
      call<FinanceDocumentOriginal>(accountId, `/v1/finance/documents/${id(documentId)}/original`, {
        method: 'DELETE',
      }),
    async downloadOriginal(accountId: string, documentId: string): Promise<Blob> {
      return (await api(`/v1/finance/documents/${id(documentId)}/original`, {
        headers: { 'X-Account-Id': accountId },
        responseType: 'blob',
        dedupe: false,
        skipLoadingIndicator: true,
      })) as Blob
    },
    getRetention: (accountId: string, signal?: AbortSignal) =>
      call<FinanceRetentionSettings>(accountId, '/v1/finance/retention', { signal }),
    retentionHistory: (accountId: string) =>
      call<FinanceRetentionEvent[]>(accountId, '/v1/finance/retention/history'),
    updateRetention: (accountId: string, receiptOriginalDays: number, expectedVersion: number) =>
      call<FinanceRetentionSettings>(accountId, '/v1/finance/retention', {
        method: 'PUT',
        body: { receiptOriginalDays, expectedVersion },
      }),
    listCustomers: (accountId: string, signal?: AbortSignal) =>
      call<FinanceFiscalCustomer[]>(accountId, '/v1/finance/fiscal/customers', { signal }),
    customerVersions: (accountId: string, customerId: string) =>
      call<FinanceFiscalCustomerVersion[]>(
        accountId,
        `/v1/finance/fiscal/customers/${id(customerId)}/versions`,
      ),
    updateCustomer: (accountId: string, customerId: string, input: FinanceFiscalCustomerInput) =>
      call<FinanceFiscalCustomer>(accountId, `/v1/finance/fiscal/customers/${id(customerId)}`, {
        method: 'PUT',
        body: input,
      }),
  }
})
