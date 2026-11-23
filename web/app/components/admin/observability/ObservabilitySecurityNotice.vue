<script setup lang="ts">
import type { ObsSecuritySummary } from '~/types/observability'
import { securityCoverage } from '~/utils/observability/health'

// Aviso honesto de cobertura de seguranca (Saude e aba Seguranca). Mostra o texto
// calculado: quantos ataques foram testados, falhas e achados abertos.
const props = defineProps<{ summary: ObsSecuritySummary | null; linkable?: boolean }>()
defineEmits<{ open: [] }>()

const coverage = computed(() => securityCoverage(props.summary))
</script>

<template>
  <div
    :class="['obs-notice', coverage.low || coverage.failed > 0 ? 'obs-notice--warn' : '']"
    role="alert"
  >
    <UIcon name="i-lucide-shield-alert" class="obs-notice__icon" />
    <div class="obs-notice__body">
      <strong>{{ coverage.text }}</strong>
      <p class="obs-muted">{{ coverage.detail }}</p>
    </div>
    <UButton
      v-if="linkable"
      label="Ver segurança"
      size="xs"
      color="neutral"
      variant="soft"
      @click="$emit('open')"
    />
  </div>
</template>
