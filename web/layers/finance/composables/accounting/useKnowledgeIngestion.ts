import { computed, watch, onBeforeUnmount } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { useFinanceAccountContext } from '../finance-account-context'
import { createApiRequest } from '~/utils/api-client'
import { createIngestionController, type IngestionTransport } from './ingestion-controller'
export function useKnowledgeIngestion(issuer: () => string, base: () => string) {
  const auth = useAuthStore(),
    account = useFinanceAccountContext(),
    runtime = useRuntimeConfig(),
    api = createApiRequest(runtime, () => auth.accessToken)
  const context = () => ({
    accountId: account.activeAccountId,
    actorId: auth.user?.id || '',
    issuerId: issuer(),
    baseId: base(),
    modules: account.enabledModules,
    permissions: account.permissions,
  })
  const transport: IngestionTransport = async <T>(
    id: string,
    path: string,
    options: Parameters<IngestionTransport>[2],
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
  const controller = createIngestionController(context, transport),
    scope = computed(() => JSON.stringify(context()))
  watch(scope, controller.reset, { flush: 'sync' })
  onBeforeUnmount(controller.reset)
  return controller
}
