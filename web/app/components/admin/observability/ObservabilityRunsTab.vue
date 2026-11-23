<script setup lang="ts">
import ObservabilityCompare from '~/components/admin/observability/ObservabilityCompare.vue'
import ObservabilityRunnerPanel from '~/components/admin/observability/ObservabilityRunnerPanel.vue'
import ObservabilityState from '~/components/admin/observability/ObservabilityState.vue'
import { useObservabilityApi, useObsQuery } from '~/composables/useObservability'
import { getApiErrorMessage } from '~/utils/api-client'
import type { ObsRun, ObsRunCompare } from '~/types/observability'
import { formatDate, formatInt } from '~/utils/observability/format'

const api = useObservabilityApi()
const { data, loading, error, reload } = useObsQuery<ObsRun[]>('/runs', () => ({ limit: 30 }))

const selected = ref<string[]>([])
const compare = ref<ObsRunCompare | null>(null)
const comparing = ref(false)
const compareError = ref('')

function toggle(id: string) {
  const current = selected.value
  if (current.includes(id)) {
    selected.value = current.filter((item) => item !== id)
  } else {
    // Mantem no maximo duas: a mais antiga selecionada sai.
    selected.value = [...current, id].slice(-2)
  }
  compare.value = null
}

async function runCompare() {
  if (selected.value.length !== 2) return
  const [first, second] = selected.value as [string, string]
  const byId = new Map((data.value || []).map((run) => [run.id, run]))
  // "antes" e a execucao mais antiga.
  const older =
    new Date(byId.get(first)?.startedAt || 0) <= new Date(byId.get(second)?.startedAt || 0)
  const [a, b] = older ? [first, second] : [second, first]
  comparing.value = true
  compareError.value = ''
  try {
    compare.value = await api.get<ObsRunCompare>('/runs/compare', { a, b })
  } catch (cause) {
    compareError.value = getApiErrorMessage(cause, 'Nao foi possivel comparar as execucoes.')
  } finally {
    comparing.value = false
  }
}

function summaryText(run: ObsRun): string {
  const summary = run.summary || {}
  const parts = ['pass', 'warn', 'fail']
    .filter((key) => typeof summary[key] === 'number')
    .map((key) => `${summary[key]} ${key}`)
  return parts.join(' / ') || '-'
}
</script>

<template>
  <div class="obs-stack">
    <ObservabilityRunnerPanel @finished="reload" />

    <section class="obs-section">
      <div class="obs-section__head">
        <h3 class="obs-section__title">Historico de execucoes</h3>
        <UButton
          label="Comparar selecionadas"
          icon="i-lucide-git-compare"
          color="neutral"
          variant="soft"
          size="sm"
          :loading="comparing"
          :disabled="selected.length !== 2 || comparing"
          @click="runCompare"
        />
      </div>
      <ObservabilityState
        :loading="loading"
        :error="error"
        :empty="!data || data.length === 0"
        empty-title="Nenhuma execucao registrada."
        empty-hint="Rode `npm run obs:run` no terminal ou use o botao acima com o worker online (`npm run obs:worker`)."
        @retry="reload"
      >
        <div class="obs-tablewrap">
          <table class="obs-table">
            <thead>
              <tr>
                <th></th>
                <th>Execucao</th>
                <th>Inicio</th>
                <th>Ambiente</th>
                <th>Resumo</th>
                <th class="obs-num">Saida</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="run in data || []" :key="run.id">
                <td>
                  <input
                    type="checkbox"
                    :checked="selected.includes(run.id)"
                    :aria-label="`Selecionar execucao ${run.id}`"
                    @change="toggle(run.id)"
                  />
                </td>
                <td class="obs-mono">{{ run.id }}</td>
                <td>{{ formatDate(run.startedAt) }}</td>
                <td>{{ run.env || '-' }}</td>
                <td class="obs-muted">{{ summaryText(run) }}</td>
                <td :class="['obs-num', run.exitCode ? 'obs-text--bad' : '']">
                  {{
                    run.exitCode === null || run.exitCode === undefined
                      ? '-'
                      : formatInt(run.exitCode)
                  }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p class="obs-hint">Marque duas execucoes para comparar antes e depois.</p>
      </ObservabilityState>
      <p v-if="compareError" class="obs-hint obs-text--bad">{{ compareError }}</p>
    </section>

    <ObservabilityCompare v-if="compare" :compare="compare" />
  </div>
</template>
