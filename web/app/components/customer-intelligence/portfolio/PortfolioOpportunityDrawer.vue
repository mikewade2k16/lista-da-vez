<script setup lang="ts">
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import type {
  PortfolioOpportunitiesPage,
  PortfolioOpportunityView,
} from '~/domain/customer-intelligence/portfolio-types'

const props = defineProps<{
  open: boolean
  opportunity: PortfolioOpportunityView | null
  canManage: boolean
  busy: boolean
  reasons: PortfolioOpportunitiesPage['decisionReasons']
}>()
const emit = defineEmits<{
  'update:open': [value: boolean]
  decide: [
    input: {
      opportunity: PortfolioOpportunityView
      decision: 'approve' | 'reject'
      reasonCode: string
      reason: string
    },
  ]
}>()

const decision = ref<'approve' | 'reject'>('approve')
const reasonCode = ref('')
const reason = ref('')
const confirmed = ref(false)

watch(
  () => [props.open, props.opportunity?.id] as const,
  () => {
    decision.value = 'approve'
    reasonCode.value = ''
    reason.value = ''
    confirmed.value = false
  },
)

const reasonOptions = computed(() => props.reasons[decision.value])

function submit(): void {
  if (!props.opportunity || !props.canManage || !reasonCode.value || !confirmed.value) {
    return
  }
  emit('decide', {
    opportunity: props.opportunity,
    decision: decision.value,
    reasonCode: reasonCode.value,
    reason: reason.value,
  })
}
</script>

<template>
  <OmniEntityDrawer
    :model-value="open"
    :title="opportunity?.title || 'Oportunidade de portfolio'"
    :subtitle="opportunity ? `${opportunity.status} · ${opportunity.cohortClass}` : ''"
    @update:model-value="emit('update:open', $event)"
  >
    <template v-if="opportunity">
      <p>{{ opportunity.summary }}</p>
      <dl class="portfolio-detail">
        <div>
          <dt>Finalidade</dt>
          <dd>{{ opportunity.purposeKey }}</dd>
        </div>
        <div>
          <dt>Tamanho</dt>
          <dd>{{ opportunity.cohortSizeBucket || 'bucket protegido' }}</dd>
        </div>
        <div>
          <dt>Freshness</dt>
          <dd>{{ opportunity.freshnessStatus }}</dd>
        </div>
        <div>
          <dt>Policy</dt>
          <dd>{{ opportunity.policyVersionRef }}</dd>
        </div>
        <div>
          <dt>Prompt/modelo</dt>
          <dd>{{ opportunity.promptBindingRef || '—' }} / {{ opportunity.modelRef || '—' }}</dd>
        </div>
      </dl>
      <section>
        <h3>Clientes-alvo</h3>
        <article v-for="target in opportunity.targetClients" :key="target.clientAccountRef">
          <strong>{{ target.displayName }}</strong>
          <p>{{ target.rationale }}</p>
        </article>
      </section>
      <section>
        <h3>Protecoes aplicadas</h3>
        <ul>
          <li>Agregado: {{ opportunity.protection.aggregateOnly ? 'sim' : 'nao' }}</li>
          <li>
            Contributors suprimidos:
            {{ opportunity.protection.contributorsSuppressed ? 'sim' : 'nao' }}
          </li>
          <li>PII suprimida: {{ opportunity.protection.piiSuppressed ? 'sim' : 'nao' }}</li>
          <li v-for="code in opportunity.protection.reasonCodes" :key="code">{{ code }}</li>
        </ul>
      </section>
      <form
        v-if="canManage && opportunity.allowedActions.length"
        class="portfolio-decision"
        @submit.prevent="submit"
      >
        <AppSelectField
          v-model="decision"
          label="Decisao"
          :options="
            opportunity.allowedActions.map((action) => ({
              value: action,
              label: action === 'approve' ? 'Aprovar' : 'Rejeitar',
            }))
          "
        />
        <AppSelectField v-model="reasonCode" label="Motivo allowlisted" :options="reasonOptions" />
        <label>
          Observacao
          <input v-model="reason" type="text" maxlength="240" />
        </label>
        <label class="portfolio-decision__confirm">
          <input v-model="confirmed" type="checkbox" />
          Confirmo o escopo agregado, a finalidade e a policy exibidos.
        </label>
      </form>
      <p class="portfolio-warning">
        Esta tela nao revela contributors, nao abre perfis individuais e nao cria campanha.
      </p>
    </template>

    <template #footer>
      <button
        v-if="opportunity && canManage && opportunity.allowedActions.length"
        type="button"
        :disabled="busy || !confirmed || !reasonCode"
        @click="submit"
      >
        Registrar decisao
      </button>
    </template>
  </OmniEntityDrawer>
</template>

<style scoped>
.portfolio-detail,
.portfolio-decision {
  display: grid;
  gap: 0.65rem;
}

.portfolio-detail div {
  display: grid;
  grid-template-columns: minmax(8rem, 0.4fr) 1fr;
  gap: 0.75rem;
}

.portfolio-detail dt,
.portfolio-warning {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.portfolio-detail dd {
  margin: 0;
  overflow-wrap: anywhere;
}

.portfolio-decision label {
  display: grid;
  gap: 0.3rem;
}

.portfolio-decision__confirm {
  grid-template-columns: auto 1fr;
}
</style>
