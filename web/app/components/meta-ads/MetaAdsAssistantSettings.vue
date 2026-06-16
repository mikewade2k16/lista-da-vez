<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useMetaAdsStore } from '~/stores/meta-ads'

const store = useMetaAdsStore()

interface ModelOption {
  value: string
  label: string
}

const MODEL_OPTIONS: ModelOption[] = [
  { value: '', label: 'Padrao da assinatura' },
  { value: 'claude-haiku-4-5', label: 'Haiku - mais rapido' },
  { value: 'claude-sonnet-4-6', label: 'Sonnet - equilibrio' },
  { value: 'claude-opus-4-8', label: 'Opus - maxima qualidade' },
]

const model = ref('')
const systemPrompt = ref('')
const open = ref(false)

// Sincroniza os campos locais quando as configuracoes carregam/salvam.
watch(
  () => store.assistantSettings,
  (settings) => {
    if (settings) {
      model.value = settings.model
      systemPrompt.value = settings.systemPrompt
    }
  },
  { immediate: true },
)

onMounted(() => {
  void store.loadAssistantSettings()
})

async function onSave() {
  await store.saveAssistantSettings(model.value, systemPrompt.value)
}
</script>

<template>
  <section class="ma-settings">
    <header class="ma-settings__head">
      <div>
        <h3 class="ma-settings__title">Configuracoes do assistente</h3>
        <p class="ma-settings__subtitle">Escolha o modelo e edite o comportamento (prompt).</p>
      </div>
      <button type="button" class="ma-settings__toggle" :aria-expanded="open" @click="open = !open">
        {{ open ? 'Fechar' : 'Editar' }}
      </button>
    </header>

    <div v-if="open" class="ma-settings__body">
      <label class="ma-settings__field">
        <span class="ma-settings__label">Modelo</span>
        <select
          v-model="model"
          class="ma-settings__select"
          :disabled="store.assistantSettingsSaving"
        >
          <option v-for="opt in MODEL_OPTIONS" :key="opt.value" :value="opt.value">
            {{ opt.label }}
          </option>
        </select>
      </label>

      <label class="ma-settings__field">
        <span class="ma-settings__label">Comportamento (system prompt)</span>
        <textarea
          v-model="systemPrompt"
          class="ma-settings__textarea"
          rows="12"
          :disabled="store.assistantSettingsSaving"
          placeholder="Instrucoes que definem como o assistente responde..."
        ></textarea>
        <span class="ma-settings__hint">
          Voce controla o prompt inteiro. Se remover as regras de seguranca (confirmar antes de
          escrever, nunca inventar dado, usar a conta do contexto), o assistente pode se comportar
          de forma inesperada. Trocar o modelo ou o prompt reinicia a sessao do assistente — a
          conexao com a Meta e mantida automaticamente.
        </span>
      </label>

      <p v-if="store.assistantSettingsError" class="ma-settings__error">
        {{ store.assistantSettingsError }}
      </p>

      <div class="ma-settings__actions">
        <button
          type="button"
          class="ma-settings__save"
          :disabled="store.assistantSettingsSaving"
          @click="onSave"
        >
          {{ store.assistantSettingsSaving ? 'Salvando...' : 'Salvar' }}
        </button>
      </div>
    </div>
  </section>
</template>

<style scoped>
.ma-settings {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  padding: 1.25rem 1.4rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.7);
  box-shadow: var(--shadow-card);
}

.ma-settings__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.ma-settings__title {
  font-size: 1.05rem;
  font-weight: 700;
}

.ma-settings__subtitle {
  font-size: 0.85rem;
  color: var(--text-muted);
  margin-top: 0.2rem;
}

.ma-settings__toggle {
  font: inherit;
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--text-muted);
  background: transparent;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  padding: 0.35rem 0.85rem;
  cursor: pointer;
  flex-shrink: 0;
}

.ma-settings__toggle:hover {
  color: var(--text-main);
  border-color: var(--line-strong);
}

.ma-settings__body {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.ma-settings__field {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.ma-settings__label {
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.ma-settings__select {
  padding: 0.55rem 0.85rem;
  border-radius: 0.55rem;
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.5);
  color: var(--text-main);
  font: inherit;
  font-size: 0.88rem;
  cursor: pointer;
  max-width: 22rem;
}

.ma-settings__textarea {
  padding: 0.7rem 0.85rem;
  border-radius: 0.6rem;
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.5);
  color: var(--text-main);
  font: inherit;
  font-size: 0.85rem;
  line-height: 1.5;
  resize: vertical;
  min-height: 12rem;
}

.ma-settings__select:focus,
.ma-settings__textarea:focus {
  outline: none;
  border-color: rgb(var(--ring) / 0.5);
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.16);
}

.ma-settings__hint {
  font-size: 0.78rem;
  color: var(--text-muted);
  line-height: 1.5;
}

.ma-settings__error {
  font-size: 0.85rem;
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.16);
  padding: 0.55rem 0.8rem;
  border-radius: var(--radius-soft);
}

.ma-settings__actions {
  display: flex;
  justify-content: flex-end;
}

.ma-settings__save {
  font: inherit;
  font-size: 0.9rem;
  font-weight: 600;
  padding: 0.6rem 1.4rem;
  border-radius: 0.55rem;
  cursor: pointer;
  border: 1px solid transparent;
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  color: rgb(255 255 255);
}

.ma-settings__save:disabled {
  opacity: 0.6;
  cursor: progress;
}
</style>
