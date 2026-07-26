<script setup lang="ts">
import type { IntelligenceSourceConfig } from '~/domain/customer-intelligence/sources'

defineProps<{ source: IntelligenceSourceConfig | null }>()
</script>

<template>
  <div class="source-health">
    <strong>{{ source?.status || 'Nao configurada' }}</strong>
    <span v-if="source?.lastSuccessAt">
      Ultimo sucesso {{ new Date(source.lastSuccessAt).toLocaleString('pt-BR') }}
    </span>
    <span v-else>{{ source?.healthReasonCode || 'Sem coleta registrada' }}</span>
  </div>
</template>

<style scoped>
.source-health {
  display: grid;
  gap: 0.15rem;
}

.source-health strong {
  font-size: 0.78rem;
}

.source-health span {
  color: rgb(var(--muted));
  font-size: 0.7rem;
}
</style>
