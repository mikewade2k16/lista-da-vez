<script setup>
import { computed, watch } from 'vue'
import CampaignHubWorkspace from '~/components/campaigns/CampaignHubWorkspace.vue'
import { storeToRefs } from 'pinia'
import { canUseAllStoresScope, getAllowedWorkspaces } from '~/domain/utils/permissions'
import { useAuthStore } from '~/stores/auth'
import { useCampaignsStore } from '~/stores/campaigns'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'comunicados',
  alias: ['/operacao/campanhas'],
  supportsAllStoresScope: true,
})

const auth = useAuthStore()
const campaignsStore = useCampaignsStore()
const { state, integratedHistory, integratedPending, integratedError } = storeToRefs(campaignsStore)
const canSeeIntegrated = computed(() => canUseAllStoresScope(auth.accessibleStoreIds))
const canSeeCampaigns = computed(() =>
  getAllowedWorkspaces(auth.role, auth.effectivePermissionKeys, auth.permissionsResolved).includes(
    'campanhas',
  ),
)
const integratedScope = computed(() => canSeeCampaigns.value && canSeeIntegrated.value)
const storeOptions = computed(() => auth.storeContext || [])

watch(
  () => [integratedScope.value, auth.activeStoreId, auth.activeTenantId, auth.isAuthenticated],
  async () => {
    try {
      await auth.ensureSession()

      if (!auth.isAuthenticated) {
        campaignsStore.clearIntegratedHistory()
        return
      }

      if (integratedScope.value) {
        await campaignsStore.ensureIntegratedHistory()
        return
      }

      campaignsStore.clearIntegratedHistory()
    } catch {
      campaignsStore.clearIntegratedHistory()
    }
  },
  { immediate: true },
)
</script>

<template>
  <div class="page-workspace">
    <CampaignHubWorkspace
      :state="state"
      :integrated-scope="integratedScope"
      :integrated-history="integratedHistory"
      :integrated-pending="integratedPending"
      :integrated-error="integratedError"
      :stores="storeOptions"
    />
  </div>
</template>
