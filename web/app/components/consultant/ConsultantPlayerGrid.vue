<script setup lang="ts">
import { computed } from 'vue'
import ConsultantPlayerCard from '~/components/consultant/ConsultantPlayerCard.vue'
import ConsultantStaffPayoutCard from '~/components/consultant/ConsultantStaffPayoutCard.vue'
import type {
  ConsultantGoalSource,
  ErpPayout,
  ErpStorePayout,
} from '~/domain/utils/consultant-integrated-view'
import {
  consultantPayoutLabel,
  storePayoutForRole,
  storeRolePayoutLabel,
} from '~/domain/utils/consultant-payout-display'
import { useGoalQuickEditContext } from '~/composables/useGoalQuickEditContext'

interface GridRow {
  id: string
  name: string
  storeId?: string
  role?: string
  storeName?: string
  liveStatusCode?: string
  liveStatusLabel?: string
  monthlyGoal?: number
  soldValue?: number
  remainingToGoal?: number
  ticketAverage?: number
  paScore?: number
  erpOrders?: number
  soldValueSource?: string
  ticketAverageSource?: string
  paScoreSource?: string
  conversionRate?: number
  avgDurationMs?: number
  avgTicketGoal?: number
  paGoal?: number
  cancellationRate?: number
  payout?: ErpPayout | null
  // Flags de gap por consultor (contrato congelado) p/ o aviso acionável inline.
  goalSource?: ConsultantGoalSource
  missingMonthlyGoal?: boolean
  missingTicketGoal?: boolean
  missingPaGoal?: boolean
  [key: string]: unknown
}

interface StaffRow {
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

const props = withDefaults(
  defineProps<{
    rows?: GridRow[]
    staff?: StaffRow[]
    storeConversionAvgByStoreId?: Record<string, number>
    rankingPositionByKey?: Record<string, number>
    storeProgressByStoreId?: Record<string, StoreProgress>
    storePayoutByStoreId?: Record<string, ErpStorePayout>
    canOpenFeedback?: boolean
  }>(),
  {
    rows: () => [],
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

function storeProgressFor(storeId?: string): StoreProgress | null {
  return props.storeProgressByStoreId[storeId ?? ''] ?? null
}

function storePayoutFor(storeId?: string): ErpStorePayout | null {
  return props.storePayoutByStoreId[storeId ?? ''] ?? null
}

const { buildContext } = useGoalQuickEditContext()

const enrichedRows = computed(() =>
  props.rows.map((row) => {
    const store = storeProgressFor(row.storeId)
    // Consultor: payout vem PRONTO do back (% da própria venda). Display só.
    const payout = row.payout ?? null

    // Contexto do quick-edit de metas (motor plugável). Os valores efetivos de
    // ticket/PA (com herança da loja) vêm das stats da row p/ semear o popover.
    const goalContext = buildContext({
      storeId: row.storeId,
      consultantId: row.id,
      store: storePayoutFor(row.storeId),
      currentTicketGoal: row.avgTicketGoal ?? 0,
      currentPaGoal: row.paGoal ?? 0,
      consultant: {
        goalSource: row.goalSource,
        missingMonthlyGoal: row.missingMonthlyGoal,
        missingTicketGoal: row.missingTicketGoal,
        missingPaGoal: row.missingPaGoal,
        monthlyGoal: row.monthlyGoal ?? 0,
      },
    })

    return {
      consultant: {
        id: row.id,
        name: row.name,
        role: row.role,
        storeName: row.storeName,
        liveStatusCode: row.liveStatusCode,
        liveStatusLabel: row.liveStatusLabel,
      },
      stats: {
        monthlyGoal: row.monthlyGoal ?? 0,
        soldValue: row.soldValue ?? 0,
        remainingToGoal: row.remainingToGoal ?? 0,
        ticketAverage: row.ticketAverage ?? 0,
        paScore: row.paScore ?? 0,
        erpOrders: row.erpOrders,
        soldValueSource: row.soldValueSource,
        ticketAverageSource: row.ticketAverageSource,
        paScoreSource: row.paScoreSource,
        conversionRate: row.conversionRate ?? 0,
        averageDurationMs: row.avgDurationMs ?? 0,
        avgTicketGoal: row.avgTicketGoal,
        paGoal: row.paGoal,
        cancellationRate: row.cancellationRate,
      },
      storeConversionAvg: props.storeConversionAvgByStoreId[row.storeId ?? ''] ?? null,
      rankingPosition: props.rankingPositionByKey[`${row.storeId}:${row.id}`] ?? null,
      storeGoalProgress: store ? store.progress : null,
      goalPayoutAmount: payout ? payout.amount : null,
      goalPayoutLabel: payout ? consultantPayoutLabel(payout) : '',
      goalContext,
      key: `${row.storeId}:${row.id}`,
    }
  }),
)

const enrichedStaff = computed(() =>
  props.staff.map((member) => {
    const store = storeProgressFor(member.storeId)
    // Gerente/caixa: recebem pela LOJA. mapRoleToPayoutGroup escolhe manager vs support.
    const payout = storePayoutForRole(storePayoutFor(member.storeId), member.role)

    return {
      member,
      storeGoalProgress: store ? store.progress : null,
      payoutAmount: payout ? payout.amount : null,
      payoutLabel: payout ? storeRolePayoutLabel(payout) : '',
      key: `staff:${member.storeId}:${member.id}`,
    }
  }),
)

const hasCards = computed(() => enrichedRows.value.length > 0 || enrichedStaff.value.length > 0)

function handleOpen(consultantId: string) {
  emit('open-details', consultantId)
}
</script>

<template>
  <div v-if="hasCards" class="player-grid" data-testid="player-grid">
    <ConsultantPlayerCard
      v-for="row in enrichedRows"
      :key="row.key"
      :consultant="row.consultant"
      :stats="row.stats"
      :store-conversion-avg="row.storeConversionAvg"
      :ranking-position="row.rankingPosition"
      :store-goal-progress="row.storeGoalProgress"
      :goal-payout-amount="row.goalPayoutAmount"
      :goal-payout-label="row.goalPayoutLabel"
      :goal-context="row.goalContext"
      mode="mini"
      :show-details-button="false"
      :show-feedback-button="canOpenFeedback"
      @open-details="handleOpen"
      @open-feedback="emit('open-feedback', $event)"
    />
    <ConsultantStaffPayoutCard
      v-for="item in enrichedStaff"
      :key="item.key"
      :staff="item.member"
      :store-goal-progress="item.storeGoalProgress"
      :payout-amount="item.payoutAmount"
      :payout-label="item.payoutLabel"
    />
  </div>
  <div v-else class="player-grid__empty" data-testid="player-grid-empty">
    Nenhum consultor encontrado para os filtros selecionados.
  </div>
</template>

<style scoped>
.player-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(20rem, 1fr));
  gap: 0.85rem;
}

.player-grid__empty {
  padding: 2rem;
  text-align: center;
  color: rgb(var(--muted) / 0.92);
}
</style>
