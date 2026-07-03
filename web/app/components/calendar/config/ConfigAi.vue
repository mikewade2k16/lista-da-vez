<script setup lang="ts">
import { computed } from 'vue'
import {
  AI_PROVIDER_BASE_URL,
  AI_PROVIDER_LABEL,
  AI_PROVIDERS,
  type CalendarAiConfig,
  type CalendarAiProvider,
} from '~/utils/calendar'

// Secao IA (SPEC-F3): provider + modelo + baseUrl (placeholder do default por
// provider) + systemPrompt + temperature. AVISO fixo: as chaves de API vivem no
// n8n (credentials), NUNCA aqui. Contrato C2.
const props = defineProps<{ modelValue: CalendarAiConfig }>()
const emit = defineEmits<{ 'update:modelValue': [value: CalendarAiConfig] }>()

const ai = computed(() => props.modelValue)

const providerOptions = AI_PROVIDERS.map((value) => ({ value, label: AI_PROVIDER_LABEL[value] }))

// Placeholder da baseUrl = default do provider (vazio no config = usar esse default).
const baseUrlPlaceholder = computed(
  () => AI_PROVIDER_BASE_URL[ai.value.provider] || 'https://... (endpoint do provider)',
)

function patch(next: Partial<CalendarAiConfig>): void {
  emit('update:modelValue', { ...ai.value, ...next })
}

function setProvider(value: string): void {
  patch({ provider: value as CalendarAiProvider })
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
    <h3 class="calendar-config__section-title">Assistente de IA (plano do mês)</h3>

    <p class="calendar-config__warn">
      <UIcon name="i-lucide-shield-alert" aria-hidden="true" />
      As chaves de API ficam no n8n (credentials), nunca aqui.
    </p>

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

      <label class="calendar-config__field">
        <span class="calendar-config__field-label">Modelo</span>
        <input
          class="calendar-config__input"
          :value="ai.model"
          placeholder="ex.: claude-sonnet-5"
          @input="patch({ model: ($event.target as HTMLInputElement).value })"
        />
      </label>

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

      <label class="calendar-config__field calendar-config__field--full">
        <span class="calendar-config__field-label">Prompt do sistema (opcional)</span>
        <textarea
          class="calendar-config__input calendar-config__textarea"
          :value="ai.systemPrompt"
          placeholder="Vazio = prompt padrão do workflow (estratégia de conteúdo pt-BR)."
          @input="patch({ systemPrompt: ($event.target as HTMLTextAreaElement).value })"
        ></textarea>
      </label>
    </div>
  </section>
</template>
