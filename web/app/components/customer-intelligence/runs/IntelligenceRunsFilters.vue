<script setup lang="ts">
import AppSelectField from '~/components/ui/AppSelectField.vue'
import type {
  IntelligenceRunsFilters,
  IntelligenceRunsFilterOptions,
} from '~/domain/customer-intelligence/runs-types'

defineProps<{
  filters: Omit<IntelligenceRunsFilters, 'clientAccountId' | 'cursor'>
  options: IntelligenceRunsFilterOptions
  loading: boolean
}>()

const emit = defineEmits<{
  update: [filters: Omit<IntelligenceRunsFilters, 'clientAccountId' | 'cursor'>]
  apply: []
}>()

function setFilter(
  current: Omit<IntelligenceRunsFilters, 'clientAccountId' | 'cursor'>,
  key: keyof Omit<IntelligenceRunsFilters, 'clientAccountId' | 'cursor'>,
  value: string,
): void {
  emit('update', { ...current, [key]: value })
}
</script>

<template>
  <div class="runs-filters">
    <AppSelectField
      :model-value="filters.status ?? ''"
      :options="[{ value: '', label: 'Todos os status' }, ...options.statuses]"
      label="Status"
      compact
      @update:model-value="setFilter(filters, 'status', $event)"
    />
    <AppSelectField
      :model-value="filters.processKey ?? ''"
      :options="[{ value: '', label: 'Todos os processos' }, ...options.processes]"
      label="Processo"
      compact
      @update:model-value="setFilter(filters, 'processKey', $event)"
    />
    <AppSelectField
      :model-value="filters.pipelineKey ?? ''"
      :options="[{ value: '', label: 'Todos os pipelines' }, ...options.pipelines]"
      label="Pipeline"
      compact
      @update:model-value="setFilter(filters, 'pipelineKey', $event)"
    />
    <AppSelectField
      :model-value="filters.executorType ?? ''"
      :options="[{ value: '', label: 'Todos os executores' }, ...options.executors]"
      label="Executor"
      compact
      @update:model-value="setFilter(filters, 'executorType', $event)"
    />
    <label>
      Inicio
      <input
        type="datetime-local"
        :value="filters.startedFrom ?? ''"
        @input="setFilter(filters, 'startedFrom', ($event.target as HTMLInputElement).value)"
      />
    </label>
    <label>
      Fim
      <input
        type="datetime-local"
        :value="filters.startedTo ?? ''"
        @input="setFilter(filters, 'startedTo', ($event.target as HTMLInputElement).value)"
      />
    </label>
    <button type="button" :disabled="loading" @click="emit('apply')">Aplicar</button>
  </div>
</template>

<style scoped>
.runs-filters {
  display: grid;
  grid-template-columns: repeat(4, minmax(9rem, 1fr)) auto;
  align-items: end;
  gap: 0.7rem;
}

.runs-filters label {
  display: grid;
  gap: 0.25rem;
  color: rgb(var(--muted));
  font-size: 0.7rem;
  font-weight: 700;
}

.runs-filters input {
  min-height: 2.3rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.6rem;
  background: rgb(var(--surface));
  color: inherit;
}

@media (max-width: 1000px) {
  .runs-filters {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
