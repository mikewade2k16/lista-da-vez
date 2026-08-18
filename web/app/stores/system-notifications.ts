import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { useContentOperations } from '~/composables/useContentOperations'
import {
  buildSystemNotifications,
  type InAppNotificationInput,
  type SystemNotificationItem,
} from '~/domain/system-notifications/system-notifications'
import { useAlertsStore } from '~/stores/alerts'
import { useAuthStore } from '~/stores/auth'
import { useFeedbackStore } from '~/stores/feedback'
import { useUiStore } from '~/stores/ui'
import { createApiRequest } from '~/utils/api-client'
import { useCoreAccountStore } from '../../layers/core/stores/account'

const POLLING_INTERVAL_MS = 60_000
const CONTENT_REFRESH_INTERVAL_MS = 5 * 60_000

export const useSystemNotificationsStore = defineStore('system-notifications', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()
  const feedbackStore = useFeedbackStore()
  const alertsStore = useAlertsStore()
  const ui = useUiStore()
  const contentOperations = useContentOperations()
  const api = createApiRequest(runtimeConfig, () => auth.accessToken)

  const inAppNotifications = ref<InAppNotificationInput[]>([])
  const loading = ref(false)
  const feedbackSyncCursor = ref('')
  const lastContentRefreshAt = ref(0)
  let timer: number | null = null
  let subscribers = 0
  let refreshPromise: Promise<void> | null = null

  const enabled = computed(() => auth.isAuthenticated)
  const canManageFeedback = computed(() => auth.allowedWorkspaces.includes('feedback'))
  const feedbackPath = computed(() => (canManageFeedback.value ? '/suporte' : '/meus-chamados'))
  const feedbackCollection = computed(() =>
    canManageFeedback.value ? feedbackStore.feedbacks : feedbackStore.myFeedbacks,
  )
  const storeLabels = computed(() =>
    Object.fromEntries(
      auth.storeContext.map((store) => [
        String(store?.id || '').trim(),
        String(store?.name || store?.code || store?.city || 'Loja não informada').trim(),
      ]),
    ),
  )
  const items = computed(() =>
    buildSystemNotifications({
      inApp: inAppNotifications.value,
      feedback: feedbackCollection.value,
      feedbackPath: feedbackPath.value,
      feedbackManager: canManageFeedback.value,
      storeLabels: storeLabels.value,
      contentBrief: contentOperations.brief.value,
      operationalAlerts: alertsStore.items,
    }),
  )
  const count = computed(() => items.value.length)

  function isVisible(): boolean {
    return !import.meta.client || document.visibilityState === 'visible'
  }

  async function loadInAppNotifications(): Promise<void> {
    if (!accountStore.enabledModules.includes('notifications')) {
      inAppNotifications.value = []
      return
    }
    try {
      const response = (await api('/v1/notifications?limit=50', {
        method: 'GET',
        skipLoadingIndicator: true,
        dedupe: false,
      })) as { notifications?: InAppNotificationInput[] }
      inAppNotifications.value = Array.isArray(response.notifications) ? response.notifications : []
    } catch {
      // Notifications e modulo opcional e pode estar sem permissao nesta conta.
    }
  }

  async function loadFeedbackNotifications(): Promise<void> {
    if (!accountStore.enabledModules.includes('queue') || !auth.user?.id) return
    const options = feedbackSyncCursor.value ? { since: feedbackSyncCursor.value } : undefined
    const result = canManageFeedback.value
      ? await feedbackStore.fetchFeedbacks(options)
      : await feedbackStore.fetchMyFeedbacks(options)
    if (result.ok && result.cursor) feedbackSyncCursor.value = result.cursor
  }

  async function loadOperationalAlerts(): Promise<void> {
    if (
      !accountStore.enabledModules.includes('queue') ||
      !auth.allowedWorkspaces.includes('alertas')
    ) {
      return
    }
    try {
      await alertsStore.refreshAlerts()
    } catch {
      // A store ja preserva o erro da fonte; o sino continua com as demais.
    }
  }

  async function loadContentOperations(force = false): Promise<void> {
    if (!contentOperations.enabled.value) return
    const now = Date.now()
    if (!force && now - lastContentRefreshAt.value < CONTENT_REFRESH_INTERVAL_MS) return
    await contentOperations.refresh({ announce: lastContentRefreshAt.value === 0 })
    lastContentRefreshAt.value = now
  }

  async function refresh(options: { forceContent?: boolean } = {}): Promise<void> {
    if (!enabled.value || !isVisible()) return
    if (refreshPromise) return refreshPromise
    loading.value = true
    refreshPromise = Promise.allSettled([
      loadInAppNotifications(),
      loadFeedbackNotifications(),
      loadOperationalAlerts(),
      loadContentOperations(Boolean(options.forceContent)),
    ])
      .then(() => undefined)
      .finally(() => {
        loading.value = false
        refreshPromise = null
      })
    return refreshPromise
  }

  async function markRead(item: SystemNotificationItem): Promise<boolean> {
    if (!item.dismissible) return true
    if (item.bucket === 'feedback') {
      const result = await feedbackStore.markFeedbackAsRead(item.sourceId)
      if (!result.ok) ui.error(result.message || 'Não foi possível marcar o chamado como lido.')
      return result.ok
    }
    if (item.bucket !== 'system') return true
    try {
      await api(`/v1/notifications/${encodeURIComponent(item.sourceId)}/read`, {
        method: 'POST',
        skipLoadingIndicator: true,
      })
      inAppNotifications.value = inAppNotifications.value.filter(
        (notification) => String(notification.id || '').trim() !== item.sourceId,
      )
      return true
    } catch {
      ui.error('Não foi possível marcar a notificação como lida.')
      return false
    }
  }

  function clear(): void {
    inAppNotifications.value = []
    feedbackSyncCursor.value = ''
    lastContentRefreshAt.value = 0
  }

  function handleVisibilityChange(): void {
    if (isVisible()) void refresh({ forceContent: true })
  }

  function start(): void {
    subscribers += 1
    if (!import.meta.client || timer) return
    void refresh({ forceContent: true })
    timer = window.setInterval(() => void refresh(), POLLING_INTERVAL_MS)
    document.addEventListener('visibilitychange', handleVisibilityChange)
    window.addEventListener('focus', handleVisibilityChange)
  }

  function stop(): void {
    subscribers = Math.max(0, subscribers - 1)
    if (!import.meta.client || subscribers > 0) return
    if (timer) window.clearInterval(timer)
    timer = null
    document.removeEventListener('visibilitychange', handleVisibilityChange)
    window.removeEventListener('focus', handleVisibilityChange)
  }

  watch(
    () => [auth.isAuthenticated, accountStore.activeAccountId] as const,
    ([isAuthenticated], previous) => {
      if (!isAuthenticated) {
        clear()
        return
      }
      if (!previous || previous[1] !== accountStore.activeAccountId) {
        clear()
        if (subscribers > 0) void refresh({ forceContent: true })
      }
    },
  )

  return { items, count, loading, enabled, refresh, markRead, start, stop }
})
