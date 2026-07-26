<script setup lang="ts">
import { ref, watch } from 'vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import {
  INTELLIGENCE_AI_PROVIDER_OPTIONS,
  type IntelligenceAIProvider,
  type IntelligenceModel,
  type IntelligenceModelStatus,
  type IntelligenceModelWriteInput,
} from '~/domain/customer-intelligence/agent-admin-types'

interface ModelDraft {
  baseUrl: string
  status: IntelligenceModelStatus
}

const props = defineProps<{
  models: IntelligenceModel[]
  busyKey: string
  saveModel: (input: IntelligenceModelWriteInput) => Promise<boolean>
}>()

const drafts = ref<Record<string, ModelDraft>>({})
const provider = ref<IntelligenceAIProvider>('openai')
const model = ref('')
const baseUrl = ref('')
const status = ref<IntelligenceModelStatus>('enabled')
const validationError = ref('')

watch(
  () => props.models,
  (items) => {
    drafts.value = Object.fromEntries(
      items.map((item) => [
        item.id,
        { baseUrl: item.baseUrl, status: item.status } satisfies ModelDraft,
      ]),
    )
  },
  { immediate: true },
)

function setProvider(value: string): void {
  if (value === 'openai' || value === 'gemini' || value === 'glm') provider.value = value
}

function setStatus(value: string): void {
  if (value === 'enabled' || value === 'disabled') status.value = value
}

function setDraftStatus(id: string, value: string): void {
  if ((value !== 'enabled' && value !== 'disabled') || !drafts.value[id]) return
  drafts.value[id].status = value
}

async function createModel(): Promise<void> {
  const normalizedModel = model.value.trim()
  if (!normalizedModel || normalizedModel.length > 200) {
    validationError.value = 'Informe o identificador do modelo com ate 200 caracteres.'
    return
  }
  validationError.value = ''
  const saved = await props.saveModel({
    provider: provider.value,
    model: normalizedModel,
    baseUrl: baseUrl.value.trim(),
    status: status.value,
    config: {},
    revision: 0,
  })
  if (!saved) return
  model.value = ''
  baseUrl.value = ''
  status.value = 'enabled'
}

async function saveExisting(item: IntelligenceModel): Promise<void> {
  const draft = drafts.value[item.id]
  if (!draft) return
  validationError.value = ''
  await props.saveModel({
    id: item.id,
    provider: item.provider,
    model: item.model,
    baseUrl: draft.baseUrl.trim(),
    status: draft.status,
    // A API nao publica schema editavel; preserve o objeto autoritativo.
    config: item.config,
    revision: item.revision,
  })
}
</script>

<template>
  <div class="runtime-manager">
    <form class="runtime-form" @submit.prevent="createModel">
      <header>
        <div>
          <strong>Novo modelo</strong>
          <span>O backend resolve a URL padrao quando o campo fica vazio.</span>
        </div>
        <button type="submit" :disabled="Boolean(busyKey)">
          {{ busyKey === 'model:new' ? 'Salvando...' : 'Adicionar modelo' }}
        </button>
      </header>
      <div class="runtime-form__grid">
        <AppSelectField
          :model-value="provider"
          label="Provider"
          :options="[...INTELLIGENCE_AI_PROVIDER_OPTIONS]"
          :disabled="Boolean(busyKey)"
          @update:model-value="setProvider"
        />
        <label>
          <span>Modelo</span>
          <input
            v-model="model"
            maxlength="200"
            placeholder="Ex.: gpt-5-mini"
            :disabled="Boolean(busyKey)"
          />
        </label>
        <label>
          <span>Base URL opcional</span>
          <input
            v-model="baseUrl"
            type="url"
            placeholder="Padrao seguro do provider"
            :disabled="Boolean(busyKey)"
          />
        </label>
        <AppSelectField
          :model-value="status"
          label="Status"
          :options="[
            { value: 'enabled', label: 'Ativo' },
            { value: 'disabled', label: 'Desativado' },
          ]"
          :disabled="Boolean(busyKey)"
          @update:model-value="setStatus"
        />
      </div>
      <p v-if="validationError" class="runtime-form__error">{{ validationError }}</p>
    </form>

    <CustomerIntelligenceStatus
      v-if="models.length === 0"
      title="Nenhum modelo configurado"
      empty
      empty-text="Cadastre um modelo allowlisted para preparar as versoes dos agentes."
    />
    <div v-else class="runtime-list">
      <article v-for="item in models" :key="item.id">
        <header>
          <div>
            <strong>{{ item.provider }} / {{ item.model }}</strong>
            <span>Revisao {{ item.revision }}</span>
          </div>
          <code>{{ item.id }}</code>
        </header>
        <div v-if="drafts[item.id]" class="runtime-form__grid">
          <label>
            <span>Base URL</span>
            <input v-model="drafts[item.id].baseUrl" :disabled="Boolean(busyKey)" />
          </label>
          <AppSelectField
            :model-value="drafts[item.id].status"
            label="Status"
            :options="[
              { value: 'enabled', label: 'Ativo' },
              { value: 'disabled', label: 'Desativado' },
            ]"
            :disabled="Boolean(busyKey)"
            @update:model-value="setDraftStatus(item.id, $event)"
          />
        </div>
        <button type="button" :disabled="Boolean(busyKey)" @click="saveExisting(item)">
          {{ busyKey === `model:${item.id}` ? 'Salvando...' : 'Salvar modelo' }}
        </button>
      </article>
    </div>

    <p class="runtime-manager__notice">
      A resposta atual nao identifica se a linha e global ou da conta. Ao salvar uma linha global, o
      backend cria a sobreposicao da conta; capabilities opacas sao preservadas e nao ganham editor
      JSON.
    </p>
  </div>
</template>

<style scoped>
.runtime-manager,
.runtime-form,
.runtime-list,
.runtime-list article {
  display: grid;
  gap: 0.8rem;
}

.runtime-form,
.runtime-list article {
  padding: 0.85rem;
  border: 1px solid rgb(var(--border) / 0.75);
  border-radius: 0.8rem;
}

.runtime-form > header,
.runtime-list article > header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.7rem;
  flex-wrap: wrap;
}

.runtime-form header div,
.runtime-list header div,
.runtime-form label {
  display: grid;
  gap: 0.3rem;
}

.runtime-form__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem;
}

.runtime-form label > span,
.runtime-form header span,
.runtime-list header span,
.runtime-manager__notice {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.runtime-form input {
  min-height: 2.5rem;
  padding: 0 0.7rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.7rem;
  background: rgb(var(--surface));
  color: inherit;
}

.runtime-form button,
.runtime-list article > button {
  justify-self: end;
  min-height: 2.35rem;
  padding: 0 0.85rem;
  border: 0;
  border-radius: 999px;
  background: rgb(var(--primary));
  color: white;
  font-weight: 700;
}

.runtime-form button:disabled,
.runtime-list button:disabled {
  opacity: 0.55;
}

.runtime-form__error {
  margin: 0;
  color: rgb(var(--danger));
  font-size: 0.74rem;
}

.runtime-list code {
  max-width: 16rem;
  overflow: hidden;
  color: rgb(var(--muted));
  font-size: 0.65rem;
  text-overflow: ellipsis;
}

.runtime-manager__notice {
  margin: 0;
  padding: 0.7rem;
  border: 1px solid rgb(var(--warning) / 0.3);
  border-radius: 0.7rem;
}

@media (max-width: 760px) {
  .runtime-form__grid {
    grid-template-columns: 1fr;
  }
}
</style>
