<script setup lang="ts">
import CustomerIntelligenceStatus from '~/components/customer-intelligence/CustomerIntelligenceStatus.vue'
import { useRelationshipObservations } from '~/composables/customer-intelligence/useRelationshipObservations'
import {
  isObservationFieldProtected,
  safeObservationFieldDisplay,
  safeObservationProvenance,
} from '~/domain/customer-intelligence/observation-presentation'

const props = defineProps<{ relationshipId: string }>()
const relationshipId = computed(() => props.relationshipId)
const observations = useRelationshipObservations(relationshipId)
</script>

<template>
  <article class="ci-card">
    <div class="panel-heading">
      <div>
        <h3>Dados de origem usados como contexto</h3>
        <p>Snapshot coletado, minimizado e limitado pela allowlist configurada em cada fonte.</p>
      </div>
      <button type="button" :disabled="observations.loading.value" @click="observations.load()">
        Atualizar
      </button>
    </div>

    <CustomerIntelligenceStatus
      v-if="observations.loading.value && !observations.items.value.length"
      title="Carregando evidencias"
      loading
    />
    <CustomerIntelligenceStatus
      v-else-if="observations.error.value && !observations.items.value.length"
      title="Evidencias indisponiveis"
      :error="observations.error.value"
      @retry="observations.load()"
    />
    <p v-else-if="!observations.items.value.length" class="muted">
      Nenhuma observacao ativa e autorizada para este relacionamento.
    </p>

    <ul v-else class="observation-list">
      <li v-for="item in observations.items.value" :key="item.id">
        <header>
          <strong>{{ item.sourceKey }}</strong>
          <span>{{ new Date(item.observedAt).toLocaleString('pt-BR') }}</span>
        </header>
        <small>{{ safeObservationProvenance(item) }}</small>
        <dl>
          <div v-for="field in item.snapshotFields" :key="field.label">
            <dt>{{ field.label }}</dt>
            <dd>{{ safeObservationFieldDisplay(item, field) }}</dd>
            <small v-if="isObservationFieldProtected(item, field)">oculto por politica</small>
          </div>
        </dl>
      </li>
    </ul>

    <p class="notice">
      Valores pessoais, sensiveis e restritos permanecem ocultos. Esta tela nunca exibe segredo,
      ciphertext, chave de idempotencia ou payload irrestrito do provedor.
    </p>
  </article>
</template>

<style scoped>
.ci-card {
  display: grid;
  gap: 0.85rem;
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 1rem;
  background: rgb(var(--surface) / 0.9);
}

.panel-heading,
.observation-list header {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  align-items: flex-start;
}

.panel-heading h3,
.panel-heading p {
  margin: 0;
}

.panel-heading p,
.muted,
.notice,
.observation-list small {
  color: rgb(var(--muted));
  font-size: 0.75rem;
}

.observation-list {
  display: grid;
  gap: 0.75rem;
  padding: 0;
  margin: 0;
  list-style: none;
}

.observation-list > li {
  display: grid;
  gap: 0.5rem;
  padding: 0.75rem;
  border: 1px solid rgb(var(--border) / 0.65);
  border-radius: 0.75rem;
}

.observation-list dl {
  display: grid;
  gap: 0.45rem;
  margin: 0;
}

.observation-list dl div {
  display: grid;
  grid-template-columns: minmax(8rem, 0.35fr) 1fr auto;
  gap: 0.6rem;
  align-items: start;
}

.observation-list dt {
  color: rgb(var(--muted));
  font-size: 0.75rem;
}

.observation-list dd {
  margin: 0;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}
</style>
