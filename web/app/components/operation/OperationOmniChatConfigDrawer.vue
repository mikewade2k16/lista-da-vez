<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import ConfigAiCredentials from '~/components/omnichannel/config/ConfigAiCredentials.vue'
import ConfigAiRoleModelSelect from '~/components/omnichannel/config/ConfigAiRoleModelSelect.vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import {
  HISTORY_WINDOW_MAX,
  HISTORY_WINDOW_MIN,
  PERSONA_MAX_LENGTH,
  useOmniChatPersona,
} from '~/composables/useOmniChatPersona'
import { fetchAICredentials } from '~/domain/omnichannel/config-api'
import type { OmniAICredential } from '~/domain/omnichannel/config-types'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

const props = defineProps<{ open: boolean; accountId?: string }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()

type DrawerMode = 'side' | 'center' | 'fullscreen'

const auth = useAuthStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)
const config = useOmniChatPersona(() => props.accountId || '')
const mode = ref<DrawerMode>('side')
const credentials = ref<OmniAICredential[]>([])
const credentialsLoading = ref(false)
const credentialsError = ref('')

const compatibleCredentials = computed(() =>
  credentials.value
    .filter((item) => ['openai', 'gemini', 'glm'].includes(item.provider))
    .sort((left, right) => {
      if (left.provider === right.provider) return left.name.localeCompare(right.name, 'pt-BR')
      return left.provider === 'openai' ? -1 : 1
    }),
)

const selectedCredential = computed(() =>
  compatibleCredentials.value.find((item) => item.id === config.credentialId.value),
)

const canSave = computed(
  () =>
    !config.loading.value &&
    !config.saving.value &&
    (!config.enabled.value ||
      Boolean(config.credentialId.value && selectedCredential.value && config.model.value.trim())),
)

function defaultModel(provider: string): string {
  if (provider === 'gemini') return 'gemini-2.5-flash'
  if (provider === 'glm') return 'glm-4.6'
  return 'gpt-4.1-mini'
}

function selectPrincipalCredential(): void {
  if (config.credentialId.value) return
  const credential =
    compatibleCredentials.value.find((item) => item.provider === 'openai') ||
    compatibleCredentials.value[0]
  if (!credential) return
  config.credentialId.value = credential.id
  config.provider.value = credential.provider
  config.model.value = defaultModel(credential.provider)
}

async function loadCredentials(): Promise<void> {
  credentialsLoading.value = true
  credentialsError.value = ''
  try {
    credentials.value = await fetchAICredentials(api)
    selectPrincipalCredential()
  } catch (error) {
    credentialsError.value = getApiErrorMessage(
      error,
      'Não foi possível carregar as chaves globais da conta.',
    )
  } finally {
    credentialsLoading.value = false
  }
}

function selectCredential(event: Event): void {
  const credentialId = (event.target as HTMLSelectElement).value
  const credential = compatibleCredentials.value.find((item) => item.id === credentialId)
  config.credentialId.value = credentialId
  if (!credential) return
  const providerChanged = config.provider.value !== credential.provider
  config.provider.value = credential.provider
  if (providerChanged || !config.model.value.trim()) {
    config.model.value = defaultModel(credential.provider)
  }
}

async function refreshCredentials(): Promise<void> {
  await loadCredentials()
}

async function save(): Promise<void> {
  if (!canSave.value) return
  await config.savePersona(config.draft.value)
}

watch(
  () => props.open,
  (open) => {
    if (!open) return
    config.resetFeedback()
    void config
      .fetchPersona()
      .catch(() => null)
      .then(() => loadCredentials())
  },
  { immediate: true },
)
</script>

<template>
  <OmniEntityDrawer
    v-model:mode="mode"
    :model-value="open"
    title="Configurar Omni Chat"
    subtitle="Uma única configuração para todas as lojas e usuários desta conta."
    @update:model-value="emit('update:open', $event)"
  >
    <section class="calendar-config__section omni-chat-config">
      <div class="calendar-config__block">
        <span class="calendar-config__label">Status da IA</span>
        <div class="calendar-config__seg" role="group" aria-label="Ligar ou desligar o Omni Chat">
          <button
            type="button"
            class="calendar-config__seg-btn"
            :class="{ 'is-active': config.enabled.value }"
            @click="config.enabled.value = true"
          >
            Ligada
          </button>
          <button
            type="button"
            class="calendar-config__seg-btn"
            :class="{ 'is-active': !config.enabled.value }"
            @click="config.enabled.value = false"
          >
            Desligada
          </button>
        </div>
        <span class="calendar-config__hint">
          Quando ligada, esta configuração vale para todos os usuários e todas as lojas da conta
          ativa.
        </span>
      </div>

      <details class="calendar-config__collapse" open>
        <summary class="calendar-config__collapse-head">Prompt do sistema (a lei da IA)</summary>
        <div class="calendar-config__collapse-body">
          <label class="calendar-config__field">
            <textarea
              v-model="config.draft.value"
              class="calendar-config__input calendar-config__textarea calendar-config__textarea--tall"
              :maxlength="PERSONA_MAX_LENGTH"
              :disabled="config.loading.value || config.saving.value"
              placeholder="Defina o comportamento, tom, regras e contexto do Omni Chat."
            ></textarea>
            <span class="calendar-config__hint">
              Este prompt é soberano. Vazio restaura o comportamento padrão do Omni.
            </span>
          </label>
        </div>
      </details>

      <details class="calendar-config__collapse" open>
        <summary class="calendar-config__collapse-head">Provedor e modelo</summary>
        <div class="calendar-config__collapse-body omni-chat-config__form">
          <label class="calendar-config__field">
            <span class="calendar-config__field-label">Chave global da conta</span>
            <select
              class="calendar-config__input"
              :value="config.credentialId.value"
              :disabled="credentialsLoading || config.saving.value"
              @change="selectCredential"
            >
              <option value="">
                {{ credentialsLoading ? 'Carregando chaves…' : 'Selecione uma chave' }}
              </option>
              <option
                v-for="credential in compatibleCredentials"
                :key="credential.id"
                :value="credential.id"
              >
                {{ credential.name }} · {{ credential.provider }} · final {{ credential.last4 }}
              </option>
            </select>
            <span class="calendar-config__hint">
              A chave é reutilizada do cofre global do painel e nunca aparece no navegador.
            </span>
          </label>

          <ConfigAiRoleModelSelect
            v-model="config.model.value"
            :credential-id="config.credentialId.value"
            capability="response"
            :disabled="!config.enabled.value || config.saving.value"
          />

          <label class="calendar-config__field">
            <span class="calendar-config__field-label">Temperatura</span>
            <input
              v-model.number="config.temperature.value"
              class="calendar-config__input"
              type="number"
              min="0"
              max="1"
              step="0.1"
              :disabled="config.saving.value"
            />
          </label>
        </div>
      </details>

      <details class="calendar-config__collapse">
        <summary class="calendar-config__collapse-head">Memória da conversa</summary>
        <div class="calendar-config__collapse-body">
          <label class="calendar-config__field">
            <span class="calendar-config__field-label">Últimas interações consideradas</span>
            <input
              v-model.number="config.historyWindow.value"
              class="calendar-config__input"
              type="number"
              :min="HISTORY_WINDOW_MIN"
              :max="HISTORY_WINDOW_MAX"
              :disabled="config.saving.value"
            />
            <span class="calendar-config__hint">
              Cada interação representa uma pergunta e uma resposta.
            </span>
          </label>
        </div>
      </details>

      <details class="calendar-config__collapse">
        <summary class="calendar-config__collapse-head">Chaves de API globais</summary>
        <div class="calendar-config__collapse-body">
          <ConfigAiCredentials :disabled="false" @changed="refreshCredentials" />
        </div>
      </details>

      <p v-if="credentialsError" class="calendar-config__warn">{{ credentialsError }}</p>
      <p v-if="config.errorMessage.value" class="calendar-config__warn">
        {{ config.errorMessage.value }}
      </p>
      <p v-else-if="config.successMessage.value" class="omni-chat-config__success">
        {{ config.successMessage.value }}
      </p>
    </section>

    <template #footer>
      <footer class="omni-chat-config__footer">
        <span>O salvamento é único por conta e entra em vigor imediatamente.</span>
        <AppPanelButton variant="primary" :disabled="!canSave" @click="save">
          {{ config.saving.value ? 'Salvando…' : 'Salvar configuração' }}
        </AppPanelButton>
      </footer>
    </template>
  </OmniEntityDrawer>
</template>

<style scoped>
.omni-chat-config,
.omni-chat-config__form {
  display: grid;
  gap: 0.85rem;
}

.omni-chat-config__success {
  margin: 0;
  color: rgb(var(--success));
  font-size: 0.8rem;
  font-weight: 700;
}

.omni-chat-config__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  width: 100%;
}

.omni-chat-config__footer span {
  color: var(--text-muted);
  font-size: 0.76rem;
}
</style>
