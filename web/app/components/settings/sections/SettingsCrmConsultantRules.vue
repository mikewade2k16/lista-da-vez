<script setup>
import { computed, reactive, watch } from 'vue'

const props = defineProps({
  ctx: {
    type: Object,
    required: true,
  },
})

// Rascunho local editavel. So persistimos no change/blur de cada campo via
// ctx.saveCrmConsultantRules (merge em crmGoalPayoutPolicy.consultantRules).
const draft = reactive({
  base: 'self',
  qualityPenaltyPercent: 0.1,
  storeFloorPercent: 50,
  storeFullPercent: 80,
  reducedRate: 1.5,
  reducedRequiresOwnPercent: 100,
})

function syncFromSource() {
  const rules = props.ctx.crmGoalPayoutPolicy?.consultantRules || {}
  draft.base = rules.base === 'store' ? 'store' : 'self'
  draft.qualityPenaltyPercent = rules.qualityPenaltyPercent ?? 0.1
  draft.storeFloorPercent = rules.storeFloorPercent ?? 50
  draft.storeFullPercent = rules.storeFullPercent ?? 80
  draft.reducedRate = rules.reducedRate ?? 1.5
  draft.reducedRequiresOwnPercent = rules.reducedRequiresOwnPercent ?? 100
}

watch(() => props.ctx.crmGoalPayoutPolicy?.consultantRules, syncFromSource, {
  immediate: true,
  deep: true,
})

const baseLabel = computed(() => (draft.base === 'store' ? 'total da loja' : 'própria venda'))

async function saveRules() {
  if (!props.ctx.canEditCrmCommercialPolicy) return
  const result = await props.ctx.saveCrmConsultantRules({
    base: draft.base,
    qualityPenaltyPercent: Number(draft.qualityPenaltyPercent),
    storeFloorPercent: Number(draft.storeFloorPercent),
    storeFullPercent: Number(draft.storeFullPercent),
    reducedRate: Number(draft.reducedRate),
    reducedRequiresOwnPercent: Number(draft.reducedRequiresOwnPercent),
  })
  if (result?.ok !== false) syncFromSource()
}
</script>

<template>
  <article class="settings-card settings-card--wide">
    <header class="settings-card__header">
      <h3 class="settings-card__title">Regras do consultor (gate da loja)</h3>
    </header>

    <p class="consultant-rules__hint">
      O consultor recebe % sobre a
      <strong>{{ baseLabel }}</strong>
      , escolhendo a faixa pela
      <strong>própria meta</strong>
      (abaixo). A meta da loja é um gate sobre esse valor:
    </p>

    <ul class="consultant-rules__list">
      <li>
        Loja abaixo de
        <strong>{{ draft.storeFloorPercent }}%</strong>
        → consultor recebe
        <strong>R$ 0</strong>
        .
      </li>
      <li>
        Loja entre
        <strong>{{ draft.storeFloorPercent }}%</strong>
        e
        <strong>{{ draft.storeFullPercent }}%</strong>
        → só recebe quem bater
        <strong>{{ draft.reducedRequiresOwnPercent }}%</strong>
        da própria meta, e recebe
        <strong>{{ draft.reducedRate }}%</strong>
        da {{ baseLabel }}.
      </li>
      <li>
        Loja em
        <strong>{{ draft.storeFullPercent }}%</strong>
        ou mais → aplica a faixa pela própria meta do consultor (tabela "Consultor" acima).
      </li>
      <li>
        Penalidade:
        <strong>−{{ draft.qualityPenaltyPercent }}%</strong>
        por métrica (P.A. e Ticket Médio) não atingida.
      </li>
    </ul>

    <div class="consultant-rules__grid">
      <label class="settings-field">
        <span>Base de cálculo</span>
        <select
          v-model="draft.base"
          :disabled="!ctx.canEditCrmCommercialPolicy"
          @change="saveRules"
        >
          <option value="self">Própria venda</option>
          <option value="store">Total da loja</option>
        </select>
      </label>

      <label class="settings-field">
        <span>Loja mínima % (abaixo = 0)</span>
        <input
          v-model="draft.storeFloorPercent"
          type="number"
          min="0"
          max="1000"
          step="1"
          :disabled="!ctx.canEditCrmCommercialPolicy"
          @blur="saveRules"
        />
      </label>

      <label class="settings-field">
        <span>Loja cheia % (faixa normal)</span>
        <input
          v-model="draft.storeFullPercent"
          type="number"
          min="0"
          max="1000"
          step="1"
          :disabled="!ctx.canEditCrmCommercialPolicy"
          @blur="saveRules"
        />
      </label>

      <label class="settings-field">
        <span>Faixa reduzida % (loja parcial)</span>
        <input
          v-model="draft.reducedRate"
          type="number"
          min="0"
          step="0.1"
          :disabled="!ctx.canEditCrmCommercialPolicy"
          @blur="saveRules"
        />
      </label>

      <label class="settings-field">
        <span>Meta própria p/ faixa reduzida %</span>
        <input
          v-model="draft.reducedRequiresOwnPercent"
          type="number"
          min="0"
          max="1000"
          step="1"
          :disabled="!ctx.canEditCrmCommercialPolicy"
          @blur="saveRules"
        />
      </label>

      <label class="settings-field">
        <span>Penalidade por métrica %</span>
        <input
          v-model="draft.qualityPenaltyPercent"
          type="number"
          min="0"
          step="0.1"
          :disabled="!ctx.canEditCrmCommercialPolicy"
          @blur="saveRules"
        />
      </label>
    </div>
  </article>
</template>

<style scoped>
.consultant-rules__hint {
  margin: 0 0 0.5rem;
  font-size: 0.78rem;
  color: var(--text-muted);
}

.consultant-rules__list {
  margin: 0 0 0.85rem;
  padding-left: 1.1rem;
  display: grid;
  gap: 0.25rem;
  font-size: 0.78rem;
  color: var(--text-muted);
}

.consultant-rules__list strong {
  color: rgb(var(--text) / 0.95);
}

.consultant-rules__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.65rem;
  align-items: end;
}
</style>
