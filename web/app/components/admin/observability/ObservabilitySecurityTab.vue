<script setup lang="ts">
import ObservabilityFindingsQueue from '~/components/admin/observability/ObservabilityFindingsQueue.vue'
import ObservabilitySecurityNotice from '~/components/admin/observability/ObservabilitySecurityNotice.vue'
import ObservabilityState from '~/components/admin/observability/ObservabilityState.vue'
import { useObsQuery } from '~/composables/useObservability'
import type {
  ObsSecurity,
  ObsSecurityFinding,
  ObsSecuritySummary,
  ObsSeverity,
} from '~/types/observability'
import { formatDate } from '~/utils/observability/format'
import {
  CHECK_STATUS_LABEL,
  checkStatusBand,
  groupChecksByCategory,
  SEVERITY_ORDER,
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
    if (finding.status === 'fixed') fixed += 1
    else if (finding.status === 'open' && SEVERITY_ORDER.includes(finding.severity))
      open[finding.severity] += 1
  }
  return { checks, findings: { open, fixed } }
})

// Troca so a linha salva (o servidor devolve o achado ja com `overdue` calculado).
function onFindingSaved(updated: ObsSecurityFinding) {
  if (!data.value) return
  data.value = {
    ...data.value,
    findings: data.value.findings.map((f) => (f.id === updated.id ? updated : f)),
  }
}
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

      <ObservabilityFindingsQueue :findings="findings" @saved="onFindingSaved" />
    </div>
  </ObservabilityState>
</template>
