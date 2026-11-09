<script setup lang="ts">
import PlusExplanationConfig from '../accounting/PlusExplanationConfig.vue'
import { emptyPlusExplanation } from '../../types/plus-explanation'
import { computed, onMounted, onBeforeUnmount, reactive, ref, watch } from 'vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import ConfigAiCredentials from '~/components/omnichannel/config/ConfigAiCredentials.vue'
import { useAuthStore } from '~/stores/auth'
import { canManageAssistantConfiguration } from '~/utils/assistant-access'
import ConfigAiRoleModelSelect from '~/components/omnichannel/config/ConfigAiRoleModelSelect.vue'
import { useFinanceAssistantStore } from '~/stores/finance-assistant'
import { useCoreAccountStore } from '../../../core/stores/account'
import { getApiErrorMessage } from '~/utils/api-client'
import type { OmniAICredential } from '~/domain/omnichannel/config-types'
import { ASSISTANT_AI_CREDENTIALS_PATH } from '~/domain/omnichannel/config-api'
const props = defineProps<{ canManage: boolean }>()
const store = useFinanceAssistantStore()
const auth = useAuthStore()
const canManageCredentials = computed(() => canManageAssistantConfiguration(auth))
const account = useCoreAccountStore()
const route = useRoute()
const cfg = reactive({
  plus: { enabled: false, toolPolicyVersion: 1, explanation: emptyPlusExplanation() },
  enabled: false,
  credentialId: '',
  model: '',
  instructions: '',
  temperature: 0.2,
  historyWindow: 5,
  updatedAt: '',
})
const credentials = ref<OmniAICredential[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const status = ref('')
let generation = 0
const options = computed(() =>
  credentials.value.map((item) => ({ value: item.id, label: `${item.name} (${item.provider})` })),
)
async function load() {
  const ticket = ++generation
  const id = account.activeAccountId
  loading.value = true
  error.value = ''
  status.value = ''
  credentials.value = []
  Object.assign(cfg, {
    plus: { enabled: false, toolPolicyVersion: 1, explanation: emptyPlusExplanation() },
    enabled: false,
    credentialId: '',
    model: '',
    instructions: '',
    temperature: 0.2,
    historyWindow: 5,
    updatedAt: '',
  })
  try {
    const value = await store.get(id)
    if (ticket !== generation) return
    Object.assign(cfg, value)
    if (props.canManage) {
      const list = await store.listCredentials(id)
      if (ticket === generation) credentials.value = list
    }
  } catch (cause) {
    if (ticket === generation)
      error.value = getApiErrorMessage(
        cause,
        'Não foi possível carregar a configuração do chat Finance.',
      )
  } finally {
    if (ticket === generation) loading.value = false
  }
}
async function refreshCredentials() {
  const ticket = generation
  const id = account.activeAccountId
  try {
    const list = await store.listCredentials(id)
    if (ticket === generation) credentials.value = list
  } catch (cause) {
    if (ticket === generation)
      error.value = getApiErrorMessage(cause, 'Não foi possível atualizar as credenciais de IA.')
  }
}
async function save() {
  if (!props.canManage || saving.value) return
  const ticket = generation
  const id = account.activeAccountId
  saving.value = true
  error.value = ''
  status.value = ''
  try {
    const value = await store.save(id, {
      ...cfg,
      temperature: Number(cfg.temperature),
      historyWindow: Number(cfg.historyWindow),
    })
    if (ticket === generation) {
      Object.assign(cfg, value)
      status.value = 'Configuração do chat Finance salva.'
    }
  } catch (cause) {
    if (ticket === generation)
      error.value = getApiErrorMessage(cause, 'Não foi possível salvar o chat Finance.')
  } finally {
    if (ticket === generation) saving.value = false
  }
}
onMounted(load)
watch(
  () => account.activeAccountId,
  () => {
    saving.value = false
    void load()
  },
)
onBeforeUnmount(() => generation++)
</script>
<template>
  <details class="finance-assistant-config" :open="route.query.config === 'ia'">
    <summary>Crow Assistant Plus — IA {{ cfg.enabled ? 'permitida' : 'desativada' }}</summary>
    <p>
      Configuração própria desta conta. O chat consulta planilhas e os dados fiscais do seu perfil;
      não emite notas nem acessa o portal.
    </p>
    <p v-if="loading" role="status">Carregando configuração...</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-if="status" role="status">{{ status }}</p>
    <p v-if="!loading && !error && credentials.length === 0 && canManage" role="status">
      Cadastre uma credencial e selecione um modelo para permitir respostas por IA. Consultas locais
      e preparação continuam disponíveis sem ela.
    </p>
    <details v-if="canManageCredentials && !loading">
      <summary>Contas e chaves de IA</summary>
      <ConfigAiCredentials
        :key="account.activeAccountId"
        :account-id="account.activeAccountId"
        :credential-base-path="ASSISTANT_AI_CREDENTIALS_PATH"
        :allowed-providers="['openai', 'anthropic', 'gemini', 'glm']"
        @changed="refreshCredentials"
      />
    </details>
    <p v-else-if="!loading && credentials.length === 0 && canManage">
      Solicite ao administrador da conta o cadastro de uma chave no cofre do assistente.
    </p>
    <form class="finance-assistant-config__form" @submit.prevent="save">
      <fieldset
        :disabled="loading || saving || !canManage"
        class="finance-assistant-config__fields"
      >
        <UCheckbox
          v-model="cfg.plus.enabled"
          label="Ativar ferramentas avançadas do Plus (empresas e fontes)"
        />
        <PlusExplanationConfig
          v-model="cfg.plus.explanation"
          :account-id="account.activeAccountId"
          :options="
            credentials
              .filter((item) => ['openai', 'gemini', 'glm'].includes(item.provider))
              .map((item) => ({ value: item.id, label: item.name }))
          "
        />
        <UCheckbox v-model="cfg.enabled" label="Permitir respostas com IA no Crow Assistant Plus" />
        <AppSelectField
          v-model="cfg.credentialId"
          label="Credencial de IA"
          :options="options"
          :disabled="loading || saving || !canManage"
          @change="cfg.model = ''"
        />
        <ConfigAiRoleModelSelect
          v-model="cfg.model"
          :credential-id="cfg.credentialId"
          capability="response"
          :account-id="account.activeAccountId"
          :credential-base-path="ASSISTANT_AI_CREDENTIALS_PATH"
          :disabled="loading || saving || !canManage"
        />
        <label>
          Instruções do Finance
          <textarea v-model="cfg.instructions" maxlength="12000" rows="4"></textarea>
        </label>
        <label>
          Temperatura
          <input v-model.number="cfg.temperature" type="number" min="0" max="2" step="0.1" />
        </label>
        <label>
          Interações na memória
          <input v-model.number="cfg.historyWindow" type="number" min="1" max="20" />
        </label>
        <AppPanelButton type="submit" :disabled="loading || saving || !canManage">
          {{ saving ? 'Salvando...' : 'Salvar configuração do chat' }}
        </AppPanelButton>
      </fieldset>
    </form>
  </details>
</template>
<style scoped>
.finance-assistant-config {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  padding: 12px;
  margin-top: 12px;
}
.finance-assistant-config > summary {
  cursor: pointer;
  font-weight: 600;
}
.finance-assistant-config > p {
  font-size: 0.8rem;
  color: rgb(var(--muted));
  margin-top: 8px;
}
.finance-assistant-config__form {
  margin-top: 12px;
}
.finance-assistant-config__fields {
  display: grid;
  gap: 12px;
  border: 0;
  padding: 0;
  min-width: 0;
}
.finance-assistant-config__fields label {
  display: grid;
  gap: 6px;
  font-size: 0.8rem;
}
.finance-assistant-config__fields input,
.finance-assistant-config__fields textarea {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  padding: 8px;
  color: rgb(var(--text));
  background: rgb(var(--surface));
}
</style>
