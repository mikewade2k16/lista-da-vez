<script setup lang="ts">
import ConsultantHistoryPanel from '~/components/consultant/ConsultantHistoryPanel.vue'
import ConsultantPlayerCard from '~/components/consultant/ConsultantPlayerCard.vue'
import ConsultantRecentAttendancesTable from '~/components/consultant/ConsultantRecentAttendancesTable.vue'
import ConsultantSelector from '~/components/consultant/ConsultantSelector.vue'
import ConsultantSimulator from '~/components/consultant/ConsultantSimulator.vue'
import ConsultantStaffPayoutCard from '~/components/consultant/ConsultantStaffPayoutCard.vue'
import InlineFieldGuard from '~/components/quick-edit/InlineFieldGuard.vue'
import { storeTicketGoal } from '~/domain/quick-edit/fields/storeTicketGoal'
import { storePaGoal } from '~/domain/quick-edit/fields/storePaGoal'
import type { ConsultantRow } from '~/composables/useConsultantIntegratedRows'
import type { GoalQuickEditContext } from '~/domain/quick-edit/fields/goalContext'
import type { PlayerCardConsultant, PlayerCardStats } from './player-card-types'

interface SingleStoreCard {
  consultant: PlayerCardConsultant
  stats: PlayerCardStats
  storeConversionAvg: number | null
  rankingPosition: number | null
  storeGoalProgress: number | null
  goalPayoutAmount: number | null
  goalPayoutLabel: string
  goalContext: GoalQuickEditContext
}

interface StaffMember {
  id: string
  name: string
  role?: string
  roleLabel?: string
  storeId?: string
  storeName?: string
}

interface SingleStoreStaffItem {
  member: StaffMember
  storeGoalProgress: number | null
  payoutAmount: number | null
  payoutLabel: string
}

const storeGoalGuards = { storeTicketGoal, storePaGoal }

withDefaults(
  defineProps<{
    rows?: ConsultantRow[]
    selectedConsultant?: ConsultantRow | null
    card?: SingleStoreCard | null
    staff?: SingleStoreStaffItem[]
    history?: Array<Record<string, unknown>>
    simulationAdditionalSales?: number
  }>(),
  {
    rows: () => [],
    selectedConsultant: null,
    card: null,
    staff: () => [],
    history: () => [],
    simulationAdditionalSales: 0,
  },
)

const emit = defineEmits<{
  (e: 'select', consultantId: string): void
  (e: 'update:simulation-additional-sales', value: number): void
}>()
</script>

<template>
  <ConsultantSelector
    v-if="rows.length"
    :roster="rows"
    :selected-consultant-id="selectedConsultant?.id || ''"
    @select="emit('select', $event)"
  />
  <div
    v-if="card"
    class="consultant-integrated-group__alerts consultant-integrated-group__alerts--single"
  >
    <InlineFieldGuard :descriptor="storeGoalGuards.storeTicketGoal" :context="card.goalContext" />
    <InlineFieldGuard :descriptor="storeGoalGuards.storePaGoal" :context="card.goalContext" />
  </div>
  <ConsultantPlayerCard
    v-if="card"
    :consultant="card.consultant"
    :stats="card.stats"
    :store-conversion-avg="card.storeConversionAvg"
    :ranking-position="card.rankingPosition"
    :store-goal-progress="card.storeGoalProgress"
    :goal-payout-amount="card.goalPayoutAmount"
    :goal-payout-label="card.goalPayoutLabel"
    :goal-context="card.goalContext"
    mode="full"
    :show-details-button="false"
  />
  <div v-if="card" class="consultant-integrated-insights">
    <ConsultantHistoryPanel
      :consultant-id="card.consultant.id"
      :store-id="selectedConsultant?.storeId"
      :entries="history"
    />
    <section class="consultant-integrated-insight-panel">
      <ConsultantSimulator
        :sold-value="card.stats.soldValue"
        :monthly-goal="card.stats.monthlyGoal"
        :commission-rate="card.stats.commissionRate"
        :payout-rate-percent="selectedConsultant?.payout?.ratePercent ?? null"
        :simulation-additional-sales="simulationAdditionalSales"
        @update:simulation-additional-sales="emit('update:simulation-additional-sales', $event)"
      />
    </section>
  </div>
  <section v-if="staff.length" class="consultant-integrated-staff">
    <header class="consultant-integrated-staff__header">
      <h3 class="consultant-integrated-staff__title">Equipe da loja (sem fila)</h3>
      <p class="consultant-integrated-staff__text">
        Recebem pela meta da loja; nao atendem na fila.
      </p>
    </header>
    <div class="consultant-integrated-staff__grid">
      <ConsultantStaffPayoutCard
        v-for="item in staff"
        :key="`staff:${item.member.storeId}:${item.member.id}`"
        :staff="item.member"
        :store-goal-progress="item.storeGoalProgress"
        :payout-amount="item.payoutAmount"
        :payout-label="item.payoutLabel"
      />
    </div>
  </section>

  <ConsultantRecentAttendancesTable
    v-if="selectedConsultant"
    :consultant-id="selectedConsultant.id"
    :consultant-name="selectedConsultant.name"
    :store-id="selectedConsultant.storeId"
    :store-name="selectedConsultant.storeName"
    :entries="history"
  />
  <div v-else class="player-grid__empty" data-testid="player-grid-empty">
    Nenhum consultor encontrado para os filtros selecionados.
  </div>
</template>

<style scoped>
.consultant-integrated-group__alerts {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem;
  justify-content: flex-end;
}
.consultant-integrated-group__alerts--single {
  justify-content: flex-start;
  padding-bottom: 0.1rem;
}
.consultant-integrated-insights {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr);
  gap: 0.85rem;
  align-items: start;
}
.consultant-integrated-insight-panel {
  padding: 1rem;
  border-radius: 1rem;
  border: 1px solid rgb(var(--primary) / 0.16);
  background: rgb(var(--surface) / 0.78);
  box-shadow: var(--shadow-xs);
}
.consultant-integrated-staff {
  display: grid;
  gap: 0.65rem;
}
.consultant-integrated-staff__header {
  display: grid;
  gap: 0.15rem;
}
.consultant-integrated-staff__title {
  margin: 0;
  font-size: 0.95rem;
  color: rgb(var(--text) / 0.96);
}
.consultant-integrated-staff__text {
  margin: 0;
  font-size: 0.76rem;
  color: rgb(var(--muted) / 0.9);
}
.consultant-integrated-staff__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(18rem, 1fr));
  gap: 0.85rem;
}
.player-grid__empty {
  padding: 2rem;
  text-align: center;
  color: rgb(var(--muted) / 0.92);
}
@media (max-width: 1100px) {
  .consultant-integrated-insights {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 720px) {
  .consultant-integrated-insights {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
