<script setup lang="ts">
import type { CustomerRelationshipProfile } from '~/domain/customer-data/profile-types'
import type { IntelligenceTimelineItem } from '~/domain/customer-intelligence/types'

const props = defineProps<{
  profile: CustomerRelationshipProfile | null
  timeline: IntelligenceTimelineItem[]
}>()

const items = computed(() => [
  ...props.timeline,
  ...(props.profile?.touchpointRefs ?? []).map((item) => ({
    id: item.id,
    kind: 'touchpoint',
    title: item.sourceKind,
    occurredAt: item.occurredAt,
  })),
])
</script>

<template>
  <article class="ci-card">
    <h3>Historico autorizado</h3>
    <p v-if="!items.length" class="muted">Nenhum evento retornado.</p>
    <ol v-else>
      <li v-for="item in items" :key="`${item.kind}-${item.id}`">
        <div>
          <strong>{{ item.title }}</strong>
          <span>{{ item.kind }}</span>
        </div>
        <small>{{ new Date(item.occurredAt).toLocaleString('pt-BR') }}</small>
      </li>
    </ol>
  </article>
</template>

<style scoped>
.ci-card {
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 1rem;
  background: rgb(var(--surface) / 0.9);
}

.ci-card h3 {
  margin-top: 0;
}

.ci-card ol {
  display: grid;
  gap: 0.55rem;
  padding-left: 1.25rem;
}

.ci-card li div {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
}

.ci-card span,
.ci-card small,
.muted {
  color: rgb(var(--muted));
  font-size: 0.75rem;
}
</style>
