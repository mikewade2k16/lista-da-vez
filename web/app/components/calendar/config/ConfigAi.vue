<script setup lang="ts">
import { computed } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import ConfigAiKeys from '~/components/calendar/config/ConfigAiKeys.vue'
import ConfigAiModelSelect from '~/components/calendar/config/ConfigAiModelSelect.vue'
import ConfigAiClientScope from '~/components/calendar/config/ConfigAiClientScope.vue'
import { useCalendarChat } from '~/composables/useCalendarChat'
import { useCalendarStore } from '~/stores/calendar'
import {
  AI_PROVIDER_BASE_URL,
  AI_PROVIDER_LABEL,
  AI_PROVIDERS,
  type CalendarAiConfig,
  type CalendarAiProvider,
  type CalendarTranscribeProvider,
} from '~/utils/calendar'

// Aba IA (SPEC-F1): o PAINEL e a fonte da verdade da IA do calendario. Kill switch
// (enabled), escopo das chaves (globais x da conta), chaves MASCARADAS por provider
// (subcomponente ConfigAiKeys), provider+modelo, transcricao e o prompt do sistema (a
// lei da IA). As chaves de API NUNCA aparecem cruas no front — so status {set,last4}.
const props = defineProps<{ modelValue: CalendarAiConfig }>()
const emit = defineEmits<{ 'update:modelValue': [value: CalendarAiConfig] }>()

// Chat flutuante: estado singleton. Abrir daqui reusa a MESMA conversa do calendario.
const chat = useCalendarChat()

// Escopo SALVO (banco) das chaves: dirige a fonte ativa que o ConfigAiKeys mostra. O
// rascunho (ai.useGlobalKeys) so vira fonte ativa apos salvar as configuracoes.
const store = useCalendarStore()
const savedUseGlobalKeys = computed(() => store.config.ai.useGlobalKeys)

const ai = computed(() => props.modelValue)

const providerOptions = AI_PROVIDERS.map((value) => ({ value, label: AI_PROVIDER_LABEL[value] }))

const TRANSCRIBE_OPTIONS: { value: CalendarTranscribeProvider; label: string }[] = [
  { value: 'local', label: 'Whisper (self-hosted, grátis)' },
  { value: 'openai', label: 'OpenAI (Whisper)' },
  { value: 'gemini', label: 'Gemini (não aceita áudio do navegador)' },
]

// Placeholder da baseUrl = default do provider (vazio no config = usar esse default).
const baseUrlPlaceholder = computed(
  () => AI_PROVIDER_BASE_URL[ai.value.provider] || 'https://... (endpoint do provider)',
)

const TRANSCRIBE_MODEL_PLACEHOLDER: Record<CalendarTranscribeProvider, string> = {
  local: 'Systran/faster-whisper-base',
  openai: 'whisper-1',
  gemini: 'gemini-2.5-flash',
}
const transcribeModelPlaceholder = computed(
  () => TRANSCRIBE_MODEL_PLACEHOLDER[ai.value.transcribeProvider] || 'Systran/faster-whisper-base',
)

// Modelos do Whisper self-hosted, selecionaveis no painel (value = id do faster-whisper).
// Maior = mais preciso e mais lento; o whisper baixa o modelo na 1a vez e mantem em cache.
const WHISPER_LOCAL_MODELS: { value: string; label: string }[] = [
  { value: 'Systran/faster-whisper-tiny', label: 'Tiny — mais rápido, menos preciso' },
  { value: 'Systran/faster-whisper-base', label: 'Base — equilíbrio (padrão)' },
  { value: 'Systran/faster-whisper-small', label: 'Small — mais preciso' },
  { value: 'Systran/faster-whisper-medium', label: 'Medium — máxima qualidade, mais lento' },
]
// Vazio na config = base (default do n8n/container).
const localModel = computed(() => ai.value.transcribeModel || 'Systran/faster-whisper-base')

// Aviso honesto (principio 5): a troca de escopo so vale apos salvar as configuracoes.
const scopePending = computed(() => ai.value.useGlobalKeys !== savedUseGlobalKeys.value)

function patch(next: Partial<CalendarAiConfig>): void {
  emit('update:modelValue', { ...ai.value, ...next })
}

// Escopo por cliente (SPEC-F3): o subcomponente muda scopeMode/disabledClientIds no
// proprio `ai` e emite o objeto completo; so repassamos pro rascunho compartilhado.
function onScope(value: CalendarAiConfig): void {
  emit('update:modelValue', value)
}

function setProvider(value: string): void {
  patch({ provider: value as CalendarAiProvider })
}

function setTranscribeProvider(value: string): void {
  // Aceita os 3 providers validos (local | openai | gemini); fora disso, default gemini.
  const valid: CalendarTranscribeProvider[] = ['local', 'openai', 'gemini']
  const next = valid.includes(value as CalendarTranscribeProvider)
    ? (value as CalendarTranscribeProvider)
    : 'gemini'
  patch({ transcribeProvider: next })
}

function setTemperature(value: string): void {
  const num = Number(value)
  patch({
    temperature: Number.isFinite(num) ? Math.min(1, Math.max(0, num)) : ai.value.temperature,
  })
}
</script>

<template>
  <section class="calendar-config__section">
    <h3 class="calendar-config__section-title">Assistente de IA do calendário</h3>

    <!-- Kill switch: desligada, chat/plano/transcricao respondem "IA desligada". -->
    <div class="calendar-config__block">
      <span class="calendar-config__label">Status da IA</span>
      <div class="calendar-config__seg" role="group" aria-label="Ligar ou desligar a IA">
        <button
          type="button"
          class="calendar-config__seg-btn"
          :class="{ 'is-active': ai.enabled }"
          @click="patch({ enabled: true })"
        >
          Ligada
        </button>
        <button
          type="button"
          class="calendar-config__seg-btn"
          :class="{ 'is-active': !ai.enabled }"
          @click="patch({ enabled: false })"
        >
          Desligada
        </button>
      </div>
      <span class="calendar-config__hint">
        Desligada, o assistente, o plano do mês e a transcrição respondem que a IA está desligada,
        sem chamar nenhum provedor.
      </span>
    </div>

    <!-- Escopo das chaves + chaves mascaradas por provider. Colapsavel: abre so o que
         vai mexer (aberto por padrao por ser o setup mais comum). -->
    <details class="calendar-config__collapse" open>
      <summary class="calendar-config__collapse-head">Chaves de API</summary>
      <div class="calendar-config__collapse-body">
        <div class="calendar-config__seg" role="group" aria-label="Escopo das chaves de API">
          <button
            type="button"
            class="calendar-config__seg-btn"
            :class="{ 'is-active': ai.useGlobalKeys }"
            @click="patch({ useGlobalKeys: true })"
          >
            Globais da plataforma
          </button>
          <button
            type="button"
            class="calendar-config__seg-btn"
            :class="{ 'is-active': !ai.useGlobalKeys }"
            @click="patch({ useGlobalKeys: false })"
          >
            Desta conta
          </button>
        </div>
        <span v-if="scopePending" class="calendar-config__hint">
          Salve as configurações para aplicar o novo escopo das chaves.
        </span>
        <ConfigAiKeys :use-global-keys="savedUseGlobalKeys" />
      </div>
    </details>

    <details class="calendar-config__collapse">
      <summary class="calendar-config__collapse-head">Provedor e modelo</summary>
      <div class="calendar-config__collapse-body">
        <div class="calendar-config__grid2">
          <label class="calendar-config__field">
            <span class="calendar-config__field-label">Provedor</span>
            <select
              class="calendar-config__input"
              :value="ai.provider"
              @change="setProvider(($event.target as HTMLSelectElement).value)"
            >
              <option v-for="opt in providerOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </option>
            </select>
          </label>

          <ConfigAiModelSelect
            :provider="ai.provider"
            :model-value="ai.model"
            @update:model-value="patch({ model: $event })"
          />

          <label class="calendar-config__field calendar-config__field--full">
            <span class="calendar-config__field-label">Base URL (opcional)</span>
            <input
              class="calendar-config__input"
              :value="ai.baseUrl"
              :placeholder="baseUrlPlaceholder"
              @input="patch({ baseUrl: ($event.target as HTMLInputElement).value })"
            />
            <span class="calendar-config__hint">Vazio = usa o endpoint padrão do provedor.</span>
          </label>

          <label class="calendar-config__field">
            <span class="calendar-config__field-label">Temperatura (0 a 1)</span>
            <input
              class="calendar-config__input"
              type="number"
              min="0"
              max="1"
              step="0.1"
              :value="ai.temperature"
              @input="setTemperature(($event.target as HTMLInputElement).value)"
            />
          </label>
        </div>
      </div>
    </details>

    <details class="calendar-config__collapse">
      <summary class="calendar-config__collapse-head">Transcrição de voz</summary>
      <div class="calendar-config__collapse-body">
        <div class="calendar-config__grid2">
          <label class="calendar-config__field">
            <span class="calendar-config__field-label">Transcrição — provedor</span>
            <select
              class="calendar-config__input"
              :value="ai.transcribeProvider"
              @change="setTranscribeProvider(($event.target as HTMLSelectElement).value)"
            >
              <option v-for="opt in TRANSCRIBE_OPTIONS" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </option>
            </select>
          </label>

          <label class="calendar-config__field">
            <span class="calendar-config__field-label">Transcrição — modelo</span>
            <select
              v-if="ai.transcribeProvider === 'local'"
              class="calendar-config__input"
              :value="localModel"
              @change="patch({ transcribeModel: ($event.target as HTMLSelectElement).value })"
            >
              <option v-for="opt in WHISPER_LOCAL_MODELS" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </option>
            </select>
            <input
              v-else
              class="calendar-config__input"
              :value="ai.transcribeModel"
              :placeholder="transcribeModelPlaceholder"
              @input="patch({ transcribeModel: ($event.target as HTMLInputElement).value })"
            />
            <span v-if="ai.transcribeProvider === 'local'" class="calendar-config__hint">
              O modelo baixa na 1ª vez que for usado (pode demorar); depois fica em cache.
            </span>
          </label>
        </div>
      </div>
    </details>

    <details class="calendar-config__collapse">
      <summary class="calendar-config__collapse-head">Prompt do sistema (a lei da IA)</summary>
      <div class="calendar-config__collapse-body">
        <label class="calendar-config__field">
          <textarea
            class="calendar-config__input calendar-config__textarea calendar-config__textarea--tall"
            :value="ai.systemPrompt"
            placeholder="Defina o comportamento do assistente: tom, foco, regras. Vazio = prompt padrão do workflow."
            @input="patch({ systemPrompt: ($event.target as HTMLTextAreaElement).value })"
          ></textarea>
          <span class="calendar-config__hint">
            Este texto comanda o assistente, o plano do mês e as respostas. É a instrução principal.
          </span>
        </label>
      </div>
    </details>

    <details class="calendar-config__collapse">
      <summary class="calendar-config__collapse-head">Escopo por cliente</summary>
      <div class="calendar-config__collapse-body">
        <ConfigAiClientScope :model-value="ai" @update:model-value="onScope" />
      </div>
    </details>

    <div class="calendar-config__section-actions">
      <AppPanelButton variant="secondary" @click="chat.openPanel()">
        <UIcon name="i-lucide-sparkles" aria-hidden="true" />
        Abrir chat com o assistente
      </AppPanelButton>
    </div>
  </section>
</template>
