<script setup lang="ts">
import type { IntelligenceAuditDiffItem } from '~/domain/customer-intelligence/audit-types'

defineProps<{ items: IntelligenceAuditDiffItem[] }>()
</script>

<template>
  <div class="audit-diff">
    <p v-if="!items.length">Diff nao disponibilizado.</p>
    <div v-for="item in items" :key="`${item.fieldLabel}:${item.changeType}`">
      <strong>{{ item.fieldLabel }}</strong>
      <span>{{ item.changeType }}</span>
      <del v-if="item.oldDisplay">{{ item.oldDisplay }}</del>
      <ins v-if="item.newDisplay">{{ item.newDisplay }}</ins>
    </div>
  </div>
</template>

<style scoped>
.audit-diff {
  display: grid;
  gap: 0.4rem;
}

.audit-diff div {
  display: grid;
  grid-template-columns: minmax(8rem, 0.7fr) auto minmax(7rem, 1fr) minmax(7rem, 1fr);
  gap: 0.5rem;
  align-items: center;
  font-size: 0.72rem;
}

.audit-diff span,
.audit-diff p {
  color: rgb(var(--muted));
}

.audit-diff ins {
  color: rgb(var(--success));
  text-decoration: none;
}
</style>
