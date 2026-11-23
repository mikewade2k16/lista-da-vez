<script setup lang="ts">
import ObservabilityHealthAlerts from '~/components/admin/observability/ObservabilityHealthAlerts.vue'
import ObservabilityHealthErrors from '~/components/admin/observability/ObservabilityHealthErrors.vue'
import ObservabilityHealthIssues from '~/components/admin/observability/ObservabilityHealthIssues.vue'
import ObservabilityHealthOnline from '~/components/admin/observability/ObservabilityHealthOnline.vue'
import ObservabilitySecurityNotice from '~/components/admin/observability/ObservabilitySecurityNotice.vue'
import ObservabilityState from '~/components/admin/observability/ObservabilityState.vue'
import { useObsAutoRefresh, useObsQuery } from '~/composables/useObservability'
import type { ObsHealth, ObsTabKey } from '~/types/observability'
import {
  formatAge,
  HEALTH_REFRESH_MS,
  STATUS_LABEL,
  statusBand,
} from '~/utils/observability/health'

// Aba "Saúde": visão resumida do que está ruim agora. Atualiza sozinha a cada 20 s
// enquanto a aba do navegador está visível.
const emit = defineEmits<{ navigate: [tab: ObsTabKey | 'alerts'] }>()

const { data, loading, error, reload } = useObsQuery<ObsHealth>('/health')
useObsAutoRefresh(
  () => void reload(),
  () => loading.value,
  HEALTH_REFRESH_MS,
)

const now = ref(Date.now())
let clock: ReturnType<typeof setInterval> | null = null
onMounted(() => {
  clock = setInterval(() => {
    now.value = Date.now()
  }, 1000)
})
onBeforeUnmount(() => {
  if (clock) clearInterval(clock)
})

// Falha ao atualizar com dados em tela: mantém os últimos dados e avisa.
const hardError = computed(() => (data.value ? '' : error.value))
const overall = computed(() => data.value?.overall ?? 'ok')
</script>

<template>
  <ObservabilityState
    :loading="loading"
    :error="hardError"
    :empty="!data"
    empty-title="Sem dados de saúde ainda."
    empty-hint="O coletor de sistema grava amostras a cada 15 s; volte em instantes."
    @retry="reload"
  >
    <div v-if="data" class="obs-stack">
      <header class="obs-health__head">
        <span :class="['obs-band', 'obs-health__overall', `obs-band--${statusBand(overall)}`]">
          {{ STATUS_LABEL[overall] }}
        </span>
        <span class="obs-muted">
          Atualizado {{ formatAge(data.generatedAt, now) }} · atualização automática a cada 20 s
        </span>
        <span v-if="error" class="obs-text--warn obs-muted">Falha ao atualizar: {{ error }}</span>
        <UButton
          label="Atualizar"
          icon="i-lucide-refresh-cw"
          size="xs"
          color="neutral"
          variant="soft"
          :loading="loading"
          @click="reload"
        />
      </header>

      <div class="obs-health__areas">
        <article
          v-for="area in data.areas"
          :key="area.key"
          :class="['obs-health__area', `obs-health__area--${statusBand(area.status)}`]"
        >
          <span class="obs-card__label">{{ area.label }}</span>
          <strong :class="`obs-text--${statusBand(area.status)}`">
            {{ STATUS_LABEL[area.status] }}
          </strong>
          <span class="obs-muted">{{ area.summary }}</span>
        </article>
      </div>

      <ObservabilityHealthIssues
        :issues="data.issues"
        :now="now"
        @navigate="emit('navigate', $event)"
      />

      <div class="obs-health__cols">
        <ObservabilityHealthErrors :errors="data.errors" />
        <ObservabilityHealthOnline :online="data.online" :now="now" />
      </div>

      <div class="obs-health__cols">
        <ObservabilityHealthAlerts
          :alerts="data.alerts"
          :now="now"
          @open="emit('navigate', 'alerts')"
        />
        <ObservabilitySecurityNotice
          :summary="data.security"
          linkable
          @open="emit('navigate', 'security')"
        />
      </div>
    </div>
  </ObservabilityState>
</template>

<style>
.obs-health__head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.7rem;
}

.obs-health__overall {
  font-size: 0.95rem;
  padding: 0.2rem 0.9rem;
}

.obs-health__areas {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(11rem, 1fr));
  gap: 0.6rem;
}

.obs-health__area {
  display: grid;
  gap: 0.2rem;
  padding: 0.7rem 0.8rem;
  border: 1px solid rgb(var(--border));
  border-left-width: 4px;
  border-radius: var(--radius-md);
  background: rgb(var(--surface));
  font-size: 0.8rem;
}

.obs-health__area--good {
  border-left-color: rgb(var(--success));
}

.obs-health__area--warn {
  border-left-color: rgb(var(--warning));
}

.obs-health__area--bad {
  border-left-color: rgb(var(--danger));
}

.obs-health__area--none {
  border-left-color: rgb(var(--muted));
}

.obs-health__cols {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
  gap: 1rem;
  align-items: start;
}

.obs-health__ok {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  margin: 0;
  font-size: 0.85rem;
}

.obs-health__issues {
  display: grid;
  gap: 0.3rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.obs-health__issue {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 0.7rem;
  width: 100%;
  padding: 0.5rem 0.6rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-md);
  background: none;
  color: inherit;
  font-size: 0.8rem;
  text-align: left;
  cursor: pointer;
}

.obs-health__issue:disabled {
  cursor: default;
}

.obs-health__issue:not(:disabled):hover {
  background: rgb(var(--surface-2));
}

.obs-health__issuetext {
  display: grid;
  min-width: 0;
}

.obs-health__since {
  white-space: nowrap;
}

.obs-notice {
  display: flex;
  align-items: flex-start;
  gap: 0.7rem;
  padding: 0.9rem 1rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-md);
  background: rgb(var(--surface));
}

.obs-notice--warn {
  border-color: rgb(var(--warning));
  background: rgb(var(--warning) / 0.12);
}

.obs-notice__icon {
  flex: none;
  margin-top: 0.15rem;
  color: rgb(var(--warning));
}

.obs-notice__body {
  flex: 1;
  min-width: 0;
  font-size: 0.85rem;
}

.obs-notice__body p {
  margin-top: 0.25rem;
}
</style>
