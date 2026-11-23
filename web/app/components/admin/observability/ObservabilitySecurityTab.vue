<script setup lang="ts">
import ObservabilitySecurityNotice from '~/components/admin/observability/ObservabilitySecurityNotice.vue'
import ObservabilityState from '~/components/admin/observability/ObservabilityState.vue'
import { useObsQuery } from '~/composables/useObservability'
import type { ObsSecurity, ObsSecuritySummary, ObsSeverity } from '~/types/observability'
import { formatDate } from '~/utils/observability/format'
import {
  CHECK_STATUS_LABEL,
  checkStatusBand,
  groupChecksByCategory,
  SEVERITY_LABEL,
  SEVERITY_ORDER,
  severityBand,
} from '~/utils/observability/health'

const { data, loading, error, reload } = useObsQuery<ObsSecurity>('/security')

const groups = computed(() => groupChecksByCategory(data.value?.checks || []))
const findings = computed(() => data.value?.findings || [])

// Resumo derivado das listas (o banner usa o mesmo texto da aba Saúde).
const summary = computed<ObsSecuritySummary>(() => {
  const checks = { pass: 0, fail: 0, partial: 0, not_tested: 0 }
  for (const check of data.value?.checks || []) checks[check.status] += 1
  const open = { critical: 0, high: 0, medium: 0, low: 0 } as Record<ObsSeverity, number>
  let fixed = 0
  for (const finding of findings.value) {
    if (/^(fixed|closed|resolved)/i.test(finding.status)) fixed += 1
    else if (SEVERITY_ORDER.includes(finding.severity)) open[finding.severity] += 1
  }
  return { checks, findings: { open, fixed } }
})
</script>

<template>
  <ObservabilityState
    :loading="loading"
    :error="error"
    :empty="!data || (data.checks.length === 0 && data.findings.length === 0)"
    empty-title="O catálogo de segurança ainda não foi sincronizado."
    empty-hint="Rode `npm run obs:security -- --sync` para carregar os ataques e os achados da auditoria."
    @retry="reload"
  >
    <div v-if="data" class="obs-stack">
      <ObservabilitySecurityNotice :summary="summary" />

      <section v-for="group in groups" :key="group.category" class="obs-section">
        <h3 class="obs-section__title">{{ group.category }}</h3>
        <div class="obs-tablewrap">
          <table class="obs-table">
            <thead>
              <tr>
                <th>Verificação</th>
                <th>Status</th>
                <th>Padrão</th>
                <th>Detalhe</th>
                <th>Data</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="check in group.items" :key="check.key">
                <td>{{ check.title }}</td>
                <td>
                  <span :class="['obs-band', `obs-band--${checkStatusBand(check.status)}`]">
                    {{ CHECK_STATUS_LABEL[check.status] || check.status }}
                  </span>
                </td>
                <td class="obs-muted">{{ check.standard }}</td>
                <td class="obs-muted">{{ check.detail || '-' }}</td>
                <td class="obs-muted">{{ formatDate(check.checkedAt) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="obs-section">
        <h3 class="obs-section__title">Achados da auditoria</h3>
        <p v-if="findings.length === 0" class="obs-hint">Nenhum achado importado.</p>
        <div v-else class="obs-tablewrap">
          <table class="obs-table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Severidade</th>
                <th>Título</th>
                <th>Área</th>
                <th>Local</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="finding in findings" :key="finding.id">
                <td class="obs-mono">{{ finding.id }}</td>
                <td>
                  <span :class="['obs-band', `obs-band--${severityBand(finding.severity)}`]">
                    {{ SEVERITY_LABEL[finding.severity] || finding.severity }}
                  </span>
                </td>
                <td>{{ finding.title }}</td>
                <td>{{ finding.area }}</td>
                <td class="obs-mono">{{ finding.location }}</td>
                <td>{{ finding.status }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </ObservabilityState>
</template>
