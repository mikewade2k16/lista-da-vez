<script setup lang="ts">
import CustomerIntelligenceStatus from '~/components/customer-intelligence/CustomerIntelligenceStatus.vue'
import { useRelationshipIntelligenceRefresh } from '~/composables/customer-intelligence/useRelationshipIntelligenceRefresh'

defineProps<{
  subjectId: string
  relationshipId: string
}>()
const refresh = useRelationshipIntelligenceRefresh()
</script>

<template>
  <section class="refresh-control">
    <div>
      <small>Execucao headless e duravel</small>
      <h3>Atualizar inteligencia do cliente</h3>
      <p>
        Enfileira prompts separados para resumo, follow-up, ofertas, datas importantes e sugestoes
        de fontes. O painel nao precisa permanecer aberto.
      </p>
    </div>
    <button
      type="button"
      :disabled="
        refresh.enqueuing.value ||
        !refresh.access.canManageIntelligenceProfile.value ||
        !subjectId ||
        !relationshipId
      "
      @click="refresh.enqueue(subjectId, relationshipId)"
    >
      {{ refresh.enqueuing.value ? 'Agendando...' : 'Gerar / atualizar' }}
    </button>
    <p v-if="refresh.lastJob.value" class="refresh-control__success" role="status">
      Job {{ refresh.lastJob.value.created ? 'agendado' : 'ja existente' }}. Acompanhe cada processo
      em Atendimentos/Runs e recarregue o perfil ao concluir.
    </p>
    <CustomerIntelligenceStatus
      v-if="refresh.error.value"
      title="Atualizacao nao agendada"
      :error="refresh.error.value"
    />
  </section>
</template>

<style scoped>
.refresh-control {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 0.75rem;
  align-items: center;
  padding: 1rem;
  border: 1px solid rgb(var(--primary) / 0.25);
  border-radius: 1rem;
  background: rgb(var(--primary) / 0.05);
}

.refresh-control h3,
.refresh-control p {
  margin: 0.2rem 0;
}

.refresh-control small,
.refresh-control p {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.refresh-control button {
  min-height: 2.5rem;
  padding: 0 0.9rem;
  border: 0;
  border-radius: 999px;
  background: rgb(var(--primary));
  color: white;
  font-weight: 700;
}

.refresh-control__success {
  grid-column: 1 / -1;
  color: rgb(var(--success)) !important;
}

@media (max-width: 720px) {
  .refresh-control {
    grid-template-columns: 1fr;
  }
}
</style>
