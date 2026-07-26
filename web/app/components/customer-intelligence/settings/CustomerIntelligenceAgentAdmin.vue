<script setup lang="ts">
import { computed } from 'vue'
import { useCustomerIntelligenceAgentAdmin } from '~/composables/customer-intelligence/useCustomerIntelligenceAgentAdmin'
import IntelligenceAgentsManager from './IntelligenceAgentsManager.vue'
import IntelligenceCredentialsManager from './IntelligenceCredentialsManager.vue'
import IntelligenceModelsManager from './IntelligenceModelsManager.vue'

const admin = useCustomerIntelligenceAgentAdmin()
const hasData = computed(
  () =>
    admin.models.value.length > 0 ||
    admin.credentials.value.length > 0 ||
    admin.agents.value.length > 0,
)
</script>

<template>
  <section class="agent-admin">
    <header>
      <div>
        <small>Runtime de IA - API real</small>
        <h2>Modelos, credenciais e agentes</h2>
        <p>
          Configure os recursos tecnicos da inteligencia por conta e os agentes do cliente
          selecionado. As secoes iniciam fechadas para evitar alteracoes acidentais.
        </p>
      </div>
      <span>customer_intelligence.agents.manage</span>
    </header>

    <CustomerIntelligenceStatus
      v-if="!admin.access.canManageAgents.value"
      title="Sem permissao para administrar agentes"
      empty
      empty-text="Solicite a permissao customer_intelligence.agents.manage. Nenhum recurso e consultado sem ela."
    />
    <CustomerIntelligenceStatus
      v-else-if="!admin.access.clientScopeReady.value"
      title="Selecione um cliente"
      empty
      empty-text="O escopo explicito e obrigatorio antes de carregar agentes e configuracoes do runtime."
    />
    <CustomerIntelligenceStatus
      v-else-if="admin.loading.value && !hasData"
      title="Carregando runtime de IA"
      loading
    />
    <div v-else-if="admin.error.value && !hasData" class="agent-admin__blocking-error">
      <CustomerIntelligenceStatus title="Runtime de IA indisponivel" :error="admin.error.value" />
      <button type="button" @click="admin.load">Tentar novamente</button>
    </div>
    <div v-else class="agent-admin__content">
      <div v-if="admin.error.value" class="agent-admin__inline-error" role="alert">
        <span>{{ admin.error.value.message }}</span>
        <button type="button" @click="admin.load">Recarregar dados</button>
      </div>

      <p class="agent-admin__contract-gap">
        Lacuna registrada: o backend ainda nao expoe GET/PATCH de
        <code>agent versions</code>
        . O painel permite editar o formulario antes da criacao e publicar o draft retornado nesta
        sessao; drafts antigos ficam bloqueados, sem endpoints inventados.
      </p>

      <details class="agent-admin__panel">
        <summary>
          <span>
            <strong>Modelos</strong>
            <small>Provider, identificador, endpoint e ativacao</small>
          </span>
          <b>{{ admin.models.value.length }}</b>
        </summary>
        <IntelligenceModelsManager
          :models="admin.models.value"
          :busy-key="admin.busyKey.value"
          :save-model="admin.saveModel"
        />
      </details>

      <details class="agent-admin__panel">
        <summary>
          <span>
            <strong>Credenciais write-only</strong>
            <small>Criacao, rotacao e revogacao sem reidratacao</small>
          </span>
          <b>{{ admin.credentials.value.length }}</b>
        </summary>
        <IntelligenceCredentialsManager
          :credentials="admin.credentials.value"
          :busy-key="admin.busyKey.value"
          :save-credential="admin.saveCredential"
          :revoke-credential="admin.revokeCredential"
        />
      </details>

      <details class="agent-admin__panel">
        <summary>
          <span>
            <strong>Agentes do cliente</strong>
            <small>Identidade, estado, novo draft e publicacao</small>
          </span>
          <b>{{ admin.agents.value.length }}</b>
        </summary>
        <IntelligenceAgentsManager
          :agents="admin.agents.value"
          :models="admin.models.value"
          :credentials="admin.credentials.value"
          :session-versions="admin.sessionVersions.value"
          :busy-key="admin.busyKey.value"
          :add-agent="admin.addAgent"
          :save-agent="admin.saveAgent"
          :add-agent-version="admin.addAgentVersion"
          :publish-agent-version="admin.publishAgentVersion"
        />
      </details>
    </div>
  </section>
</template>

<style scoped>
.agent-admin,
.agent-admin__content,
.agent-admin__panel {
  display: grid;
  gap: 0.85rem;
}

.agent-admin {
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 1rem;
}

.agent-admin > header,
.agent-admin__panel > summary,
.agent-admin__inline-error {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.agent-admin h2,
.agent-admin p {
  margin: 0;
}

.agent-admin header p,
.agent-admin header small,
.agent-admin__panel small {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.agent-admin > header > span {
  padding: 0.3rem 0.55rem;
  border-radius: 999px;
  background: rgb(var(--success) / 0.12);
  color: rgb(var(--success));
  font-size: 0.68rem;
  font-weight: 700;
}

.agent-admin__blocking-error {
  display: grid;
  gap: 0.65rem;
}

.agent-admin__blocking-error > button,
.agent-admin__inline-error button {
  justify-self: center;
  min-height: 2.25rem;
  padding: 0 0.8rem;
  border: 0;
  border-radius: 999px;
  background: rgb(var(--primary));
  color: white;
  font-weight: 700;
}

.agent-admin__inline-error,
.agent-admin__contract-gap {
  padding: 0.7rem;
  border: 1px solid rgb(var(--warning) / 0.35);
  border-radius: 0.75rem;
  color: rgb(var(--muted));
  font-size: 0.73rem;
}

.agent-admin__panel {
  padding: 0 0.85rem 0.85rem;
  border: 1px solid rgb(var(--border) / 0.75);
  border-radius: 0.8rem;
}

.agent-admin__panel:not([open]) {
  padding-bottom: 0;
}

.agent-admin__panel > summary {
  align-items: center;
  padding: 0.85rem 0;
  cursor: pointer;
}

.agent-admin__panel summary span {
  display: grid;
  gap: 0.25rem;
}

.agent-admin__panel summary b {
  min-width: 1.7rem;
  padding: 0.2rem 0.4rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
  text-align: center;
}
</style>
