<script setup lang="ts">
import type { SegmentVersionView } from '~/domain/customer-data/segment-types'

defineProps<{
  versions: SegmentVersionView[]
  draft?: SegmentVersionView
  dirty: boolean
  busy: boolean
  canManage: boolean
  canPublish: boolean
}>()

const emit = defineEmits<{
  validate: []
  publish: []
}>()
</script>

<template>
  <section class="segment-compact-panel">
    <header>
      <div>
        <h3>Versoes e publicacao</h3>
        <p>Published e imutavel; alteracoes continuam em draft revisionado.</p>
      </div>
      <div v-if="draft" class="segment-actions">
        <button type="button" :disabled="!canManage || dirty || busy" @click="emit('validate')">
          Validar
        </button>
        <button
          type="button"
          :disabled="!canPublish || dirty || busy || draft.validationStatus !== 'valid'"
          @click="emit('publish')"
        >
          Publicar
        </button>
      </div>
    </header>
    <ul>
      <li v-for="version in versions" :key="version.id">
        <strong>v{{ version.versionNumber }}</strong>
        <span>{{ version.status }}</span>
        <span>rev {{ version.revision }}</span>
        <span>{{ version.validationStatus || 'sem validacao' }}</span>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.segment-compact-panel {
  display: grid;
  gap: 0.75rem;
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 0.9rem;
}

.segment-compact-panel header,
.segment-actions,
.segment-compact-panel li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.segment-compact-panel h3,
.segment-compact-panel p {
  margin: 0;
}

.segment-compact-panel p,
.segment-compact-panel span {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.segment-compact-panel ul {
  display: grid;
  gap: 0.45rem;
  margin: 0;
  padding: 0;
  list-style: none;
}
</style>
