<script setup lang="ts">
import { computed } from 'vue'
import { CalendarDays } from 'lucide-vue-next'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import {
  buildCurrentMonthRange,
  buildMonthWeekRange,
  buildPreviousMonthRange,
} from '~/domain/utils/consultant-transforms'

interface FilterOption {
  value: string
  label: string
}

const props = withDefaults(
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
  (e: 'set-week', value: number): void
}>()
/* eslint-enable @typescript-eslint/unified-signatures */

// Metas por semana: fatias fixas do mes (S1 1-7, S2 8-14, S3 15-21, S4 22-fim).
// O titulo vira tooltip; o botao ativo destaca a semana do periodo atual.
const WEEK_PRESETS = [
  { week: 1, label: 'S1', title: 'Semana 1 (dias 1 a 7)' },
  { week: 2, label: 'S2', title: 'Semana 2 (dias 8 a 14)' },
  { week: 3, label: 'S3', title: 'Semana 3 (dias 15 a 21)' },
  { week: 4, label: 'S4', title: 'Semana 4 (dia 22 ao fim do mes)' },
]

function rangeMatches(range: { dateFrom: string; dateTo: string }) {
  return range.dateFrom === props.dateFrom && range.dateTo === props.dateTo
}
// Detecta qual atalho corresponde ao range ativo (destaque visual). A semana usa o
// mes do proprio dateFrom como ancora, entao respeita "Mes anterior".
const activeWeek = computed(() => {
  for (const preset of WEEK_PRESETS) {
    if (rangeMatches(buildMonthWeekRange(props.dateFrom, preset.week))) return preset.week
  }
  return 0
})
const isCurrentMonth = computed(() => rangeMatches(buildCurrentMonthRange()))
const isPreviousMonth = computed(() => rangeMatches(buildPreviousMonthRange()))
</script>

<template>
  <article class="settings-card consultant-integrated-filters">
    <div class="consultant-integrated-filters__bar">
      <label
        class="settings-field consultant-integrated-filters__field consultant-integrated-filters__field--search"
      >
        <span>Buscar consultor</span>
        <input
          :value="searchTerm"
          type="text"
          placeholder="Nome, loja ou cargo"
          @input="emit('update:search-term', ($event.target as HTMLInputElement).value)"
        />
      </label>
      <label class="settings-field consultant-integrated-filters__field">
        <span>Loja</span>
        <AppSelectField
          :model-value="storeFilter"
          :options="storeOptions"
          placeholder="Filtrar loja"
          @update:model-value="emit('update:store-filter', $event)"
        />
      </label>
      <label class="settings-field consultant-integrated-filters__field">
        <span>Status</span>
        <AppSelectField
          :model-value="statusFilter"
          :options="statusOptions"
          placeholder="Filtrar status"
          @update:model-value="emit('update:status-filter', $event)"
        />
      </label>
      <label class="settings-field consultant-integrated-filters__field">
        <span>Meta</span>
        <AppSelectField
          :model-value="goalFilter"
          :options="goalOptions"
          placeholder="Filtrar meta"
          @update:model-value="emit('update:goal-filter', $event)"
        />
      </label>
      <label
        class="settings-field consultant-integrated-filters__field consultant-integrated-filters__field--period"
      >
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

      <div
        class="consultant-integrated-filters__field consultant-integrated-filters__actions-field"
      >
        <span>Semana da meta</span>
        <div class="consultant-integrated-filters__actions">
          <div class="consultant-integrated-weeks" role="group" aria-label="Semana da meta">
            <button
              v-for="preset in WEEK_PRESETS"
              :key="preset.week"
              type="button"
              class="consultant-integrated-week"
              :class="{ 'consultant-integrated-week--active': activeWeek === preset.week }"
              :title="preset.title"
              :aria-pressed="activeWeek === preset.week"
              :disabled="pending"
              @click="emit('set-week', preset.week)"
            >
              {{ preset.label }}
            </button>
          </div>
          <button
            type="button"
            class="consultant-integrated-btn consultant-integrated-btn--ghost"
            :class="{ 'consultant-integrated-btn--active': isPreviousMonth }"
            :aria-pressed="isPreviousMonth"
            :disabled="pending"
            @click="emit('set-previous-month')"
          >
            Mes anterior
          </button>
          <button
            type="button"
            class="consultant-integrated-btn consultant-integrated-btn--ghost"
            :class="{ 'consultant-integrated-btn--active': isCurrentMonth }"
            :aria-pressed="isCurrentMonth"
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
/* Tudo numa linha so: campos + periodo + atalhos (semana/mes/atualizar) alinhados
   pela base. Em telas estreitas quebra sozinho (flex-wrap) para seguir usavel. */
.consultant-integrated-filters__bar {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0.7rem;
}
.consultant-integrated-filters__field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  min-width: 0;
  flex: 0 1 8.5rem;
}
.consultant-integrated-filters__field > span {
  font-size: 0.78rem;
  color: rgb(var(--muted));
  white-space: nowrap;
}
.consultant-integrated-filters__field--search {
  flex: 1 1 11rem;
}
.consultant-integrated-filters__field--period {
  flex: 0 1 9.5rem;
}
.consultant-integrated-filters__actions-field {
  flex: 1 1 auto;
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
  align-items: center;
  gap: 0.4rem;
  flex-wrap: wrap;
}
/* Grupo segmentado das 4 semanas: botoes colados, o ativo pinta de primary. */
.consultant-integrated-weeks {
  display: inline-flex;
  border: 1px solid rgb(var(--border) / 0.9);
  border-radius: 12px;
  overflow: hidden;
}
.consultant-integrated-week {
  min-height: 38px;
  min-width: 2.6rem;
  padding: 0 0.55rem;
  border: none;
  border-left: 1px solid rgb(var(--border) / 0.9);
  background: rgb(var(--surface) / 0.95);
  color: rgb(var(--text));
  font-weight: 700;
  font-size: 0.82rem;
  cursor: pointer;
}
.consultant-integrated-week:first-child {
  border-left: none;
}
.consultant-integrated-week:hover:not(:disabled) {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}
.consultant-integrated-week--active {
  background: rgb(var(--primary));
  color: rgb(255 255 255);
}
.consultant-integrated-week:disabled {
  cursor: wait;
  opacity: 0.72;
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
.consultant-integrated-btn--active {
  background: rgb(var(--primary));
  color: rgb(255 255 255);
}
@media (max-width: 720px) {
  .consultant-integrated-filters__field {
    flex: 1 1 100%;
  }
  .consultant-integrated-filters__actions {
    width: 100%;
  }
}
</style>
