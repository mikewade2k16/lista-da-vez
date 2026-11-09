import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest } from '~/utils/api-client'
import { useFinanceAccountContext } from './finance-account-context'
import { createCrowPlusController } from './crow-plus-controller'
import type { CrowPlusTransport } from './crow-plus-controller'
export function useCrowPlus() {
  const account = useFinanceAccountContext(),
    auth = useAuthStore(),
    runtime = useRuntimeConfig()
  const selected = useState<{ accountId: string; sheetId: string; month: string }>(
    'assistant:finance-context',
    () => ({ accountId: '', sheetId: '', month: '' }),
  )
  const api = createApiRequest(runtime, () => auth.accessToken)
  const transport: CrowPlusTransport = async <T>(
    accountId: string,
    path: string,
    options: Parameters<CrowPlusTransport>[2],
  ) =>
    (await api(path, {
      method: options.method,
      body: options.body,
      signal: options.signal,
      retry: 0,
      dedupe: false,
      headers: {
        'X-Account-Id': accountId,
        ...(options.key ? { 'Idempotency-Key': options.key } : {}),
      },
    })) as T
  const controller = createCrowPlusController(
    () => ({
      accountId: account.activeAccountId,
      actorId: auth.user?.id || '',
      modules: account.enabledModules,
      permissions: account.permissions,
      sheetId: selected.value.accountId === account.activeAccountId ? selected.value.sheetId : '',
      month: selected.value.accountId === account.activeAccountId ? selected.value.month : '',
    }),
    transport,
    () => crypto.randomUUID(),
  )
  const scope = computed(
    () =>
      `${account.activeAccountId}:${auth.user?.id}:${account.enabledModules.join(',')}:${account.permissions.join(',')}`,
  )
  let mounted = false
  onMounted(() => {
    mounted = true
    void controller.load()
  })
  watch(
    scope,
    () => {
      controller.reset()
      if (mounted) void controller.load()
    },
    { flush: 'sync' },
  )
  onBeforeUnmount(() => controller.reset())
  return { ...controller, accountId: computed(() => account.activeAccountId) }
}
