<script setup lang="ts">
import { computed, ref } from 'vue'
import { Plus, Trash2, X } from 'lucide-vue-next'

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

const addingWeight = ref(false)

function getWeight(fieldId: string, defaultValue: number): number {
  return Number(props.settings[fieldId] ?? defaultValue)
}

const activeFields = computed(() =>
  WEIGHT_FIELDS.filter((field) => getWeight(field.id, field.defaultValue) > 0),
)

const availableFields = computed(() =>
  WEIGHT_FIELDS.filter((field) => getWeight(field.id, field.defaultValue) <= 0),
)

const totalWeight = computed(() =>
  WEIGHT_FIELDS.reduce((sum, field) => sum + getWeight(field.id, field.defaultValue), 0),
)

const totalClass = computed(() =>
  totalWeight.value === 100 ? 'score-weights-card__total--ok' : 'score-weights-card__total--warn',
)

function updateWeight(fieldId: string, event: Event) {
  const target = event.target as HTMLInputElement
  props.onChangeWeight(fieldId, target.value)
}

function removeWeight(field: WeightField) {
  props.onChangeWeight(field.id, '0')
}

function addWeight(field: WeightField) {
  props.onChangeWeight(field.id, String(field.defaultValue))
  addingWeight.value = false
}
</script>

<template>
  <article class="settings-card score-weights-card">
    <header class="score-weights-card__header">
      <div>
        <h3 class="settings-card__title">Pesos do Score 360</h3>
        <p class="settings-card__text">
          Componentes reais usados no ranking. Remover define peso zero; adicionar reativa uma
          metrica disponivel.
        </p>
      </div>

      <div class="score-weights-card__header-actions">
        <span :class="['score-weights-card__total', totalClass]">{{ totalWeight }}%</span>
        <button
          v-if="canEdit"
          class="score-weights-card__icon-button"
          type="button"
          :title="
            availableFields.length
              ? 'Adicionar componente ao Score 360'
              : 'Todas as metricas disponiveis ja estao ativas'
          "
          aria-label="Adicionar componente ao Score 360"
          :disabled="!availableFields.length"
          @click="addingWeight = !addingWeight"
        >
          <X v-if="addingWeight" :size="16" />
          <Plus v-else :size="16" />
        </button>
      </div>
    </header>

    <div v-if="addingWeight" class="score-weights-card__available">
      <span>Adicionar metrica:</span>
      <button
        v-for="field in availableFields"
        :key="field.id"
        type="button"
        @click="addWeight(field)"
      >
        <Plus :size="14" />
        {{ field.label }}
      </button>
    </div>

    <div class="score-weights-card__grid">
      <label v-for="field in activeFields" :key="field.id" class="score-weights-card__item">
        <span class="score-weights-card__item-label">{{ field.label }}</span>
        <span class="score-weights-card__input-wrap">
          <input
            type="number"
            min="0"
            max="100"
            step="1"
            :value="getWeight(field.id, field.defaultValue)"
            :disabled="!canEdit"
            @change="updateWeight(field.id, $event)"
          />
          <span>%</span>
          <button
            v-if="canEdit && activeFields.length > 1"
            type="button"
            title="Remover componente do Score 360"
            :aria-label="`Remover ${field.label} do Score 360`"
            @click.prevent="removeWeight(field)"
          >
            <Trash2 :size="14" />
          </button>
        </span>
      </label>
    </div>

    <p :class="['score-weights-card__hint', totalClass]">
      <template v-if="totalWeight === 100">Total correto para a escala de 0 a 100.</template>
      <template v-else>Ajuste os pesos para totalizar 100%.</template>
    </p>
  </article>
</template>

<style scoped>
.score-weights-card {
  display: grid;
  gap: 0.55rem;
  padding: 0.65rem;
}

.score-weights-card__header,
.score-weights-card__header-actions,
.score-weights-card__input-wrap,
.score-weights-card__available,
.score-weights-card__available button {
  display: flex;
  align-items: center;
}

.score-weights-card__header {
  justify-content: space-between;
  gap: 0.65rem;
}

.score-weights-card__header .settings-card__text {
  margin: 0.1rem 0 0;
  font-size: 0.7rem;
  line-height: 1.25;
}

.score-weights-card__header-actions,
.score-weights-card__input-wrap,
.score-weights-card__available,
.score-weights-card__available button {
  gap: 0.35rem;
}

.score-weights-card__total {
  font-size: 0.76rem;
  font-weight: 700;
}

.score-weights-card__icon-button,
.score-weights-card__input-wrap button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: var(--bg-muted);
  color: var(--text-main);
  cursor: pointer;
}

.score-weights-card__icon-button {
  width: 1.75rem;
  height: 1.75rem;
}

.score-weights-card__available {
  flex-wrap: wrap;
  padding: 0.4rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-soft);
  color: var(--text-muted);
  font-size: 0.7rem;
}

.score-weights-card__available button {
  border: 1px solid rgb(var(--primary) / 0.28);
  border-radius: 999px;
  padding: 0.25rem 0.45rem;
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
  cursor: pointer;
}

.score-weights-card__grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 0.4rem;
}

.score-weights-card__item {
  display: grid;
  gap: 0.25rem;
  min-width: 0;
  padding: 0.4rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface) / 0.55);
}

.score-weights-card__item-label {
  min-width: 0;
  overflow: hidden;
  color: var(--text-main);
  font-size: 0.72rem;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.score-weights-card__input-wrap input {
  min-width: 0;
  width: 100%;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  padding: 0.32rem 0.4rem;
  background: rgb(var(--surface-2) / 0.76);
  color: var(--text-main);
}

.score-weights-card__input-wrap > span {
  color: var(--text-muted);
  font-size: 0.7rem;
}

.score-weights-card__input-wrap button {
  width: 1.55rem;
  height: 1.55rem;
  flex: 0 0 1.55rem;
  color: rgb(var(--danger));
}

.score-weights-card__icon-button:hover,
.score-weights-card__input-wrap button:hover,
.score-weights-card__available button:hover {
  border-color: rgb(var(--primary) / 0.48);
  transform: translateY(-1px);
}

.score-weights-card__icon-button:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.score-weights-card__hint {
  margin: 0;
  font-size: 0.69rem;
  line-height: 1.2;
}

.score-weights-card__total--ok {
  color: rgb(var(--success));
}

.score-weights-card__total--warn {
  color: rgb(var(--danger));
}

@media (max-width: 1100px) {
  .score-weights-card__grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .score-weights-card__header {
    align-items: flex-start;
  }

  .score-weights-card__grid {
    grid-template-columns: 1fr;
  }
}
</style>
