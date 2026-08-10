<script setup lang="ts">
import ConsultantPlayerGrid from '~/components/consultant/ConsultantPlayerGrid.vue'
import InlineFieldGuard from '~/components/quick-edit/InlineFieldGuard.vue'
import { storeTicketGoal } from '~/domain/quick-edit/fields/storeTicketGoal'
import { storePaGoal } from '~/domain/quick-edit/fields/storePaGoal'
import type { ConsultantRow } from '~/composables/useConsultantIntegratedRows'
import type { GoalQuickEditContext } from '~/domain/quick-edit/fields/goalContext'
import type { ErpStorePayout } from '~/domain/utils/consultant-integrated-view'

interface StaffItem {
  id: string
  name: string
  role?: string
  roleLabel?: string
  storeId?: string
  storeName?: string
}

interface StoreProgress {
  storeSold: number
  storeGoal: number
  progress: number
}

interface StoreGroup {
  storeId: string
  storeName: string
  rows: ConsultantRow[]
  storeContext: GoalQuickEditContext
}

const storeGoalGuards = { storeTicketGoal, storePaGoal }

withDefaults(
  defineProps<{
    group: StoreGroup
    staff?: StaffItem[]
    storeConversionAvgByStoreId?: Record<string, number>
    rankingPositionByKey?: Record<string, number>
    storeProgressByStoreId?: Record<string, StoreProgress>
    storePayoutByStoreId?: Record<string, ErpStorePayout>
    canOpenFeedback?: boolean
  }>(),
  {
    staff: () => [],
    storeConversionAvgByStoreId: () => ({}),
    rankingPositionByKey: () => ({}),
    storeProgressByStoreId: () => ({}),
    storePayoutByStoreId: () => ({}),
    canOpenFeedback: false,
  },
)

const emit = defineEmits<{
  (e: 'open-details' | 'open-feedback', consultantId: string): void
}>()
</script>

<template>
  <section class="consultant-integrated-group">
    <header class="consultant-integrated-group__header">
      <div class="consultant-integrated-group__title-row">
        <h3 class="consultant-integrated-group__title">{{ group.storeName }}</h3>
        <div class="consultant-integrated-group__alerts">
          <InlineFieldGuard
            :descriptor="storeGoalGuards.storeTicketGoal"
            :context="group.storeContext"
          />
          <InlineFieldGuard
            :descriptor="storeGoalGuards.storePaGoal"
            :context="group.storeContext"
          />
        </div>
      </div>
      <p class="consultant-integrated-group__text">
        {{ group.rows.length }} consultor(es) nos filtros atuais.
      </p>
    </header>
    <ConsultantPlayerGrid
      :rows="group.rows"
      :staff="staff"
      :store-conversion-avg-by-store-id="storeConversionAvgByStoreId"
      :ranking-position-by-key="rankingPositionByKey"
      :store-progress-by-store-id="storeProgressByStoreId"
      :store-payout-by-store-id="storePayoutByStoreId"
      :can-open-feedback="canOpenFeedback"
      @open-details="emit('open-details', $event)"
      @open-feedback="emit('open-feedback', $event)"
    />
  </section>
</template>

<style scoped>
.consultant-integrated-group {
  display: grid;
  gap: 0.85rem;
}
.consultant-integrated-group__header {
  display: grid;
  gap: 0.2rem;
  padding-bottom: 0.35rem;
  border-bottom: 1px solid rgb(var(--border) / 0.72);
}
.consultant-integrated-group__title-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 0.6rem;
}
.consultant-integrated-group__alerts {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem;
}
.consultant-integrated-group__title {
  margin: 0;
  font-size: 1rem;
  color: rgb(var(--text) / 0.96);
}
.consultant-integrated-group__text {
  margin: 0;
  font-size: 0.78rem;
  color: rgb(var(--muted) / 0.9);
}
</style>
