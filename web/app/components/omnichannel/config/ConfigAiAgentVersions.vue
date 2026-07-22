<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import ConfigAiAgentAdvancedSettings from '~/components/omnichannel/config/ConfigAiAgentAdvancedSettings.vue'
import ConfigAiAgentModelSelect from '~/components/omnichannel/config/ConfigAiAgentModelSelect.vue'
import { AI_PROVIDER_LABEL, AI_PROVIDERS, type CalendarAiProvider } from '~/utils/calendar'
import type {
  OmniAgentVersion,
  OmniAgentVersionInput,
  OmniMediaConfig,
} from '~/domain/omnichannel/config-types'

const props = defineProps<{
  agentId: string
  versions: OmniAgentVersion[]
  activeVersionId: string | null
  disabled?: boolean
}>()
const emit = defineEmits<{
  save: [payload: OmniAgentVersionInput]
}>()

const form = reactive({
  provider: 'gemini' as CalendarAiProvider,
  model: '',
  temperature: 0.2,
  systemPrompt: '',
  debounceMs: 2500,
  maxContextMessages: 30,
  maxAiTurns: 6,
  minConfidence: 0.65,
  handoffOnError: true,
  handoffOnLimit: true,
  mediaConfig: {} as OmniMediaConfig,
})
const hydratedVersionId = ref('')

const providerOptions = AI_PROVIDERS.map((value) => ({
  value,
  label: AI_PROVIDER_LABEL[value],
}))
const activeVersion = computed(
  () => props.versions.find((version) => version.id === props.activeVersionId) || null,
)
const latestDraft = computed(() =>
  props.versions.reduce<OmniAgentVersion | null>(
    (latest, version) =>
      version.status === 'draft' && (!latest || version.version > latest.version)
        ? version
        : latest,
    null,
  ),
)
const latestVersion = computed(() =>
  props.versions.reduce<OmniAgentVersion | null>(
    (latest, version) => (!latest || version.version > latest.version ? version : latest),
    null,
  ),
)
const editingVersion = computed(
  () => latestDraft.value || activeVersion.value || latestVersion.value,
)
const versionContext = computed(() => {
  const editing = editingVersion.value
  if (!editing) return ''
  if (editing.id === props.activeVersionId) return `Configuração ativa v${editing.version}.`
  return `Configuração recuperada da v${editing.version}. Salve para aplicar no atendimento.`
})
const createBlockedReason = computed(() => {
  if (!form.model.trim()) return 'Selecione o modelo.'
  if (!form.systemPrompt.trim()) return 'Informe o prompt principal da IA.'
  return ''
})

function isActive(version: OmniAgentVersion): boolean {
  return props.activeVersionId === version.id
}

function promptFromLayers(layers: unknown): string {
  if (!layers || typeof layers !== 'object' || Array.isArray(layers)) return ''
  const identity = (layers as Record<string, unknown>).identity
  return typeof identity === 'string' ? identity : ''
}

function supportedProvider(value: string): CalendarAiProvider {
  return AI_PROVIDERS.find((provider) => provider === value) || 'gemini'
}

function hydrateFromCurrentVersion(): void {
  const base = editingVersion.value
  if (!base || hydratedVersionId.value === base.id) return
  hydratedVersionId.value = base.id
  form.provider = supportedProvider(base.provider)
  form.model = base.model
  form.temperature = base.temperature
  form.systemPrompt = promptFromLayers(base.layers)
  form.debounceMs = base.debounceMs
  form.maxContextMessages = base.maxContextMessages
  form.maxAiTurns = base.maxAiTurns
  form.minConfidence = base.minConfidence
  form.handoffOnError = base.handoffOnError
  form.handoffOnLimit = base.handoffOnLimit
  form.mediaConfig = base.mediaConfig || {}
}

watch([() => props.activeVersionId, () => props.versions], hydrateFromCurrentVersion, {
  immediate: true,
})

function submitConfiguration(): void {
  if (createBlockedReason.value) return
  emit('save', {
    provider: form.provider,
    model: form.model.trim(),
    temperature: Number(form.temperature) || 0,
    layers: { identity: form.systemPrompt.trim() },
    debounceMs: Number(form.debounceMs) || 2500,
    maxContextMessages: Number(form.maxContextMessages) || 30,
    maxAiTurns: Number(form.maxAiTurns) || 6,
    minConfidence: Number.isFinite(Number(form.minConfidence)) ? Number(form.minConfidence) : 0.65,
    handoffOnError: form.handoffOnError,
    handoffOnLimit: form.handoffOnLimit,
    workflowContractVersion: 'brain.v3',
    mediaConfig: form.mediaConfig,
  })
}
</script>

<template>
  <section class="calendar-config__section cfg-vers">
    <p v-if="versionContext" class="calendar-config__hint cfg-vers__context">
      {{ versionContext }}
    </p>

    <details class="calendar-config__collapse">
      <summary class="calendar-config__collapse-head">Prompt do sistema (a lei da IA)</summary>
      <div class="calendar-config__collapse-body">
        <label class="calendar-config__field">
          <textarea
            v-model="form.systemPrompt"
            class="calendar-config__input calendar-config__textarea calendar-config__textarea--tall"
            placeholder="Defina o comportamento, o tom, as regras de triagem e quando transferir para um atendente."
            :disabled="disabled"
          ></textarea>
          <span class="calendar-config__hint">
            O contexto do cliente e as regras de segurança são acrescentados pelo Go.
          </span>
        </label>
      </div>
    </details>

    <details class="calendar-config__collapse">
      <summary class="calendar-config__collapse-head">Provedor e modelo</summary>
      <div class="calendar-config__collapse-body">
        <div class="calendar-config__grid2">
          <label class="calendar-config__field">
            <span class="calendar-config__field-label">Provedor</span>
            <select v-model="form.provider" class="calendar-config__input" :disabled="disabled">
              <option v-for="option in providerOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
          </label>
          <ConfigAiAgentModelSelect
            v-model="form.model"
            :agent-id="agentId"
            :provider="form.provider"
            :disabled="disabled"
          />
          <label class="calendar-config__field">
            <span class="calendar-config__field-label">Temperatura (0 a 1)</span>
            <input
              v-model.number="form.temperature"
              class="calendar-config__input"
              type="number"
              min="0"
              max="1"
              step="0.1"
              :disabled="disabled"
            />
          </label>
        </div>
      </div>
    </details>

    <details class="calendar-config__collapse">
      <summary class="calendar-config__collapse-head">Escopo por cliente</summary>
      <div class="calendar-config__collapse-body">
        <slot name="client-scope"></slot>
      </div>
    </details>

    <details class="calendar-config__collapse">
      <summary class="calendar-config__collapse-head">Chaves de API</summary>
      <div class="calendar-config__collapse-body">
        <slot name="api-key"></slot>
      </div>
    </details>

    <ConfigAiAgentAdvancedSettings
      v-model:debounce-ms="form.debounceMs"
      v-model:max-context-messages="form.maxContextMessages"
      v-model:max-ai-turns="form.maxAiTurns"
      v-model:min-confidence="form.minConfidence"
      v-model:handoff-on-error="form.handoffOnError"
      v-model:handoff-on-limit="form.handoffOnLimit"
      :disabled="disabled"
    />

    <details class="calendar-config__collapse">
      <summary class="calendar-config__collapse-head">
        Versões e testes ({{ versions.length }})
      </summary>
      <div class="calendar-config__collapse-body cfg-vers__history">
        <p v-if="versions.length === 0" class="calendar-config__empty">
          Nenhuma versão criada ainda.
        </p>
        <ul v-else class="cfg-vers__list">
          <li v-for="version in versions" :key="version.id" class="cfg-vers__item">
            <div class="cfg-vers__meta">
              <strong>v{{ version.version }}</strong>
              <span>{{ version.provider }}/{{ version.model }}</span>
              <span v-if="isActive(version)" class="cfg-vers__tag">ativa</span>
            </div>
            <span v-if="!isActive(version)" class="calendar-config__hint">
              {{ version.status === 'draft' ? 'alteração pendente' : 'histórico' }}
            </span>
          </li>
        </ul>
        <slot name="simulator"></slot>
      </div>
    </details>

    <details class="calendar-config__collapse">
      <summary class="calendar-config__collapse-head">Identidade do agente</summary>
      <div class="calendar-config__collapse-body">
        <slot name="identity"></slot>
      </div>
    </details>

    <div class="calendar-config__section-actions cfg-vers__save">
      <span v-if="createBlockedReason" class="calendar-config__hint">
        {{ createBlockedReason }}
      </span>
      <AppPanelButton
        variant="primary"
        :disabled="disabled || !!createBlockedReason"
        @click="submitConfiguration"
      >
        Salvar configurações
      </AppPanelButton>
    </div>
  </section>
</template>

<style scoped>
.cfg-vers__history {
  display: grid;
  gap: 0.8rem;
}

.cfg-vers__context {
  margin: 0;
}

.cfg-vers__list {
  display: grid;
  gap: 0.4rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.cfg-vers__item,
.cfg-vers__meta,
.cfg-vers__save {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.cfg-vers__item {
  justify-content: space-between;
  padding: 0.5rem 0.6rem;
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.5);
}

.cfg-vers__meta {
  flex-wrap: wrap;
  color: rgb(var(--muted));
  font-size: 0.76rem;
}

.cfg-vers__meta strong {
  color: rgb(var(--text));
}

.cfg-vers__tag {
  padding: 0.1rem 0.4rem;
  border-radius: 999px;
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
  font-weight: 700;
}

.cfg-vers__save {
  justify-content: flex-end;
}

@media (max-width: 700px) {
  .cfg-vers__item,
  .cfg-vers__save {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
