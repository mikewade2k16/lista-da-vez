<script setup lang="ts">
import AppSegmentedFilter from '~/components/ui/AppSegmentedFilter.vue'
import ObservabilityState from '~/components/admin/observability/ObservabilityState.vue'
import ObservabilitySystemCharts from '~/components/admin/observability/ObservabilitySystemCharts.vue'
import { useObsQuery } from '~/composables/useObservability'
import type { ObsSystem } from '~/types/observability'
import { formatInt, formatPercentValue } from '~/utils/observability/format'
import { formatAge, formatBytes } from '~/utils/observability/health'

const HOURS = [
  { value: '1', label: '1 h' },
  { value: '6', label: '6 h' },
  { value: '24', label: '24 h' },
]
const hours = ref('6')
const { data, loading, error, reload } = useObsQuery<ObsSystem>('/system', () => ({
  hours: hours.value,
}))

const latestApi = computed(() => data.value?.api?.[data.value.api.length - 1] || null)
const latestAgent = computed(() => data.value?.agent?.[data.value.agent.length - 1] || null)
const disks = computed(() => latestAgent.value?.metrics.disk || [])
const jobs = computed(() => latestApi.value?.metrics.jobs || [])
const containers = computed(() => data.value?.containers || [])

function containerBand(state: string, health: string): string {
  if (state !== 'running' || health === 'unhealthy') return 'bad'
  return health === 'starting' ? 'warn' : 'good'
}
function diskBand(pct: number): string {
  return pct >= 95 ? 'bad' : pct >= 85 ? 'warn' : 'good'
}
</script>

<template>
  <div class="obs-stack">
    <div class="obs-toolbar">
      <AppSegmentedFilter v-model="hours" :options="HOURS" aria-label="Janela" />
      <UButton
        label="Atualizar"
        icon="i-lucide-refresh-cw"
        size="sm"
        color="neutral"
        variant="soft"
        :loading="loading"
        @click="reload"
      />
    </div>

    <ObservabilityState
      :loading="loading"
      :error="error"
      :empty="!data || (data.api.length === 0 && data.containers.length === 0)"
      empty-title="Ainda não há amostras de sistema."
      empty-hint="A API grava uma amostra a cada 15 s e o agente de host (obs-agent) a cada 30 s."
      @retry="reload"
    >
      <div v-if="data" class="obs-stack">
        <ObservabilitySystemCharts v-if="data.api.length > 1" :samples="data.api" />
        <p v-else class="obs-hint">Gráficos aparecem a partir de duas amostras da API.</p>

        <section class="obs-section">
          <h3 class="obs-section__title">Containers</h3>
          <p v-if="containers.length === 0" class="obs-hint">
            Sem dados do agente de host (obs-agent). Verifique se o container está de pé.
          </p>
          <div v-else class="obs-tablewrap">
            <table class="obs-table">
              <thead>
                <tr>
                  <th>Container</th>
                  <th>Estado</th>
                  <th>Saúde</th>
                  <th class="obs-num">Reinícios</th>
                  <th class="obs-num">CPU</th>
                  <th class="obs-num">Memória</th>
                  <th>Amostra</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="c in containers" :key="c.container">
                  <td>{{ c.service || c.container }}</td>
                  <td>
                    <span :class="['obs-band', `obs-band--${containerBand(c.state, c.health)}`]">
                      {{ c.state }}
                    </span>
                  </td>
                  <td>{{ c.health || '-' }}</td>
                  <td :class="['obs-num', c.restartCount > 0 ? 'obs-text--warn' : '']">
                    {{ formatInt(c.restartCount) }}
                  </td>
                  <td class="obs-num">{{ formatPercentValue(c.cpuPct) }}</td>
                  <td class="obs-num">
                    {{ formatBytes(c.memBytes) }}
                    <span v-if="c.memLimitBytes" class="obs-muted">
                      / {{ formatBytes(c.memLimitBytes) }}
                    </span>
                  </td>
                  <td class="obs-muted">{{ formatAge(c.at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="obs-section">
          <h3 class="obs-section__title">Discos</h3>
          <p v-if="disks.length === 0" class="obs-hint">Sem amostra de disco do agente.</p>
          <div v-else class="obs-tablewrap">
            <table class="obs-table">
              <thead>
                <tr>
                  <th>Ponto</th>
                  <th class="obs-num">Total</th>
                  <th class="obs-num">Livre</th>
                  <th class="obs-num">Usado</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="disk in disks" :key="disk.path">
                  <td class="obs-mono">{{ disk.path }}</td>
                  <td class="obs-num">{{ formatBytes(disk.totalBytes) }}</td>
                  <td class="obs-num">{{ formatBytes(disk.freeBytes) }}</td>
                  <td class="obs-num">
                    <span :class="['obs-band', `obs-band--${diskBand(disk.usedPct)}`]">
                      {{ formatPercentValue(disk.usedPct) }}
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section v-if="jobs.length > 0" class="obs-section">
          <h3 class="obs-section__title">Filas de jobs</h3>
          <div class="obs-tablewrap">
            <table class="obs-table">
              <thead>
                <tr>
                  <th>Tabela</th>
                  <th class="obs-num">Pendentes</th>
                  <th class="obs-num">Vencidos</th>
                  <th class="obs-num">Atraso (s)</th>
                  <th class="obs-num">Travados</th>
                  <th class="obs-num">Mortos</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="job in jobs" :key="job.table">
                  <td class="obs-mono">{{ job.table }}</td>
                  <td class="obs-num">{{ formatInt(job.pending) }}</td>
                  <td class="obs-num">{{ formatInt(job.due) }}</td>
                  <td class="obs-num">{{ formatInt(job.oldestDueLagSeconds) }}</td>
                  <td class="obs-num">{{ formatInt(job.stuckProcessing) }}</td>
                  <td :class="['obs-num', job.dead > 0 ? 'obs-text--bad' : '']">
                    {{ formatInt(job.dead) }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </ObservabilityState>
  </div>
</template>
