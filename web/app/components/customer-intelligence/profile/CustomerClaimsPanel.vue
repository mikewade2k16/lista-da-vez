<script setup lang="ts">
import { computed, ref } from 'vue'
import CustomerIntelligenceStatus from '~/components/customer-intelligence/CustomerIntelligenceStatus.vue'
import AppSegmentedFilter from '~/components/ui/AppSegmentedFilter.vue'
import { useCandidateClaims } from '~/composables/customer-intelligence/useCandidateClaims'
import {
  CUSTOMER_CLAIM_STATUS_OPTIONS,
  type CustomerClaimReviewStatus,
  type CustomerClaimView,
  validCustomerClaimReasonCode,
} from '~/domain/customer-intelligence/claim-types'

const props = defineProps<{
  relationshipId: string
  canManage: boolean
}>()

const claims = useCandidateClaims(() => props.relationshipId)
const reasonCodes = ref<Record<string, string>>({})
const validationMessages = ref<Record<string, string>>({})
const canReview = computed(
  () => props.canManage && claims.access.canManageIntelligenceProfile.value,
)

function displayValue(value: unknown): string {
  if (typeof value === 'string') return value
  if (typeof value === 'number') return String(value)
  if (typeof value === 'boolean') return value ? 'Sim' : 'Nao'
  if (value === null || value === undefined) return 'Sem valor'
  try {
    const serialized = JSON.stringify(value, null, 2)
    if (!serialized) return 'Sem valor'
    return serialized.length > 2_000 ? `${serialized.slice(0, 2_000)}...` : serialized
  } catch {
    return 'Valor estruturado indisponivel para exibicao.'
  }
}

function dateTime(value: string): string {
  if (!value) return '-'
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString('pt-BR')
}

function statusLabel(value: CustomerClaimView['status']): string {
  if (value === 'accepted') return 'Aceita'
  if (value === 'rejected') return 'Rejeitada'
  return 'Candidata'
}

async function review(claim: CustomerClaimView, status: CustomerClaimReviewStatus): Promise<void> {
  const reasonCode = (reasonCodes.value[claim.id] ?? '').trim()
  if (!validCustomerClaimReasonCode(reasonCode)) {
    validationMessages.value[claim.id] =
      'Use um codigo iniciado por letra minuscula; depois use letras, numeros, ponto, hifen ou underline.'
    return
  }
  validationMessages.value[claim.id] = ''
  const reviewed = await claims.review(claim, status, reasonCode)
  if (reviewed) reasonCodes.value[claim.id] = ''
}
</script>

<template>
  <section class="claims-panel">
    <header>
      <div>
        <small>Curadoria humana com revisao otimista</small>
        <h2>Claims extraidas</h2>
        <p>
          Aceitar cura somente a claim. Nao cria, confirma ou sobrescreve um fact automaticamente.
        </p>
      </div>
      <button
        type="button"
        :disabled="claims.loading.value || !claims.access.canViewIntelligenceProfile.value"
        @click="claims.load"
      >
        Atualizar
      </button>
    </header>

    <CustomerIntelligenceStatus
      v-if="!claims.access.canViewIntelligenceProfile.value"
      title="Sem permissao para consultar claims"
      :error="{
        kind: claims.access.hasCustomerIntelligenceModule.value ? 'forbidden' : 'capability_off',
        message: '',
        reasonCode: claims.access.hasCustomerIntelligenceModule.value
          ? 'customer_intelligence_profile_view_required'
          : 'customer_intelligence_module_disabled',
        statusCode: claims.access.hasCustomerIntelligenceModule.value ? 403 : 0,
      }"
    />
    <template v-else>
      <AppSegmentedFilter
        :model-value="claims.activeStatus.value"
        :options="[...CUSTOMER_CLAIM_STATUS_OPTIONS]"
        aria-label="Status das claims"
        @update:model-value="claims.selectStatus"
      />

      <CustomerIntelligenceStatus
        v-if="claims.loading.value && !claims.items.value.length"
        title="Carregando claims"
        loading
      />
      <div
        v-else-if="claims.error.value && !claims.items.value.length"
        class="claims-panel__blocking-error"
      >
        <CustomerIntelligenceStatus title="Claims indisponiveis" :error="claims.error.value" />
        <button type="button" @click="claims.load">Tentar novamente</button>
      </div>
      <div v-else class="claims-panel__content">
        <div v-if="claims.error.value" class="claims-panel__inline-error" role="alert">
          <span>{{ claims.error.value.message }}</span>
          <button type="button" @click="claims.load">Recarregar</button>
        </div>
        <CustomerIntelligenceStatus
          v-if="!claims.items.value.length"
          :title="`Sem claims ${claims.activeStatus.value}`"
          empty
          empty-text="Nenhuma claim foi retornada para este relacionamento e status."
        />

        <article v-for="claim in claims.items.value" v-else :key="claim.id">
          <header>
            <div>
              <small>{{ claim.factKey }} · {{ claim.valueType }}</small>
              <h3>{{ statusLabel(claim.status) }}</h3>
            </div>
            <span class="claims-panel__status" :class="`is-${claim.status}`">
              {{ Math.round(claim.confidence * 100) }}% · rev {{ claim.revision }}
            </span>
          </header>

          <pre class="claims-panel__value">{{ displayValue(claim.value) }}</pre>

          <dl class="claims-panel__meta">
            <div>
              <dt>Verificacao</dt>
              <dd>{{ claim.verificationState || '-' }}</dd>
            </div>
            <div>
              <dt>Sensibilidade</dt>
              <dd>{{ claim.sensitivity || '-' }}</dd>
            </div>
            <div>
              <dt>Valida desde</dt>
              <dd>{{ dateTime(claim.validFrom) }}</dd>
            </div>
            <div>
              <dt>Valida ate</dt>
              <dd>{{ dateTime(claim.validUntil) }}</dd>
            </div>
          </dl>

          <details class="claims-panel__provenance">
            <summary>Origem tecnica e evidencias minimizadas</summary>
            <dl>
              <div>
                <dt>Extrator</dt>
                <dd>
                  {{ claim.extractorKey || '-' }}
                  <template v-if="claim.extractorVersion">/ {{ claim.extractorVersion }}</template>
                </dd>
              </div>
              <div>
                <dt>Metodo</dt>
                <dd>{{ claim.extractionMethod || '-' }}</dd>
              </div>
              <div>
                <dt>Runtime run</dt>
                <dd>{{ claim.runtimeRunId || '-' }}</dd>
              </div>
              <div>
                <dt>Prompt binding</dt>
                <dd>{{ claim.promptBindingId || '-' }}</dd>
              </div>
              <div>
                <dt>Outcome / ordinal</dt>
                <dd>
                  {{ claim.sourceOutcomeEventId || '-' }} /
                  {{ claim.sourceClaimOrdinal ?? '-' }}
                </dd>
              </div>
            </dl>
            <ul v-if="claim.evidence.length">
              <li v-for="reference in claim.evidence" :key="reference.observationId">
                <strong>{{ reference.sourceKey || 'Fonte protegida' }}</strong>
                <span>{{ reference.locator || 'Referencia sem locator' }}</span>
                <code>{{ reference.observationId }}</code>
              </li>
            </ul>
            <p v-else>Nenhuma referencia de evidencia autorizada foi retornada.</p>
          </details>

          <div v-if="claim.status !== 'candidate'" class="claims-panel__reviewed">
            <span>Revisada em {{ dateTime(claim.reviewedAt) }}</span>
            <code>{{ claim.reviewReasonCode || 'reason_code_indisponivel' }}</code>
          </div>

          <div v-else-if="canReview" class="claims-panel__actions">
            <label>
              <span>Reason code auditavel</span>
              <input
                v-model="reasonCodes[claim.id]"
                maxlength="160"
                autocomplete="off"
                placeholder="ex.: quality.confirmed_by_operator"
                :disabled="Boolean(claims.reviewingId.value)"
              />
            </label>
            <div>
              <button
                type="button"
                :disabled="
                  Boolean(claims.reviewingId.value) ||
                  !validCustomerClaimReasonCode(reasonCodes[claim.id] ?? '')
                "
                @click="review(claim, 'accepted')"
              >
                {{ claims.reviewingId.value === claim.id ? 'Revisando...' : 'Aceitar' }}
              </button>
              <button
                class="claims-panel__reject"
                type="button"
                :disabled="
                  Boolean(claims.reviewingId.value) ||
                  !validCustomerClaimReasonCode(reasonCodes[claim.id] ?? '')
                "
                @click="review(claim, 'rejected')"
              >
                Rejeitar
              </button>
            </div>
            <p v-if="validationMessages[claim.id]">
              {{ validationMessages[claim.id] }}
            </p>
            <small>
              O review envia a revisao {{ claim.revision }}. Se outra pessoa revisar antes, o
              servidor recusa o update e exige recarregar.
            </small>
          </div>
          <p v-else class="claims-panel__permission">
            Leitura permitida. Aceitar ou rejeitar exige
            <code>customer_intelligence.profile.manage</code>
            .
          </p>
        </article>
      </div>
    </template>

    <p class="claims-panel__notice">
      Invariante: claim aceita continua sendo uma claim curada. Promover seu valor para fact exige
      um fluxo autoritativo separado, que esta tela nao executa.
    </p>
  </section>
</template>

<style scoped>
.claims-panel,
.claims-panel__content,
.claims-panel article,
.claims-panel__actions,
.claims-panel__provenance {
  display: grid;
  gap: 0.8rem;
}

.claims-panel {
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 1rem;
}

.claims-panel > header,
.claims-panel article > header,
.claims-panel__inline-error,
.claims-panel__reviewed,
.claims-panel__actions > div {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.65rem;
  flex-wrap: wrap;
}

.claims-panel h2,
.claims-panel h3,
.claims-panel p,
.claims-panel dl {
  margin: 0;
}

.claims-panel header p,
.claims-panel header small,
.claims-panel__permission,
.claims-panel__notice {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.claims-panel button {
  min-height: 2.3rem;
  padding: 0 0.8rem;
  border: 0;
  border-radius: 999px;
  background: rgb(var(--primary));
  color: white;
  font-weight: 700;
}

.claims-panel button:disabled {
  opacity: 0.55;
}

.claims-panel article {
  padding: 0.85rem;
  border: 1px solid rgb(var(--border) / 0.7);
  border-radius: 0.8rem;
}

.claims-panel__status {
  padding: 0.3rem 0.55rem;
  border-radius: 999px;
  background: rgb(var(--warning) / 0.12);
  color: rgb(var(--warning));
  font-size: 0.7rem;
  font-weight: 700;
}

.claims-panel__status.is-accepted {
  background: rgb(var(--success) / 0.12);
  color: rgb(var(--success));
}

.claims-panel__status.is-rejected {
  background: rgb(var(--danger) / 0.12);
  color: rgb(var(--danger));
}

.claims-panel__value {
  max-height: 14rem;
  margin: 0;
  padding: 0.7rem;
  overflow: auto;
  border-radius: 0.7rem;
  background: rgb(var(--surface-2) / 0.65);
  color: inherit;
  font-family: inherit;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.claims-panel__meta,
.claims-panel__provenance dl {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.55rem;
}

.claims-panel__meta div,
.claims-panel__provenance dl div,
.claims-panel__actions label {
  display: grid;
  gap: 0.2rem;
}

.claims-panel dt,
.claims-panel__actions label > span {
  color: rgb(var(--muted));
  font-size: 0.68rem;
  font-weight: 700;
  text-transform: uppercase;
}

.claims-panel dd {
  margin: 0;
  overflow-wrap: anywhere;
  font-size: 0.76rem;
}

.claims-panel__provenance {
  padding: 0.7rem;
  border: 1px solid rgb(var(--border) / 0.65);
  border-radius: 0.7rem;
}

.claims-panel__provenance summary {
  cursor: pointer;
  font-weight: 700;
}

.claims-panel__provenance ul {
  display: grid;
  gap: 0.45rem;
  padding: 0;
  list-style: none;
}

.claims-panel__provenance li {
  display: grid;
  gap: 0.2rem;
  padding-left: 0.6rem;
  border-left: 3px solid rgb(var(--primary) / 0.3);
}

.claims-panel__provenance span,
.claims-panel__provenance code,
.claims-panel__reviewed {
  color: rgb(var(--muted));
  font-size: 0.7rem;
  overflow-wrap: anywhere;
}

.claims-panel__actions input {
  min-height: 2.35rem;
  padding: 0 0.65rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.65rem;
  background: rgb(var(--surface));
  color: inherit;
}

.claims-panel__actions .claims-panel__reject {
  background: rgb(var(--danger) / 0.12);
  color: rgb(var(--danger));
}

.claims-panel__actions p,
.claims-panel__actions small {
  margin: 0;
  color: rgb(var(--danger));
  font-size: 0.7rem;
}

.claims-panel__actions small {
  color: rgb(var(--muted));
}

.claims-panel__blocking-error {
  display: grid;
  gap: 0.65rem;
}

.claims-panel__blocking-error > button {
  justify-self: center;
}

.claims-panel__inline-error,
.claims-panel__notice {
  padding: 0.7rem;
  border: 1px solid rgb(var(--warning) / 0.35);
  border-radius: 0.7rem;
}

@media (max-width: 760px) {
  .claims-panel__meta,
  .claims-panel__provenance dl {
    grid-template-columns: 1fr;
  }
}
</style>
