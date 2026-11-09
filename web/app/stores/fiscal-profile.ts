import { defineStore } from 'pinia'
import { useAuthStore } from './auth'
import { createApiRequest } from '~/utils/api-client'
import type {
  FiscalPortalCheck,
  FiscalPortalSession,
  FiscalProfile,
  FiscalProfileView,
  FiscalTemplate,
  FiscalTemplateFields,
} from '../../layers/finance/types/fiscal'

interface Envelope<T> {
  data: T
}
interface TemplateInput {
  name: string
  fields: FiscalTemplateFields
  isDefault: boolean
  expectedUpdatedAt: string
}
const BASE = '/v1/finance/fiscal'

// Porta canônica de dados fiscais pessoais. Rascunhos e ciclo de vida da tela ficam no editor.
export const useFiscalProfileStore = defineStore('fiscal-profile', () => {
  const auth = useAuthStore()
  const runtime = useRuntimeConfig()
  const api = createApiRequest(runtime, () => auth.accessToken)
  async function request<T>(
    accountId: string,
    path: string,
    method: 'GET' | 'POST' | 'PUT' | 'DELETE' = 'GET',
    body?: object,
  ): Promise<T> {
    const response = (await api(BASE + path, {
      method,
      headers: { 'X-Account-Id': accountId },
      ...(body ? { body } : {}),
      ...(path === '/portal-session' ? { timeout: 90000, retry: 0 } : {}),
    })) as Envelope<T>
    return response.data
  }
  return {
    portalSessionStatus: (accountId: string) =>
      request<FiscalPortalSession>(accountId, '/portal-session'),
    connectPortal: (accountId: string, expectedUpdatedAt: string) =>
      request<FiscalPortalSession>(accountId, '/portal-session', 'POST', { expectedUpdatedAt }),
    portalStatus: (accountId: string) => request<FiscalPortalCheck>(accountId, '/portal-check'),
    testPortal: (accountId: string, expectedUpdatedAt: string) =>
      request<FiscalPortalCheck>(accountId, '/portal-check', 'POST', { expectedUpdatedAt }),
    getProfile: (accountId: string) => request<FiscalProfileView>(accountId, '/profile'),
    listTemplates: (accountId: string) => request<FiscalTemplate[]>(accountId, '/templates'),
    saveProfile: (accountId: string, body: { profile: FiscalProfile; expectedUpdatedAt: string }) =>
      request<FiscalProfileView>(accountId, '/profile', 'PUT', body),
    saveCredentials: (
      accountId: string,
      body: { username: string; password: string; expectedUpdatedAt: string },
    ) => request<FiscalProfileView>(accountId, '/credentials', 'PUT', body),
    deleteCredentials: (accountId: string, body: { expectedUpdatedAt: string }) =>
      request<FiscalProfileView>(accountId, '/credentials', 'DELETE', body),
    saveTemplate: (accountId: string, id: string, body: TemplateInput) =>
      request<FiscalTemplate>(
        accountId,
        `/templates${id ? `/${id}` : ''}`,
        id ? 'PUT' : 'POST',
        body,
      ),
    deleteTemplate: (accountId: string, id: string, body: { expectedUpdatedAt: string }) =>
      request<null>(accountId, `/templates/${id}`, 'DELETE', body),
  }
})
