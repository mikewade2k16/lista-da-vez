<script setup lang="ts">
import CustomerIntelligenceStatus from '~/components/customer-intelligence/CustomerIntelligenceStatus.vue'
import type {
  IntelligenceAuditEventView,
  IntelligenceAuditFilters as IntelligenceAuditFilterState,
} from '~/domain/customer-intelligence/audit-types'
import { useIntelligenceAudit } from '~/composables/customer-intelligence/useIntelligenceAudit'
import IntelligenceAuditEventList from './IntelligenceAuditEventList.vue'
import IntelligenceAuditFilters from './IntelligenceAuditFilters.vue'
import IntelligenceObservationDrawer from './IntelligenceObservationDrawer.vue'

const audit = useIntelligenceAudit()
const filters = ref<Omit<IntelligenceAuditFilterState, 'clientAccountId' | 'cursor'>>({})

async function openObservation(event: IntelligenceAuditEventView): Promise<void> {
  await audit.openObservation(event)
}

async function revealObservation(reasonCode: string): Promise<void> {
  await audit.revealObservation(reasonCode)
}
</script>

<template>
  <div class="audit-workspace">
    <IntelligenceAuditFilters
      :filters="filters"
      :options="audit.options.value"
      :loading="audit.loading.value"
      @update="filters = $event"
      @apply="audit.load(filters)"
    />
    <CustomerIntelligenceStatus
      v-if="audit.error.value && !audit.events.value.length"
      title="Auditoria indisponivel"
      :error="audit.error.value"
      @retry="audit.load(filters)"
    />
    <IntelligenceAuditEventList
      v-else
      :events="audit.events.value"
      :loading="audit.loading.value"
      @observation="openObservation"
    />
    <button
      v-if="audit.nextCursor.value"
      type="button"
      :disabled="audit.loading.value"
      @click="audit.load(filters, true)"
    >
      Carregar mais
    </button>
    <IntelligenceObservationDrawer
      :open="audit.observationOpen.value"
      :observation="audit.observation.value"
      :loading="audit.loadingObservation.value"
      :revealing="audit.revealingObservation.value"
      :error="audit.observationError.value"
      @reveal="revealObservation"
      @update:open="audit.setObservationOpen"
    />
  </div>
</template>

<style scoped>
.audit-workspace {
  display: grid;
  gap: 1rem;
}

.audit-workspace > button {
  justify-self: center;
}
</style>
