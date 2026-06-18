<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { formatCurrencyBRL, formatPercent } from '~/domain/utils/admin-metrics'
import { goalProgressTier } from '~/domain/utils/goal-progress-color'

// Contrato alinhado com o snapshot do back (`person.goalStats`). Pode vir null/undefined
// ate o rebuild da api; o componente trata isso sem quebrar.
interface GoalStats {
  monthlyGoal: number
  soldValue: number
  remainingToGoal: number
  progress: number
  hasGoal: boolean
}

const props = withDefaults(
  defineProps<{
    initials: string
    color?: string
    goalStats?: GoalStats | null
  }>(),
  {
    color: '',
    goalStats: null,
  },
)

// Geometria do anel SVG (mesma tecnica do gauge do ConsultantPlayerCard: stroke-dasharray).
const RADIUS = 20
const CIRCUMFERENCE = 2 * Math.PI * RADIUS

const hasGoal = computed(() => Boolean(props.goalStats?.hasGoal))

const progress = computed(() => {
  if (!props.goalStats || !hasGoal.value) return 0
  const value = Number(props.goalStats.progress)
  return Number.isFinite(value) ? value : 0
})

const clampedProgress = computed(() => Math.min(100, Math.max(0, progress.value)))

const ringDasharray = computed(
  () => `${(clampedProgress.value / 100) * CIRCUMFERENCE} ${CIRCUMFERENCE}`,
)

// Tier so para o estado neutro (sem meta) e para acessibilidade do aria-label.
const tier = computed(() => goalProgressTier(progress.value, hasGoal.value))
const isNeutral = computed(() => tier.value === 'none')

// Anel verde solido quando ha meta cadastrada; cinza (muted) quando nao ha.
const ringStroke = computed(() =>
  isNeutral.value ? 'rgb(var(--muted) / 0.45)' : 'rgb(var(--success))',
)

const ariaLabel = computed(() => {
  if (!hasGoal.value || !props.goalStats) return 'Sem meta cadastrada'
  return `Meta mensal em ${formatPercent(progress.value)}`
})

// Linhas de stat do popover (lista para facilitar acrescentar campos depois).
interface StatRow {
  label: string
  value: string
}

const statRows = computed<StatRow[]>(() => {
  const stats = props.goalStats
  if (!stats || !stats.hasGoal) return []
  return [
    { label: 'Meta', value: formatCurrencyBRL(stats.monthlyGoal) },
    {
      label: 'Atingido',
      value: `${formatCurrencyBRL(stats.soldValue)} (${formatPercent(progress.value)})`,
    },
    { label: 'Falta', value: formatCurrencyBRL(stats.remainingToGoal) },
  ]
})

// Popover (Teleport para o body) — evita corte por overflow do board.
const open = ref(false)
const anchorRef = ref<HTMLElement | null>(null)
const popoverStyle = ref<Record<string, string>>({})

function updatePosition() {
  const anchor = anchorRef.value
  if (!anchor) return
  const rect = anchor.getBoundingClientRect()
  popoverStyle.value = {
    top: `${rect.bottom + 8}px`,
    left: `${rect.left + rect.width / 2}px`,
  }
}

function show() {
  updatePosition()
  open.value = true
}

function hide() {
  open.value = false
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') hide()
}

// Dropdown/popover SEMPRE fecha no Esc; reposiciona se a pagina rolar/redimensionar.
watch(open, (value) => {
  if (value) {
    document.addEventListener('keydown', handleKeydown)
    window.addEventListener('scroll', updatePosition, true)
    window.addEventListener('resize', updatePosition)
  } else {
    document.removeEventListener('keydown', handleKeydown)
    window.removeEventListener('scroll', updatePosition, true)
    window.removeEventListener('resize', updatePosition)
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('scroll', updatePosition, true)
  window.removeEventListener('resize', updatePosition)
})

const avatarAccent = computed(() => props.color || 'rgb(var(--primary))')
</script>

<template>
  <span
    ref="anchorRef"
    class="operation-avatar-ring"
    tabindex="0"
    :aria-label="ariaLabel"
    @mouseenter="show"
    @mouseleave="hide"
    @focus="show"
    @blur="hide"
  >
    <svg class="operation-avatar-ring__svg" viewBox="0 0 48 48" aria-hidden="true">
      <circle
        class="operation-avatar-ring__track"
        cx="24"
        cy="24"
        :r="RADIUS"
        fill="none"
        stroke-width="3"
      />
      <circle
        class="operation-avatar-ring__progress"
        cx="24"
        cy="24"
        :r="RADIUS"
        fill="none"
        stroke-width="3"
        stroke-linecap="round"
        :stroke="ringStroke"
        :stroke-dasharray="ringDasharray"
        transform="rotate(-90 24 24)"
      />
    </svg>
    <span
      class="queue-card__avatar operation-avatar-ring__avatar"
      :style="{ '--avatar-accent': avatarAccent }"
    >
      {{ initials }}
    </span>

    <Teleport to="body">
      <div v-if="open" class="operation-avatar-ring__popover" role="tooltip" :style="popoverStyle">
        <p class="operation-avatar-ring__popover-title">Meta mensal</p>
        <ul v-if="statRows.length" class="operation-avatar-ring__stats">
          <li v-for="row in statRows" :key="row.label" class="operation-avatar-ring__stat">
            <span class="operation-avatar-ring__stat-label">{{ row.label }}</span>
            <span class="operation-avatar-ring__stat-value">{{ row.value }}</span>
          </li>
        </ul>
        <p v-else class="operation-avatar-ring__empty">Sem meta cadastrada</p>
      </div>
    </Teleport>
  </span>
</template>

<style scoped>
.operation-avatar-ring {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 58px;
  height: 58px;
  flex-shrink: 0;
  outline: none;
}

.operation-avatar-ring__svg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}

.operation-avatar-ring__track {
  stroke: rgb(var(--border) / 0.6);
}

.operation-avatar-ring__progress {
  transition:
    stroke-dasharray 240ms ease,
    stroke 200ms ease;
}

/* Reaproveita o visual da classe global .queue-card__avatar (44px, accent, texto branco).
   So forca o tamanho menor para caber dentro do anel sem cobrir o stroke. */
.operation-avatar-ring__avatar {
  position: relative;
  z-index: 1;
  width: 38px;
  height: 38px;
}

.operation-avatar-ring:focus-visible .operation-avatar-ring__avatar {
  box-shadow:
    inset 0 0 0 2px rgb(255 255 255 / 0.15),
    0 0 0 2px rgb(var(--ring) / 0.6);
}

.operation-avatar-ring__popover {
  position: fixed;
  z-index: 1000;
  transform: translateX(-50%);
  min-width: 11rem;
  padding: 0.6rem 0.7rem;
  border-radius: 0.7rem;
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface) / 0.98);
  box-shadow: var(--shadow-card);
  color: var(--text-main);
  pointer-events: none;
}

.operation-avatar-ring__popover-title {
  margin: 0 0 0.4rem;
  font-size: 0.7rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: rgb(var(--muted));
}

.operation-avatar-ring__stats {
  display: grid;
  gap: 0.25rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.operation-avatar-ring__stat {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
  font-size: 0.78rem;
}

.operation-avatar-ring__stat-label {
  color: rgb(var(--muted));
}

.operation-avatar-ring__stat-value {
  font-weight: 700;
  color: var(--text-main);
}

.operation-avatar-ring__empty {
  margin: 0;
  font-size: 0.76rem;
  color: rgb(var(--muted));
}
</style>
