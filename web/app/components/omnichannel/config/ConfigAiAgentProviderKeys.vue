<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { AI_PROVIDER_LABEL, AI_PROVIDERS, type CalendarAiProvider } from '~/utils/calendar'
import {
  clearAgentProviderKey,
  fetchAgentProviderKeys,
  putAgentProviderKey,
} from '~/domain/omnichannel/config-api'
import type { OmniCredentialStatus } from '~/domain/omnichannel/config-types'

const props = defineProps<{ agentId: string; disabled?: boolean }>()
const emit = defineEmits<{ changed: [] }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)
const drafts = reactive<Record<CalendarAiProvider, string>>({ gemini: '', glm: '', openai: '' })
const statuses = reactive<Record<CalendarAiProvider, OmniCredentialStatus>>({
  gemini: { set: false, last4: '' },
  glm: { set: false, last4: '' },
  openai: { set: false, last4: '' },
})
const loading = ref(false)
const savingProvider = ref('')

function hydrate(keys: Partial<Record<CalendarAiProvider, OmniCredentialStatus>>): void {
  for (const provider of AI_PROVIDERS) {
    statuses[provider] = keys[provider] || { set: false, last4: '' }
    drafts[provider] = ''
  }
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const result = await fetchAgentProviderKeys(api, props.agentId)
    hydrate(result.keys)
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível carregar as chaves de API.'))
  } finally {
    loading.value = false
  }
}

async function save(provider: CalendarAiProvider): Promise<void> {
  const apiKey = drafts[provider].trim()
  if (!apiKey) return
  savingProvider.value = provider
  try {
    const result = await putAgentProviderKey(api, props.agentId, provider, apiKey)
    hydrate(result.keys)
    ui.success(`Chave ${AI_PROVIDER_LABEL[provider]} salva.`)
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível salvar a chave.'))
  } finally {
    savingProvider.value = ''
  }
}

async function clear(provider: CalendarAiProvider): Promise<void> {
  savingProvider.value = provider
  try {
    const result = await clearAgentProviderKey(api, props.agentId, provider)
    hydrate(result.keys)
    ui.success(`Chave ${AI_PROVIDER_LABEL[provider]} removida.`)
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível remover a chave.'))
  } finally {
    savingProvider.value = ''
  }
}

watch(
  () => props.agentId,
  () => void load(),
)
onMounted(() => void load())
</script>

<template>
  <div class="cfg-provider-keys">
    <p class="calendar-config__hint">
      Cadastre uma chave por provedor. Elas ficam cifradas no servidor e nunca retornam ao navegador
      ou ao n8n.
    </p>
    <div v-for="provider in AI_PROVIDERS" :key="provider" class="cfg-provider-keys__item">
      <div class="cfg-provider-keys__status">
        <span class="calendar-config__field-label">{{ AI_PROVIDER_LABEL[provider] }}</span>
        <strong :class="statuses[provider].set ? 'is-set' : 'is-unset'">
          <template v-if="statuses[provider].set">
            Configurada
            <template v-if="statuses[provider].last4">····{{ statuses[provider].last4 }}</template>
          </template>
          <template v-else>Não configurada</template>
        </strong>
      </div>
      <div class="cfg-provider-keys__row">
        <input
          v-model="drafts[provider]"
          class="calendar-config__input"
          type="password"
          autocomplete="off"
          :placeholder="
            statuses[provider].set ? 'Digite para trocar a chave' : 'Cole a chave da API'
          "
          :disabled="disabled || loading || !!savingProvider"
        />
        <AppPanelButton
          variant="secondary"
          :disabled="disabled || loading || !!savingProvider || !drafts[provider].trim()"
          @click="save(provider)"
        >
          Salvar
        </AppPanelButton>
        <AppPanelButton
          v-if="statuses[provider].set"
          variant="ghost"
          :disabled="disabled || loading || !!savingProvider"
          @click="clear(provider)"
        >
          Limpar
        </AppPanelButton>
      </div>
    </div>
  </div>
</template>

<style scoped>
.cfg-provider-keys,
.cfg-provider-keys__item {
  display: grid;
  gap: 0.65rem;
}

.cfg-provider-keys__item {
  padding-bottom: 0.75rem;
  border-bottom: 1px solid rgb(var(--border) / 0.55);
}

.cfg-provider-keys__item:last-child {
  padding-bottom: 0;
  border-bottom: 0;
}

.cfg-provider-keys__status,
.cfg-provider-keys__row {
  display: flex;
  align-items: center;
  gap: 0.65rem;
}

.cfg-provider-keys__status {
  justify-content: space-between;
}

.cfg-provider-keys__status strong {
  font-size: 0.74rem;
}

.cfg-provider-keys__status .is-set {
  color: rgb(var(--success));
}

.cfg-provider-keys__status .is-unset {
  color: rgb(var(--muted));
}

.cfg-provider-keys__row .calendar-config__input {
  flex: 1;
  min-width: 0;
}

@media (max-width: 640px) {
  .cfg-provider-keys__row {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
