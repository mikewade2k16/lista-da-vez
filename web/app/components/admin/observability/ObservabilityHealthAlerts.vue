<script setup lang="ts">
import type { ObsHealth } from '~/types/observability'
import {
  alertChannelText,
  formatAge,
  SEVERITY_LABEL,
  severityBand,
} from '~/utils/observability/health'

defineProps<{ alerts: ObsHealth['alerts']; now: number }>()
defineEmits<{ open: [] }>()
</script>

<template>
  <section class="obs-section">
    <div class="obs-section__head">
      <h3 class="obs-section__title">Alertas</h3>
      <UButton
        label="Ver alertas"
        size="xs"
        color="neutral"
        variant="soft"
        @click="$emit('open')"
      />
    </div>
    <p :class="alerts.configured ? 'obs-text--good' : 'obs-text--warn'">
      <strong>{{ alertChannelText(alerts) }}</strong>
    </p>
    <p class="obs-muted">WhatsApp: planejado — ainda não disponível.</p>
    <p v-if="alerts.open.length === 0" class="obs-hint">Nenhum alerta aberto.</p>
    <ul v-else class="obs-list">
      <li v-for="alert in alerts.open.slice(0, 5)" :key="alert.id">
        <span :class="['obs-band', `obs-band--${severityBand(alert.severity)}`]">
          {{ SEVERITY_LABEL[alert.severity] || alert.severity }}
        </span>
        <span class="obs-clip">{{ alert.title }}</span>
        <span class="obs-muted">{{ formatAge(alert.since, now) }}</span>
      </li>
      <li v-if="alerts.open.length > 5" class="obs-muted">+{{ alerts.open.length - 5 }} alertas</li>
    </ul>
  </section>
</template>
