<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useAuthStore } from '~/stores/auth'
import { useCalendarStore } from '~/stores/calendar'
import { useUiStore } from '~/stores/ui'
import { createApiRequest } from '~/utils/api-client'
import * as calendarApi from '~/domain/calendar/calendar-api'
import {
  AI_PROVIDER_LABEL,
  AI_PROVIDERS,
  defaultClientAiOverride,
  isEmptyClientAiOverride,
  type CalendarAiConfig,
  type CalendarAiProvider,
  type CalendarClientAiOverride,
} from '~/utils/calendar'

// Escopo por cliente (SPEC-F3 / WAVE 3.1). Extraido do ConfigAi p/ nao passar de 450
// linhas. Dois modos (ai.scopeMode): GERAL (uma config p/ todos, com multi-select de
// clientes para DESATIVAR a IA — ai.disabledClientIds) e INDIVIDUAL (override de
// COMPORTAMENTO por cliente, salvo via putClientAiConfig). As CHAVES de API NUNCA
// aparecem aqui — seguem no nivel conta/global (contrato SEC). scopeMode e
// disabledClientIds vivem no draft compartilhado da config (v-model no `ai`); o
// override por cliente salva com botao proprio (fonte unica = banco).
const props = defineProps<{ modelValue: CalendarAiConfig }>()
const emit = defineEmits<{ 'update:modelValue': [value: CalendarAiConfig] }>()

const store = useCalendarStore()
const ui = useUiStore()
const auth = useAuthStore()
const runtimeConfig = useRuntimeConfig()
const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

const ai = computed(() => props.modelValue)
const clients = computed(() => store.clients)

// Opcoes de provider do override: "" = herdar a config geral + os providers ligados.
const providerOptions = [
  { value: '', label: 'Usar config geral' },
  ...AI_PROVIDERS.map((value) => ({ value, label: AI_PROVIDER_LABEL[value] })),
]

function patch(next: Partial<CalendarAiConfig>): void {
  emit('update:modelValue', { ...ai.value, ...next })
}

function setScopeMode(mode: CalendarAiConfig['scopeMode']): void {
  if (ai.value.scopeMode === mode) return
  patch({ scopeMode: mode })
}

// --- Modo GERAL: clientes com a IA desligada ------------------------------------
function isDisabled(clientId: string): boolean {
  return ai.value.disabledClientIds.includes(clientId)
}

function toggleDisabled(clientId: string): void {
  const set = new Set(ai.value.disabledClientIds)
  if (set.has(clientId)) set.delete(clientId)
  else set.add(clientId)
  patch({ disabledClientIds: [...set] })
}

// --- Modo INDIVIDUAL: override por cliente --------------------------------------
const selectedClientId = ref('')
// Rascunho editavel do override do cliente selecionado. Re-hidrata do back ao trocar
// de cliente (fonte unica = banco); so preserva enquanto ha edicao pendente (touched).
const draft = ref<CalendarClientAiOverride>(defaultClientAiOverride())
// Snapshot do que esta PERSISTIDO (dirige o badge "usa config geral" — estado real do
// banco, nao o rascunho em edicao).
const saved = ref<CalendarClientAiOverride>(defaultClientAiOverride())
const touched = ref(false)
const loading = ref(false)
const saving = ref(false)

const usesGeneral = computed(() => isEmptyClientAiOverride(saved.value))
const selectedClientName = computed(
  () => clients.value.find((client) => client.id === selectedClientId.value)?.name || '',
)

function mark(): void {
  touched.value = true
}

// Troca de cliente com edicao pendente: confirma antes de descartar o rascunho.
// Dirty-guard unico da casa via ui.confirm (padrao do drawer de config — SPEC-F6).
async function selectClient(clientId: string): Promise<void> {
  if (clientId === selectedClientId.value) return
  if (touched.value) {
    const { confirmed } = await ui.confirm({
      title: 'Descartar alterações?',
      message:
        'Há alterações não salvas neste cliente. Trocar de cliente vai descartá-las. Continuar?',
      confirmLabel: 'Descartar',
      cancelLabel: 'Continuar editando',
    })
    if (!confirmed) return
  }
  selectedClientId.value = clientId
  touched.value = false
  loading.value = true
  try {
    const loaded = await calendarApi.fetchClientAiConfig(apiRequest, clientId)
    draft.value = loaded
    saved.value = loaded
  } catch {
    draft.value = defaultClientAiOverride()
    saved.value = defaultClientAiOverride()
  } finally {
    loading.value = false
  }
}

function setEnabled(value: boolean | null): void {
  draft.value = { ...draft.value, enabled: value }
  mark()
}

function setProvider(value: string): void {
  const provider = value === '' ? '' : (value as CalendarAiProvider)
  draft.value = { ...draft.value, provider }
  mark()
}

function setModel(value: string): void {
  draft.value = { ...draft.value, model: value }
  mark()
}

function setBaseUrl(value: string): void {
  draft.value = { ...draft.value, baseUrl: value }
  mark()
}

function setSystemPrompt(value: string): void {
  draft.value = { ...draft.value, systemPrompt: value }
  mark()
}

// Temperatura: vazio = herdar (null); numero = clamp 0..1.
function setTemperature(value: string): void {
  const trimmed = value.trim()
  if (!trimmed) {
    draft.value = { ...draft.value, temperature: null }
    mark()
    return
  }
  const num = Number(trimmed)
  if (Number.isFinite(num)) {
    draft.value = { ...draft.value, temperature: Math.min(1, Math.max(0, num)) }
  }
  mark()
}

async function save(): Promise<void> {
  if (!selectedClientId.value) return
  saving.value = true
  try {
    const result = await calendarApi.putClientAiConfig(
      apiRequest,
      selectedClientId.value,
      draft.value,
    )
    draft.value = result
    saved.value = result
    touched.value = false
    ui.success('Comportamento do cliente salvo.')
  } catch {
    ui.error('Não foi possível salvar o comportamento do cliente.')
  } finally {
    saving.value = false
  }
}

// Se a lista de clientes muda e o selecionado sai dela, limpa a selecao (sem
// auto-selecionar outro): mantem o estado vazio orientativo.
watch(clients, (list) => {
  if (selectedClientId.value && !list.some((client) => client.id === selectedClientId.value)) {
    selectedClientId.value = ''
    draft.value = defaultClientAiOverride()
    saved.value = defaultClientAiOverride()
    touched.value = false
  }
})
</script>

<template>
  <div class="calendar-config__client-scope">
    <div class="calendar-config__block">
      <span class="calendar-config__label">Escopo da IA</span>
      <div class="calendar-config__seg" role="group" aria-label="Escopo da IA por cliente">
        <button
          type="button"
          class="calendar-config__seg-btn"
          :class="{ 'is-active': ai.scopeMode === 'general' }"
          @click="setScopeMode('general')"
        >
          Geral
        </button>
        <button
          type="button"
          class="calendar-config__seg-btn"
          :class="{ 'is-active': ai.scopeMode === 'perClient' }"
          @click="setScopeMode('perClient')"
        >
          Individual
        </button>
      </div>
      <span class="calendar-config__hint">
        Geral: uma configuração para todos os clientes (dá para desativar a IA em clientes
        específicos). Individual: cada cliente tem o próprio comportamento.
      </span>
    </div>

    <!-- Modo GERAL: desativar a IA para clientes especificos -->
    <div v-if="ai.scopeMode === 'general'" class="calendar-config__block">
      <span class="calendar-config__field-label">Desativar a IA para clientes</span>
      <span class="calendar-config__hint">
        Os clientes marcados não usam o assistente: o chat, o plano do mês e a transcrição respondem
        que a IA está desligada.
      </span>
      <div v-if="clients.length" class="calendar-config__members">
        <label v-for="client in clients" :key="client.id" class="calendar-config__check">
          <input
            type="checkbox"
            :checked="isDisabled(client.id)"
            @change="toggleDisabled(client.id)"
          />
          <span>{{ client.name }}</span>
        </label>
      </div>
      <p v-else class="calendar-config__empty">Nenhum cliente ativo.</p>
    </div>

    <!-- Modo INDIVIDUAL: override de comportamento por cliente -->
    <template v-else>
      <div class="calendar-config__block">
        <span class="calendar-config__field-label">Cliente</span>
        <div v-if="clients.length" class="calendar-profile__clients">
          <button
            v-for="client in clients"
            :key="client.id"
            type="button"
            class="calendar-profile__client"
            :class="{ 'is-active': client.id === selectedClientId }"
            @click="selectClient(client.id)"
          >
            <span class="calendar-profile__client-name">{{ client.name }}</span>
          </button>
        </div>
        <p v-else class="calendar-config__empty">Nenhum cliente ativo.</p>
      </div>

      <p v-if="!selectedClientId" class="calendar-config__empty">
        Selecione um cliente para definir um comportamento próprio.
      </p>

      <template v-else>
        <p v-if="loading" class="calendar-config__hint">Carregando configuração do cliente…</p>

        <template v-else>
          <span v-if="usesGeneral" class="calendar-profile__badge is-empty">usa config geral</span>

          <div class="calendar-config__block">
            <span class="calendar-config__field-label">Status da IA para este cliente</span>
            <div class="calendar-config__seg" role="group" aria-label="Status da IA do cliente">
              <button
                type="button"
                class="calendar-config__seg-btn"
                :class="{ 'is-active': draft.enabled === null }"
                @click="setEnabled(null)"
              >
                Herdar
              </button>
              <button
                type="button"
                class="calendar-config__seg-btn"
                :class="{ 'is-active': draft.enabled === true }"
                @click="setEnabled(true)"
              >
                Ligada
              </button>
              <button
                type="button"
                class="calendar-config__seg-btn"
                :class="{ 'is-active': draft.enabled === false }"
                @click="setEnabled(false)"
              >
                Desligada
              </button>
            </div>
            <span class="calendar-config__hint">Herdar = segue o status geral da IA.</span>
          </div>

          <div class="calendar-config__grid2">
            <label class="calendar-config__field">
              <span class="calendar-config__field-label">Provedor</span>
              <select
                class="calendar-config__input"
                :value="draft.provider"
                @change="setProvider(($event.target as HTMLSelectElement).value)"
              >
                <option v-for="opt in providerOptions" :key="opt.value" :value="opt.value">
                  {{ opt.label }}
                </option>
              </select>
            </label>

            <label class="calendar-config__field">
              <span class="calendar-config__field-label">Modelo</span>
              <input
                class="calendar-config__input"
                :value="draft.model"
                placeholder="Vazio = usa o modelo geral"
                @input="setModel(($event.target as HTMLInputElement).value)"
              />
            </label>

            <label class="calendar-config__field calendar-config__field--full">
              <span class="calendar-config__field-label">Base URL (opcional)</span>
              <input
                class="calendar-config__input"
                :value="draft.baseUrl"
                placeholder="Vazio = usa a base URL geral"
                @input="setBaseUrl(($event.target as HTMLInputElement).value)"
              />
            </label>

            <label class="calendar-config__field">
              <span class="calendar-config__field-label">Temperatura (0 a 1)</span>
              <input
                class="calendar-config__input"
                type="number"
                min="0"
                max="1"
                step="0.1"
                placeholder="Vazio = herda"
                :value="draft.temperature === null ? '' : draft.temperature"
                @input="setTemperature(($event.target as HTMLInputElement).value)"
              />
            </label>
          </div>

          <label class="calendar-config__field">
            <span class="calendar-config__field-label">Prompt do sistema deste cliente</span>
            <textarea
              class="calendar-config__input calendar-config__textarea"
              :value="draft.systemPrompt"
              placeholder="Vazio = usa o prompt geral. Preenchido = comanda a IA só deste cliente."
              @input="setSystemPrompt(($event.target as HTMLTextAreaElement).value)"
            ></textarea>
          </label>

          <div class="calendar-config__section-actions">
            <span v-if="touched" class="calendar-config-page__dirty">Alterações não salvas</span>
            <AppPanelButton variant="ghost" :disabled="saving" @click="save">
              Salvar comportamento de {{ selectedClientName || 'cliente' }}
            </AppPanelButton>
          </div>
        </template>
      </template>
    </template>
  </div>
</template>
