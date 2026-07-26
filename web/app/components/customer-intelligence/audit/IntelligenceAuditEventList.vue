<script setup lang="ts">
import CustomerIntelligenceStatus from '~/components/customer-intelligence/CustomerIntelligenceStatus.vue'
import type { IntelligenceAuditEventView } from '~/domain/customer-intelligence/audit-types'
import IntelligenceAuditDiffPanel from './IntelligenceAuditDiffPanel.vue'

defineProps<{ events: IntelligenceAuditEventView[]; loading: boolean }>()
const emit = defineEmits<{ observation: [event: IntelligenceAuditEventView] }>()
</script>

<template>
  <div class="audit-list" :aria-busy="loading">
    <CustomerIntelligenceStatus
      v-if="loading && !events.length"
      title="Carregando auditoria"
      loading
    />
    <CustomerIntelligenceStatus
      v-else-if="!events.length"
      title="Sem eventos"
      empty
      empty-text="Nenhum evento foi retornado para os filtros."
    />
    <article v-for="event in events" v-else :key="event.id">
      <header>
        <div>
          <strong>{{ event.action }}</strong>
          <small>{{ event.entityType }} · {{ event.entityRef }}</small>
        </div>
        <time :datetime="event.occurredAt">
          {{ new Date(event.occurredAt).toLocaleString('pt-BR') }}
        </time>
      </header>
      <p>
        Ator: {{ event.actor.display || event.actor.type }} · reason:
        {{ event.reasonCode || '—' }} · correlation:
        {{ event.correlationCode || '—' }}
      </p>
      <div class="audit-list__hashes">
        <span>old {{ event.oldHash || '—' }}</span>
        <span>new {{ event.newHash || '—' }}</span>
      </div>
      <IntelligenceAuditDiffPanel :items="event.diff ?? []" />
      <button
        v-if="event.canOpenObservation && event.observationRef"
        type="button"
        @click="emit('observation', event)"
      >
        Abrir observacao minimizada
      </button>
    </article>
  </div>
</template>

<style scoped>
.audit-list {
  display: grid;
  gap: 0.75rem;
}

.audit-list article {
  display: grid;
  gap: 0.65rem;
  padding: 0.9rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 0.85rem;
}

.audit-list header,
.audit-list__hashes {
  display: flex;
  justify-content: space-between;
  gap: 0.65rem;
  flex-wrap: wrap;
}

.audit-list header div {
  display: grid;
  gap: 0.2rem;
}

.audit-list small,
.audit-list time,
.audit-list p,
.audit-list__hashes {
  color: rgb(var(--muted));
  font-size: 0.7rem;
}

.audit-list p {
  margin: 0;
}
</style>
