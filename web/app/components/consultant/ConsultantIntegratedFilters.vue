<script setup lang="ts">
import { CalendarDays } from 'lucide-vue-next'
import AppSelectField from '~/components/ui/AppSelectField.vue'

interface FilterOption {
  value: string
  label: string
}

withDefaults(
  defineProps<{
    searchTerm?: string
    storeFilter?: string
    statusFilter?: string
    goalFilter?: string
    storeOptions?: FilterOption[]
    statusOptions?: FilterOption[]
    goalOptions?: FilterOption[]
    dateFrom?: string
    dateTo?: string
    pending?: boolean
  }>(),
  {
    searchTerm: '',
    storeFilter: 'all',
    statusFilter: 'all',
    goalFilter: 'all',
    storeOptions: () => [],
    statusOptions: () => [],
    goalOptions: () => [],
    dateFrom: '',
    dateTo: '',
    pending: false,
  },
)

/* eslint-disable @typescript-eslint/unified-signatures -- nomes de evento literais
   exigidos pelo vue/require-explicit-emits (o plugin nao resolve unioes via type alias). */
const emit = defineEmits<{
  (e: 'update:search-term', value: string): void
  (e: 'update:store-filter', value: string): void
  (e: 'update:status-filter', value: string): void
  (e: 'update:goal-filter', value: string): void
  (e: 'update:date-from', value: string): void
  (e: 'update:date-to', value: string): void
  (e: 'apply'): void
  (e: 'reset-current-month'): void
  (e: 'set-previous-month'): void
}>()
/* eslint-enable @typescript-eslint/unified-signatures */
</script>

<template>
  <article class="settings-card consultant-integrated-filters">
    <div class="consultant-integrated-filters__grid">
      <label class="settings-field consultant-integrated-filters__search">
        <span>Buscar consultor</span>
        <input
          :value="searchTerm"
          type="text"
          placeholder="Nome, loja ou cargo"
          @input="emit('update:search-term', ($event.target as HTMLInputElement).value)"
        />
      </label>
      <label class="settings-field">
        <span>Loja</span>
        <AppSelectField
          :model-value="storeFilter"
          :options="storeOptions"
          placeholder="Filtrar loja"
          @update:model-value="emit('update:store-filter', $event)"
        />
      </label>
      <label class="settings-field">
        <span>Status</span>
        <AppSelectField
          :model-value="statusFilter"
          :options="statusOptions"
          placeholder="Filtrar status"
          @update:model-value="emit('update:status-filter', $event)"
        />
      </label>
      <label class="settings-field">
        <span>Meta</span>
        <AppSelectField
          :model-value="goalFilter"
          :options="goalOptions"
          placeholder="Filtrar meta"
          @update:model-value="emit('update:goal-filter', $event)"
        />
      </label>
      <div class="consultant-integrated-filters__period">
        <label class="settings-field consultant-integrated-filters__period-field">
          <span>Periodo</span>
          <AppDatePicker
            :model-value="dateFrom"
            :end-date="dateTo"
            @update:model-value="emit('update:date-from', $event)"
            @update:end-date="emit('update:date-to', $event)"
          >
            <template #default="{ label }">
              <button type="button" class="consultant-integrated-date-trigger">
                <CalendarDays :size="14" />
                <span>{{ label || 'Mes atual' }}</span>
              </button>
            </template>
          </AppDatePicker>
        </label>
        <div class="consultant-integrated-filters__actions">
          <button
            type="button"
            class="consultant-integrated-btn consultant-integrated-btn--ghost"
            :disabled="pending"
            @click="emit('set-previous-month')"
          >
            Mes anterior
          </button>
          <button
            type="button"
            class="consultant-integrated-btn consultant-integrated-btn--ghost"
            :disabled="pending"
            @click="emit('reset-current-month')"
          >
            Mes atual
          </button>
          <button
            type="button"
            class="consultant-integrated-btn"
            :disabled="pending"
            @click="emit('apply')"
          >
            {{ pending ? 'Atualizando...' : 'Atualizar' }}
          </button>
        </div>
      </div>
    </div>
  </article>
</template>

<style scoped>
.consultant-integrated-filters__grid {
  display: grid;
  grid-template-columns: minmax(0, 1.5fr) repeat(3, minmax(0, 1fr)) minmax(0, 1.4fr);
  gap: 0.85rem;
}
.consultant-integrated-filters__search {
  min-width: 0;
}
.consultant-integrated-filters__period {
  display: grid;
  gap: 0.65rem;
  min-width: 0;
}
.consultant-integrated-filters__period-field {
  min-width: 0;
}
.consultant-integrated-date-trigger {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 100%;
  min-height: 42px;
  padding: 0 0.85rem;
  border-radius: 12px;
  border: 1px solid rgb(var(--border) / 0.9);
  background: rgb(var(--surface) / 0.95);
  color: rgb(var(--text));
  font-size: 0.88rem;
  font-weight: 600;
  text-align: left;
  cursor: pointer;
  white-space: nowrap;
}
.consultant-integrated-filters__actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}
.consultant-integrated-btn {
  min-height: 38px;
  border: none;
  border-radius: 12px;
  padding: 0.6rem 0.9rem;
  background: rgb(var(--primary));
  color: rgb(255 255 255);
  font-weight: 700;
  cursor: pointer;
  white-space: nowrap;
}
.consultant-integrated-btn:disabled {
  cursor: wait;
  opacity: 0.72;
}
.consultant-integrated-btn--ghost {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}
@media (max-width: 1100px) {
  .consultant-integrated-filters__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 720px) {
  .consultant-integrated-filters__grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
