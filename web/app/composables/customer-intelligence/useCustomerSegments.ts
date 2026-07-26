import { computed, onBeforeUnmount, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useCustomerSegmentsStore } from '~/stores/customer-segments'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'
import { useCustomerIntelligenceAccess } from './useCustomerIntelligenceAccess'

export function useCustomerSegments() {
  const access = useCustomerIntelligenceAccess()
  const scope = useCustomerIntelligenceStore()
  const store = useCustomerSegmentsStore()
  const state = storeToRefs(store)

  const scopeAllowed = computed(() => access.clientScopeReady.value && access.canViewSegments.value)

  watch(
    [() => scope.scopeKey, scopeAllowed],
    ([, allowed]) => {
      store.setScope(scope.ownerAccountId, scope.clientAccountId, allowed)
      if (allowed) void store.loadWorkspace()
    },
    { immediate: true },
  )

  onBeforeUnmount(() => store.abortAll())

  return {
    ...state,
    access,
    refresh: store.loadWorkspace,
    selectSegment: store.selectSegment,
    createNew: store.createNew,
    setLocalAst: store.setLocalAst,
    discardDraft: store.discardDraft,
    saveDraft: store.saveDraft,
    versionAction: store.versionAction,
  }
}
