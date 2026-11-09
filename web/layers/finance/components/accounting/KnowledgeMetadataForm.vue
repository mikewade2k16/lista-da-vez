<script setup lang="ts">
import type { KnowledgeMetadata } from '../../types/knowledge'
const model = defineModel<KnowledgeMetadata>({ required: true })
const labels = {
  authority: 'Órgão / autoridade',
  jurisdiction: 'Jurisdição (ou unknown)',
  regime: 'Regime (ou unknown)',
  classification: 'Classificação do material',
  validitySource: 'Fonte da vigência verificada',
} as const
const dates = {
  publishedOn: 'Publicação (se verificada)',
  validFrom: 'Vigência inicial verificada',
  validTo: 'Fim exclusivo da vigência (se conhecido)',
} as const
function date(key: keyof typeof dates, event: Event) {
  model.value[key] = (event.target as HTMLInputElement).value || null
}
</script>
<template>
  <fieldset class="knowledge-metadata">
    <legend>Metadados da fonte</legend>
    <label v-for="(label, key) in labels" :key="key">
      {{ label }}
      <input v-model="model[key]" class="omni-input" maxlength="1000" />
    </label>
    <label v-for="(label, key) in dates" :key="key">
      {{ label }}
      <input class="omni-input" type="date" :value="model[key] || ''" @input="date(key, $event)" />
    </label>
  </fieldset>
</template>
<style scoped>
.knowledge-metadata {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.5rem;
  min-width: 0;
}
.knowledge-metadata label {
  display: grid;
  gap: 0.25rem;
}
</style>
