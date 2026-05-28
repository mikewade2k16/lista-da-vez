<script setup lang="ts">
import { computed } from 'vue'
import type { BadgeRule, BadgeRuleId } from '~/composables/useGamificationConfig'

interface BadgeStats {
  monthlyGoal: number
  soldValue: number
  conversionRate: number
  ticketAverage: number
  paScore: number
  avgTicketGoal?: number
  paGoal?: number
}

const props = defineProps<{
  stats: BadgeStats
  badges: BadgeRule[]
  rankingPosition?: number | null
  storeConversionAvg?: number | null
}>()

interface ResolvedBadge {
  id: BadgeRuleId
  label: string
  icon: string
}

function interpolateLabel(rule: BadgeRule): string {
  if (rule.threshold === undefined) return rule.label
  return rule.label.replace('{threshold}', String(rule.threshold))
}

const resolvedBadges = computed<ResolvedBadge[]>(() => {
  const stats = props.stats
  const goalPct = stats.monthlyGoal > 0 ? (stats.soldValue / stats.monthlyGoal) * 100 : 0
  const result: ResolvedBadge[] = []

  for (const rule of props.badges) {
    if (!rule.enabled) continue
    let earned = false

    switch (rule.id) {
      case 'goal-hit':
        earned = stats.monthlyGoal > 0 && goalPct >= 100
        break
      case 'top-rank': {
        const threshold = rule.threshold ?? 3
        const position = props.rankingPosition ?? null
        earned = position !== null && position > 0 && position <= threshold
        break
      }
      case 'conversion-above-store': {
        const storeAvg = props.storeConversionAvg ?? null
        earned = storeAvg !== null && stats.conversionRate > storeAvg
        break
      }
      case 'ticket-above-goal':
        earned = (stats.avgTicketGoal ?? 0) > 0 && stats.ticketAverage >= (stats.avgTicketGoal ?? 0)
        break
      case 'pa-above-goal':
        earned = (stats.paGoal ?? 0) > 0 && stats.paScore >= (stats.paGoal ?? 0)
        break
    }

    if (earned) {
      result.push({ id: rule.id, label: interpolateLabel(rule), icon: rule.icon })
    }
  }

  return result
})
</script>

<template>
  <div v-if="resolvedBadges.length" class="consultant-badges" data-testid="consultant-badges">
    <span
      v-for="badge in resolvedBadges"
      :key="badge.id"
      class="consultant-badges__item"
      :data-badge-id="badge.id"
    >
      <span class="consultant-badges__icon" aria-hidden="true">{{ badge.icon }}</span>
      <span class="consultant-badges__label">{{ badge.label }}</span>
    </span>
  </div>
</template>

<style scoped>
.consultant-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.consultant-badges__item {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.3rem 0.65rem;
  border-radius: 999px;
  background: rgb(var(--success) / 0.12);
  border: 1px solid rgb(var(--success) / 0.32);
  color: rgb(var(--success));
  font-size: 0.75rem;
  font-weight: 600;
  white-space: nowrap;
}

.consultant-badges__icon {
  font-size: 0.95rem;
  line-height: 1;
}
</style>
