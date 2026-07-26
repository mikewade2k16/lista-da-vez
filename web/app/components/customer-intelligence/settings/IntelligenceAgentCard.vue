<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import type {
  IntelligenceAgent,
  IntelligenceAgentPatchInput,
  IntelligenceAgentVersion,
  IntelligenceAgentVersionWriteInput,
  IntelligenceCredential,
  IntelligenceModel,
} from '~/domain/customer-intelligence/agent-admin-types'

const props = defineProps<{
  agent: IntelligenceAgent
  models: IntelligenceModel[]
  credentials: IntelligenceCredential[]
  sessionVersion: IntelligenceAgentVersion | null
  busyKey: string
  saveAgent: (agentId: string, input: IntelligenceAgentPatchInput) => Promise<boolean>
  addAgentVersion: (
    agentId: string,
    input: IntelligenceAgentVersionWriteInput,
  ) => Promise<IntelligenceAgentVersion | null>
  publishAgentVersion: (agentId: string, versionId: string) => Promise<boolean>
}>()

const name = ref('')
const enabled = ref(false)
const modelId = ref('')
const credentialId = ref('')
const temperature = ref(0.2)
const maxOutputTokens = ref(2_000)
const timeoutMs = ref(30_000)
const promptOverride = ref('')
const agentError = ref('')
const versionError = ref('')

const enabledModels = computed(() => props.models.filter((item) => item.status === 'enabled'))
const modelOptions = computed(() =>
  enabledModels.value.map((item) => ({
    value: item.id,
    label: `${item.provider} / ${item.model}`,
  })),
)
const selectedModel = computed(
  () => enabledModels.value.find((item) => item.id === modelId.value) ?? null,
)
const credentialOptions = computed(() => {
  const provider = selectedModel.value?.provider
  const items = provider
    ? props.credentials.filter((item) => item.provider === provider && item.secret.set)
    : []
  return [
    { value: '', label: 'Sem credencial vinculada' },
    ...items.map((item) => ({
      value: item.id,
      label: `${item.label} (•••• ${item.secret.last4})`,
    })),
  ]
})
const agentChanged = computed(
  () =>
    name.value.trim() !== props.agent.name || enabled.value !== (props.agent.status === 'enabled'),
)

watch(
  () => [props.agent.id, props.agent.revision] as const,
  () => {
    name.value = props.agent.name
    enabled.value = props.agent.status === 'enabled'
    agentError.value = ''
  },
  { immediate: true },
)

watch(
  enabledModels,
  (items) => {
    if (!items.some((item) => item.id === modelId.value)) {
      modelId.value = items[0]?.id ?? ''
      credentialId.value = ''
    }
  },
  { immediate: true },
)

function chooseModel(value: string): void {
  modelId.value = enabledModels.value.some((item) => item.id === value) ? value : ''
  const provider = selectedModel.value?.provider
  if (
    credentialId.value &&
    !props.credentials.some(
      (item) => item.id === credentialId.value && item.provider === provider && item.secret.set,
    )
  ) {
    credentialId.value = ''
  }
}

function chooseCredential(value: string): void {
  credentialId.value = credentialOptions.value.some((item) => item.value === value) ? value : ''
}

async function submitAgent(): Promise<void> {
  const normalizedName = name.value.trim()
  if (!normalizedName || normalizedName.length > 200) {
    agentError.value = 'Informe um nome com ate 200 caracteres.'
    return
  }
  if (enabled.value && !props.agent.activeVersionId) {
    agentError.value = 'Publique uma versao antes de ativar o agente.'
    return
  }
  agentError.value = ''
  await props.saveAgent(props.agent.id, {
    name: normalizedName,
    enabled: enabled.value,
    expectedRevision: props.agent.revision,
  })
}

function validateVersion(): string {
  const model = selectedModel.value
  if (!model) return 'Selecione um modelo ativo.'
  if (
    credentialId.value &&
    !props.credentials.some(
      (item) =>
        item.id === credentialId.value && item.provider === model.provider && item.secret.set,
    )
  ) {
    return 'A credencial precisa estar ativa e pertencer ao mesmo provider do modelo.'
  }
  if (!Number.isFinite(temperature.value) || temperature.value < 0 || temperature.value > 2) {
    return 'Temperatura deve ficar entre 0 e 2.'
  }
  if (
    !Number.isInteger(maxOutputTokens.value) ||
    maxOutputTokens.value < 16 ||
    maxOutputTokens.value > 100_000
  ) {
    return 'Tokens de saida devem ficar entre 16 e 100.000.'
  }
  if (!Number.isInteger(timeoutMs.value) || timeoutMs.value < 1_000 || timeoutMs.value > 300_000) {
    return 'Timeout deve ficar entre 1.000 e 300.000 ms.'
  }
  if (promptOverride.value.length > 200_000) {
    return 'O prompt override aceita no maximo 200.000 caracteres.'
  }
  return ''
}

async function createDraft(): Promise<void> {
  versionError.value = validateVersion()
  if (versionError.value) return
  await props.addAgentVersion(props.agent.id, {
    modelId: modelId.value,
    credentialId: credentialId.value,
    temperature: temperature.value,
    maxOutputTokens: maxOutputTokens.value,
    timeoutMs: timeoutMs.value,
    promptOverride: promptOverride.value,
    config: {},
  })
}

async function publishDraft(): Promise<void> {
  if (!props.sessionVersion || props.sessionVersion.status === 'published') return
  await props.publishAgentVersion(props.agent.id, props.sessionVersion.id)
}
</script>

<template>
  <article class="agent-card">
    <header>
      <div>
        <small>{{ agent.purpose }}</small>
        <strong>{{ agent.name }}</strong>
        <span>
          Revisao {{ agent.revision }} ·
          {{ agent.activeVersionId ? `ativa ${agent.activeVersionId}` : 'sem versao ativa' }}
        </span>
      </div>
      <span class="agent-card__status" :class="{ 'is-enabled': agent.status === 'enabled' }">
        {{ agent.status === 'enabled' ? 'Ativo' : 'Desativado' }}
      </span>
    </header>

    <form class="agent-card__identity" @submit.prevent="submitAgent">
      <label>
        <span>Nome</span>
        <input v-model="name" maxlength="200" :disabled="Boolean(busyKey)" />
      </label>
      <AppToggleSwitch
        v-model="enabled"
        label="Agente ativo"
        :disabled="Boolean(busyKey) || (!agent.activeVersionId && agent.status !== 'enabled')"
        compact
      />
      <button type="submit" :disabled="Boolean(busyKey) || !agentChanged">
        {{ busyKey === `agent:${agent.id}` ? 'Salvando...' : 'Salvar agente' }}
      </button>
      <p v-if="agentError">{{ agentError }}</p>
    </form>

    <details class="agent-card__draft">
      <summary>
        <span>
          <strong>Editor do proximo draft</strong>
          <small>Modelo, credencial, limites e prompt override</small>
        </span>
        <span>{{ sessionVersion ? `v${sessionVersion.version}` : 'Sem draft nesta sessao' }}</span>
      </summary>

      <form @submit.prevent="createDraft">
        <div class="agent-card__grid">
          <AppSelectField
            :model-value="modelId"
            label="Modelo ativo"
            :options="modelOptions"
            :disabled="Boolean(busyKey)"
            empty-label="Nenhum modelo ativo."
            @update:model-value="chooseModel"
          />
          <AppSelectField
            :model-value="credentialId"
            label="Credencial"
            :options="credentialOptions"
            :disabled="Boolean(busyKey) || !modelId"
            @update:model-value="chooseCredential"
          />
          <label>
            <span>Temperatura</span>
            <input
              v-model.number="temperature"
              type="number"
              min="0"
              max="2"
              step="0.1"
              :disabled="Boolean(busyKey)"
            />
          </label>
          <label>
            <span>Max output tokens</span>
            <input
              v-model.number="maxOutputTokens"
              type="number"
              min="16"
              max="100000"
              step="1"
              :disabled="Boolean(busyKey)"
            />
          </label>
          <label>
            <span>Timeout (ms)</span>
            <input
              v-model.number="timeoutMs"
              type="number"
              min="1000"
              max="300000"
              step="1000"
              :disabled="Boolean(busyKey)"
            />
          </label>
        </div>
        <label>
          <span>Prompt override opcional</span>
          <textarea
            v-model="promptOverride"
            rows="6"
            maxlength="200000"
            placeholder="Deixe vazio para usar o comportamento publicado do processo."
            :disabled="Boolean(busyKey)"
          ></textarea>
        </label>
        <p v-if="versionError" class="agent-card__error">{{ versionError }}</p>
        <button type="submit" :disabled="Boolean(busyKey) || enabledModels.length === 0">
          {{ busyKey === `version:${agent.id}` ? 'Criando...' : 'Criar draft imutavel' }}
        </button>
      </form>

      <section v-if="sessionVersion" class="agent-card__session-version">
        <div>
          <strong>Draft v{{ sessionVersion.version }}</strong>
          <span>Status: {{ sessionVersion.status }} · ID {{ sessionVersion.id }}</span>
        </div>
        <button
          type="button"
          :disabled="Boolean(busyKey) || sessionVersion.status === 'published'"
          @click="publishDraft"
        >
          {{
            sessionVersion.status === 'published'
              ? 'Publicado'
              : busyKey === `publish:${sessionVersion.id}`
                ? 'Publicando...'
                : 'Publicar draft'
          }}
        </button>
      </section>

      <p class="agent-card__contract-gap">
        O backend nao possui GET nem PATCH de versoes. Este formulario edita apenas o draft local
        antes do POST; depois de criado ele pode ser publicado nesta sessao, mas nao e reidratado
        nem alterado pelo painel.
      </p>
    </details>
  </article>
</template>

<style scoped>
.agent-card,
.agent-card__identity,
.agent-card__draft form,
.agent-card__session-version {
  display: grid;
  gap: 0.75rem;
}

.agent-card {
  padding: 0.85rem;
  border: 1px solid rgb(var(--border) / 0.75);
  border-radius: 0.8rem;
}

.agent-card > header,
.agent-card__session-version,
.agent-card__draft > summary {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.7rem;
  flex-wrap: wrap;
}

.agent-card header div,
.agent-card__draft summary span,
.agent-card__session-version div,
.agent-card label {
  display: grid;
  gap: 0.3rem;
}

.agent-card small,
.agent-card header span,
.agent-card label > span,
.agent-card__contract-gap,
.agent-card__session-version span {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.agent-card__status {
  padding: 0.3rem 0.55rem;
  border-radius: 999px;
  background: rgb(var(--border) / 0.25);
}

.agent-card__status.is-enabled {
  background: rgb(var(--success) / 0.12);
  color: rgb(var(--success));
}

.agent-card__identity {
  grid-template-columns: minmax(12rem, 1fr) auto auto;
  align-items: end;
}

.agent-card input,
.agent-card textarea {
  min-height: 2.5rem;
  padding: 0.55rem 0.7rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.7rem;
  background: rgb(var(--surface));
  color: inherit;
}

.agent-card textarea {
  resize: vertical;
}

.agent-card button {
  min-height: 2.35rem;
  padding: 0 0.85rem;
  border: 0;
  border-radius: 999px;
  background: rgb(var(--primary));
  color: white;
  font-weight: 700;
}

.agent-card button:disabled {
  opacity: 0.55;
}

.agent-card__identity p,
.agent-card__error {
  grid-column: 1 / -1;
  margin: 0;
  color: rgb(var(--danger));
  font-size: 0.74rem;
}

.agent-card__draft {
  border-top: 1px solid rgb(var(--border) / 0.7);
}

.agent-card__draft > summary {
  padding: 0.75rem 0;
  cursor: pointer;
}

.agent-card__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem;
}

.agent-card__draft form > button {
  justify-self: end;
}

.agent-card__session-version {
  align-items: center;
  padding: 0.7rem;
  border-radius: 0.7rem;
  background: rgb(var(--primary) / 0.07);
}

.agent-card__contract-gap {
  margin: 0;
  padding: 0.7rem;
  border: 1px solid rgb(var(--warning) / 0.3);
  border-radius: 0.7rem;
}

@media (max-width: 760px) {
  .agent-card__identity,
  .agent-card__grid {
    grid-template-columns: 1fr;
  }
}
</style>
