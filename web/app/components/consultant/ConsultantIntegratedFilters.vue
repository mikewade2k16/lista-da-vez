<script setup lang="ts">
import { computed } from 'vue'

import PerformanceFeedbackSettingsButton from '~/components/performance-feedback/PerformanceFeedbackSettingsButton.vue'
import AppDateRangeFilter from '~/components/ui/AppDateRangeFilter.vue'
import AppFilterField from '~/components/ui/AppFilterField.vue'
import AppFilterToolbar from '~/components/ui/AppFilterToolbar.vue'
import AppGoalPeriodFilter from '~/components/ui/AppGoalPeriodFilter.vue'
import AppSearchInput from '~/components/ui/AppSearchInput.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import AppToolbarButton from '~/components/ui/AppToolbarButton.vue'
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
    feedbackStoreId?: string
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
    feedbackStoreId: '',
    pending: false,
  },
)

const periodMonth = computed(() => props.dateFrom.slice(0, 7))

/* eslint-disable @typescript-eslint/unified-signatures -- nomes de evento literais
   exigidos pelo vue/require-explicit-emits (o plugin nao resolve unioes via type alias). */
const emit = defineEmits<{
  (event: 'update:search-term', value: string): void
  (event: 'update:store-filter', value: string): void
  (event: 'update:status-filter', value: string): void
  (event: 'update:goal-filter', value: string): void
  (event: 'update:date-from', value: string): void
  (event: 'update:date-to', value: string): void
  (event: 'apply'): void
  (event: 'reset-current-month'): void
  (event: 'set-previous-month'): void
  (event: 'set-week', value: number): void
}>()
/* eslint-enable @typescript-eslint/unified-signatures */

function rangeMatches(range: { dateFrom: string; dateTo: string }): boolean {
  return range.dateFrom === props.dateFrom && range.dateTo === props.dateTo
}

const activeWeek = computed(() => {
  for (let week = 1; week <= 4; week += 1) {
    if (rangeMatches(buildMonthWeekRange(props.dateFrom, week))) return `p${week}`
  }
  return 'none'
})
const isCurrentMonth = computed(() => rangeMatches(buildCurrentMonthRange()))
const isPreviousMonth = computed(() => rangeMatches(buildPreviousMonthRange()))

function selectWeek(value: string): void {
  const week = Number(value.replace(/^p/, ''))
  if (week >= 1 && week <= 4) emit('set-week', week)
}
</script>

<template>
  <AppFilterToolbar class="consultant-filters" aria-label="Filtros dos consultores">
    <AppFilterField class="consultant-filters__search" label="Buscar consultor">
      <AppSearchInput
        :model-value="searchTerm"
        placeholder="Nome, loja ou cargo"
        aria-label="Buscar consultor"
        :debounce-ms="0"
        compact
        @update:model-value="emit('update:search-term', $event)"
      />
    </AppFilterField>

    <AppFilterField class="consultant-filters__select" label="Loja">
      <AppSelectField
        :model-value="storeFilter"
        :options="storeOptions"
        placeholder="Todas as lojas"
        :show-leading-icon="false"
        compact
        @update:model-value="emit('update:store-filter', $event)"
      />
    </AppFilterField>

    <AppFilterField class="consultant-filters__select" label="Status">
      <AppSelectField
        :model-value="statusFilter"
        :options="statusOptions"
        placeholder="Todos os status"
        :show-leading-icon="false"
        compact
        @update:model-value="emit('update:status-filter', $event)"
      />
    </AppFilterField>

    <AppFilterField class="consultant-filters__select" label="Meta">
      <AppSelectField
        :model-value="goalFilter"
        :options="goalOptions"
        placeholder="Todas as metas"
        :show-leading-icon="false"
        compact
        @update:model-value="emit('update:goal-filter', $event)"
      />
    </AppFilterField>

    <AppFilterField class="consultant-filters__period" label="Período">
      <AppDateRangeFilter
        :model-value="dateFrom"
        :end-date="dateTo"
        placeholder="Mês atual"
        :disabled="pending"
        @update:model-value="emit('update:date-from', $event)"
        @update:end-date="emit('update:date-to', $event)"
      />
    </AppFilterField>

    <AppFilterField class="consultant-filters__weeks" label="Semana da meta">
      <AppGoalPeriodFilter
        :month="periodMonth"
        :model-value="activeWeek"
        :include-month="false"
        aria-label="Semana da meta"
        :disabled="pending"
        @update:model-value="selectWeek"
      />
    </AppFilterField>

    <template #actions>
      <AppToolbarButton
        label="Mês anterior"
        variant="ghost"
        :active="isPreviousMonth"
        :disabled="pending"
        @click="emit('set-previous-month')"
      />
      <AppToolbarButton
        label="Mês atual"
        variant="ghost"
        :active="isCurrentMonth"
        :disabled="pending"
        @click="emit('reset-current-month')"
      />
      <AppToolbarButton
        :label="pending ? 'Atualizando...' : 'Atualizar'"
        icon="i-lucide-refresh-cw"
        variant="primary"
        :loading="pending"
        @click="emit('apply')"
      />
      <PerformanceFeedbackSettingsButton :store-id="feedbackStoreId" :disabled="pending" />
    </template>
  </AppFilterToolbar>
</template>

<style scoped>
.consultant-filters__search {
  flex: 1 1 14rem;
  min-width: 11rem;
}

.consultant-filters__select {
  flex: 0 1 8.5rem;
  min-width: 7.5rem;
}

.consultant-filters__period {
  flex: 0 0 11.5rem;
  min-width: 11.5rem;
}

.consultant-filters__weeks {
  flex: 0 0 auto;
}

@media (max-width: 720px) {
  .consultant-filters__search,
  .consultant-filters__select,
  .consultant-filters__period {
    flex: 1 1 100%;
    width: 100%;
  }
}
</style>
