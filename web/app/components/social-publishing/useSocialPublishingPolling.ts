import { computed, onBeforeUnmount, onMounted, ref, watch, type ComputedRef } from 'vue'

const POLL_INTERVAL_MS = 30_000
const MAX_TIMEOUT_MS = 2_147_000_000

interface SocialPublishingPollingOptions {
  enabled: ComputedRef<boolean>
  wakeAt: ComputedRef<number | null>
  poll: (force?: boolean) => Promise<unknown>
}

export function useSocialPublishingPolling(options: SocialPublishingPollingOptions): void {
  const pageVisible = ref(true)
  const active = computed(() => pageVisible.value && options.enabled.value)
  let intervalTimer: ReturnType<typeof setInterval> | null = null
  let wakeTimer: ReturnType<typeof setTimeout> | null = null
  let disposed = false

  function stop(): void {
    if (intervalTimer) clearInterval(intervalTimer)
    if (wakeTimer) clearTimeout(wakeTimer)
    intervalTimer = null
    wakeTimer = null
  }

  function sync(): void {
    stop()
    if (disposed || !import.meta.client || !pageVisible.value) return
    if (active.value) {
      intervalTimer = setInterval(() => {
        void options.poll()
      }, POLL_INTERVAL_MS)
      return
    }
    const wakeAt = options.wakeAt.value
    if (wakeAt === null) return
    const remaining = wakeAt - Date.now()
    const delay = remaining > 0 ? Math.min(remaining, MAX_TIMEOUT_MS) : POLL_INTERVAL_MS
    wakeTimer = setTimeout(() => {
      void options.poll(true).finally(() => {
        if (!disposed) sync()
      })
    }, delay)
  }

  function onVisibilityChange(): void {
    if (!import.meta.client) return
    const wasVisible = pageVisible.value
    pageVisible.value = document.visibilityState === 'visible'
    if (!wasVisible && pageVisible.value) {
      const wakeAt = options.wakeAt.value
      if (options.enabled.value) void options.poll()
      else if (wakeAt !== null && wakeAt <= Date.now()) void options.poll(true)
    }
  }

  watch([active, options.wakeAt], sync)

  onMounted(() => {
    disposed = false
    pageVisible.value = document.visibilityState === 'visible'
    document.addEventListener('visibilitychange', onVisibilityChange)
    sync()
  })

  onBeforeUnmount(() => {
    disposed = true
    stop()
    if (import.meta.client) document.removeEventListener('visibilitychange', onVisibilityChange)
  })
}
