<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import ConfigAiCredentials from '~/components/omnichannel/config/ConfigAiCredentials.vue'
import ConfigAiRoleModelSelect from '~/components/omnichannel/config/ConfigAiRoleModelSelect.vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import { fetchAICredentials } from '~/domain/omnichannel/config-api'
import type { OmniAICredential } from '~/domain/omnichannel/config-types'
import type { AttendanceAnalysisConfig } from '~/domain/operation/attendance-transcriptions'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

const props = defineProps<{
  open: boolean
  config: AttendanceAnalysisConfig | null
  loading: boolean
  saving: boolean
  errorMessage: string
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  reload: []
  save: [config: AttendanceAnalysisConfig]
}>()

type DrawerMode = 'side' | 'center' | 'fullscreen'
type ConfigTab = 'credentials' | 'transcription' | 'summary'

const auth = useAuthStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)
const mode = ref<DrawerMode>('side')
const activeTab = ref<ConfigTab>('credentials')
const credentials = ref<OmniAICredential[]>([])
const credentialsLoading = ref(false)
const credentialsError = ref('')
const draft = ref<AttendanceAnalysisConfig | null>(null)

const tabs: Array<{ key: ConfigTab; label: string; icon: string }> = [
  { key: 'credentials', label: 'Chaves de IA', icon: 'i-lucide-key-round' },
  { key: 'transcription', label: 'Transcrição', icon: 'i-lucide-audio-lines' },
  { key: 'summary', label: 'Resumo e prompt', icon: 'i-lucide-sparkles' },
]

const summaryCredentials = computed(() =>
  credentials.value
    .filter((credential) => credential.provider === 'gemini' || credential.provider === 'openai')
    .sort((left, right) => {
      if (left.provider === right.provider) return left.name.localeCompare(right.name, 'pt-BR')
      return left.provider === 'openai' ? -1 : 1
    }),
)
const selectedCredential = computed(() =>
  summaryCredentials.value.find((credential) => credential.id === draft.value?.credentialId),
)
const canSave = computed(
  () =>
    Boolean(draft.value) &&
    (!draft.value?.enabled || Boolean(draft.value?.credentialId && selectedCredential.value)) &&
    !props.saving,
)

function cloneConfig(value: AttendanceAnalysisConfig | null): AttendanceAnalysisConfig | null {
  return value ? JSON.parse(JSON.stringify(value)) : null
}

async function loadCredentials(): Promise<void> {
  credentialsLoading.value = true
  credentialsError.value = ''
  try {
    credentials.value = await fetchAICredentials(api)
    selectPrincipalCredential()
  } catch (cause) {
    credentialsError.value = getApiErrorMessage(
      cause,
      'Não foi possível carregar as chaves globais da conta.',
    )
  } finally {
    credentialsLoading.value = false
  }
}

function selectCredential(event: Event): void {
  if (!draft.value) return
  const credentialId = (event.target as HTMLSelectElement).value
  const credential = summaryCredentials.value.find((item) => item.id === credentialId)
  draft.value.credentialId = credentialId
  if (!credential || credential.provider === 'glm') return
  const providerChanged = draft.value.provider !== credential.provider
  draft.value.provider = credential.provider
  if (providerChanged) {
    draft.value.model = credential.provider === 'openai' ? 'gpt-4.1-mini' : 'gemini-2.5-flash'
  }
}

function selectPrincipalCredential(): void {
  if (!draft.value || draft.value.credentialId) return
  const credential =
    summaryCredentials.value.find((item) => item.provider === 'openai') ||
    summaryCredentials.value[0]
  if (!credential || credential.provider === 'glm') return
  draft.value.credentialId = credential.id
  draft.value.provider = credential.provider
  draft.value.model = credential.provider === 'openai' ? 'gpt-4.1-mini' : 'gemini-2.5-flash'
}

async function onCredentialsChanged(): Promise<void> {
  await loadCredentials()
}

function submit(): void {
  if (!draft.value || !canSave.value) return
  emit('save', cloneConfig(draft.value) as AttendanceAnalysisConfig)
}

watch(
  () => props.open,
  (open) => {
    if (!open) return
    activeTab.value = 'credentials'
    draft.value = cloneConfig(props.config)
    void loadCredentials()
    if (!props.config && !props.loading) emit('reload')
  },
  { immediate: true },
)

watch(
  () => props.config,
  (value) => {
    if (props.open) draft.value = cloneConfig(value)
  },
)
</script>

<template>
  <OmniEntityDrawer
    v-model:mode="mode"
    :model-value="open"
    title="Configurar transcrições"
    subtitle="Whisper local, chaves globais da conta e regras soberanas para resumir cada atendimento."
    @update:model-value="emit('update:open', $event)"
  >
    <div class="calendar-config-drawer transcription-config-drawer">
      <nav class="calendar-config__tabs" aria-label="Seções da configuração">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          class="calendar-config__tab"
          :class="{ 'is-active': activeTab === tab.key }"
          @click="activeTab = tab.key"
        >
          <UIcon :name="tab.icon" aria-hidden="true" />
          <span>{{ tab.label }}</span>
        </button>
      </nav>

      <div class="calendar-config-drawer__panel">
        <div v-show="activeTab === 'credentials'">
          <ConfigAiCredentials :disabled="false" @changed="onCredentialsChanged" />
        </div>

        <div v-show="activeTab === 'transcription'" class="transcription-config__content">
          <details class="settings-collapse" open>
            <summary class="settings-collapse__summary">
              <div class="settings-collapse__title-wrap">
                <strong class="settings-collapse__title">Whisper local</strong>
                <span class="settings-collapse__text">
                  Modelo e idioma usados para transformar o áudio em texto.
                </span>
              </div>
              <span class="settings-collapse__meta">Local</span>
              <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
                expand_more
              </span>
            </summary>
            <div v-if="draft" class="settings-collapse__body transcription-config__grid">
              <label class="calendar-config__field">
                <span class="calendar-config__field-label">Modelo do Whisper</span>
                <input
                  v-model="draft.transcriptionModel"
                  class="calendar-config__input"
                  type="text"
                />
              </label>
              <label class="calendar-config__field">
                <span class="calendar-config__field-label">Idioma</span>
                <input
                  v-model="draft.transcriptionLanguage"
                  class="calendar-config__input"
                  type="text"
                  maxlength="12"
                />
              </label>
            </div>
          </details>
        </div>

        <div v-show="activeTab === 'summary'" class="transcription-config__content">
          <p v-if="credentialsError" class="calendar-config__warn">{{ credentialsError }}</p>
          <details class="settings-collapse" open>
            <summary class="settings-collapse__summary">
              <div class="settings-collapse__title-wrap">
                <strong class="settings-collapse__title">Resumo do atendimento</strong>
                <span class="settings-collapse__text">
                  A chave é global da conta; o prompt define a interpretação e o relatório.
                </span>
              </div>
              <span class="settings-collapse__meta">
                {{ draft?.enabled ? (config?.credentialId ? 'Ativo' : 'Não salvo') : 'Inativo' }}
              </span>
              <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
                expand_more
              </span>
            </summary>
            <div v-if="draft" class="settings-collapse__body transcription-config__form">
              <label class="transcription-config__toggle">
                <input v-model="draft.enabled" type="checkbox" />
                <span>Gerar resumo automaticamente depois do Whisper</span>
              </label>

              <label class="calendar-config__field">
                <span class="calendar-config__field-label">Chave global da conta</span>
                <select
                  class="calendar-config__input"
                  :value="draft.credentialId"
                  :disabled="credentialsLoading"
                  @change="selectCredential"
                >
                  <option value="">
                    {{ credentialsLoading ? 'Carregando chaves…' : 'Selecione uma chave' }}
                  </option>
                  <option
                    v-for="credential in summaryCredentials"
                    :key="credential.id"
                    :value="credential.id"
                  >
                    {{ credential.name }} · {{ credential.provider
                    }}{{ credential.provider === 'openai' ? ' · principal' : '' }} · final
                    {{ credential.last4 }}
                  </option>
                </select>
                <span v-if="summaryCredentials.length === 0" class="calendar-config__hint">
                  Crie uma credencial OpenAI ou Gemini na aba Chaves de IA.
                </span>
              </label>

              <ConfigAiRoleModelSelect
                v-model="draft.model"
                :credential-id="draft.credentialId"
                capability="response"
                :disabled="!draft.enabled"
              />

              <label class="calendar-config__field">
                <span class="calendar-config__field-label">Temperatura</span>
                <input
                  v-model.number="draft.temperature"
                  class="calendar-config__input"
                  type="number"
                  min="0"
                  max="1"
                  step="0.1"
                />
              </label>

              <label class="calendar-config__field transcription-config__prompt">
                <span class="calendar-config__field-label">Prompt soberano</span>
                <textarea
                  v-model="draft.systemPrompt"
                  class="calendar-config__input transcription-config__textarea"
                  rows="12"
                ></textarea>
                <span class="calendar-config__hint">
                  Este prompt é a regra principal para corrigir contexto, resumir e gerar
                  relatórios.
                </span>
              </label>
            </div>
          </details>
        </div>

        <p v-if="loading" class="transcription-config__status">Carregando configuração…</p>
        <p v-if="errorMessage" class="calendar-config__warn">{{ errorMessage }}</p>
      </div>
    </div>

    <template #footer>
      <footer v-if="activeTab !== 'credentials'" class="transcription-config__footer">
        <span>As alterações só entram em vigor depois de salvar.</span>
        <div>
          <AppPanelButton variant="primary" :disabled="!canSave" @click="submit">
            {{ saving ? 'Salvando…' : 'Salvar configurações' }}
          </AppPanelButton>
        </div>
      </footer>
    </template>
  </OmniEntityDrawer>
</template>

<style scoped>
.transcription-config-drawer,
.transcription-config__content {
  min-height: 100%;
}

.transcription-config__content,
.transcription-config__form {
  display: grid;
  gap: 0.85rem;
}

.transcription-config__grid {
  grid-template-columns: minmax(0, 2fr) minmax(8rem, 1fr);
}

.transcription-config__toggle {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  color: var(--text-main);
  font-size: 0.82rem;
  font-weight: 700;
}

.transcription-config__prompt {
  grid-column: 1 / -1;
}

.transcription-config__textarea {
  min-height: 15rem;
  resize: vertical;
}

.transcription-config__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  width: 100%;
}

.transcription-config__footer span {
  color: var(--text-muted);
  font-size: 0.76rem;
}

.transcription-config__status {
  color: var(--text-muted);
  font-size: 0.8rem;
}

@media (max-width: 720px) {
  .transcription-config__grid {
    grid-template-columns: 1fr;
  }
}
</style>
