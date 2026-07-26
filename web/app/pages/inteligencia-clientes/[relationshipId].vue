<script setup lang="ts">
import { computed } from 'vue'
import CustomerProfileHeader from '~/components/customer-intelligence/profile/CustomerProfileHeader.vue'
import CustomerProfileSummary from '~/components/customer-intelligence/profile/CustomerProfileSummary.vue'
import CustomerFactsPanel from '~/components/customer-intelligence/profile/CustomerFactsPanel.vue'
import CustomerEvidencePanel from '~/components/customer-intelligence/profile/CustomerEvidencePanel.vue'
import CustomerClaimsPanel from '~/components/customer-intelligence/profile/CustomerClaimsPanel.vue'
import RelationshipObservationsPanel from '~/components/customer-intelligence/profile/RelationshipObservationsPanel.vue'
import CustomerHistoryPanel from '~/components/customer-intelligence/profile/CustomerHistoryPanel.vue'
import CustomerSourceCoverage from '~/components/customer-intelligence/profile/CustomerSourceCoverage.vue'
import CustomerRecommendationsPanel from '~/components/customer-intelligence/profile/CustomerRecommendationsPanel.vue'
import CustomerIntelligenceRefreshControl from '~/components/customer-intelligence/profile/CustomerIntelligenceRefreshControl.vue'
import CustomerSourceSuggestionsPanel from '~/components/customer-intelligence/profile/CustomerSourceSuggestionsPanel.vue'
import OfflineInteractionTimeline from '~/components/customer-intelligence/profile/OfflineInteractionTimeline.vue'
import { useCustomerProfile } from '~/composables/customer-intelligence/useCustomerProfile'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'customer_intelligence',
  pageLabel: 'Perfil do cliente',
})

const route = useRoute()
const relationshipId = computed(() => String(route.params.relationshipId || '').trim())
const profile = useCustomerProfile(relationshipId)
</script>

<template>
  <div class="page-workspace">
    <CustomerIntelligencePageShell
      title="Perfil 360 do cliente"
      description="O identificador da rota e o relacionamento no client scope; subjectId isolado nunca concede acesso."
    >
      <CustomerProfileHeader
        :profile="profile.deterministicProfile.value"
        :relationship-id="relationshipId"
      />
      <CustomerIntelligenceRefreshControl
        v-if="
          profile.deterministicProfile.value && profile.access.canManageIntelligenceProfile.value
        "
        :subject-id="profile.deterministicProfile.value.subject.id"
        :relationship-id="relationshipId"
      />

      <CustomerIntelligenceStatus
        v-if="profile.profileStatus.value === 'loading'"
        title="Carregando perfil deterministico"
        loading
      />
      <CustomerIntelligenceStatus
        v-else-if="profile.profileError.value"
        title="Perfil indisponivel"
        :error="profile.profileError.value"
      />

      <div v-if="profile.deterministicProfile.value" class="ci-profile-grid">
        <CustomerHistoryPanel
          :profile="profile.deterministicProfile.value"
          :timeline="profile.timeline.value"
        />
        <CustomerSourceCoverage :sources="profile.deterministicProfile.value.sourceLinks || []" />
      </div>
      <OfflineInteractionTimeline
        v-if="profile.access.canViewOffline.value"
        :relationship-id="relationshipId"
      />

      <CustomerIntelligenceStatus
        v-if="!profile.access.canViewIntelligenceProfile.value"
        title="Inteligencia opcional indisponivel"
        :error="{
          kind: profile.access.hasCustomerIntelligenceModule.value ? 'forbidden' : 'capability_off',
          message: '',
          reasonCode: profile.access.hasCustomerIntelligenceModule.value
            ? 'customer_intelligence_profile_view_required'
            : 'customer_intelligence_module_disabled',
          statusCode: profile.access.hasCustomerIntelligenceModule.value ? 403 : 0,
        }"
      />
      <CustomerIntelligenceStatus
        v-else-if="profile.intelligenceStatus.value === 'loading'"
        title="Carregando inteligencia"
        loading
      />
      <template v-else>
        <CustomerIntelligenceStatus
          v-if="profile.intelligenceError.value && profile.intelligenceStatus.value === 'empty'"
          title="Inteligencia indisponivel"
          :error="profile.intelligenceError.value"
        />
        <div class="ci-profile-grid">
          <CustomerProfileSummary :summaries="profile.summaries.value" />
          <CustomerFactsPanel :facts="profile.facts.value" />
          <CustomerEvidencePanel :facts="profile.facts.value" />
        </div>
        <RelationshipObservationsPanel :relationship-id="relationshipId" />
        <CustomerClaimsPanel
          :relationship-id="relationshipId"
          :can-manage="profile.access.canManageIntelligenceProfile.value"
        />
        <CustomerRecommendationsPanel
          :relationship-id="relationshipId"
          :can-manage="profile.access.canManageIntelligenceProfile.value"
        />
        <CustomerSourceSuggestionsPanel
          :relationship-id="relationshipId"
          :can-manage="profile.access.canManageSources.value"
        />
      </template>
    </CustomerIntelligencePageShell>
  </div>
</template>

<style scoped>
.ci-profile-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
}

@media (max-width: 900px) {
  .ci-profile-grid {
    grid-template-columns: 1fr;
  }
}
</style>
