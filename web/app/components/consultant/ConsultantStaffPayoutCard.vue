<script setup lang="ts">
import { computed } from 'vue'
import { formatCurrencyBRL } from '~/domain/utils/admin-metrics'
import ConsultantStoreGoalBar from './ConsultantStoreGoalBar.vue'

interface StaffMember {
  id: string
  name: string
  role?: string
  roleLabel?: string
  storeName?: string
}

const props = withDefaults(
  defineProps<{
    staff: StaffMember
    storeGoalProgress?: number | null
    payoutAmount?: number | null
    payoutLabel?: string
  }>(),
  {
    storeGoalProgress: null,
    payoutAmount: null,
    payoutLabel: '',
  },
)

const roleText = computed(() => props.staff.roleLabel || props.staff.role || 'Equipe da loja')
const showStoreBar = computed(() => typeof props.storeGoalProgress === 'number')
const showPayout = computed(() => typeof props.payoutAmount === 'number')
</script>

<template>
  <article class="staff-card" :data-staff-id="staff.id">
    <header class="staff-card__header">
      <span class="staff-card__avatar" aria-hidden="true">
        {{ staff.name.charAt(0).toUpperCase() }}
      </span>
      <div class="staff-card__identity">
        <strong class="staff-card__name">{{ staff.name }}</strong>
        <span class="staff-card__role">
          {{ roleText }}
          <template v-if="staff.storeName">· {{ staff.storeName }}</template>
        </span>
      </div>
      <span class="staff-card__tag">Sem fila</span>
    </header>

    <ConsultantStoreGoalBar v-if="showStoreBar" :progress="storeGoalProgress" />

    <div class="staff-card__payout">
      <span class="staff-card__payout-label">Recebimento pela loja</span>
      <strong class="staff-card__payout-value">
        {{ showPayout ? formatCurrencyBRL(payoutAmount || 0) : 'Sem faixa' }}
      </strong>
      <span v-if="showPayout && payoutLabel" class="staff-card__payout-note">
        {{ payoutLabel }}
      </span>
    </div>
  </article>
</template>

<style scoped>
.staff-card {
  display: grid;
  gap: 0.85rem;
  padding: 1rem;
  border-radius: 1rem;
  border: 1px dashed rgb(var(--border) / 0.9);
  background: rgb(var(--surface) / 0.7);
  color: rgb(var(--text) / 0.92);
}

.staff-card__header {
  display: flex;
  align-items: center;
  gap: 0.7rem;
}

.staff-card__avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 999px;
  background: rgb(var(--muted) / 0.16);
  color: rgb(var(--text) / 0.82);
  font-weight: 700;
  font-size: 0.95rem;
}

.staff-card__identity {
  display: grid;
  gap: 0.1rem;
  min-width: 0;
  flex: 1;
}

.staff-card__name {
  font-size: 0.95rem;
  line-height: 1.2;
  color: rgb(var(--text) / 0.96);
}

.staff-card__role {
  font-size: 0.72rem;
  color: rgb(var(--muted) / 0.92);
}

.staff-card__tag {
  display: inline-flex;
  align-items: center;
  min-height: 1.5rem;
  padding: 0 0.5rem;
  border-radius: 999px;
  border: 1px solid rgb(var(--border) / 0.86);
  background: rgb(var(--surface-2) / 0.8);
  color: rgb(var(--muted) / 0.95);
  font-size: 0.62rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  white-space: nowrap;
}

.staff-card__payout {
  display: grid;
  gap: 0.15rem;
  padding: 0.6rem 0.7rem;
  border-radius: 0.7rem;
  background: rgb(var(--surface-2) / 0.76);
  border: 1px solid rgb(var(--border) / 0.72);
}

.staff-card__payout-label {
  font-size: 0.68rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: rgb(var(--muted) / 0.88);
}

.staff-card__payout-value {
  font-size: 1.05rem;
  color: rgb(var(--success));
  font-weight: 800;
}

.staff-card__payout-note {
  font-size: 0.72rem;
  color: rgb(var(--muted) / 0.92);
}
</style>
