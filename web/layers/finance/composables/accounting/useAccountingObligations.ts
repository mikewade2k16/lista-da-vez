import { computed, watch, onBeforeUnmount } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { useFinanceAccountContext } from '../finance-account-context'
import { createApiRequest } from '~/utils/api-client'
import { createObligationsController, type ObligationTransport } from './obligations-controller'
export function useAccountingObligations(issuer: () => string) {
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
  const transport: ObligationTransport = async <T>(
    id: string,
    path: string,
    options: Parameters<ObligationTransport>[2],
  ) => {
    const response = (await api(path, {
      method: options.method,
      body: options.body,
      signal: options.signal,
      retry: 0,
      headers: { 'X-Account-Id': id, ...(options.key ? { 'Idempotency-Key': options.key } : {}) },
    })) as { data: T }
    return response.data
  }
  const controller = createObligationsController(context, transport)
  const scope = computed(() => JSON.stringify(context()))
  watch(scope, controller.reset, { flush: 'sync' })
  onBeforeUnmount(controller.reset)
  return controller
}
