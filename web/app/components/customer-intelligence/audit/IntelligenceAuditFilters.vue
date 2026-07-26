<script setup lang="ts">
import AppSelectField from '~/components/ui/AppSelectField.vue'
import type {
  IntelligenceAuditFilters,
  IntelligenceAuditPage,
} from '~/domain/customer-intelligence/audit-types'

defineProps<{
  filters: Omit<IntelligenceAuditFilters, 'clientAccountId' | 'cursor'>
  options: IntelligenceAuditPage['filterOptions']
  loading: boolean
}>()
const emit = defineEmits<{
  update: [filters: Omit<IntelligenceAuditFilters, 'clientAccountId' | 'cursor'>]
  apply: []
}>()

function change(
  filters: Omit<IntelligenceAuditFilters, 'clientAccountId' | 'cursor'>,
  key: keyof Omit<IntelligenceAuditFilters, 'clientAccountId' | 'cursor'>,
  value: string,
): void {
  emit('update', { ...filters, [key]: value })
}
</script>

<template>
  <div class="audit-filters">
    <AppSelectField
      :model-value="filters.action ?? ''"
      :options="[{ value: '', label: 'Todas as acoes' }, ...options.actions]"
      label="Acao"
      compact
      @update:model-value="change(filters, 'action', $event)"
    />
    <AppSelectField
      :model-value="filters.entityType ?? ''"
      :options="[{ value: '', label: 'Todas as entidades' }, ...options.entityTypes]"
      label="Entidade"
      compact
      @update:model-value="change(filters, 'entityType', $event)"
    />
    <label>
      <span>De</span>
      <input
        type="datetime-local"
        :value="filters.occurredFrom ?? ''"
        @input="change(filters, 'occurredFrom', ($event.target as HTMLInputElement).value)"
      />
    </label>
    <label>
      <span>Ate</span>
      <input
        type="datetime-local"
        :value="filters.occurredTo ?? ''"
        @input="change(filters, 'occurredTo', ($event.target as HTMLInputElement).value)"
      />
    </label>
    <button type="button" :disabled="loading" @click="emit('apply')">Aplicar</button>
  </div>
</template>

<style scoped>
.audit-filters {
  display: grid;
  grid-template-columns: repeat(4, minmax(9rem, 1fr)) auto;
  align-items: end;
  gap: 0.7rem;
}

.audit-filters label {
  display: grid;
  gap: 0.3rem;
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.audit-filters input {
  min-height: 2.35rem;
  padding: 0.45rem 0.6rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 0.55rem;
  background: rgb(var(--surface-1));
  color: rgb(var(--text));
}

@media (max-width: 900px) {
  .audit-filters {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
