<script setup lang="ts">
import CustomerIntelligenceAgentAdmin from '~/components/customer-intelligence/settings/CustomerIntelligenceAgentAdmin.vue'
import CustomerDataControlPlane from '~/components/customer-intelligence/settings/CustomerDataControlPlane.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useCustomerControlPlane } from '~/composables/customer-intelligence/useCustomerControlPlane'
import {
  CUSTOMER_INTELLIGENCE_INVARIANTS,
  INTELLIGENCE_CAPABILITY_DEFINITIONS,
} from '~/domain/customer-intelligence/control-plane-types'

const control = useCustomerControlPlane()
</script>

<template>
  <div class="control-plane">
    <section class="control-invariants">
      <header>
        <h2>Guardrails invariantes</h2>
        <span>Somente leitura - plataforma</span>
      </header>
      <ul>
        <li v-for="invariant in CUSTOMER_INTELLIGENCE_INVARIANTS" :key="invariant">
          {{ invariant }}
        </li>
      </ul>
    </section>

    <CustomerDataControlPlane :control="control" />

    <section v-if="control.access.hasCustomerIntelligenceModule.value" class="control-section">
      <header>
        <div>
          <small>Customer Intelligence - API real</small>
          <h2>Ativacao por capability</h2>
          <p>
            Modos efetivos por cliente. Um modo ativo nao substitui modulo, permissao, finalidade,
            fonte, prompt publicado ou binding.
          </p>
        </div>
        <span class="control-badge">GET/PUT /capabilities/:key</span>
      </header>

      <CustomerIntelligenceStatus
        v-if="control.loading.value"
        title="Carregando capabilities"
        loading
      />
      <CustomerIntelligenceStatus
        v-else-if="control.error.value"
        title="Capabilities indisponiveis"
        :error="control.error.value"
        @retry="control.loadIntelligence"
      />
      <div v-else class="control-capabilities">
        <article v-for="definition in INTELLIGENCE_CAPABILITY_DEFINITIONS" :key="definition.key">
          <div>
            <small>{{ definition.key }}</small>
            <h3>{{ definition.label }}</h3>
            <p>{{ definition.description }}</p>
          </div>
          <AppSelectField
            :model-value="
              control.draftModes.value[definition.key] ??
              control.capability(definition.key)?.mode ??
              'off'
            "
            :options="definition.modes.map((mode) => ({ value: mode, label: mode }))"
            label="Modo"
            :disabled="!control.access.canManageIntelligenceProfile.value"
            @update:model-value="control.setDraftMode(definition.key, $event)"
          />
          <label
            v-if="
              definition.key === 'customer_intelligence.runtime' &&
              control.draftModes.value[definition.key] === 'canary'
            "
            class="control-canary"
          >
            <span>Coorte canary (%)</span>
            <input
              :value="control.canaryAllocationPercent.value"
              type="number"
              min="1"
              max="100"
              :disabled="!control.access.canManageIntelligenceProfile.value"
              @input="
                control.setCanaryAllocationPercent(
                  Number(($event.target as HTMLInputElement).value),
                )
              "
            />
            <small>
              Bucket HMAC deterministico por cliente e relacionamento. Fora da coorte, a execucao
              fica shadow e sem efeito.
            </small>
          </label>
          <button
            type="button"
            :disabled="
              !control.access.canManageIntelligenceProfile.value ||
              control.savingKey.value === definition.key ||
              !control.intelligenceCapabilityDirty(definition.key)
            "
            @click="control.save(definition.key)"
          >
            {{ control.savingKey.value === definition.key ? 'Salvando...' : 'Salvar' }}
          </button>
        </article>
      </div>
      <p class="control-gap">
        O runtime publica somente a configuracao tipada da coorte canary. Nao existe editor JSON
        livre; os demais guardrails continuam definidos pelo servidor.
      </p>
    </section>

    <CustomerIntelligenceAgentAdmin v-if="control.access.hasCustomerIntelligenceModule.value" />

    <section v-if="control.access.hasCustomerIntelligenceModule.value" class="control-section">
      <header>
        <div>
          <small>Comportamento customizavel</small>
          <h2>Fontes, prompts e runtime</h2>
          <p>
            Cada processo tem prompt proprio; sources e policies estruturadas limitam o que o texto
            pode fazer.
          </p>
        </div>
      </header>
      <div class="control-links">
        <NuxtLink v-if="control.access.canViewSources.value" to="/inteligencia-clientes/fontes">
          <strong>Fontes</strong>
          <span>Ativar, configurar e sincronizar conectores registrados.</span>
        </NuxtLink>
        <NuxtLink v-if="control.access.canViewPrompts.value" to="/inteligencia-clientes/prompts">
          <strong>Prompt Studio</strong>
          <span>13 processos especificos, versoes, validacao e testes.</span>
        </NuxtLink>
        <NuxtLink v-if="control.access.canViewRuns.value" to="/inteligencia-clientes/atendimentos">
          <strong>Runs</strong>
          <span>Verificar status, custo, latencia e reason codes do runtime.</span>
        </NuxtLink>
      </div>
    </section>
  </div>
</template>

<style scoped>
.control-plane,
.control-section,
.control-invariants {
  display: grid;
  gap: 1rem;
}

.control-section,
.control-invariants {
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 1rem;
}

.control-invariants {
  border-color: rgb(var(--warning) / 0.35);
  background: rgb(var(--warning) / 0.05);
}

.control-section > header,
.control-invariants header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.control-section h2,
.control-section h3,
.control-section p,
.control-invariants h2 {
  margin: 0;
}

.control-section p,
.control-section small,
.control-invariants,
.control-gap {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.control-badge {
  padding: 0.3rem 0.55rem;
  border-radius: 999px;
  background: rgb(var(--success) / 0.12);
  color: rgb(var(--success));
  font-size: 0.68rem;
  font-weight: 700;
}

.control-capabilities,
.control-links {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

.control-capabilities article,
.control-links a {
  display: grid;
  align-content: start;
  gap: 0.65rem;
  padding: 0.8rem;
  border: 1px solid rgb(var(--border) / 0.7);
  border-radius: 0.75rem;
}

.control-canary {
  display: grid;
  gap: 0.35rem;
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.control-canary input {
  min-height: 2.4rem;
  padding: 0 0.65rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.7rem;
  background: rgb(var(--surface-2));
  color: rgb(var(--text));
}

.control-links span {
  color: rgb(var(--muted));
  font-size: 0.7rem;
}

.control-links a {
  color: inherit;
  text-decoration: none;
}

@media (max-width: 760px) {
  .control-capabilities,
  .control-links {
    grid-template-columns: 1fr;
  }
}
</style>
