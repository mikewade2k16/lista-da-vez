<script setup lang="ts">
import ObservabilityState from '~/components/admin/observability/ObservabilityState.vue'
import { useObsQuery } from '~/composables/useObservability'
import type { ObsIntegration } from '~/types/observability'
import { formatAge } from '~/utils/observability/health'
import {
  errorRateBand,
  formatInt,
  formatMs,
  formatRate,
  latencyBand,
} from '~/utils/observability/format'

const props = defineProps<{ days: number }>()

const { data, loading, error, reload } = useObsQuery<ObsIntegration[]>('/integrations', () => ({
  days: props.days,
}))
</script>

<template>
  <div class="obs-stack">
    <p class="obs-hint">
      Chamadas da API a serviços externos (WhatsApp, Meta, Google, IA, armazenamento). Erro =
      timeout, falha de rede, resposta 5xx ou mensagem de erro; 4xx sozinho não conta.
    </p>
    <ObservabilityState
      :loading="loading"
      :error="error"
      :empty="!data || data.length === 0"
      empty-title="Nenhuma chamada externa registrada neste período."
      empty-hint="As chamadas a integrações aparecem aqui assim que a API as fizer."
      @retry="reload"
    >
      <div class="obs-tablewrap">
        <table class="obs-table">
          <thead>
            <tr>
              <th>Host</th>
              <th class="obs-num">Chamadas</th>
              <th class="obs-num">Erros</th>
              <th class="obs-num">Taxa de erro</th>
              <th class="obs-num">Média</th>
              <th class="obs-num">p95</th>
              <th>Último erro</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in data || []" :key="row.host">
              <td class="obs-mono">{{ row.host }}</td>
              <td class="obs-num">{{ formatInt(row.calls) }}</td>
              <td class="obs-num">{{ formatInt(row.errors) }}</td>
              <td :class="['obs-num', `obs-text--${errorRateBand(row.errorRate)}`]">
                {{ formatRate(row.errorRate) }}
              </td>
              <td class="obs-num">{{ formatMs(row.avgMs) }}</td>
              <td :class="['obs-num', `obs-text--${latencyBand(row.p95Ms)}`]">
                {{ formatMs(row.p95Ms) }}
              </td>
              <td>
                <template v-if="row.lastErrorAt">
                  {{ formatAge(row.lastErrorAt) }}
                  <div class="obs-muted obs-clip" :title="row.lastError">{{ row.lastError }}</div>
                </template>
                <span v-else class="obs-muted">-</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </ObservabilityState>
  </div>
</template>
