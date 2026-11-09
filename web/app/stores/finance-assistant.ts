import type { PlusExplanationConfig } from '../../layers/finance/types/plus-explanation'
import { defineStore } from 'pinia'
import { useAuthStore } from './auth'
import { createApiRequest } from '~/utils/api-client'
import { fetchAICredentials, ASSISTANT_AI_CREDENTIALS_PATH } from '~/domain/omnichannel/config-api'
export interface FinanceAssistantConfig {
  plus?: { enabled: boolean; toolPolicyVersion: number; explanation?: PlusExplanationConfig }
  enabled: boolean
  credentialId: string
  model: string
  instructions: string
  temperature: number
  historyWindow: number
  updatedAt: string
}
export const useFinanceAssistantStore = defineStore('finance-assistant', () => {
  const auth = useAuthStore()
  const runtime = useRuntimeConfig()
  const api = createApiRequest(runtime, () => auth.accessToken)
  return {
    async get(accountId: string): Promise<FinanceAssistantConfig> {
      const response = (await api('/v1/finance/assistant/config', {
        headers: { 'X-Account-Id': accountId },
      })) as { data: FinanceAssistantConfig }
      return response.data
    },
    async save(accountId: string, config: FinanceAssistantConfig): Promise<FinanceAssistantConfig> {
      const response = (await api('/v1/finance/assistant/config', {
        method: 'PUT',
        headers: { 'X-Account-Id': accountId },
        body: { config, expectedUpdatedAt: config.updatedAt },
      })) as { data: FinanceAssistantConfig }
      return response.data
    },
    listCredentials: (accountId: string) =>
      fetchAICredentials(api, {
        basePath: ASSISTANT_AI_CREDENTIALS_PATH,
        headers: { 'X-Account-Id': accountId },
      }),
  }
})
