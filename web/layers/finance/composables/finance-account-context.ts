import { hasInjectionContext, inject, type InjectionKey, type Ref } from 'vue'
import { useCoreAccountStore, type AccountContext } from '../../core/stores/account'
export interface FinanceChatAccount {
  accountId: Ref<string>
  context: Ref<AccountContext | null>
}
export const FINANCE_CHAT_ACCOUNT: InjectionKey<FinanceChatAccount> = Symbol('finance-chat-account')
export function useFinanceAccountContext() {
  const root = useCoreAccountStore()
  const scoped = hasInjectionContext() ? inject(FINANCE_CHAT_ACCOUNT, null) : null
  if (!scoped) return root
  return {
    get activeAccountId() {
      return scoped.accountId.value
    },
    get activeAccount() {
      return scoped.context.value?.account || null
    },
    get permissions() {
      return scoped.context.value?.permissions || []
    },
    get enabledModules() {
      return scoped.context.value?.account.modules || []
    },
  }
}
