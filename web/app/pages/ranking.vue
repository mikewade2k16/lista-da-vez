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
const {
  ranking: analyticsRanking,
  pending: analyticsPending,
  errorMessage: analyticsError,
  dateFrom: analyticsDateFrom,
  dateTo: analyticsDateTo,
} = storeToRefs(analyticsStore)
const {
  integratedRanking,
  integratedPending,
  integratedError,
  integratedDateFrom,
  integratedDateTo,
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
const rankingDateFrom = computed({
  get: () => (integratedScope.value ? integratedDateFrom.value : analyticsDateFrom.value),
  set: (value) => {
    if (integratedScope.value) {
      integratedDateFrom.value = String(value || '').trim()
      return
    }
    analyticsDateFrom.value = String(value || '').trim()
  },
})
const rankingDateTo = computed({
  get: () => (integratedScope.value ? integratedDateTo.value : analyticsDateTo.value),
  set: (value) => {
    if (integratedScope.value) {
      integratedDateTo.value = String(value || '').trim()
      return
    }
    analyticsDateTo.value = String(value || '').trim()
  },
})

function formatDateInput(date) {
  return [
    date.getUTCFullYear(),
    String(date.getUTCMonth() + 1).padStart(2, '0'),
    String(date.getUTCDate()).padStart(2, '0'),
  ].join('-')
}

async function applyRankingPeriod() {
  if (integratedScope.value) {
    await consultantsStore.applyIntegratedFilters()
    return
  }

  await analyticsStore.applyRankingFilters()
}

async function resetRankingCurrentMonth() {
  if (integratedScope.value) {
    consultantsStore.resetIntegratedCurrentMonth()
  } else {
    analyticsStore.resetCurrentMonth()
  }

  await applyRankingPeriod()
}

async function setRankingPreviousMonth() {
  const now = new Date()
  rankingDateFrom.value = formatDateInput(
    new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - 1, 1)),
  )
  rankingDateTo.value = formatDateInput(
    new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 0)),
  )
  await applyRankingPeriod()
}

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
        analyticsStore.setIntegratedScope(true)
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
      :date-from="rankingDateFrom"
      :date-to="rankingDateTo"
      @update:date-from="rankingDateFrom = $event"
      @update:date-to="rankingDateTo = $event"
      @apply-period="applyRankingPeriod"
      @set-current-month="resetRankingCurrentMonth"
      @set-previous-month="setRankingPreviousMonth"
    />
  </div>
</template>
