import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { fetchContentOperationsBrief } from '~/domain/content-operations/content-operations-api'
import type { ContentOperationsBrief } from '~/domain/content-operations/content-operations-api'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { useCoreAccountStore } from '../../layers/core/stores/account'

export function useContentOperations() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()
  const ui = useUiStore()
  const { isAuthenticated } = storeToRefs(auth)
  const api = createApiRequest(runtimeConfig, () => auth.accessToken)
  const brief = useState<ContentOperationsBrief | null>('content-operations-brief', () => null)
  const loading = useState<boolean>('content-operations-loading', () => false)
  const error = useState<string>('content-operations-error', () => '')
  const notifiedKeys = useState<Record<string, boolean>>('content-operations-notified', () => ({}))

  const enabled = computed(() => {
    const owner = auth.role === 'platform_admin' || auth.role === 'owner'
    const permissions = new Set(auth.effectivePermissionKeys)
    const canReadSources =
      owner ||
      ((permissions.has('tasks.tasks.view') || permissions.has('tasks.client_view')) &&
        permissions.has('calendar.view'))
    return (
      isAuthenticated.value &&
      canReadSources &&
      accountStore.enabledModules.includes('tasks') &&
      accountStore.enabledModules.includes('calendar') &&
      accountStore.enabledModules.includes('content_operations')
    )
  })

  async function refresh(options: { announce?: boolean } = {}) {
    if (!enabled.value || !accountStore.activeAccountId) {
      brief.value = null
      return
    }
    loading.value = true
    error.value = ''
    try {
      const next = await fetchContentOperationsBrief(api)
      brief.value = next
      const key = `${accountStore.activeAccountId}:${next.today}:${next.mode}`
      if (options.announce && next.counts.total > 0 && !notifiedKeys.value[key]) {
        notifiedKeys.value = { ...notifiedKeys.value, [key]: true }
        ui.notify({
          type: next.counts.critical > 0 ? 'warning' : 'info',
          title: next.headline,
          message: `${next.counts.total} lembrete${next.counts.total === 1 ? '' : 's'} para organizar. Veja as notificações do sistema.`,
          duration: 9000,
        })
      }
    } catch (requestError: unknown) {
      error.value = getApiErrorMessage(requestError, 'Não foi possível calcular os alertas.')
    } finally {
      loading.value = false
    }
  }

  return { brief, loading, error, enabled, refresh }
}
