<script setup lang="ts">
import { computed } from 'vue'
import { formatPercent } from '~/domain/utils/admin-metrics'
import { goalProgressTier } from '~/domain/utils/goal-progress-color'

const props = withDefaults(
  defineProps<{
    progress?: number | null
  }>(),
  {
    progress: null,
  },
)

const hasProgress = computed(() => typeof props.progress === 'number')
const percent = computed(() => Math.max(0, Number(props.progress || 0)))
const clampedPercent = computed(() => Math.min(100, percent.value))
const tierClass = computed(
  () => `store-goal-bar--${goalProgressTier(percent.value, hasProgress.value)}`,
)
</script>

<template>
  <div class="store-goal-bar" :class="tierClass">
    <div class="store-goal-bar__head">
      <span class="store-goal-bar__label">Meta da loja</span>
      <span class="store-goal-bar__pct">{{ formatPercent(percent) }}</span>
    </div>
    <div class="store-goal-bar__track">
      <div class="store-goal-bar__fill" :style="{ width: `${clampedPercent}%` }"></div>
    </div>
  </div>
</template>

<style scoped>
.store-goal-bar {
  display: grid;
  gap: 0.3rem;
}

.store-goal-bar__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.store-goal-bar__label {
  font-size: 0.68rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: rgb(var(--muted) / 0.9);
}

.store-goal-bar__pct {
  font-size: 0.85rem;
  font-weight: 800;
}

.store-goal-bar__track {
  height: 0.4rem;
  border-radius: 999px;
  background: rgb(var(--border) / 0.5);
  overflow: hidden;
}

.store-goal-bar__fill {
  height: 100%;
  border-radius: 999px;
  background: rgb(var(--primary));
  transition:
    width 240ms ease,
    background 200ms ease;
}

.store-goal-bar--none .store-goal-bar__fill {
  background: rgb(var(--muted) / 0.6);
}

.store-goal-bar--none .store-goal-bar__pct {
  color: rgb(var(--muted));
}

.store-goal-bar--low .store-goal-bar__fill {
  background: rgb(var(--danger));
}

.store-goal-bar--low .store-goal-bar__pct {
  color: rgb(var(--danger));
}

.store-goal-bar--mid .store-goal-bar__fill {
  background: var(--accent-warning);
}

.store-goal-bar--mid .store-goal-bar__pct {
  color: var(--accent-warning);
}

.store-goal-bar--high .store-goal-bar__fill {
  background: rgb(var(--primary));
}

.store-goal-bar--high .store-goal-bar__pct {
  color: rgb(var(--primary));
}

.store-goal-bar--hit .store-goal-bar__fill {
  background: rgb(var(--success));
}

.store-goal-bar--hit .store-goal-bar__pct {
  color: rgb(var(--success));
}
</style>
