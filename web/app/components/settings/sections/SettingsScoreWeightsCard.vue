<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  settings: Record<string, unknown>
  canEdit: boolean
  onChangeWeight: (settingId: string, value: string) => void
}>()

interface WeightField {
  id: string
  label: string
  defaultValue: number
}

const WEIGHT_FIELDS: WeightField[] = [
  { id: 'scoreWeightConversion', label: 'Conversao', defaultValue: 35 },
  { id: 'scoreWeightSoldValue', label: 'Valor vendido', defaultValue: 25 },
  { id: 'scoreWeightQuality', label: 'Qualidade', defaultValue: 20 },
  { id: 'scoreWeightPa', label: 'P.A.', defaultValue: 15 },
  { id: 'scoreWeightQueueDiscipline', label: 'Disciplina de fila', defaultValue: 5 },
]

function getWeight(fieldId: string, defaultValue: number): number {
  return Number(props.settings[fieldId] ?? defaultValue)
}

const totalWeight = computed(() => {
  return WEIGHT_FIELDS.reduce((sum, f) => sum + getWeight(f.id, f.defaultValue), 0)
})

const totalClass = computed<string>(() => {
  if (totalWeight.value === 100) return 'score-weights-card__total--ok'
  return 'score-weights-card__total--warn'
})

function handleSliderInput(fieldId: string, event: Event) {
  const target = event.target as HTMLInputElement
  props.onChangeWeight(fieldId, target.value)
}
</script>

<template>
  <article class="settings-card score-weights-card">
    <header class="settings-card__header">
      <h3 class="settings-card__title">Pesos do Score 360</h3>
      <p class="settings-card__text">
        Define a importancia de cada componente no calculo do ranking. Os pesos devem somar 100 para
        manter a escala padrao.
      </p>
    </header>

    <div class="score-weights-card__list">
      <div v-for="field in WEIGHT_FIELDS" :key="field.id" class="score-weights-card__row">
        <label :for="`score-weight-${field.id}`" class="score-weights-card__label">
          {{ field.label }}
        </label>
        <div class="score-weights-card__controls">
          <input
            :id="`score-weight-${field.id}`"
            type="range"
            min="0"
            max="100"
            step="1"
            class="score-weights-card__slider"
            :value="getWeight(field.id, field.defaultValue)"
            :disabled="!canEdit"
            @change="handleSliderInput(field.id, $event)"
          />
          <span class="score-weights-card__value">
            {{ getWeight(field.id, field.defaultValue) }}
          </span>
        </div>
      </div>
    </div>

    <footer class="score-weights-card__footer">
      <span :class="['score-weights-card__total', totalClass]">
        Total: {{ totalWeight }}
        <template v-if="totalWeight !== 100">— deve ser 100 para salvar corretamente</template>
      </span>
    </footer>
  </article>
</template>

<style scoped>
.score-weights-card__list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.score-weights-card__row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.score-weights-card__label {
  flex: 0 0 9rem;
  font-size: 0.875rem;
  color: var(--text-main);
  font-weight: 500;
}

.score-weights-card__controls {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex: 1;
}

.score-weights-card__slider {
  flex: 1;
  accent-color: rgb(var(--primary));
  cursor: pointer;
}

.score-weights-card__slider:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.score-weights-card__value {
  flex: 0 0 2.5rem;
  text-align: right;
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--text-main);
  font-variant-numeric: tabular-nums;
}

.score-weights-card__footer {
  margin-top: 0.75rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--line-soft);
}

.score-weights-card__total {
  font-size: 0.8125rem;
  font-weight: 500;
}

.score-weights-card__total--ok {
  color: rgb(var(--success));
}

.score-weights-card__total--warn {
  color: rgb(var(--danger));
}
</style>
