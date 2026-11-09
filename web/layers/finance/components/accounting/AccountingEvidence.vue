<script setup lang="ts">
import { computed } from 'vue'
import { evidenceCards, plusUsage } from '../../utils/plus-evidence'
const props = defineProps<{ evidence: unknown[]; data: Record<string, unknown> }>()
const sources = computed(() => evidenceCards(props.evidence))
const strings = (value: unknown) =>
  Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : []
const usage = computed(() => plusUsage(props.data.usage))
</script>
<template>
  <div class="accounting-evidence">
    <p v-if="data.coverage === 'informational'">
      Resumo informativo; não confirma regra tributária aplicável.
    </p>
    <p v-if="data.coverage === 'abstained'">
      Abstenção: faltam condições para uma resposta fundamentada.
    </p>
    <details v-if="strings(data.assumptions).length || strings(data.missingData).length">
      <summary>Hipóteses e dados faltantes</summary>
      <ul>
        <li v-for="item in strings(data.assumptions)" :key="item">Hipótese: {{ item }}</li>
        <li v-for="item in strings(data.missingData)" :key="item">Pendente: {{ item }}</li>
      </ul>
    </details>
    <article v-for="source in sources" :key="source.id">
      <strong v-if="source.title">{{ source.title }}</strong>
      <blockquote>{{ source.excerpt }}</blockquote>
      <small>
        <a v-if="source.href" :href="source.href" target="_blank" rel="noopener noreferrer">
          {{ source.source }}
        </a>
        <span v-else>{{ source.source }}</span>
        · v{{ source.version }} · {{ source.id }}
      </small>
      <small v-if="source.review" class="accounting-evidence__review">{{ source.review }}</small>
    </article>
    <p v-if="usage">
      Modelo: {{ usage.model }} · {{ usage.tokens }} · {{ usage.cost }} · {{ usage.latency }}
    </p>
  </div>
</template>
<style scoped>
.accounting-evidence {
  min-width: 0;
  overflow-wrap: anywhere;
}
.accounting-evidence__review {
  display: block;
  color: rgb(var(--muted));
}
.accounting-evidence blockquote {
  white-space: pre-wrap;
}
</style>
