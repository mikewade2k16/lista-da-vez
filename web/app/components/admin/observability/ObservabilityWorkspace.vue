<script setup lang="ts">
import AdminPageHeader from '../../../../layers/core/components/admin/AdminPageHeader.vue'
import AppSegmentedFilter from '~/components/ui/AppSegmentedFilter.vue'
import ObservabilityAlertsTab from '~/components/admin/observability/ObservabilityAlertsTab.vue'
import ObservabilityDbTab from '~/components/admin/observability/ObservabilityDbTab.vue'
import ObservabilityDeploysTab from '~/components/admin/observability/ObservabilityDeploysTab.vue'
import ObservabilityEndpointsTab from '~/components/admin/observability/ObservabilityEndpointsTab.vue'
import ObservabilityErrorsTab from '~/components/admin/observability/ObservabilityErrorsTab.vue'
import ObservabilityHealthTab from '~/components/admin/observability/ObservabilityHealthTab.vue'
import ObservabilityIntegrationsTab from '~/components/admin/observability/ObservabilityIntegrationsTab.vue'
import ObservabilityOverviewTab from '~/components/admin/observability/ObservabilityOverviewTab.vue'
import ObservabilityPagesTab from '~/components/admin/observability/ObservabilityPagesTab.vue'
import ObservabilityRunsTab from '~/components/admin/observability/ObservabilityRunsTab.vue'
import ObservabilitySecurityTab from '~/components/admin/observability/ObservabilitySecurityTab.vue'
import ObservabilitySystemTab from '~/components/admin/observability/ObservabilitySystemTab.vue'
import ObservabilityTraceDrawer from '~/components/admin/observability/ObservabilityTraceDrawer.vue'
import { useAuthStore } from '~/stores/auth'

const auth = useAuthStore()
const isPlatformAdmin = computed(() => auth.role === 'platform_admin')

const TABS = [
  { value: 'health', label: 'Saúde' },
  { value: 'overview', label: 'Notas' },
  { value: 'endpoints', label: 'Endpoints' },
  { value: 'db', label: 'Banco' },
  { value: 'integrations', label: 'Integrações' },
  { value: 'pages', label: 'Paginas' },
  { value: 'errors', label: 'Erros' },
  { value: 'system', label: 'Sistema' },
  { value: 'security', label: 'Segurança' },
  { value: 'alerts', label: 'Alertas' },
  { value: 'deploys', label: 'Deploys' },
  { value: 'runs', label: 'Execucoes' },
]
const PERIODS = [
  { value: '1', label: '1 dia' },
  { value: '7', label: '7 dias' },
  { value: '30', label: '30 dias' },
]

const tab = ref('health')
const period = ref('7')
const days = computed(() => Number(period.value))
const PERIOD_TABS = ['overview', 'endpoints', 'db', 'integrations', 'pages', 'errors']
const usesPeriod = computed(() => PERIOD_TABS.includes(tab.value))
const traceId = ref('')
</script>

<template>
  <section class="obs-workspace">
    <AdminPageHeader
      eyebrow="Plataforma"
      title="Observabilidade"
      description="Saúde da API, do banco e das paginas, com notas por modulo e historico das execucoes do runner."
    />

    <UAlert
      v-if="!isPlatformAdmin"
      color="error"
      variant="soft"
      icon="i-lucide-shield-alert"
      title="Acesso restrito"
      description="Esta area e exclusiva para administradores da plataforma."
    />

    <template v-else>
      <div class="obs-toolbar obs-toolbar--head">
        <AppSegmentedFilter v-model="tab" :options="TABS" aria-label="Aba de observabilidade" />
        <AppSegmentedFilter
          v-if="usesPeriod"
          v-model="period"
          :options="PERIODS"
          aria-label="Periodo"
        />
      </div>

      <ObservabilityHealthTab v-if="tab === 'health'" @navigate="tab = $event" />
      <ObservabilityOverviewTab v-else-if="tab === 'overview'" :days="days" />
      <ObservabilityEndpointsTab v-else-if="tab === 'endpoints'" :days="days" />
      <ObservabilityDbTab v-else-if="tab === 'db'" :days="days" @trace="traceId = $event" />
      <ObservabilityIntegrationsTab v-else-if="tab === 'integrations'" :days="days" />
      <ObservabilityPagesTab v-else-if="tab === 'pages'" :days="days" />
      <ObservabilityErrorsTab v-else-if="tab === 'errors'" :days="days" @trace="traceId = $event" />
      <ObservabilitySystemTab v-else-if="tab === 'system'" />
      <ObservabilitySecurityTab v-else-if="tab === 'security'" />
      <ObservabilityAlertsTab v-else-if="tab === 'alerts'" />
      <ObservabilityDeploysTab v-else-if="tab === 'deploys'" />
      <ObservabilityRunsTab v-else />

      <ObservabilityTraceDrawer
        v-if="traceId"
        :key="traceId"
        :correlation-id="traceId"
        @close="traceId = ''"
      />
    </template>
  </section>
</template>

<!-- Estilos compartilhados (prefixo obs-) usados por todas as abas filhas. -->
<style>
.obs-workspace {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: grid;
  align-content: start;
  gap: 1rem;
  padding: 1rem 1.2rem 2rem;
}

.obs-stack {
  display: grid;
  gap: 1.1rem;
  min-width: 0;
}

.obs-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.6rem;
}

.obs-toolbar--head {
  justify-content: space-between;
}

.obs-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
  gap: 0.75rem;
}

.obs-card,
.obs-section {
  min-width: 0;
  display: grid;
  gap: 0.6rem;
  padding: 0.9rem 1rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-md);
  background: rgb(var(--surface));
  box-shadow: var(--shadow-card);
  align-content: start;
}

.obs-card__label {
  font-size: 0.74rem;
  color: rgb(var(--muted));
}

.obs-card__value {
  font-size: 1.35rem;
  line-height: 1.2;
}

.obs-section__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.obs-section__title {
  margin: 0;
  font-size: 0.9rem;
  font-weight: 600;
}

.obs-hint,
.obs-muted {
  margin: 0;
  font-size: 0.78rem;
  color: rgb(var(--muted));
}

.obs-tablewrap {
  min-width: 0;
  overflow-x: auto;
}

.obs-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8rem;
}

.obs-table th,
.obs-table td {
  padding: 0.45rem 0.6rem;
  border-bottom: 1px solid rgb(var(--border));
  text-align: left;
  vertical-align: top;
}

.obs-table th {
  font-size: 0.72rem;
  font-weight: 600;
  color: rgb(var(--muted));
  white-space: nowrap;
}

.obs-num {
  text-align: right !important;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.obs-mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.74rem;
}

.obs-clip {
  max-width: 28rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.obs-chip {
  display: inline-block;
  margin: 0 0.25rem 0.15rem 0;
  padding: 0.05rem 0.4rem;
  border-radius: 999px;
  background: rgb(var(--surface-2));
  font-size: 0.7rem;
}

.obs-band {
  display: inline-block;
  min-width: 2.2rem;
  padding: 0.05rem 0.5rem;
  border-radius: 999px;
  font-size: 0.74rem;
  font-weight: 600;
  text-align: center;
  background: rgb(var(--surface-2));
  color: rgb(var(--muted));
}

.obs-band--good {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.obs-band--warn {
  background: rgb(var(--warning) / 0.2);
  color: rgb(var(--warning));
}

.obs-band--bad {
  background: rgb(var(--danger) / 0.16);
  color: rgb(var(--danger));
}

.obs-text--good {
  color: rgb(var(--success));
}

.obs-text--warn {
  color: rgb(var(--warning));
}

.obs-text--bad {
  color: rgb(var(--danger));
}

.obs-row--regression {
  background: rgb(var(--danger) / 0.07);
}

.obs-link {
  display: inline-block;
  margin: 0 0.35rem 0.15rem 0;
  padding: 0;
  border: 0;
  background: none;
  color: rgb(var(--primary));
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.72rem;
  cursor: pointer;
  text-decoration: underline;
}

.obs-state {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 1.2rem;
  border: 1px dashed rgb(var(--border));
  border-radius: var(--radius-md);
  color: rgb(var(--muted));
  font-size: 0.85rem;
}

.obs-state p {
  margin: 0.25rem 0 0;
}

.obs-state--error {
  color: rgb(var(--danger));
}

.obs-runner {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.6rem;
}

.obs-workerstate {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.8rem;
}

.obs-dot {
  width: 0.55rem;
  height: 0.55rem;
  border-radius: 999px;
  background: rgb(var(--danger));
}

.obs-dot--on {
  background: rgb(var(--success));
}

.obs-checkgroup ul,
.obs-list {
  margin: 0.3rem 0 0;
  padding-left: 1rem;
  display: grid;
  gap: 0.35rem;
}

.obs-list {
  list-style: none;
  padding-left: 0;
}

.obs-list li {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.4rem;
}

.obs-trace {
  display: grid;
  gap: 0.9rem;
  padding: 1rem;
}

.obs-kv {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 0.25rem 0.9rem;
  margin: 0;
  font-size: 0.8rem;
}

.obs-kv dt {
  color: rgb(var(--muted));
}

.obs-kv dd {
  margin: 0;
  word-break: break-word;
}
</style>
