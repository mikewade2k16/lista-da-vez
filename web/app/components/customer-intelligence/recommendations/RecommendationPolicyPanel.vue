<script setup lang="ts">
import CustomerIntelligenceStatus from '~/components/customer-intelligence/CustomerIntelligenceStatus.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import { useRecommendationPolicies } from '~/composables/customer-intelligence/useRecommendationPolicies'
import type {
  RecommendationPolicyField,
  RecommendationPolicyValue,
} from '~/domain/customer-intelligence/recommendation-policy-types'

const policies = useRecommendationPolicies()

function normalizeFieldValue(
  field: RecommendationPolicyField,
  raw: string | boolean,
): RecommendationPolicyValue {
  if (field.type === 'number') return Number(raw)
  if (field.type === 'boolean') return raw === true || raw === 'true'
  return String(raw)
}
</script>

<template>
  <section class="policy-panel">
    <header>
      <div>
        <h2>Policies de recomendacao</h2>
        <p>
          Limites estruturados vencem prompts. Tenant, consentimento, piso de coorte e
          anti-reidentificacao nao sao customizaveis.
        </p>
      </div>
      <AppSelectField
        v-if="policies.policies.value.length"
        :model-value="policies.selectedPolicyKey.value"
        :options="
          policies.policies.value.map((policy) => ({
            value: policy.definition.policyKey,
            label: policy.definition.name,
          }))
        "
        label="Policy"
        @update:model-value="policies.selectPolicy($event)"
      />
    </header>

    <CustomerIntelligenceStatus v-if="policies.loading.value" title="Carregando policies" loading />
    <CustomerIntelligenceStatus
      v-else-if="policies.error.value"
      title="Policies indisponiveis"
      :error="policies.error.value"
    />
    <CustomerIntelligenceStatus
      v-else-if="!policies.selected.value"
      title="Sem policies registradas"
      empty
      empty-text="O backend ainda nao publicou definitions allowlisted."
    />

    <template v-else>
      <div class="policy-panel__heading">
        <div>
          <h3>{{ policies.selected.value.definition.name }}</h3>
          <p>{{ policies.selected.value.definition.description }}</p>
        </div>
        <span>
          efetiva v{{ policies.selected.value.effective?.versionNumber ?? '—' }} ·
          {{ policies.selected.value.binding?.scopeLabel || 'sem binding' }}
        </span>
      </div>

      <section class="policy-invariants">
        <h3>Invariantes da plataforma</h3>
        <article
          v-for="invariant in policies.selected.value.definition.invariants"
          :key="invariant.key"
        >
          <strong>{{ invariant.label }}</strong>
          <p>{{ invariant.description }}</p>
        </article>
      </section>

      <div class="policy-fields">
        <template v-for="field in policies.selected.value.definition.fields" :key="field.key">
          <AppToggleSwitch
            v-if="field.type === 'boolean'"
            :model-value="Boolean(policies.editorValues.value[field.key])"
            :label="field.label"
            :disabled="
              !policies.selected.value.draft ||
              !policies.selected.value.canEdit ||
              field.immutableFloor
            "
            @update:model-value="policies.setValue(field.key, normalizeFieldValue(field, $event))"
          />
          <AppSelectField
            v-else-if="field.type === 'select'"
            :model-value="String(policies.editorValues.value[field.key] ?? '')"
            :options="field.options ?? []"
            :label="field.label"
            :disabled="
              !policies.selected.value.draft ||
              !policies.selected.value.canEdit ||
              field.immutableFloor
            "
            @update:model-value="policies.setValue(field.key, normalizeFieldValue(field, $event))"
          />
          <label v-else>
            <span>{{ field.label }}</span>
            <input
              :type="field.type === 'number' ? 'number' : 'text'"
              :value="String(policies.editorValues.value[field.key] ?? '')"
              :min="field.min"
              :max="field.max"
              :disabled="
                !policies.selected.value.draft ||
                !policies.selected.value.canEdit ||
                field.immutableFloor
              "
              @input="
                policies.setValue(
                  field.key,
                  normalizeFieldValue(field, ($event.target as HTMLInputElement).value),
                )
              "
            />
            <small>{{ field.description }}</small>
          </label>
        </template>
      </div>

      <footer>
        <span v-if="policies.dirty.value">Alteracoes locais nao salvas</span>
        <button
          v-if="!policies.selected.value.draft"
          type="button"
          :disabled="!policies.access.canManagePortfolio.value || policies.saving.value"
          @click="policies.ensureDraft"
        >
          Criar draft
        </button>
        <button
          v-else
          type="button"
          :disabled="!policies.dirty.value || policies.saving.value"
          @click="policies.save"
        >
          Salvar
        </button>
        <button
          type="button"
          :disabled="
            policies.dirty.value || policies.saving.value || !policies.selected.value.draft
          "
          @click="policies.action('validate')"
        >
          Validar
        </button>
        <button
          type="button"
          :disabled="
            policies.dirty.value ||
            policies.saving.value ||
            !policies.selected.value.canPublish ||
            policies.selected.value.draft?.validationStatus !== 'valid'
          "
          @click="policies.action('publish')"
        >
          Publicar
        </button>
        <button
          type="button"
          :disabled="policies.saving.value || !policies.selected.value.canRollback"
          @click="policies.rollback"
        >
          Rollback
        </button>
      </footer>
    </template>
  </section>
</template>

<style scoped>
.policy-panel {
  display: grid;
  gap: 1rem;
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 1rem;
}

.policy-panel > header,
.policy-panel__heading,
.policy-panel footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.policy-panel h2,
.policy-panel h3,
.policy-panel p {
  margin: 0;
}

.policy-panel p,
.policy-panel__heading span,
.policy-panel footer span,
.policy-fields small {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.policy-invariants,
.policy-fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.7rem;
}

.policy-invariants article {
  padding: 0.7rem;
  border: 1px solid rgb(var(--warning) / 0.25);
  border-radius: 0.7rem;
  background: rgb(var(--warning) / 0.05);
}

.policy-fields label {
  display: grid;
  gap: 0.3rem;
}

.policy-fields input {
  min-height: 2.35rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.6rem;
  background: rgb(var(--surface));
  color: inherit;
}

@media (max-width: 800px) {
  .policy-invariants,
  .policy-fields {
    grid-template-columns: 1fr;
  }
}
</style>
