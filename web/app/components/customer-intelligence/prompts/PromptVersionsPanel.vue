<script setup lang="ts">
import type { PromptVersionView } from '~/domain/customer-intelligence/prompt-types'

defineProps<{ versions: PromptVersionView[]; activeVersionId?: string | null }>()
</script>

<template>
  <section class="prompt-versions">
    <h3>Versoes</h3>
    <p v-if="!versions.length">Nenhuma versao retornada.</p>
    <ul v-else>
      <li v-for="version in versions" :key="version.id">
        <strong>v{{ version.version }}</strong>
        <span>{{ version.status }}</span>
        <small>{{ version.checksum || version.id }}</small>
        <b v-if="version.id === activeVersionId">ativa</b>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.prompt-versions ul {
  display: grid;
  gap: 0.35rem;
  padding: 0;
  list-style: none;
}

.prompt-versions li {
  display: grid;
  grid-template-columns: auto auto minmax(0, 1fr) auto;
  gap: 0.6rem;
  padding: 0.55rem;
  border-radius: 0.65rem;
  background: rgb(var(--surface-2) / 0.6);
}

.prompt-versions p,
.prompt-versions span,
.prompt-versions small {
  color: rgb(var(--muted));
  font-size: 0.72rem;
  overflow-wrap: anywhere;
}
</style>
