import { computed, onBeforeUnmount, watch } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest } from '~/utils/api-client'
import { useFinanceAccountContext } from '../finance-account-context'
import { createKnowledgeController, type KnowledgeTransport } from './knowledge-controller'
export function useAccountingKnowledge(issuer: () => string) {
  const auth = useAuthStore(),
    account = useFinanceAccountContext(),
    runtime = useRuntimeConfig()
  const api = createApiRequest(runtime, () => auth.accessToken)
  const context = () => ({
    accountId: account.activeAccountId,
    actorId: auth.user?.id || '',
    issuerId: issuer(),
    modules: account.enabledModules,
    permissions: account.permissions,
  })
  const scope = computed(() => JSON.stringify(context()))
  const transport: KnowledgeTransport = async <T>(
    accountId: string,
    path: string,
    options: Parameters<KnowledgeTransport>[2],
  ) => {
    const response = (await api(path, { ...options, headers: { 'X-Account-Id': accountId } })) as {
      data: T
    }
    return response.data
  }
  const controller = createKnowledgeController(context, transport)
  watch(scope, controller.reset, { flush: 'sync' })
  onBeforeUnmount(controller.reset)
  return { ...controller, scope }
}
