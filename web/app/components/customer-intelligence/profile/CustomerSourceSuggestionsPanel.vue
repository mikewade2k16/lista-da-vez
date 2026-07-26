<script setup lang="ts">
import { computed, ref } from 'vue'
import CustomerIntelligenceStatus from '~/components/customer-intelligence/CustomerIntelligenceStatus.vue'
import { useSourceSuggestions } from '~/composables/customer-intelligence/useSourceSuggestions'
import type {
  SourceSuggestionReviewStatus,
  SourceSuggestionStatus,
  SourceSuggestionView,
} from '~/domain/customer-intelligence/source-suggestion-types'

const props = defineProps<{
  relationshipId: string
  canManage: boolean
}>()

const suggestions = useSourceSuggestions(() => props.relationshipId)
const selectedReasons = ref<Record<string, string>>({})
const canReview = computed(() => props.canManage && suggestions.access.canManageSources.value)

const statusLabels: Record<SourceSuggestionStatus, string> = {
  proposed: 'Pendente de revisao',
  accepted: 'Aceita',
  rejected: 'Rejeitada',
  expired: 'Expirada',
  unknown: 'Status desconhecido',
}

function selectionKey(id: string, status: SourceSuggestionReviewStatus): string {
  return `${id}:${status}`
}

function formatDate(value: string): string {
  if (!value) return 'Sem expiracao informada'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return 'Validade indisponivel'
  return new Intl.DateTimeFormat('pt-BR', {
    dateStyle: 'short',
    timeStyle: 'short',
  }).format(parsed)
}

async function review(
  suggestion: SourceSuggestionView,
  status: SourceSuggestionReviewStatus,
): Promise<void> {
  const key = selectionKey(suggestion.id, status)
  const reason = selectedReasons.value[key] ?? ''
  if (!reason) return
  if (await suggestions.review(suggestion, status, reason)) {
    selectedReasons.value[key] = ''
  }
}
</script>

<template>
  <section class="source-suggestions">
    <header class="source-suggestions__header">
      <div>
        <h2>Sugestoes de fontes</h2>
        <p>
          A IA aponta onde uma lacuna pode ser investigada. A decisao continua auditavel e
          controlada por permissao.
        </p>
      </div>
      <button type="button" :disabled="suggestions.loading.value" @click="suggestions.load()">
        Atualizar
      </button>
    </header>

    <CustomerIntelligenceStatus
      v-if="suggestions.loading.value && !suggestions.items.value.length"
      title="Carregando sugestoes de fontes"
      loading
    />
    <CustomerIntelligenceStatus
      v-else-if="suggestions.error.value"
      title="Sugestoes de fontes indisponiveis"
      :error="suggestions.error.value"
    />
    <CustomerIntelligenceStatus
      v-else-if="!suggestions.items.value.length"
      title="Sem sugestoes de fontes"
      empty
      empty-text="Nenhuma fonte adicional foi sugerida para este relacionamento."
    />

    <div v-else class="source-suggestions__list">
      <article
        v-for="suggestion in suggestions.items.value"
        :key="suggestion.id"
        class="source-suggestions__card"
      >
        <header>
          <div>
            <small>{{ statusLabels[suggestion.status] }}</small>
            <h3>{{ suggestion.sourceKey }}</h3>
          </div>
          <strong>{{ Math.round(suggestion.confidence * 100) }}%</strong>
        </header>

        <p>{{ suggestion.rationale || 'Racional nao disponibilizado.' }}</p>

        <dl class="source-suggestions__meta">
          <div>
            <dt>Validade</dt>
            <dd>{{ formatDate(suggestion.expiresAt) }}</dd>
          </div>
          <div>
            <dt>Codigo do racional</dt>
            <dd>{{ suggestion.rationaleCode || '-' }}</dd>
          </div>
        </dl>

        <div class="source-suggestions__gaps">
          <span>Lacunas identificadas</span>
          <ul v-if="suggestion.gapCodes.length">
            <li v-for="gapCode in suggestion.gapCodes" :key="gapCode">{{ gapCode }}</li>
          </ul>
          <small v-else>Nenhuma lacuna detalhada.</small>
        </div>

        <div
          v-if="canReview && suggestion.allowedActions.length"
          class="source-suggestions__actions"
        >
          <div
            v-for="action in suggestion.allowedActions"
            :key="action"
            class="source-suggestions__action"
          >
            <label>
              <span>
                Motivo registrado para
                {{ action === 'accepted' ? 'aceitar' : 'rejeitar' }}
              </span>
              <select v-model="selectedReasons[selectionKey(suggestion.id, action)]">
                <option value="" disabled>Selecione um motivo</option>
                <option
                  v-for="option in suggestions.reviewReasons[action]"
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
                suggestions.reviewingId.value === suggestion.id ||
                !selectedReasons[selectionKey(suggestion.id, action)]
              "
              @click="review(suggestion, action)"
            >
              {{ action === 'accepted' ? 'Aceitar sugestao' : 'Rejeitar sugestao' }}
            </button>
          </div>
        </div>
      </article>
    </div>

    <p class="source-suggestions__notice">
      Aceitar registra que a sugestao faz sentido. Isso nao ativa, conecta ou sincroniza a fonte,
      nao solicita credenciais e nao libera novos dados para a IA. A configuracao continua no modulo
      de Fontes.
    </p>
  </section>
</template>

<style scoped>
.source-suggestions,
.source-suggestions__list,
.source-suggestions__card,
.source-suggestions__gaps,
.source-suggestions__actions,
.source-suggestions__action {
  display: grid;
  gap: 0.8rem;
}

.source-suggestions {
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 1rem;
}

.source-suggestions__header,
.source-suggestions__card > header,
.source-suggestions__action {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.source-suggestions h2,
.source-suggestions h3,
.source-suggestions p,
.source-suggestions dl,
.source-suggestions dd {
  margin: 0;
}

.source-suggestions__card {
  padding: 0.9rem;
  border: 1px solid rgb(var(--border) / 0.65);
  border-radius: 0.8rem;
}

.source-suggestions__meta {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.6rem;
}

.source-suggestions__meta div {
  padding: 0.6rem;
  border-radius: 0.6rem;
  background: rgb(var(--surface-2) / 0.7);
}

.source-suggestions__meta dt,
.source-suggestions__gaps > span,
.source-suggestions__action label,
.source-suggestions__notice,
.source-suggestions small {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.source-suggestions__meta dd {
  margin-top: 0.2rem;
  overflow-wrap: anywhere;
}

.source-suggestions__gaps ul {
  display: flex;
  gap: 0.4rem;
  flex-wrap: wrap;
  margin: 0;
  padding: 0;
  list-style: none;
}

.source-suggestions__gaps li {
  padding: 0.25rem 0.5rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
  font-size: 0.72rem;
}

.source-suggestions__action {
  align-items: end;
}

.source-suggestions__action label {
  display: grid;
  flex: 1 1 16rem;
  gap: 0.3rem;
}

.source-suggestions__action select {
  width: 100%;
  min-height: 2.35rem;
  padding: 0.45rem 0.6rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 0.55rem;
  background: rgb(var(--surface-1));
  color: rgb(var(--text));
}

@media (max-width: 720px) {
  .source-suggestions__meta {
    grid-template-columns: 1fr;
  }
}
</style>
