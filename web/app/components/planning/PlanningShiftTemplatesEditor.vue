<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import { isShiftTemplateValid } from '~/domain/planning/scheduler'
import type {
  PlanningShiftTemplate,
  PlanningShiftTemplatesByLocationType,
  StoreLocationType,
  WorkShiftTemplateId,
} from '~/domain/planning/types'

const props = defineProps<{
  templatesByLocationType: PlanningShiftTemplatesByLocationType
  activeLocationType: StoreLocationType
  readonly?: boolean
}>()

const emit = defineEmits<{
  update: [
    locationType: StoreLocationType,
    templateId: WorkShiftTemplateId,
    patch: Pick<PlanningShiftTemplate, 'name'>,
  ]
}>()

const selectedLocationType = ref<StoreLocationType>(props.activeLocationType)
const selectedTemplates = computed(() => props.templatesByLocationType[selectedLocationType.value])

watch(
  () => props.activeLocationType,
  (value) => {
    selectedLocationType.value = value
  },
)

const templateKinds: Record<WorkShiftTemplateId, string> = {
  opening: 'Base de abertura',
  middle: 'Base intermediária',
  closing: 'Base de fechamento',
}

function textValue(event: Event): string {
  return (event.target as HTMLInputElement).value
}
</script>

<template>
  <details class="planning-shift-templates">
    <summary class="planning-shift-templates__summary">
      <span>
        <strong>Modelos de turno</strong>
        <small>Configurações independentes por tipo de loja</small>
      </span>
      <span class="planning-shift-templates__badge">2 perfis automáticos</span>
    </summary>

    <div class="planning-shift-templates__body">
      <div
        class="planning-shift-templates__profile-tabs"
        role="tablist"
        aria-label="Tipo de loja dos modelos de turno"
      >
        <button
          type="button"
          role="tab"
          :aria-selected="selectedLocationType === 'street'"
          :class="{ 'is-active': selectedLocationType === 'street' }"
          @click="selectedLocationType = 'street'"
        >
          Loja de rua
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="selectedLocationType === 'shopping'"
          :class="{ 'is-active': selectedLocationType === 'shopping' }"
          @click="selectedLocationType = 'shopping'"
        >
          Shopping
        </button>
      </div>

      <p>
        Editando o perfil de
        <strong>{{ selectedLocationType === 'shopping' ? 'shopping' : 'loja de rua' }}</strong>
        . Os horários são calculados pelo funcionamento predominante desse perfil e limitados em
        cada dia pelo expediente cadastrado.
      </p>

      <div class="planning-shift-templates__list">
        <article
          v-for="template in selectedTemplates"
          :key="template.id"
          class="planning-shift-templates__card"
          :class="{ 'has-error': !isShiftTemplateValid(template) }"
        >
          <header>
            <strong>{{ templateKinds[template.id] }}</strong>
            <small>{{ template.startsAt || '--:--' }}–{{ template.endsAt || '--:--' }}</small>
          </header>

          <div class="planning-shift-templates__fields">
            <label>
              <span>Nome exibido</span>
              <input
                :value="template.name"
                type="text"
                maxlength="40"
                :disabled="readonly"
                :aria-label="`Nome do modelo ${templateKinds[template.id]}`"
                @change="
                  emit('update', selectedLocationType, template.id, { name: textValue($event) })
                "
              />
            </label>
            <label>
              <span>Início</span>
              <output>{{ template.startsAt }}</output>
            </label>
            <label>
              <span>Fim</span>
              <output>{{ template.endsAt }}</output>
            </label>
          </div>

          <small v-if="!isShiftTemplateValid(template)" class="planning-shift-templates__error">
            O horário final precisa ser posterior ao horário inicial.
          </small>
        </article>
      </div>
    </div>
  </details>
</template>

<style scoped>
.planning-shift-templates {
  border: 1px solid rgb(var(--border) / 0.72);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.78);
  overflow: clip;
}

.planning-shift-templates__summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.85rem 1rem;
  cursor: pointer;
  list-style: none;
}

.planning-shift-templates__summary::-webkit-details-marker {
  display: none;
}

.planning-shift-templates__summary > span:first-child,
.planning-shift-templates__card header {
  display: grid;
  gap: 0.16rem;
}

.planning-shift-templates__summary strong,
.planning-shift-templates__card strong {
  color: var(--text-main);
  font-size: 0.86rem;
}

.planning-shift-templates__summary small,
.planning-shift-templates__card small,
.planning-shift-templates__body > p {
  color: var(--text-muted);
  font-size: 0.72rem;
}

.planning-shift-templates__badge {
  flex-shrink: 0;
  border-radius: 999px;
  padding: 0.28rem 0.58rem;
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
  font-size: 0.7rem;
  font-weight: 700;
}

.planning-shift-templates__body {
  display: grid;
  gap: 0.8rem;
  padding: 0.9rem 1rem 1rem;
  border-top: 1px solid rgb(var(--border) / 0.55);
}

.planning-shift-templates__body > p {
  margin: 0;
}

.planning-shift-templates__body > p strong {
  color: var(--text-main);
}

.planning-shift-templates__profile-tabs {
  display: inline-flex;
  align-items: center;
  justify-self: start;
  gap: 0.2rem;
  min-height: 2rem;
  padding: 0.15rem;
  border: 1px solid rgb(var(--ring) / 0.14);
  border-radius: 999px;
  background: rgb(var(--surface-2) / 0.72);
}

.planning-shift-templates__profile-tabs button {
  min-height: 1.7rem;
  border: none;
  border-radius: 999px;
  padding: 0 0.65rem;
  background: transparent;
  color: var(--text-muted);
  font-size: 0.68rem;
  font-weight: 800;
  cursor: pointer;
  white-space: nowrap;
}

.planning-shift-templates__profile-tabs button:hover {
  color: var(--text-main);
}

.planning-shift-templates__profile-tabs button.is-active {
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.planning-shift-templates__list {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.65rem;
}

.planning-shift-templates__card {
  display: grid;
  gap: 0.65rem;
  border: 1px solid rgb(var(--border) / 0.58);
  border-radius: 0.8rem;
  padding: 0.75rem;
  background: rgb(var(--surface-2) / 0.6);
}

.planning-shift-templates__card.has-error {
  border-color: rgb(var(--danger) / 0.58);
}

.planning-shift-templates__fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.5rem;
}

.planning-shift-templates__fields label:first-child {
  grid-column: 1 / -1;
}

.planning-shift-templates__fields label {
  display: grid;
  gap: 0.3rem;
  color: var(--text-muted);
  font-size: 0.68rem;
  font-weight: 700;
}

.planning-shift-templates__fields input,
.planning-shift-templates__fields output {
  box-sizing: border-box;
  width: 100%;
  min-width: 0;
  min-height: 2.2rem;
  border: 1px solid rgb(var(--border) / 0.78);
  border-radius: 0.65rem;
  padding: 0.42rem 0.55rem;
  background: rgb(var(--surface-2) / 0.84);
  color: var(--text-main);
  font: inherit;
}

.planning-shift-templates__fields output {
  display: inline-flex;
  align-items: center;
  font-weight: 800;
}

.planning-shift-templates__error {
  color: rgb(var(--danger)) !important;
}

@media (max-width: 1100px) {
  .planning-shift-templates__list {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .planning-shift-templates__summary {
    align-items: stretch;
    flex-direction: column;
  }

  .planning-shift-templates__badge {
    align-self: flex-start;
  }

  .planning-shift-templates__fields {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
