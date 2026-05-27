<script setup>
import { computed, watch } from 'vue'
import ConsultantWorkspace from '~/components/consultant/ConsultantWorkspace.vue'
import ArchivedStoreBanner from '~/components/operation/ArchivedStoreBanner.vue'
import { storeToRefs } from 'pinia'
import { canUseAllStoresScope } from '~/domain/utils/permissions'
import { useAuthStore } from '~/stores/auth'
import { useConsultantsStore } from '~/stores/consultants'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'consultor',
  alias: ['/operacao/consultor'],
  supportsAllStoresScope: true,
})

const auth = useAuthStore()
const consultantsStore = useConsultantsStore()
const {
  state,
  integratedRoster,
  integratedRanking,
  integratedOverview,
  integratedHistory,
  integratedPending,
  integratedError,
} = storeToRefs(consultantsStore)
const canSeeIntegrated = computed(() => canUseAllStoresScope(auth.accessibleStoreIds))
const integratedScope = computed(() => canSeeIntegrated.value)

watch(
  () => [integratedScope.value, auth.activeTenantId, auth.isAuthenticated],
  async () => {
    try {
      await auth.ensureSession()

      if (!auth.isAuthenticated) {
        consultantsStore.clearIntegratedView()
        return
      }

      if (integratedScope.value) {
        await consultantsStore.ensureIntegratedView()
        return
      }

      consultantsStore.clearIntegratedView()
      await consultantsStore.refreshActiveStore()
    } catch {
      consultantsStore.clearIntegratedView()
    }
  },
  { immediate: true },
)
</script>

<template>
  <div class="page-workspace">
    <ArchivedStoreBanner v-if="!integratedScope" />
    <ConsultantWorkspace
      :state="state"
      :integrated-scope="integratedScope"
      :integrated-roster="integratedRoster"
      :integrated-ranking="integratedRanking"
      :integrated-overview="integratedOverview"
      :integrated-history="integratedHistory"
      :integrated-pending="integratedPending"
      :integrated-error="integratedError"
    />
  </div>
</template>
