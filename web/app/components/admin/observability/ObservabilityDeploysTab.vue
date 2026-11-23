<script setup lang="ts">
import ObservabilityState from '~/components/admin/observability/ObservabilityState.vue'
import { useObsQuery } from '~/composables/useObservability'
import type { ObsDeploy, ObsDeploys } from '~/types/observability'
import {
  formatDeployDuration,
  shortSha,
  slowestStageIndex,
  stageBarPercent,
  STATUS_TEXT,
  VERDICT_LABEL,
  verdictBand,
} from '~/utils/observability/deploys'
import { formatDate, formatRate } from '~/utils/observability/format'
import { formatAge } from '~/utils/observability/health'

const { data, loading, error, reload } = useObsQuery<ObsDeploys>('/deploys', () => ({ limit: 30 }))

const expanded = ref('')
function toggle(deploy: ObsDeploy) {
  expanded.value = expanded.value === deploy.id ? '' : deploy.id
}

const items = computed(() => data.value?.items ?? [])
const stats = computed(() => data.value?.stats ?? null)
const last = computed(() => items.value[0] ?? null)
</script>

<template>
  <div class="obs-stack">
    <p class="obs-hint">
      Tempo de ponta a ponta: do início do comando na máquina até os smoke tests passarem na VPS.
      Mediana e p90 consideram os últimos 10 deploys com sucesso do mesmo comando; “lento” é mais de
      1,5 vez a mediana.
    </p>

    <ObservabilityState
      :loading="loading"
      :error="error"
      :empty="items.length === 0"
      empty-title="Nenhum deploy registrado."
      empty-hint="O histórico é gravado ao fim de cada deploy. Para importar os já feitos: npm run obs:deploys -- --sync"
      @retry="reload"
    >
      <div v-if="stats" class="obs-cards">
        <div class="obs-card">
          <span class="obs-card__label">Mediana</span>
          <strong class="obs-card__value">{{ formatDeployDuration(stats.medianMs) }}</strong>
          <span class="obs-muted">
            limite de lento: {{ formatDeployDuration(stats.slowThresholdMs) }}
          </span>
        </div>
        <div class="obs-card">
          <span class="obs-card__label">p90</span>
          <strong class="obs-card__value">{{ formatDeployDuration(stats.p90Ms) }}</strong>
        </div>
        <div class="obs-card">
          <span class="obs-card__label">Taxa de sucesso</span>
          <strong class="obs-card__value">{{ formatRate(stats.successRate) }}</strong>
          <span class="obs-muted">{{ stats.count }} deploys</span>
        </div>
        <div v-if="last" class="obs-card">
          <span class="obs-card__label">Último deploy</span>
          <strong :class="['obs-card__value', `obs-text--${verdictBand(last.verdict)}`]">
            {{ VERDICT_LABEL[last.verdict] }}
          </strong>
          <span class="obs-muted">
            {{ formatDeployDuration(last.durationMs) }}, {{ formatAge(last.startedAt) }}
          </span>
        </div>
      </div>

      <div class="obs-tablewrap">
        <table class="obs-table obs-deploys">
          <thead>
            <tr>
              <th>Data</th>
              <th>Comando</th>
              <th>Ambiente</th>
              <th class="obs-num">Duração</th>
              <th>Status</th>
              <th>Veredito</th>
              <th>Commit</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="deploy in items" :key="deploy.id">
              <tr
                class="obs-deploys__row"
                tabindex="0"
                role="button"
                :aria-expanded="expanded === deploy.id"
                @click="toggle(deploy)"
                @keydown.enter.prevent="toggle(deploy)"
                @keydown.space.prevent="toggle(deploy)"
              >
                <td>
                  <UIcon
                    :name="
                      expanded === deploy.id ? 'i-lucide-chevron-down' : 'i-lucide-chevron-right'
                    "
                    class="obs-deploys__chevron"
                  />
                  {{ formatDate(deploy.startedAt) }}
                </td>
                <td class="obs-mono">{{ deploy.command }}</td>
                <td>{{ deploy.environment || '-' }}</td>
                <td class="obs-num">{{ formatDeployDuration(deploy.durationMs) }}</td>
                <td>{{ STATUS_TEXT[deploy.status] || deploy.status }}</td>
                <td>
                  <span :class="['obs-band', `obs-band--${verdictBand(deploy.verdict)}`]">
                    {{ VERDICT_LABEL[deploy.verdict] }}
                  </span>
                </td>
                <td class="obs-mono" :title="deploy.gitBranch || deploy.imageTag">
                  {{ shortSha(deploy.gitSha) }}
                </td>
              </tr>
              <tr v-if="expanded === deploy.id" class="obs-deploys__detail">
                <td colspan="7">
                  <p v-if="deploy.stages.length === 0" class="obs-muted">
                    Sem etapas registradas para este deploy.
                  </p>
                  <ul v-else class="obs-deploys__stages">
                    <li
                      v-for="(stage, index) in deploy.stages"
                      :key="index"
                      :class="{
                        'obs-deploys__stage--slowest': index === slowestStageIndex(deploy.stages),
                      }"
                      :style="{ paddingLeft: `${stage.depth * 1.1}rem` }"
                    >
                      <span class="obs-deploys__name" :title="stage.name">{{ stage.name }}</span>
                      <span class="obs-deploys__track">
                        <span
                          class="obs-deploys__bar"
                          :style="{ width: `${stageBarPercent(stage, deploy.stages)}%` }"
                        ></span>
                      </span>
                      <span class="obs-num obs-mono">
                        {{ formatDeployDuration(stage.durationMs) }}
                      </span>
                      <span class="obs-muted">{{ stage.status }}</span>
                    </li>
                  </ul>
                  <p class="obs-muted">
                    Etapa mais lenta destacada. Etapas aninhadas se sobrepõem às do nível acima e
                    não somam.
                  </p>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
    </ObservabilityState>
  </div>
</template>

<style scoped>
.obs-deploys__row {
  cursor: pointer;
}

.obs-deploys__row:hover,
.obs-deploys__row:focus-visible {
  background: rgb(var(--surface-2));
  outline: none;
}

.obs-deploys__chevron {
  vertical-align: -0.15rem;
  margin-right: 0.25rem;
  color: rgb(var(--muted));
}

.obs-deploys__detail > td {
  background: rgb(var(--surface-2) / 0.5);
}

.obs-deploys__stages {
  display: grid;
  gap: 0.2rem;
  margin: 0 0 0.5rem;
  padding: 0;
  list-style: none;
}

.obs-deploys__stages li {
  display: grid;
  grid-template-columns: minmax(8rem, 22rem) minmax(6rem, 1fr) 4rem 5rem;
  align-items: center;
  gap: 0.6rem;
  font-size: 0.76rem;
}

.obs-deploys__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.obs-deploys__track {
  height: 0.5rem;
  border-radius: 999px;
  background: rgb(var(--border) / 0.5);
  overflow: hidden;
}

.obs-deploys__bar {
  display: block;
  height: 100%;
  border-radius: 999px;
  background: rgb(var(--primary));
}

.obs-deploys__stage--slowest .obs-deploys__bar {
  background: rgb(var(--warning));
}

.obs-deploys__stage--slowest .obs-deploys__name {
  font-weight: 600;
}
</style>
