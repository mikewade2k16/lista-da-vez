<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest } from '~/utils/api-client'
import * as calendarApi from '~/domain/calendar/calendar-api'
import type { CalendarAiKeys } from '~/domain/calendar/calendar-api'
import {
  AI_PROVIDER_LABEL,
  AI_SECRET_PROVIDERS,
  type CalendarAiSecretProvider,
} from '~/utils/calendar'

// Chaves de API da IA (SPEC-F1 / contrato SEC). SEGURANCA: o front NUNCA ve a chave
// crua — so o status MASCARADO {set,last4} do GET /ai-keys. A escrita e write-only
// (PUT; vazio = limpar) e vai pra FONTE ATIVA: conta -> PUT /ai-keys; global -> PUT
// /ai-keys/global (so platform_admin). O escopo ativo vem do banco (config salva),
// nao do rascunho: por isso a prop `useGlobalKeys` reflete o valor JA salvo.
const props = defineProps<{ useGlobalKeys: boolean }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

// Chaves globais so o platform_admin edita (mesma regra do limite de midia global).
const isPlatformAdmin = computed(() => auth.role === 'platform_admin')

const providers = AI_SECRET_PROVIDERS
const state = ref<CalendarAiKeys | null>(null)
const drafts = ref<Record<CalendarAiSecretProvider, string>>({ gemini: '', glm: '', openai: '' })
const busy = ref<CalendarAiSecretProvider | ''>('')

// Escopo autoritativo = o que a API respondeu (fonte ativa por config salva); enquanto
// nao carregou, cai no valor salvo recebido por prop.
const activeScope = computed(
  () => state.value?.scope || (props.useGlobalKeys ? 'global' : 'account'),
)
const canEdit = computed(() => (activeScope.value === 'global' ? isPlatformAdmin.value : true))

function label(provider: CalendarAiSecretProvider): string {
  return AI_PROVIDER_LABEL[provider]
}

function statusOf(provider: CalendarAiSecretProvider): { set: boolean; last4: string } {
  return state.value?.keys[provider] || { set: false, last4: '' }
}

async function load(): Promise<void> {
  try {
    state.value = await calendarApi.fetchAiKeys(apiRequest)
  } catch {
    state.value = null
  }
}

// Grava (ou limpa, com clear=true) na fonte ativa e re-le do banco (status mascarado).
async function writeKey(provider: CalendarAiSecretProvider, clear: boolean): Promise<void> {
  if (!canEdit.value) return
  const value = clear ? '' : drafts.value[provider].trim()
  if (!clear && !value) return
  busy.value = provider
  try {
    if (activeScope.value === 'global') {
      await calendarApi.putGlobalAiKey(apiRequest, provider, value)
    } else {
      await calendarApi.putAiKey(apiRequest, provider, value)
    }
    drafts.value[provider] = ''
    await load()
    ui.success(clear ? `Chave ${label(provider)} removida.` : `Chave ${label(provider)} salva.`)
  } catch {
    ui.error(`Nao foi possivel atualizar a chave ${label(provider)}.`)
  } finally {
    busy.value = ''
  }
}

onMounted(() => void load())
// Escopo salvo mudou (footer "Salvar configuracoes" trocou global<->conta): DESCARTA
// qualquer chave digitada e nao salva (senao uma chave da conta iria parar no store
// global, ou vice-versa) e re-le a fonte ativa para o status refletir a nova origem.
watch(
  () => props.useGlobalKeys,
  () => {
    drafts.value = { gemini: '', glm: '', openai: '' }
    void load()
  },
)
</script>

<template>
  <div class="calendar-config__keys">
    <p class="calendar-config__hint">
      As chaves ficam guardadas com segurança no servidor — nunca no navegador. Aqui só aparece o
      status mascarado; para trocar, digite uma nova chave.
    </p>
    <p class="calendar-config__hint">
      Fonte ativa:
      <strong>
        {{ activeScope === 'global' ? 'chaves globais da plataforma' : 'chaves desta conta' }}
      </strong>
      .
    </p>

    <p v-if="activeScope === 'global' && !canEdit" class="calendar-config__warn">
      <UIcon name="i-lucide-lock" aria-hidden="true" />
      Apenas administradores da plataforma podem alterar as chaves globais. Peça ao admin.
    </p>

    <div v-for="provider in providers" :key="provider" class="calendar-config__key-row">
      <div class="calendar-config__key-head">
        <span class="calendar-config__field-label">{{ label(provider) }}</span>
        <span
          class="calendar-config__key-status"
          :class="statusOf(provider).set ? 'is-set' : 'is-unset'"
        >
          <template v-if="statusOf(provider).set">
            configurada &bull;&bull;&bull;&bull;{{ statusOf(provider).last4 }}
          </template>
          <template v-else>não configurada</template>
        </span>
      </div>
      <div class="calendar-config__key-actions">
        <input
          v-model="drafts[provider]"
          class="calendar-config__input"
          type="password"
          autocomplete="off"
          :placeholder="
            statusOf(provider).set ? 'Digite para trocar a chave' : 'Cole a chave da API'
          "
          :disabled="!canEdit || busy === provider"
        />
        <AppPanelButton
          variant="ghost"
          :disabled="!canEdit || busy === provider || !drafts[provider].trim()"
          @click="writeKey(provider, false)"
        >
          Salvar
        </AppPanelButton>
        <AppPanelButton
          v-if="statusOf(provider).set"
          variant="ghost"
          :disabled="!canEdit || busy === provider"
          @click="writeKey(provider, true)"
        >
          Limpar
        </AppPanelButton>
      </div>
    </div>
  </div>
</template>
