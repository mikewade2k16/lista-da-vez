<script setup lang="ts">
import { ref } from 'vue'
import CustomerIntelligenceStatus from '~/components/customer-intelligence/CustomerIntelligenceStatus.vue'
import { useRecommendations } from '~/composables/customer-intelligence/useRecommendations'
import type { CustomerRecommendationView } from '~/domain/customer-intelligence/recommendation-types'

const props = defineProps<{
  relationshipId: string
  canManage: boolean
}>()

const recommendations = useRecommendations(() => props.relationshipId)
const selectedReasons = ref<Record<string, string>>({})

function actionKey(recommendationId: string, action: string): string {
  return `${recommendationId}:${action}`
}

function act(recommendation: CustomerRecommendationView, action: 'approve' | 'reject'): void {
  const reason = selectedReasons.value[actionKey(recommendation.id, action)] ?? ''
  if (!reason.trim()) return
  void recommendations.act(recommendation, action, reason)
}

function reasonOptions(action: 'approve' | 'reject') {
  return action === 'approve'
    ? recommendations.decisionOptions.value.approveReasons
    : recommendations.decisionOptions.value.rejectReasons
}
</script>

<template>
  <section class="recommendations-panel">
    <header>
      <div>
        <h2>Recomendacoes</h2>
        <p>Propostas explicaveis. Aprovar nao transforma recomendacao em fato.</p>
      </div>
      <button
        type="button"
        :disabled="recommendations.loading.value"
        @click="recommendations.load()"
      >
        Atualizar
      </button>
    </header>

    <CustomerIntelligenceStatus
      v-if="recommendations.loading.value && !recommendations.items.value.length"
      title="Carregando recomendacoes"
      loading
    />
    <CustomerIntelligenceStatus
      v-else-if="recommendations.error.value"
      title="Recomendacoes indisponiveis"
      :error="recommendations.error.value"
    />
    <CustomerIntelligenceStatus
      v-else-if="!recommendations.items.value.length"
      title="Sem recomendacoes"
      empty
      empty-text="Nenhuma proposta valida foi retornada para este relacionamento."
    />

    <article v-for="recommendation in recommendations.items.value" v-else :key="recommendation.id">
      <header>
        <div>
          <small>
            {{ recommendation.type }} - {{ recommendation.status }}
            <template v-if="recommendation.generatedByAi">- gerada por IA</template>
          </small>
          <h3>{{ recommendation.title }}</h3>
        </div>
        <span v-if="recommendation.confidence != null">
          {{ Math.round(recommendation.confidence * 100) }}%
        </span>
      </header>
      <p>{{ recommendation.rationale }}</p>
      <div class="recommendations-panel__meta">
        <span>freshness {{ recommendation.freshnessStatus || '-' }}</span>
        <span>policy {{ recommendation.policyVersionRef || '-' }}</span>
        <span>prompt {{ recommendation.promptVersionRef || '-' }}</span>
      </div>

      <div v-if="canManage" class="recommendations-panel__actions">
        <div
          v-for="action in recommendation.allowedActions"
          :key="action"
          class="recommendations-panel__action"
        >
          <label>
            <span>Motivo para {{ action }}</span>
            <select v-model="selectedReasons[actionKey(recommendation.id, action)]">
              <option value="">Selecione um motivo auditavel</option>
              <option
                v-for="option in reasonOptions(action)"
                :key="option.value"
                :value="option.value"
              >
                {{ option.label }}
              </option>
            </select>
          </label>
          <button
            type="button"
            :disabled="
              recommendations.mutatingId.value === recommendation.id ||
              !selectedReasons[actionKey(recommendation.id, action)]?.trim()
            "
            @click="act(recommendation, action)"
          >
            {{ action }}
          </button>
        </div>
      </div>
    </article>
    <p class="recommendations-panel__notice">
      Aprovar ou rejeitar registra feedback; nao envia mensagem. Quando a resposta por IA esta
      ativa, o runtime produz o draft e a entrega passa pelo Omnichannel, FSM e outbox.
    </p>
  </section>
</template>

<style scoped>
.recommendations-panel,
.recommendations-panel article {
  display: grid;
  gap: 0.8rem;
}

.recommendations-panel {
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 1rem;
}

.recommendations-panel > header,
.recommendations-panel article > header,
.recommendations-panel__meta,
.recommendations-panel__actions,
.recommendations-panel__action {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.65rem;
  flex-wrap: wrap;
}

.recommendations-panel h2,
.recommendations-panel h3,
.recommendations-panel p {
  margin: 0;
}

.recommendations-panel article {
  padding: 0.85rem;
  border: 1px solid rgb(var(--border) / 0.65);
  border-radius: 0.75rem;
}

.recommendations-panel small,
.recommendations-panel__meta,
.recommendations-panel__notice,
.recommendations-panel__action label {
  color: rgb(var(--muted));
  font-size: 0.7rem;
}

.recommendations-panel__action {
  align-items: end;
}

.recommendations-panel__action label {
  display: grid;
  gap: 0.25rem;
}

.recommendations-panel__action select {
  min-height: 2.25rem;
  padding: 0.45rem 0.6rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 0.55rem;
  background: rgb(var(--surface-1));
  color: rgb(var(--text));
}
</style>
