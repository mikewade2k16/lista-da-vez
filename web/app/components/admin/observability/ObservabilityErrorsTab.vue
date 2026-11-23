<script setup lang="ts">
import AppSearchInput from '~/components/ui/AppSearchInput.vue'
import AppSegmentedFilter from '~/components/ui/AppSegmentedFilter.vue'
import ObservabilityState from '~/components/admin/observability/ObservabilityState.vue'
import { useObsQuery } from '~/composables/useObservability'
import type { ObsErrorGroup } from '~/types/observability'
import { formatDate, formatInt } from '~/utils/observability/format'

const props = defineProps<{ days: number }>()
const emit = defineEmits<{ trace: [id: string] }>()

const search = ref('')
const source = ref('all')
const SOURCES = [
  { value: 'all', label: 'Todos' },
  { value: 'api', label: 'API' },
  { value: 'front', label: 'Front' },
]

const { data, loading, error, reload } = useObsQuery<ObsErrorGroup[]>('/errors', () => ({
  days: props.days,
}))

const MAX_IDS = 5
const rows = computed(() => {
  const term = search.value.trim().toLowerCase()
  return (data.value || []).filter((row) => {
    if (source.value !== 'all' && row.source !== source.value) return false
    if (!term) return true
    return `${row.signature} ${row.route || ''} ${row.page || ''}`.toLowerCase().includes(term)
  })
})
</script>

<template>
  <div class="obs-stack">
    <div class="obs-toolbar">
      <AppSearchInput v-model="search" placeholder="Buscar erro ou rota" aria-label="Buscar erro" />
      <AppSegmentedFilter v-model="source" :options="SOURCES" aria-label="Origem do erro" />
    </div>

    <ObservabilityState
      :loading="loading"
      :error="error"
      :empty="!data || data.length === 0"
      empty-title="Nenhum erro neste periodo."
      empty-hint="Erros 5xx da API e erros JS do front aparecem aqui, agrupados por assinatura."
      @retry="reload"
    >
      <p v-if="rows.length === 0" class="obs-hint">Nenhum resultado para o filtro atual.</p>
      <div v-else class="obs-tablewrap">
        <table class="obs-table">
          <thead>
            <tr>
              <th>Origem</th>
              <th>Assinatura</th>
              <th>Local</th>
              <th class="obs-num">Ocorrencias</th>
              <th>Ultima</th>
              <th>Correlation IDs</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in rows"
              :key="`${row.source}-${row.signature}-${row.route || row.page}`"
            >
              <td>
                <span
                  :class="['obs-band', row.source === 'api' ? 'obs-band--bad' : 'obs-band--warn']"
                >
                  {{ row.source === 'api' ? 'API' : 'Front' }}
                </span>
              </td>
              <td class="obs-clip" :title="row.signature">{{ row.signature }}</td>
              <td class="obs-mono">{{ row.route || row.page || '-' }}</td>
              <td class="obs-num">{{ formatInt(row.count) }}</td>
              <td>{{ formatDate(row.lastSeen) }}</td>
              <td>
                <button
                  v-for="id in (row.correlationIds || []).slice(0, MAX_IDS)"
                  :key="id"
                  type="button"
                  class="obs-link"
                  @click="emit('trace', id)"
                >
                  {{ id }}
                </button>
                <span v-if="(row.correlationIds || []).length > MAX_IDS" class="obs-muted">
                  +{{ row.correlationIds.length - MAX_IDS }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </ObservabilityState>
  </div>
</template>
