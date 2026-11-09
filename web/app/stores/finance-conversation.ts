import { defineStore } from 'pinia'
import { useAuthStore } from './auth'
import { createApiRequest } from '~/utils/api-client'
export interface FinanceAccessMessage {
  evidence?: unknown[]

  id: string
  role: 'user' | 'assistant'
  text: string
  // ask/issuer/pick keep a pending question; sheet/sheetpick keep the chat's own sheet choice.
  action:
    | ''
    | 'access'
    | 'document'
    | 'profile'
    | 'prepare'
    | `ask:${string}`
    | `issuer:${string}`
    | `pick:${string}`
    | `sheet:${string}`
    | `sheetpick:${string}`
    | `receipt:${string}`
  draftId?: string
}
export interface FinanceAccessConversation {
  messages: FinanceAccessMessage[]
}
export const useFinanceConversationStore = defineStore('finance-conversation', () => {
  const auth = useAuthStore()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
  const selected = useState<{ accountId: string; sheetId: string; month: string }>(
    'assistant:finance-context',
    () => ({ accountId: '', sheetId: '', month: '' }),
  )
  async function request(
    accountId: string,
    body?: { requestId: string; text: string; draftId?: string },
  ) {
    const context =
      selected.value.accountId === accountId
        ? {
            ...(selected.value.sheetId ? { financeSheetId: selected.value.sheetId } : {}),
            ...(selected.value.month ? { month: selected.value.month } : {}),
          }
        : {}
    const out = (await api('/v1/finance/conversation', {
      method: body ? 'POST' : 'GET',
      headers: { 'X-Account-Id': accountId },
      ...(body ? { body: { ...body, ...context } } : {}),
      timeout: 90000,
      retry: 0,
      dedupe: false,
      skipLoadingIndicator: true,
    })) as { data: FinanceAccessConversation }
    return out.data
  }
  return {
    list: (id: string) => request(id),
    send: (id: string, body: { requestId: string; text: string; draftId?: string }) =>
      request(id, body),
  }
})
