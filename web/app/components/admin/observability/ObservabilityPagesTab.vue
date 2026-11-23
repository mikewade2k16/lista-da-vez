<script setup lang="ts">
import AppSearchInput from '~/components/ui/AppSearchInput.vue'
import ObservabilityRunResultsTable from '~/components/admin/observability/ObservabilityRunResultsTable.vue'
import ObservabilityState from '~/components/admin/observability/ObservabilityState.vue'
import { useObsQuery } from '~/composables/useObservability'
import type { ObsPages } from '~/types/observability'
import { formatInt, formatVital, vitalBand } from '~/utils/observability/format'

const props = defineProps<{ days: number }>()

const search = ref('')
const { data, loading, error, reload } = useObsQuery<ObsPages>('/pages', () => ({
  days: props.days,
}))

const VITAL_COLUMNS = [
  { key: 'lcpP75', name: 'LCP' },
  { key: 'inpP75', name: 'INP' },
  { key: 'clsP75', name: 'CLS' },
  { key: 'ttfbP75', name: 'TTFB' },
  { key: 'loadP75', name: 'Load' },
] as const

const live = computed(() => {
  const term = search.value.trim().toLowerCase()
  const rows = data.value?.live || []
  return term ? rows.filter((row) => row.page.toLowerCase().includes(term)) : rows
})
const lastRun = computed(() => data.value?.lastRun || [])
const isEmpty = computed(
  () => !data.value || ((data.value.live || []).length === 0 && lastRun.value.length === 0),
)
</script>

<template>
  <ObservabilityState
    :loading="loading"
    :error="error"
    :empty="isEmpty"
    empty-title="Nenhuma medicao de pagina neste periodo."
    empty-hint="O coletor do front envia Web Vitals de quem usa o painel. Para medir todas as paginas de uma vez, rode `npm run obs:run`."
    @retry="reload"
  >
    <div class="obs-stack">
      <section class="obs-section">
        <h3 class="obs-section__title">Uso real (p75 por pagina)</h3>
        <div class="obs-toolbar">
          <AppSearchInput v-model="search" placeholder="Buscar pagina" aria-label="Buscar pagina" />
        </div>
        <p v-if="(data?.live || []).length === 0" class="obs-hint">
          Sem amostras de uso real ainda. Elas aparecem conforme as pessoas navegam.
        </p>
        <p v-else-if="live.length === 0" class="obs-hint">Nenhum resultado para "{{ search }}".</p>
        <div v-else class="obs-tablewrap">
          <table class="obs-table">
            <thead>
              <tr>
                <th>Pagina</th>
                <th class="obs-num">Amostras</th>
                <th v-for="col in VITAL_COLUMNS" :key="col.key" class="obs-num">{{ col.name }}</th>
                <th class="obs-num">Erros JS</th>
                <th class="obs-num">Falhas de API</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in live" :key="row.page">
                <td class="obs-mono">{{ row.page }}</td>
                <td class="obs-num">{{ formatInt(row.samples) }}</td>
                <td v-for="col in VITAL_COLUMNS" :key="col.key" class="obs-num">
                  <span :class="['obs-band', `obs-band--${vitalBand(col.name, row[col.key])}`]">
                    {{ formatVital(col.name, row[col.key]) }}
                  </span>
                </td>
                <td :class="['obs-num', row.jsErrors > 0 ? 'obs-text--bad' : '']">
                  {{ formatInt(row.jsErrors) }}
                </td>
                <td :class="['obs-num', row.apiFailures > 0 ? 'obs-text--warn' : '']">
                  {{ formatInt(row.apiFailures) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p class="obs-hint">
          Faixas: verde dentro do limite bom, amarelo precisa melhorar, vermelho ruim (LCP 2,5 s,
          INP 200 ms, CLS 0,1, TTFB 800 ms).
        </p>
      </section>

      <section class="obs-section">
        <h3 class="obs-section__title">Resultado da ultima execucao do runner</h3>
        <ObservabilityRunResultsTable v-if="lastRun.length" :results="lastRun" />
        <p v-else class="obs-hint">Nenhuma execucao com paginas ainda: `npm run obs:run`.</p>
      </section>
    </div>
  </ObservabilityState>
</template>
