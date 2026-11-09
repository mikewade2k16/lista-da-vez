import { computed, onBeforeUnmount, onMounted, onActivated, onDeactivated, watch } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest } from '~/utils/api-client'
import { useFinanceAccountContext } from '../finance-account-context'
import { confirmFiscalAction } from '../../utils/fiscal-confirm'
import { createAccountingController } from './accounting-controller'
import type { AccountingContext, AccountingTransport } from '../../types/accounting'

export function useAccountingWorkspace() {
  const auth = useAuthStore(),
    account = useFinanceAccountContext()
  const runtime = useRuntimeConfig(),
    api = createApiRequest(runtime, () => auth.accessToken)
  const context = (): AccountingContext => ({
    accountId: account.activeAccountId,
    actorId: auth.user?.id || '',
    permissions: account.permissions,
    modules: account.enabledModules,
  })
  const transport: AccountingTransport = async <T>(
    accountId: string,
    path: string,
    options: Parameters<AccountingTransport>[2],
  ) => {
    const response = (await api(path, {
      method: options.method,
      body: options.body,
      signal: options.signal,
      headers: {
        'X-Account-Id': accountId,
        ...(options.key ? { 'Idempotency-Key': options.key } : {}),
      },
    })) as { data: T }
    return response.data
  }
  const controller = createAccountingController(
    context,
    transport,
    (message) =>
      confirmFiscalAction(
        message,
        () => `${scope.value}:${controller.state.issuer?.id}:${controller.state.document?.version}`,
      ),
    () => crypto.randomUUID(),
  )
  const scope = computed(
    () =>
      `${account.activeAccountId}:${auth.user?.id}:${account.enabledModules.join(',')}:${account.permissions.join(',')}`,
  )
  let mounted = false
  let active = true
  onMounted(() => {
    mounted = true
    void controller.load()
  })
  onActivated(() => {
    active = true
    if (mounted && !controller.state.issuers.length && !controller.state.issuer)
      void controller.load()
  })
  onDeactivated(() => {
    active = false
  })
  watch(
    scope,
    () => {
      controller.reset()
      if (mounted && active) void controller.load()
    },
    { flush: 'sync' },
  )
  onBeforeUnmount(controller.reset)
  return { ...controller, accountId: computed(() => account.activeAccountId) }
}
