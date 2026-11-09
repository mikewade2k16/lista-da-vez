<script setup lang="ts">
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useAccountingObligations } from '../../composables/accounting/useAccountingObligations'
const props = defineProps<{ issuerId: string }>()
const obligations = useAccountingObligations(() => props.issuerId)
const { state } = obligations
const labels: Record<string, string> = {
  'mei.das': 'MEI · DAS',
  'mei.dasn_simei': 'MEI · declaração anual',
  'company.tax_review': 'Empresa não MEI · revisão de impostos',
  'company.declaration_review': 'Empresa não MEI · revisão de declarações',
}
const states: Record<string, string> = {
  pending_data: 'Dados ou regra pendentes',
  calculated: 'Calculada',
  reviewed: 'Revisada',
  payment_reported: 'Pagamento informado, sem conciliação bancária',
}
const pendingLabels: Record<string, string> = {
  rule_unavailable: 'Definir e revisar a regra aplicável',
  rule_outside_validity: 'Revisar regra para esta competência',
  regime_not_confirmed: 'Confirmar enquadramento do emitente',
  rule_context_calendar_unqualified: 'Qualificar regra, fontes e calendário',
  due_date_unverified: 'Verificar vencimento em fonte aplicável',
  base_missing: 'Informar e revisar a base de cálculo',
  missing_rule: 'Implementar e qualificar o cálculo',
  explicit_revision_required: 'Recalcular após a revisão solicitada',
}
</script>
<template>
  <details v-if="obligations.allowed()" class="accounting-obligations">
    <summary>Obrigações e pendências</summary>
    <p>
      Estrutura em preparação. Regras fiscais ainda não qualificadas: sem cálculo real, guia ou
      pagamento automático.
    </p>
    <AppPanelButton :disabled="state.busy" @click="obligations.load()">
      Consultar obrigações
    </AppPanelButton>
    <form
      v-if="obligations.allowed(true)"
      class="accounting-obligations__form"
      @submit.prevent="obligations.create()"
    >
      <label>
        Frente a organizar (não confirma enquadramento)
        <select v-model="state.code" class="omni-input" :disabled="state.busy" required>
          <option disabled value="">Escolha explicitamente</option>
          <option v-for="(label, code) in labels" :key="code" :value="code">{{ label }}</option>
        </select>
      </label>
      <label>
        Competência (AAAA-MM ou AAAA)
        <input
          v-model="state.period"
          class="omni-input"
          placeholder="Informe o período"
          :disabled="state.busy"
          required
        />
      </label>
      <AppPanelButton type="submit" :disabled="state.busy">Registrar pendência</AppPanelButton>
    </form>
    <p v-if="state.error" role="alert">{{ state.error }}</p>
    <p v-if="state.busy" role="status">Consultando o servidor...</p>
    <article v-for="item in state.items" :key="item.id">
      <strong>{{ labels[item.code] || item.code }} · {{ item.period }}</strong>
      <p>{{ states[item.state] || item.state }} · versão {{ item.version }}</p>
      <p>
        Valor: {{ item.amount === null ? 'Não apurado' : item.amount }} · Vencimento:
        {{ item.dueDate || 'Não verificado' }}
      </p>
      <p v-if="item.pending.length">
        Pendências:
        {{
          item.pending
            .map((code) => pendingLabels[code] || 'Revisar os dados aplicáveis')
            .join('; ')
        }}
      </p>
      <details>
        <summary>Memória de cálculo para revisão</summary>
        <pre>{{ JSON.stringify(item.memory, null, 2) }}</pre>
      </details>
    </article>
    <AppPanelButton v-if="state.more" :disabled="state.busy" @click="obligations.load(true)">
      Carregar mais
    </AppPanelButton>
  </details>
</template>
<style scoped>
.accounting-obligations__form {
  display: grid;
  gap: 0.5rem;
  margin-block: 0.75rem;
}
.accounting-obligations__form label {
  display: grid;
  gap: 0.25rem;
}
.accounting-obligations pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.accounting-obligations article {
  border-top: 1px solid rgb(var(--border));
  padding-block: 0.5rem;
}
</style>
