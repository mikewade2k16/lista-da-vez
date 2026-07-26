<script setup lang="ts">
import AppEntityGrid from '~/components/ui/AppEntityGrid.vue'
import type { RuntimeRunListItem } from '~/domain/customer-intelligence/runs-types'

defineProps<{
  items: RuntimeRunListItem[]
  loading: boolean
}>()
const emit = defineEmits<{ open: [run: RuntimeRunListItem] }>()

const columns = [
  { id: 'process', label: 'Processo', width: 'minmax(180px, 1fr)', locked: true },
  { id: 'status', label: 'Status', width: 'minmax(100px, 0.55fr)' },
  { id: 'timing', label: 'Duracao', width: 'minmax(110px, 0.6fr)' },
  { id: 'usage', label: 'Uso/custo', width: 'minmax(140px, 0.7fr)' },
  { id: 'executor', label: 'Executor', width: 'minmax(150px, 0.8fr)' },
  { id: 'started', label: 'Inicio', width: 'minmax(150px, 0.8fr)' },
  { id: 'actions', label: '', width: 'minmax(80px, 0.4fr)', align: 'end' },
]

function runRow(row: unknown): RuntimeRunListItem {
  return row as RuntimeRunListItem
}
</script>

<template>
  <AppEntityGrid
    :columns="columns"
    :rows="items"
    row-key="id"
    :show-search="false"
    :loading="loading"
    empty-title="Nenhum run encontrado"
    empty-text="A API nao retornou execucoes para os filtros selecionados."
  >
    <template #cell-process="{ row }">
      <div class="run-cell">
        <strong>
          {{ runRow(row).processKey || runRow(row).pipelineKey || 'processo nao informado' }}
        </strong>
        <small>{{ runRow(row).id.slice(0, 12) }}</small>
      </div>
    </template>
    <template #cell-status="{ row }">{{ runRow(row).status }}</template>
    <template #cell-timing="{ row }">
      {{ runRow(row).durationMs != null ? `${runRow(row).durationMs} ms` : '—' }}
    </template>
    <template #cell-usage="{ row }">
      {{ runRow(row).inputUnits ?? 0 }} + {{ runRow(row).outputUnits ?? 0 }}
      <small v-if="runRow(row).costAmount != null">
        · {{ runRow(row).currency || '' }} {{ runRow(row).costAmount }}
      </small>
    </template>
    <template #cell-executor="{ row }">
      {{ runRow(row).providerName || runRow(row).executorType || '—' }}
      <small v-if="runRow(row).modelName">· {{ runRow(row).modelName }}</small>
    </template>
    <template #cell-started="{ row }">
      {{
        runRow(row).startedAt
          ? new Date(runRow(row).startedAt as string).toLocaleString('pt-BR')
          : '—'
      }}
    </template>
    <template #cell-actions="{ row }">
      <button type="button" @click="emit('open', runRow(row))">Detalhes</button>
    </template>
  </AppEntityGrid>
</template>

<style scoped>
.run-cell {
  display: grid;
  gap: 0.15rem;
}

small {
  color: rgb(var(--muted));
  font-size: 0.68rem;
}
</style>
