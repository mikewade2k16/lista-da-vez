<script setup lang="ts">
import { ref } from 'vue'
import type {
  IntelligenceAgent,
  IntelligenceAgentPatchInput,
  IntelligenceAgentVersion,
  IntelligenceAgentVersionWriteInput,
  IntelligenceCredential,
  IntelligenceModel,
} from '~/domain/customer-intelligence/agent-admin-types'
import IntelligenceAgentCard from './IntelligenceAgentCard.vue'

const props = defineProps<{
  agents: IntelligenceAgent[]
  models: IntelligenceModel[]
  credentials: IntelligenceCredential[]
  sessionVersions: Record<string, IntelligenceAgentVersion>
  busyKey: string
  addAgent: (slug: string, name: string) => Promise<boolean>
  saveAgent: (agentId: string, input: IntelligenceAgentPatchInput) => Promise<boolean>
  addAgentVersion: (
    agentId: string,
    input: IntelligenceAgentVersionWriteInput,
  ) => Promise<IntelligenceAgentVersion | null>
  publishAgentVersion: (agentId: string, versionId: string) => Promise<boolean>
}>()

const slug = ref('')
const name = ref('')
const validationError = ref('')
const SAFE_SLUG = /^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$/

async function createAgent(): Promise<void> {
  const normalizedSlug = slug.value.trim().toLowerCase()
  const normalizedName = name.value.trim()
  if (!SAFE_SLUG.test(normalizedSlug)) {
    validationError.value =
      'Use slug em minusculas, iniciado por letra, com numeros, ponto, hifen ou underline.'
    return
  }
  if (!normalizedName || normalizedName.length > 200) {
    validationError.value = 'Informe um nome com ate 200 caracteres.'
    return
  }
  validationError.value = ''
  const saved = await props.addAgent(normalizedSlug, normalizedName)
  if (!saved) return
  slug.value = ''
  name.value = ''
}
</script>

<template>
  <div class="agents-manager">
    <form class="agent-create" @submit.prevent="createAgent">
      <header>
        <div>
          <strong>Novo agente</strong>
          <span>O agente nasce desativado e precisa de uma versao publicada.</span>
        </div>
        <button type="submit" :disabled="Boolean(busyKey)">
          {{ busyKey === 'agent:new' ? 'Criando...' : 'Criar agente' }}
        </button>
      </header>
      <div class="agent-create__grid">
        <label>
          <span>Slug tecnico</span>
          <input
            v-model="slug"
            autocomplete="off"
            placeholder="ex.: resumo_cliente"
            :disabled="Boolean(busyKey)"
          />
        </label>
        <label>
          <span>Nome</span>
          <input
            v-model="name"
            maxlength="200"
            placeholder="Ex.: Resumo do cliente"
            :disabled="Boolean(busyKey)"
          />
        </label>
      </div>
      <p v-if="validationError">{{ validationError }}</p>
    </form>

    <CustomerIntelligenceStatus
      v-if="agents.length === 0"
      title="Nenhum agente neste cliente"
      empty
      empty-text="Crie o primeiro agente para preparar um draft de configuracao."
    />
    <div v-else class="agent-list">
      <IntelligenceAgentCard
        v-for="agent in agents"
        :key="agent.id"
        :agent="agent"
        :models="models"
        :credentials="credentials"
        :session-version="sessionVersions[agent.id] ?? null"
        :busy-key="busyKey"
        :save-agent="saveAgent"
        :add-agent-version="addAgentVersion"
        :publish-agent-version="publishAgentVersion"
      />
    </div>
  </div>
</template>

<style scoped>
.agents-manager,
.agent-create,
.agent-list {
  display: grid;
  gap: 0.8rem;
}

.agent-create {
  padding: 0.85rem;
  border: 1px solid rgb(var(--border) / 0.75);
  border-radius: 0.8rem;
}

.agent-create > header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.7rem;
  flex-wrap: wrap;
}

.agent-create header div,
.agent-create label {
  display: grid;
  gap: 0.3rem;
}

.agent-create__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem;
}

.agent-create label > span,
.agent-create header span {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.agent-create input {
  min-height: 2.5rem;
  padding: 0 0.7rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.7rem;
  background: rgb(var(--surface));
  color: inherit;
}

.agent-create button {
  min-height: 2.35rem;
  padding: 0 0.85rem;
  border: 0;
  border-radius: 999px;
  background: rgb(var(--primary));
  color: white;
  font-weight: 700;
}

.agent-create button:disabled {
  opacity: 0.55;
}

.agent-create p {
  margin: 0;
  color: rgb(var(--danger));
  font-size: 0.74rem;
}

@media (max-width: 760px) {
  .agent-create__grid {
    grid-template-columns: 1fr;
  }
}
</style>
