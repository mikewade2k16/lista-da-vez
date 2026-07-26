<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import CustomerIntelligenceStatus from '~/components/customer-intelligence/CustomerIntelligenceStatus.vue'
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import type { CustomerApiErrorState } from '~/domain/customer-intelligence/api-error'
import type { IntelligenceObservationView } from '~/domain/customer-intelligence/audit-types'
import {
  isObservationFieldProtected,
  safeObservationFieldDisplay,
  safeObservationProvenance,
} from '~/domain/customer-intelligence/observation-presentation'

const props = defineProps<{
  open: boolean
  loading: boolean
  revealing: boolean
  error: CustomerApiErrorState | null
  observation: IntelligenceObservationView | null
}>()
const emit = defineEmits<{
  'update:open': [value: boolean]
  reveal: [reasonCode: string]
}>()

const REVEAL_REASONS = [
  { value: 'customer_support_investigation', label: 'Investigacao de atendimento' },
  { value: 'data_quality_verification', label: 'Verificacao de qualidade dos dados' },
  { value: 'privacy_request', label: 'Solicitacao do titular' },
  { value: 'compliance_audit', label: 'Auditoria de conformidade' },
  { value: 'manual_profile_review', label: 'Revisao manual do perfil' },
] as const

const reasonCode = ref('')
const canReveal = computed(() => {
  const observation = props.observation
  if (!observation || observation.revealed) return false
  const sensitivity = String(observation.sensitivity || '')
    .trim()
    .toLowerCase()
  return (
    ['personal', 'sensitive', 'restricted'].includes(sensitivity) &&
    observation.snapshotFields.some((field) => isObservationFieldProtected(observation, field))
  )
})

watch([() => props.open, () => props.observation?.id], () => {
  reasonCode.value = ''
})
</script>

<template>
  <OmniEntityDrawer
    :model-value="open"
    title="Observacao minimizada"
    :subtitle="observation?.sourceKey || 'Carregando origem autorizada'"
    @update:model-value="emit('update:open', $event)"
  >
    <CustomerIntelligenceStatus
      v-if="loading && !observation"
      title="Carregando observacao"
      loading
    />
    <CustomerIntelligenceStatus
      v-else-if="error && !observation"
      title="Observacao indisponivel"
      :error="error"
    />
    <template v-else-if="observation">
      <dl class="observation-meta">
        <div>
          <dt>Sensibilidade</dt>
          <dd>{{ observation.sensitivity }}</dd>
        </div>
        <div>
          <dt>Finalidade</dt>
          <dd>{{ observation.purposeKey }}</dd>
        </div>
        <div>
          <dt>Retencao</dt>
          <dd>{{ observation.retentionState }}</dd>
        </div>
        <div>
          <dt>Observada em</dt>
          <dd>{{ new Date(observation.observedAt).toLocaleString('pt-BR') }}</dd>
        </div>
        <div>
          <dt>Proveniencia</dt>
          <dd>{{ safeObservationProvenance(observation) }}</dd>
        </div>
      </dl>
      <section class="observation-fields">
        <h3>Snapshot permitido</h3>
        <div v-for="field in observation.snapshotFields" :key="field.label">
          <span>{{ field.label }}</span>
          <strong>{{ safeObservationFieldDisplay(observation, field) }}</strong>
          <small v-if="isObservationFieldProtected(observation, field)">oculto</small>
          <small v-else-if="observation.revealed">revelado e auditado</small>
        </div>
      </section>
      <CustomerIntelligenceStatus v-if="error" title="Falha ao revelar observacao" :error="error" />
      <form
        v-if="canReveal"
        class="observation-reveal"
        @submit.prevent="emit('reveal', reasonCode)"
      >
        <label>
          Motivo obrigatorio para revelar
          <select v-model="reasonCode" :disabled="revealing" required>
            <option value="" disabled>Selecione um motivo</option>
            <option v-for="reason in REVEAL_REASONS" :key="reason.value" :value="reason.value">
              {{ reason.label }}
            </option>
          </select>
        </label>
        <button type="submit" :disabled="revealing || !reasonCode">
          {{ revealing ? 'Registrando acesso...' : 'Revelar dados permitidos' }}
        </button>
      </form>
      <p class="observation-notice">
        {{
          observation.revealed
            ? 'Os campos allowlisted foram revelados somente nesta gaveta. O ator, motivo, origem, sensibilidade, finalidade e quantidade de campos foram registrados na auditoria; valores nao foram gravados no log.'
            : 'Valores pessoais, sensiveis e restritos ficam mascarados por padrao. Usuarios com permissao de auditoria podem fazer uma revelacao explicita e rastreavel; payloads nao allowlisted e segredos nunca sao expostos.'
        }}
      </p>
    </template>
  </OmniEntityDrawer>
</template>

<style scoped>
.observation-meta,
.observation-fields,
.observation-reveal {
  display: grid;
  gap: 0.65rem;
}

.observation-meta div,
.observation-fields div {
  display: grid;
  grid-template-columns: minmax(8rem, 0.45fr) 1fr auto;
  gap: 0.65rem;
}

.observation-meta dt,
.observation-fields span,
.observation-fields small,
.observation-notice {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.observation-meta dd {
  margin: 0;
  overflow-wrap: anywhere;
}

.observation-reveal {
  margin-top: 1rem;
  padding: 0.85rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.75rem;
}

.observation-reveal label {
  display: grid;
  gap: 0.35rem;
  color: rgb(var(--muted));
  font-size: 0.78rem;
}

.observation-reveal select,
.observation-reveal button {
  min-height: 2.5rem;
}
</style>
