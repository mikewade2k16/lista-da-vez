<script setup lang="ts">
import ObservabilityRunResultsTable from '~/components/admin/observability/ObservabilityRunResultsTable.vue'
import ObservabilityState from '~/components/admin/observability/ObservabilityState.vue'
import { useObsQuery } from '~/composables/useObservability'
import type { ObsDb, ObsRunResult } from '~/types/observability'
import { formatInt, formatMs } from '~/utils/observability/format'

const props = defineProps<{ days: number }>()
const emit = defineEmits<{ trace: [id: string] }>()

const { data, loading, error, reload } = useObsQuery<ObsDb>('/db', () => ({ days: props.days }))

const MAX_ROWS = 50
const slow = computed(() => (data.value?.slow || []).slice(0, MAX_ROWS))
const nPlusOne = computed(() => (data.value?.nPlusOne || []).slice(0, MAX_ROWS))
const top = computed(() => (data.value?.top || []).slice(0, MAX_ROWS))

// `checks` vem do ultimo run: lista de resultados (kind=db) ou objeto livre.
const checkResults = computed<ObsRunResult[]>(() =>
  Array.isArray(data.value?.checks) ? (data.value?.checks as ObsRunResult[]) : [],
)
const checkEntries = computed<Array<[string, string[]]>>(() => {
  const checks = data.value?.checks
  if (!checks || Array.isArray(checks) || typeof checks !== 'object') return []
  return Object.entries(checks as Record<string, unknown>).map(([key, value]) => [
    key,
    Array.isArray(value)
      ? value.map((item) => (typeof item === 'string' ? item : JSON.stringify(item)))
      : [JSON.stringify(value)],
  ])
})
const isEmpty = computed(
  () =>
    !data.value ||
    (slow.value.length === 0 &&
      nPlusOne.value.length === 0 &&
      top.value.length === 0 &&
      checkResults.value.length === 0 &&
      checkEntries.value.length === 0),
)
</script>

<template>
  <ObservabilityState
    :loading="loading"
    :error="error"
    :empty="isEmpty"
    empty-title="Nenhuma consulta registrada neste periodo."
    empty-hint="A API grava consultas lentas e N+1 durante o uso. Rode `npm run obs:run` para gerar carga medida."
    @retry="reload"
  >
    <div class="obs-stack">
      <section class="obs-section">
        <h3 class="obs-section__title">Consultas mais lentas</h3>
        <p v-if="slow.length === 0" class="obs-hint">Nenhuma consulta acima do limite.</p>
        <div v-else class="obs-tablewrap">
          <table class="obs-table">
            <thead>
              <tr>
                <th>Consulta</th>
                <th>Rota</th>
                <th class="obs-num">Duracao</th>
                <th>Trace</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(row, index) in slow" :key="`${row.fingerprint}-${index}`">
                <td class="obs-mono obs-clip" :title="row.statement">{{ row.statement }}</td>
                <td class="obs-mono">{{ row.route || '-' }}</td>
                <td class="obs-num obs-text--bad">{{ formatMs(row.durationMs) }}</td>
                <td>
                  <button
                    v-if="row.correlationId"
                    type="button"
                    class="obs-link"
                    @click="emit('trace', row.correlationId)"
                  >
                    {{ row.correlationId }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="obs-section">
        <h3 class="obs-section__title">Suspeitas de N+1</h3>
        <p v-if="nPlusOne.length === 0" class="obs-hint">Nenhuma repeticao suspeita.</p>
        <div v-else class="obs-tablewrap">
          <table class="obs-table">
            <thead>
              <tr>
                <th>Consulta repetida</th>
                <th>Rota</th>
                <th class="obs-num">Ocorrencias</th>
                <th class="obs-num">Max repeticoes</th>
                <th>Trace</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in nPlusOne" :key="`${row.route}-${row.fingerprint}`">
                <td class="obs-mono obs-clip" :title="row.statement">{{ row.statement }}</td>
                <td class="obs-mono">{{ row.route }}</td>
                <td class="obs-num">{{ formatInt(row.occurrences) }}</td>
                <td class="obs-num obs-text--warn">{{ formatInt(row.maxRepetitions) }}</td>
                <td>
                  <button
                    v-if="row.lastCorrelationId"
                    type="button"
                    class="obs-link"
                    @click="emit('trace', row.lastCorrelationId)"
                  >
                    {{ row.lastCorrelationId }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="obs-section">
        <h3 class="obs-section__title">Maior tempo total</h3>
        <p v-if="top.length === 0" class="obs-hint">Sem agregados no periodo.</p>
        <div v-else class="obs-tablewrap">
          <table class="obs-table">
            <thead>
              <tr>
                <th>Consulta</th>
                <th class="obs-num">Chamadas</th>
                <th class="obs-num">Total</th>
                <th class="obs-num">Media</th>
                <th class="obs-num">Max</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in top" :key="row.fingerprint">
                <td class="obs-mono obs-clip" :title="row.statement">{{ row.statement }}</td>
                <td class="obs-num">{{ formatInt(row.calls) }}</td>
                <td class="obs-num">{{ formatMs(row.totalMs) }}</td>
                <td class="obs-num">{{ formatMs(row.avgMs) }}</td>
                <td class="obs-num">{{ formatMs(row.maxMs) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="obs-section">
        <h3 class="obs-section__title">Alertas de indice e constraint (ultima execucao)</h3>
        <ObservabilityRunResultsTable v-if="checkResults.length" :results="checkResults" />
        <template v-else-if="checkEntries.length">
          <div v-for="[key, lines] in checkEntries" :key="key" class="obs-checkgroup">
            <strong>{{ key }} ({{ lines.length }})</strong>
            <ul>
              <li v-for="(line, index) in lines.slice(0, 30)" :key="index" class="obs-mono">
                {{ line }}
              </li>
            </ul>
          </div>
        </template>
        <p v-else class="obs-hint">
          Sem verificacao de banco ainda. Rode `npm run obs:run` para auditar indices e constraints.
        </p>
      </section>
    </div>
  </ObservabilityState>
</template>
