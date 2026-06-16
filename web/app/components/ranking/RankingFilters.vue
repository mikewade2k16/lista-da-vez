<script setup lang="ts">
import { CalendarDays } from 'lucide-vue-next'
import AppSelectField from '~/components/ui/AppSelectField.vue'

interface SelectOption {
  value: string
  label: string
}

withDefaults(
  defineProps<{
    dateFrom?: string
    dateTo?: string
    searchTerm: string
    storeFilter: string
    metric: string
    storeOptions?: SelectOption[]
    metricOptions?: SelectOption[]
    integratedScope?: boolean
    pending?: boolean
  }>(),
  {
    dateFrom: '',
    dateTo: '',
    storeOptions: () => [],
    metricOptions: () => [],
    integratedScope: false,
    pending: false,
  },
)

const emit = defineEmits<{
  'update:dateFrom': [value: string]
  'update:dateTo': [value: string]
  'update:searchTerm': [value: string]
  'update:storeFilter': [value: string]
  'update:metric': [value: string]
  applyPeriod: []
  setCurrentMonth: []
  setPreviousMonth: []
}>()
</script>

<template>
  <form class="settings-card ranking-filters" @submit.prevent="emit('applyPeriod')">
    <div class="ranking-filters__grid">
      <div class="settings-field ranking-filters__period">
        <span>Periodo</span>
        <AppDatePicker
          :model-value="dateFrom"
          :end-date="dateTo"
          @update:model-value="emit('update:dateFrom', $event)"
          @update:end-date="emit('update:dateTo', $event)"
        >
          <template #default="{ label }">
            <button type="button" class="ranking-filters__date-trigger">
              <CalendarDays :size="14" />
              <span>{{ label || 'Todas as datas' }}</span>
            </button>
          </template>
        </AppDatePicker>
      </div>

      <div class="ranking-filters__period-actions">
        <button
          class="ranking-filters__btn ranking-filters__btn--ghost"
          type="button"
          @click="emit('setPreviousMonth')"
        >
          Mes anterior
        </button>
        <button
          class="ranking-filters__btn ranking-filters__btn--ghost"
          type="button"
          @click="emit('setCurrentMonth')"
        >
          Mes atual
        </button>
      </div>

      <label class="settings-field ranking-filters__search">
        <span>Buscar</span>
        <input
          type="text"
          placeholder="Consultor ou loja"
          :value="searchTerm"
          @input="emit('update:searchTerm', ($event.target as HTMLInputElement).value)"
        />
      </label>

      <label v-if="integratedScope" class="settings-field">
        <span>Loja</span>
        <AppSelectField
          :model-value="storeFilter"
          :options="storeOptions"
          placeholder="Todas as lojas"
          @update:model-value="emit('update:storeFilter', String($event || 'all'))"
        />
      </label>

      <label class="settings-field">
        <span>Ordenar por</span>
        <AppSelectField
          :model-value="metric"
          :options="metricOptions"
          placeholder="Score 360"
          @update:model-value="emit('update:metric', String($event || 'score360'))"
        />
      </label>

      <div class="ranking-filters__submit">
        <button class="ranking-filters__btn" type="submit" :disabled="pending">
          {{ pending ? 'Atualizando...' : 'Atualizar' }}
        </button>
      </div>
    </div>
  </form>
</template>

<style scoped>
.ranking-filters__grid {
  display: grid;
  grid-template-columns:
    minmax(13rem, 0.95fr)
    auto
    minmax(12rem, 1.3fr)
    repeat(2, minmax(10rem, 0.85fr))
    auto;
  align-items: end;
  gap: 0.85rem;
}

.ranking-filters__period {
  min-width: 0;
}

.ranking-filters__period-actions {
  display: flex;
  gap: 0.45rem;
  align-items: end;
}

.ranking-filters__date-trigger {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  width: 100%;
  min-height: 42px;
  padding: 0 0.85rem;
  border-radius: 12px;
  border: 1px solid rgb(var(--border) / 0.9);
  background: rgb(var(--surface) / 0.95);
  color: rgb(var(--text));
  font-size: 0.88rem;
  font-weight: 700;
  text-align: left;
  cursor: pointer;
  white-space: nowrap;
}

.ranking-filters__btn {
  min-height: 42px;
  padding: 0 0.95rem;
  border: none;
  border-radius: 12px;
  background: rgb(var(--primary));
  color: rgb(255 255 255);
  font-weight: 800;
  cursor: pointer;
  white-space: nowrap;
}

.ranking-filters__btn:disabled {
  cursor: wait;
  opacity: 0.72;
}

.ranking-filters__btn--ghost {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.ranking-filters__submit {
  display: flex;
  justify-content: flex-end;
}

.ranking-filters__search {
  min-width: 0;
}

.ranking-filters__search input {
  width: 100%;
}

@media (max-width: 980px) {
  .ranking-filters__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .ranking-filters__period-actions,
  .ranking-filters__submit {
    justify-content: flex-start;
  }
}

@media (max-width: 720px) {
  .ranking-filters__grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
