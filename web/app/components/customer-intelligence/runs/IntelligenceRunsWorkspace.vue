<script setup lang="ts">
import CustomerIntelligenceStatus from '~/components/customer-intelligence/CustomerIntelligenceStatus.vue'
import type {
  IntelligenceRunsFilters as IntelligenceRunsFilterState,
  RuntimeRunListItem,
} from '~/domain/customer-intelligence/runs-types'
import { useIntelligenceRuns } from '~/composables/customer-intelligence/useIntelligenceRuns'
import IntelligenceRunDrawer from './IntelligenceRunDrawer.vue'
import IntelligenceRunsFilters from './IntelligenceRunsFilters.vue'
import IntelligenceRunsTable from './IntelligenceRunsTable.vue'

const runs = useIntelligenceRuns()
const filters = ref<Omit<IntelligenceRunsFilterState, 'clientAccountId' | 'cursor'>>({})
const selected = ref<RuntimeRunListItem | null>(null)
const drawerOpen = ref(false)

function open(run: RuntimeRunListItem): void {
  selected.value = run
  drawerOpen.value = true
}
</script>

<template>
  <div class="runs-workspace">
    <IntelligenceRunsFilters
      :filters="filters"
      :options="runs.options.value"
      :loading="runs.loading.value"
      @update="filters = $event"
      @apply="runs.load(filters)"
    />
    <CustomerIntelligenceStatus
      v-if="runs.error.value"
      title="Runs indisponiveis"
      :error="runs.error.value"
      @retry="runs.load(filters)"
    />
    <IntelligenceRunsTable
      v-else
      :items="runs.items.value"
      :loading="runs.loading.value"
      @open="open"
    />
    <button
      v-if="runs.nextCursor.value"
      type="button"
      :disabled="runs.loading.value"
      @click="runs.load(filters, true)"
    >
      Carregar mais
    </button>
    <IntelligenceRunDrawer v-model:open="drawerOpen" :run="selected" />
  </div>
</template>

<style scoped>
.runs-workspace {
  display: grid;
  gap: 1rem;
}

.runs-workspace > button {
  justify-self: center;
}
</style>
