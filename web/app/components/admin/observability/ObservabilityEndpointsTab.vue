<script setup lang="ts">
import AppSearchInput from '~/components/ui/AppSearchInput.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import ObservabilityState from '~/components/admin/observability/ObservabilityState.vue'
import { useObsQuery } from '~/composables/useObservability'
import type { ObsEndpoint } from '~/types/observability'
import {
  errorRateBand,
  formatInt,
  formatMs,
  formatRate,
  latencyBand,
} from '~/utils/observability/format'

const props = defineProps<{ days: number }>()

const moduleFilter = ref('all')
const sort = ref('p95')
const search = ref('')
const knownModules = ref<string[]>([])

const SORT_OPTIONS = [
  { value: 'p95', label: 'Mais lentos (p95)' },
  { value: 'errors', label: 'Mais erros' },
  { value: 'calls', label: 'Mais volume' },
  { value: 'queries', label: 'Mais queries por req' },
]

const { data, loading, error, reload } = useObsQuery<ObsEndpoint[]>('/endpoints', () => ({
  days: props.days,
  module: moduleFilter.value === 'all' ? undefined : moduleFilter.value,
  sort: sort.value,
}))

watch(data, (rows) => {
  const merged = new Set(knownModules.value)
  ;(rows || []).forEach((row) => row.module && merged.add(row.module))
  knownModules.value = Array.from(merged).sort()
})

const moduleOptions = computed(() => [
  { value: 'all', label: 'Todos os modulos' },
  ...knownModules.value.map((name) => ({ value: name, label: name })),
])

const rows = computed(() => {
  const term = search.value.trim().toLowerCase()
  const all = data.value || []
  if (!term) return all
  return all.filter((row) =>
    `${row.method} ${row.route} ${row.module}`.toLowerCase().includes(term),
  )
})
</script>

<template>
  <div class="obs-stack">
    <div class="obs-toolbar">
      <AppSearchInput
        v-model="search"
        placeholder="Buscar rota ou modulo"
        aria-label="Buscar endpoint"
      />
      <AppSelectField
        v-model="moduleFilter"
        :options="moduleOptions"
        compact
        :show-leading-icon="false"
      />
      <AppSelectField v-model="sort" :options="SORT_OPTIONS" compact :show-leading-icon="false" />
    </div>

    <ObservabilityState
      :loading="loading"
      :error="error"
      :empty="!data || data.length === 0"
      empty-title="Nenhuma requisicao registrada neste periodo."
      empty-hint="Navegue pelo painel (a API grava as requisicoes) ou rode `npm run obs:run` para gerar medicoes."
      @retry="reload"
    >
      <p v-if="rows.length === 0" class="obs-hint">Nenhum resultado para "{{ search }}".</p>
      <div v-else class="obs-tablewrap">
        <table class="obs-table">
          <thead>
            <tr>
              <th>Rota</th>
              <th>Modulo</th>
              <th class="obs-num">Req</th>
              <th class="obs-num">Erros</th>
              <th class="obs-num">p50</th>
              <th class="obs-num">p95</th>
              <th class="obs-num">p99</th>
              <th class="obs-num">Queries (med/max)</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="`${row.method} ${row.route}`">
              <td class="obs-mono">
                <span class="obs-chip">{{ row.method }}</span>
                {{ row.route }}
              </td>
              <td>{{ row.module }}</td>
              <td class="obs-num">{{ formatInt(row.calls) }}</td>
              <td :class="['obs-num', `obs-text--${errorRateBand(row.errorRate)}`]">
                {{ formatRate(row.errorRate) }}
              </td>
              <td class="obs-num">{{ formatMs(row.p50Ms) }}</td>
              <td :class="['obs-num', `obs-text--${latencyBand(row.p95Ms)}`]">
                {{ formatMs(row.p95Ms) }}
              </td>
              <td class="obs-num">{{ formatMs(row.p99Ms) }}</td>
              <td class="obs-num">
                {{ Number.isFinite(row.avgQueries) ? row.avgQueries.toFixed(1) : '-' }} /
                {{ formatInt(row.maxQueries) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </ObservabilityState>
  </div>
</template>
