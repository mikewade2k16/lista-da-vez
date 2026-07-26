import { computed, toValue, watch, type MaybeRefOrGetter } from 'vue'
import { storeToRefs } from 'pinia'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'

export function useCustomerProfile(relationshipId: MaybeRefOrGetter<string>) {
  const access = useCustomerIntelligenceAccess()
  const store = useCustomerIntelligenceStore()
  const state = storeToRefs(store)
  const normalizedRelationshipId = computed(() => String(toValue(relationshipId) || '').trim())

  async function refresh(): Promise<void> {
    const id = normalizedRelationshipId.value
    if (!id || !access.clientScopeReady.value) return
    await Promise.all([
      access.canViewSubjects.value ? store.loadDeterministicProfile(id) : Promise.resolve(),
      access.canViewIntelligenceProfile.value
        ? store.loadIntelligenceProfile(id)
        : Promise.resolve(),
    ])
  }

  watch(
    [
      normalizedRelationshipId,
      () => access.clientScopeReady.value,
      () => access.canViewSubjects.value,
      () => access.canViewIntelligenceProfile.value,
    ],
    ([id, scopeReady]) => {
      if (id && scopeReady) void refresh()
    },
    { immediate: true },
  )

  return { ...state, access, refresh }
}
