<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'

import ConfigAiCredentials from '~/components/omnichannel/config/ConfigAiCredentials.vue'
import ConfigAiRoleModelSelect from '~/components/omnichannel/config/ConfigAiRoleModelSelect.vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import {
  HISTORY_WINDOW_MAX,
  HISTORY_WINDOW_MIN,
  PERSONA_MAX_LENGTH,
  useOmniChatPersona,
} from '~/composables/useOmniChatPersona'
import type {
  OmniAssistantAccessMode,
  OmniAssistantModule,
  OmniAssistantSurface,
} from '~/composables/useOmniChatPersona'
import { ASSISTANT_AI_CREDENTIALS_PATH, fetchAICredentials } from '~/domain/omnichannel/config-api'
import type { OmniAICredential } from '~/domain/omnichannel/config-types'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { canManageAssistantConfiguration as canManageAssistantConfig } from '~/utils/assistant-access'

const props = withDefaults(
  defineProps<{ active?: boolean; accountId?: string; surface?: OmniAssistantSurface }>(),
  { active: true, surface: 'calendar' },
)

const auth = useAuthStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)
const config = useOmniChatPersona(() => props.accountId || '')
const credentials = ref<OmniAICredential[]>([])
const credentialsLoading = ref(false)
const credentialsError = ref('')
const activeAccountId = computed(() => String(props.accountId || '').trim())

let credentialsController: AbortController | null = null
let credentialsGeneration = 0
let credentialsAccountId = '\u0000'

const assistantModules: ReadonlyArray<{
  id: OmniAssistantModule
  label: string
  description: string
}> = [
  {
    id: 'calendar',
    label: 'Calendário',
    description: 'Consultar ou alterar compromissos e disponibilidade.',
  },
  {
    id: 'tasks',
    label: 'Tarefas',
    description: 'Consultar ou alterar tarefas vinculadas à operação.',
  },
  {
    id: 'meta_ads',
    label: 'Meta Ads',
    description: 'Consultar ou executar ações no gerenciador de anúncios.',
  },
  {
    id: 'users',
    label: 'Usuários',
    description: 'Consultar ou administrar usuários permitidos da conta.',
  },
]

const accessModes: ReadonlyArray<{ id: OmniAssistantAccessMode; label: string }> = [
  { id: 'off', label: 'Off' },
  { id: 'read', label: 'Leitura' },
  { id: 'write', label: 'Escrita' },
]

const surfaceLabel = computed(() => {
  if (props.surface === 'calendar') return 'Calendário'
  if (props.surface === 'meta_ads') return 'Meta Ads'
  return 'Global'
})

const inheritedConfigLabel = computed(() =>
  config.inheritedFrom.value
    ? `Esta conta usa a configuração herdada de ${config.inheritedFrom.value}.`
    : 'Esta conta usa a configuração herdada da agência.',
)

const compatibleCredentials = computed(() =>
  credentials.value
    .filter((item) => ['openai', 'anthropic', 'gemini', 'glm'].includes(item.provider))
    .sort((left, right) => {
      if (left.provider === right.provider) return left.name.localeCompare(right.name, 'pt-BR')
      return left.provider === 'openai' ? -1 : 1
    }),
)

const selectedCredential = computed(() =>
  compatibleCredentials.value.find((item) => item.id === config.credentialId.value),
)

const canManageAssistantConfiguration = computed(() => canManageAssistantConfig(auth))

const canSave = computed(
  () =>
    canManageAssistantConfiguration.value &&
    config.ready.value &&
    !config.loading.value &&
    !config.saving.value &&
    (!config.enabled.value ||
      Boolean(config.credentialId.value && selectedCredential.value && config.model.value.trim())),
)
const editorDisabled = computed(
  () =>
    !canManageAssistantConfiguration.value ||
    !config.ready.value ||
    config.loading.value ||
    config.saving.value,
)

function defaultModel(provider: string): string {
  if (provider === 'anthropic') return 'claude-sonnet-4-6'
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

function isAbortError(error: unknown): boolean {
  return (
    (error instanceof DOMException && error.name === 'AbortError') ||
    (error as { name?: string } | null)?.name === 'AbortError'
  )
}

function isCurrentCredentialsContext(accountId: string, generation: number): boolean {
  return (
    Boolean(accountId) &&
    accountId === credentialsAccountId &&
    accountId === activeAccountId.value &&
    generation === credentialsGeneration
  )
}

function resetCredentialsForAccount(accountId: string): void {
  if (accountId === credentialsAccountId) return
  credentialsAccountId = accountId
  credentialsGeneration += 1
  credentialsController?.abort()
  credentialsController = null
  credentials.value = []
  credentialsLoading.value = false
  credentialsError.value = ''
}

async function loadCredentials(requestAccountId = activeAccountId.value): Promise<void> {
  if (!requestAccountId) return
  credentialsController?.abort()
  const controller = new AbortController()
  credentialsController = controller
  const generation = credentialsGeneration
  credentialsLoading.value = true
  credentialsError.value = ''
  try {
    const loaded = await fetchAICredentials(api, {
      basePath: ASSISTANT_AI_CREDENTIALS_PATH,
      headers: { 'X-Account-Id': requestAccountId },
      signal: controller.signal,
    })
    if (
      credentialsController !== controller ||
      !isCurrentCredentialsContext(requestAccountId, generation)
    ) {
      return
    }
    credentials.value = loaded
    selectPrincipalCredential()
  } catch (error) {
    if (
      isAbortError(error) ||
      credentialsController !== controller ||
      !isCurrentCredentialsContext(requestAccountId, generation)
    ) {
      return
    }
    credentialsError.value = getApiErrorMessage(
      error,
      'Não foi possível carregar as chaves globais da conta.',
    )
  } finally {
    if (credentialsController === controller) {
      credentialsController = null
      if (isCurrentCredentialsContext(requestAccountId, generation)) {
        credentialsLoading.value = false
      }
    }
  }
}

function selectCredential(event: Event): void {
  if (editorDisabled.value) return
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

function setModuleAccess(module: OmniAssistantModule, mode: OmniAssistantAccessMode): void {
  if (editorDisabled.value) return
  config.surfaceModules.value = {
    ...config.surfaceModules.value,
    [props.surface]: {
      ...config.surfaceModules.value[props.surface],
      [module]: mode,
    },
  }
}

async function save(): Promise<void> {
  if (!canSave.value) return
  const requestAccountId = activeAccountId.value
  if (!requestAccountId || config.loadedAccountId.value !== requestAccountId) return
  await config.savePersona(config.draft.value)
}

watch(activeAccountId, (accountId) => resetCredentialsForAccount(accountId), {
  immediate: true,
  flush: 'sync',
})

watch(
  [() => props.active, activeAccountId],
  ([active, accountId], previous) => {
    if (!active || !accountId) return
    const [wasActive, previousAccountId] = previous || [false, '']
    if (wasActive && previousAccountId === accountId) return
    config.resetFeedback()
    void config
      .fetchPersona()
      .catch(() => null)
      .then(() => {
        if (accountId === activeAccountId.value) return loadCredentials(accountId)
        return undefined
      })
  },
  { immediate: true, flush: 'sync' },
)

onScopeDispose(() => {
  credentialsController?.abort()
  credentialsController = null
})
</script>

<template>
  <section class="calendar-config__section omni-chat-config">
    <h3 class="calendar-config__section-title">Crow Assistant</h3>
    <p class="calendar-config__hint">
      Configuração compartilhada da conta para a superfície {{ surfaceLabel }}.
    </p>
    <p v-if="!canManageAssistantConfiguration" class="omni-chat-config__access-warning" role="note">
      <UIcon name="i-lucide-lock-keyhole" aria-hidden="true" />
      <span>
        Esta configuração afeta todas as superfícies da conta. Somente administradores da conta ou
        da Automação podem alterá-la.
      </span>
    </p>

    <p v-if="config.inherited.value" class="omni-chat-config__inherited" role="note">
      <UIcon name="i-lucide-building-2" aria-hidden="true" />
      <span>
        {{ inheritedConfigLabel }} Ao salvar, você cria uma configuração própria para esta conta.
      </span>
    </p>

    <div class="calendar-config__block">
      <span class="calendar-config__label">Status da IA</span>
      <div
        class="calendar-config__seg"
        role="group"
        aria-label="Ligar ou desligar o Assistente Omni"
      >
        <button
          type="button"
          class="calendar-config__seg-btn"
          :class="{ 'is-active': config.enabled.value }"
          :disabled="editorDisabled"
          @click="config.enabled.value = true"
        >
          Ligada
        </button>
        <button
          type="button"
          class="calendar-config__seg-btn"
          :class="{ 'is-active': !config.enabled.value }"
          :disabled="editorDisabled"
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

    <details class="calendar-config__collapse">
      <summary class="calendar-config__collapse-head">Módulos acessíveis nesta página</summary>
      <div class="calendar-config__collapse-body omni-chat-config__access">
        <p class="omni-chat-config__access-intro">
          Defina o acesso solicitado pelo assistente na superfície {{ surfaceLabel }}. O servidor
          sempre cruza esta escolha com o contrato da conta e o RBAC do usuário; selecionar uma
          opção aqui não concede permissões adicionais.
        </p>
        <p class="omni-chat-config__access-warning" role="note">
          <UIcon name="i-lucide-shield-alert" aria-hidden="true" />
          <span>
            Escrita do Meta Ads permanece somente leitura até o executor idempotente; esta seleção
            registra a intenção, não libera ações.
          </span>
        </p>

        <div class="omni-chat-config__module-list">
          <div
            v-for="assistantModule in assistantModules"
            :key="assistantModule.id"
            class="omni-chat-config__module-row"
          >
            <div class="omni-chat-config__module-copy">
              <strong>{{ assistantModule.label }}</strong>
              <span>{{ assistantModule.description }}</span>
            </div>

            <div
              class="omni-chat-config__access-options"
              role="group"
              :aria-label="`Acesso a ${assistantModule.label} nesta página`"
            >
              <button
                v-for="accessMode in accessModes"
                :key="accessMode.id"
                type="button"
                class="omni-chat-config__access-button"
                :class="{
                  'is-active':
                    config.surfaceModules.value[props.surface][assistantModule.id] ===
                    accessMode.id,
                }"
                :aria-pressed="
                  config.surfaceModules.value[props.surface][assistantModule.id] === accessMode.id
                "
                :disabled="editorDisabled"
                @click="setModuleAccess(assistantModule.id, accessMode.id)"
              >
                {{ accessMode.label }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </details>

    <details class="calendar-config__collapse">
      <summary class="calendar-config__collapse-head">Prompt do sistema (a lei da IA)</summary>
      <div class="calendar-config__collapse-body">
        <label class="calendar-config__field">
          <textarea
            v-model="config.draft.value"
            class="calendar-config__input calendar-config__textarea calendar-config__textarea--tall"
            :maxlength="PERSONA_MAX_LENGTH"
            :disabled="editorDisabled"
            placeholder="Defina o comportamento, tom, regras e contexto do Assistente Omni."
          ></textarea>
          <span class="calendar-config__hint">
            Este prompt é soberano. Vazio restaura o comportamento padrão do Omni.
          </span>
        </label>
      </div>
    </details>

    <details class="calendar-config__collapse">
      <summary class="calendar-config__collapse-head">Provedor e modelo</summary>
      <div class="calendar-config__collapse-body omni-chat-config__form">
        <label class="calendar-config__field">
          <span class="calendar-config__field-label">Credencial ativa do Assistente</span>
          <select
            class="calendar-config__input"
            :value="config.credentialId.value"
            :disabled="credentialsLoading || editorDisabled"
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
              {{
                credential.readOnly
                  ? ` · compartilhada por ${credential.ownerName || 'agência'}`
                  : ''
              }}
            </option>
          </select>
          <span class="calendar-config__hint">
            Cadastre várias contas com apelidos no bloco abaixo, escolha uma aqui e clique em
            Salvar. A chave nunca aparece no navegador.
          </span>
        </label>

        <ConfigAiRoleModelSelect
          v-model="config.model.value"
          :account-id="activeAccountId"
          :credential-id="config.credentialId.value"
          :credential-base-path="ASSISTANT_AI_CREDENTIALS_PATH"
          capability="response"
          :disabled="!config.enabled.value || editorDisabled"
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
            :disabled="editorDisabled"
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
            :disabled="editorDisabled"
          />
          <span class="calendar-config__hint">
            Cada interação representa uma pergunta e uma resposta.
          </span>
        </label>
      </div>
    </details>

    <details class="calendar-config__collapse">
      <summary class="calendar-config__collapse-head">
        Contas e chaves de IA (adicione várias)
      </summary>
      <div class="calendar-config__collapse-body">
        <ConfigAiCredentials
          v-if="config.ready.value"
          :key="activeAccountId"
          :account-id="activeAccountId"
          :credential-base-path="ASSISTANT_AI_CREDENTIALS_PATH"
          :disabled="editorDisabled"
          :allowed-providers="['openai', 'anthropic', 'gemini', 'glm']"
          @changed="refreshCredentials"
        />
        <p v-else class="calendar-config__hint">
          Aguarde a configuração da conta ativa carregar antes de editar as credenciais.
        </p>
      </div>
    </details>

    <p v-if="credentialsError" class="calendar-config__warn">{{ credentialsError }}</p>
    <p v-if="config.errorMessage.value" class="calendar-config__warn">
      {{ config.errorMessage.value }}
    </p>
    <p v-else-if="config.successMessage.value" class="omni-chat-config__success">
      {{ config.successMessage.value }}
    </p>
    <footer class="omni-chat-config__footer">
      <span>O salvamento do Assistente é único por conta e entra em vigor imediatamente.</span>
      <AppPanelButton variant="primary" :disabled="!canSave" @click="save">
        {{ config.saving.value ? 'Salvando…' : 'Salvar Assistente' }}
      </AppPanelButton>
    </footer>
  </section>
</template>

<style scoped>
.omni-chat-config,
.omni-chat-config__form {
  display: grid;
  gap: 0.85rem;
  min-width: 0;
  max-width: 100%;
}

.omni-chat-config__success {
  margin: 0;
  color: rgb(var(--success));
  font-size: 0.8rem;
  font-weight: 700;
}

.omni-chat-config__inherited {
  display: flex;
  align-items: flex-start;
  gap: 0.45rem;
  margin: 0;
  padding: 0.65rem 0.7rem;
  border: 1px solid rgb(var(--primary) / 0.36);
  border-radius: 0.65rem;
  background: rgb(var(--primary) / 0.08);
  color: var(--text-main);
  font-size: 0.76rem;
  line-height: 1.45;
}

.omni-chat-config__inherited :deep(svg) {
  flex: 0 0 auto;
  margin-top: 0.08rem;
  color: rgb(var(--primary));
}

.omni-chat-config__access {
  gap: 0.75rem;
}

.omni-chat-config__access-intro {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.78rem;
  line-height: 1.5;
}

.omni-chat-config__access-warning {
  display: flex;
  align-items: flex-start;
  gap: 0.45rem;
  margin: 0;
  padding: 0.65rem 0.7rem;
  border: 1px solid rgb(var(--warning) / 0.42);
  border-radius: 0.65rem;
  background: rgb(var(--warning) / 0.1);
  color: var(--text-main);
  font-size: 0.74rem;
  line-height: 1.45;
}

.omni-chat-config__access-warning :deep(svg) {
  flex: 0 0 auto;
  margin-top: 0.08rem;
  color: rgb(var(--warning));
}

.omni-chat-config__module-list {
  display: grid;
  gap: 0.65rem;
}

.omni-chat-config__module-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  align-items: center;
  gap: 0.8rem;
  padding: 0.7rem;
  border: 1px solid rgb(var(--border) / 0.82);
  border-radius: 0.75rem;
  background: rgb(var(--surface-2) / 0.48);
}

.omni-chat-config__module-copy {
  display: grid;
  gap: 0.18rem;
  min-width: 0;
}

.omni-chat-config__module-copy strong {
  color: var(--text-main);
  font-size: 0.82rem;
}

.omni-chat-config__module-copy span {
  color: var(--text-muted);
  font-size: 0.72rem;
  line-height: 1.4;
}

.omni-chat-config__access-options {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.28rem;
  width: 100%;
  min-width: 0;
}

.omni-chat-config__access-button {
  min-width: 0;
  min-height: 2rem;
  padding: 0.35rem 0.55rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.55rem;
  background: rgb(var(--surface));
  color: var(--text-muted);
  font: inherit;
  font-size: 0.72rem;
  font-weight: 700;
  cursor: pointer;
}

.omni-chat-config__access-button:hover:not(:disabled) {
  border-color: rgb(var(--primary) / 0.72);
  color: rgb(var(--primary));
}

.omni-chat-config__access-button.is-active {
  border-color: rgb(var(--primary));
  background: rgb(var(--primary));
  color: rgb(var(--surface));
}

.omni-chat-config__access-button:focus-visible {
  outline: 2px solid rgb(var(--primary) / 0.4);
  outline-offset: 2px;
}

.omni-chat-config__access-button:disabled {
  cursor: not-allowed;
  opacity: 0.58;
}

.omni-chat-config__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  width: 100%;
  min-width: 0;
  flex-wrap: wrap;
}

.omni-chat-config__footer span {
  color: var(--text-muted);
  font-size: 0.76rem;
}

@media (max-width: 720px) {
  .omni-chat-config__module-row {
    grid-template-columns: 1fr;
  }

  .omni-chat-config__access-options {
    width: 100%;
  }

  .omni-chat-config__footer {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
