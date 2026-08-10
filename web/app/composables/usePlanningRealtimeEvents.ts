import { readonly, ref } from 'vue'

interface PlanningRealtimeEvent {
  storeId: string
  action: string
  receivedAt: number
}

const event = ref<PlanningRealtimeEvent | null>(null)
const localMutationUntilByStore = new Map<string, number>()

export function suppressPlanningRealtimeEcho(storeId: string, durationMs = 2000): void {
  const normalizedStoreId = storeId.trim()
  if (!normalizedStoreId) return
  localMutationUntilByStore.set(normalizedStoreId, Date.now() + durationMs)
}

export function isPlanningRealtimeEchoSuppressed(storeId: string, now = Date.now()): boolean {
  const normalizedStoreId = storeId.trim()
  const suppressedUntil = localMutationUntilByStore.get(normalizedStoreId) || 0
  if (suppressedUntil <= now) {
    localMutationUntilByStore.delete(normalizedStoreId)
    return false
  }
  return true
}

export function notifyPlanningRealtimeUpdate(storeId: string, action: string) {
  event.value = { storeId, action, receivedAt: Date.now() }
}

export function usePlanningRealtimeEvents() {
  return { event: readonly(event) }
}
