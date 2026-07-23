<script setup lang="ts">
import { Plus, Trash2 } from 'lucide-vue-next'

import AppSelectField from '~/components/ui/AppSelectField.vue'
import { biApiFieldLabel } from '~/domain/bi/api-catalog'
import { PEROLA_OPERATOR_LABELS, createEmptyPerolaFilterDraft } from '~/domain/bi/perola-query'
import type {
  PerolaDatasetCatalogItem,
  PerolaDatasetFilterRule,
  PerolaFilterDraft,
} from '~/domain/bi/perola-query'

const props = withDefaults(
  defineProps<{
    dataset: PerolaDatasetCatalogItem
    modelValue: PerolaFilterDraft[]
    disabled?: boolean
  }>(),
  {
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: PerolaFilterDraft[]]
}>()

const booleanOptions = [
  { value: 'true', label: 'Sim' },
  { value: 'false', label: 'Não' },
]

function filterRule(field: string): PerolaDatasetFilterRule | undefined {
  return props.dataset.filters.find((filter) => filter.field === field)
}

function fieldOptions() {
  return props.dataset.filters.map((filter) => ({
    value: filter.field,
    label: biApiFieldLabel(filter.field),
    meta: filter.valueType,
  }))
}

function operatorOptions(draft: PerolaFilterDraft) {
  return (filterRule(draft.field)?.operators || []).map((operator) => ({
    value: operator,
    label: PEROLA_OPERATOR_LABELS[operator] || operator,
  }))
}

function replaceDraft(index: number, patch: Partial<PerolaFilterDraft>) {
  emit(
    'update:modelValue',
    props.modelValue.map((draft, draftIndex) =>
      draftIndex === index ? { ...draft, ...patch } : draft,
    ),
  )
}

function updateField(index: number, field: string) {
  const rule = filterRule(field)
  replaceDraft(index, {
    field,
    operator: rule?.operators[0] || 'eq',
    value: '',
  })
}

function updateValue(index: number, event: Event) {
  const target = event.target as HTMLInputElement | null
  replaceDraft(index, { value: String(target?.value || '') })
}

function addFilter() {
  if (props.modelValue.length >= 8) return
  emit('update:modelValue', [
    ...props.modelValue,
    createEmptyPerolaFilterDraft(props.dataset, props.modelValue.length),
  ])
}

function removeFilter(index: number) {
  emit(
    'update:modelValue',
    props.modelValue.filter((_, draftIndex) => draftIndex !== index),
  )
}
</script>

<template>
  <section class="bi-query-filters">
    <header>
      <div>
        <h4>Filtros da consulta</h4>
        <p>{{ dataset.requiredFilterRule }}</p>
      </div>
      <button
        class="bi-query-filters__add"
        type="button"
        :disabled="disabled || modelValue.length >= 8"
        @click="addFilter"
      >
        <Plus :size="15" aria-hidden="true" />
        Adicionar filtro
      </button>
    </header>

    <div v-if="modelValue.length" class="bi-query-filters__list">
      <article v-for="(draft, index) in modelValue" :key="draft.id">
        <AppSelectField
          :model-value="draft.field"
          label="Campo"
          :options="fieldOptions()"
          :disabled="disabled"
          searchable
          compact
          @update:model-value="updateField(index, $event)"
        />

        <AppSelectField
          :model-value="draft.operator"
          label="Operador"
          :options="operatorOptions(draft)"
          :disabled="disabled"
          compact
          @update:model-value="replaceDraft(index, { operator: $event, value: '' })"
        />

        <AppSelectField
          v-if="filterRule(draft.field)?.valueType === 'boolean'"
          :model-value="draft.value"
          label="Valor"
          :options="booleanOptions"
          :disabled="disabled"
          placeholder="Selecione"
          compact
          @update:model-value="replaceDraft(index, { value: $event })"
        />

        <label v-else class="bi-query-filters__value">
          <span>Valor</span>
          <input
            :value="draft.value"
            :type="
              filterRule(draft.field)?.valueType === 'date'
                ? 'date'
                : filterRule(draft.field)?.valueType === 'integer'
                  ? 'number'
                  : 'text'
            "
            :min="filterRule(draft.field)?.valueType === 'integer' ? 1 : undefined"
            :placeholder="
              filterRule(draft.field)?.valueType === 'integer'
                ? 'Inteiro positivo'
                : 'Digite o valor'
            "
            :disabled="disabled"
            @input="updateValue(index, $event)"
          />
        </label>

        <button
          class="bi-query-filters__remove"
          type="button"
          :disabled="disabled"
          aria-label="Remover filtro"
          @click="removeFilter(index)"
        >
          <Trash2 :size="16" aria-hidden="true" />
        </button>
      </article>
    </div>

    <div v-else class="bi-query-filters__empty">
      Adicione uma alternativa de filtro seletivo antes de consultar.
    </div>
  </section>
</template>

<style scoped>
.bi-query-filters {
  display: grid;
  gap: 0.75rem;
}

.bi-query-filters > header,
.bi-query-filters__add,
.bi-query-filters__list article {
  display: flex;
  align-items: center;
  gap: 0.65rem;
}

.bi-query-filters > header {
  justify-content: space-between;
}

.bi-query-filters h4 {
  margin: 0;
  color: var(--text-main);
  font-size: 0.9rem;
}

.bi-query-filters p {
  margin: 0.2rem 0 0;
  color: var(--text-muted);
  font-size: 0.78rem;
}

.bi-query-filters__add,
.bi-query-filters__remove {
  min-height: 2.25rem;
  border: 1px solid var(--line-soft);
  background: var(--bg-panel);
  color: var(--text-main);
  cursor: pointer;
}

.bi-query-filters__add {
  justify-content: center;
  padding: 0 0.8rem;
  border-radius: 999px;
  font-weight: 750;
}

.bi-query-filters__list {
  display: grid;
  gap: 0.55rem;
}

.bi-query-filters__list article {
  display: grid;
  grid-template-columns: minmax(180px, 1fr) minmax(150px, 0.7fr) minmax(180px, 1fr) auto;
  align-items: end;
  padding: 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--bg-panel) 88%, transparent);
}

.bi-query-filters__value {
  display: grid;
  gap: 0.3rem;
}

.bi-query-filters__value span {
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.bi-query-filters__value input {
  min-height: 2.25rem;
  padding: 0 0.75rem;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  outline: none;
  background: var(--bg-panel);
  color: var(--text-main);
}

.bi-query-filters__value input:focus {
  border-color: color-mix(in srgb, var(--accent-info) 55%, var(--line-soft));
}

.bi-query-filters__remove {
  display: inline-grid;
  place-items: center;
  width: 2.25rem;
  padding: 0;
  border-radius: 50%;
  color: var(--accent-danger);
}

.bi-query-filters__add:disabled,
.bi-query-filters__remove:disabled,
.bi-query-filters__value input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.bi-query-filters__empty {
  padding: 1rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-md);
  color: var(--text-muted);
  font-size: 0.8rem;
  text-align: center;
}

@media (max-width: 820px) {
  .bi-query-filters__list article {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 560px) {
  .bi-query-filters > header {
    align-items: stretch;
    flex-direction: column;
  }

  .bi-query-filters__list article {
    grid-template-columns: 1fr;
  }

  .bi-query-filters__remove {
    width: 100%;
    border-radius: 999px;
  }
}
</style>
