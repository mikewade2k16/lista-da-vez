<script setup>
import { computed, watch } from 'vue'
import RankingWorkspace from '~/components/ranking/RankingWorkspace.vue'
import { storeToRefs } from 'pinia'
import { canUseAllStoresScope } from '~/domain/utils/permissions'
import { useAuthStore } from '~/stores/auth'
import { useAnalyticsStore } from '~/stores/analytics'
import { useConsultantsStore } from '~/stores/consultants'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'ranking',
  alias: ['/operacao/ranking'],
  supportsAllStoresScope: true,
})

const auth = useAuthStore()
const analyticsStore = useAnalyticsStore()
const consultantsStore = useConsultantsStore()
const { ranking: analyticsRanking, pending: analyticsPending, errorMessage: analyticsError } =
  storeToRefs(analyticsStore)
const {
  integratedRanking,
  integratedPending,
  integratedError,
} = storeToRefs(consultantsStore)
const canSeeIntegrated = computed(() => canUseAllStoresScope(auth.accessibleStoreIds))
const integratedScope = computed(() => canSeeIntegrated.value)

const report = computed(() =>
  integratedScope.value ? integratedRanking.value : analyticsRanking.value,
)
const pending = computed(() =>
  integratedScope.value ? integratedPending.value : analyticsPending.value,
)
const errorMessage = computed(() =>
  integratedScope.value ? integratedError.value : analyticsError.value,
)

watch(
  () => [integratedScope.value, auth.activeStoreId, auth.activeTenantId, auth.isAuthenticated],
  async () => {
    try {
      await auth.ensureSession()

      if (!auth.isAuthenticated) {
        consultantsStore.clearIntegratedView()
        analyticsStore.clearState()
        return
      }

      if (integratedScope.value) {
        await consultantsStore.ensureIntegratedView()
        return
      }

      consultantsStore.clearIntegratedView()
      analyticsStore.setIntegratedScope(false)
      await analyticsStore.fetchRanking()
    } catch {
      if (integratedScope.value) {
        consultantsStore.clearIntegratedView()
        return
      }

      analyticsStore.clearState()
    }
  },
  { immediate: true },
)
</script>

<template>
  <div class="page-workspace">
    <RankingWorkspace
      :report="report"
      :pending="pending"
      :error-message="errorMessage"
      :integrated-scope="integratedScope"
    />
  </div>
</template>
