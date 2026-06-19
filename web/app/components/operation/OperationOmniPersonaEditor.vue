<script setup lang="ts">
// Editor inline da PERSONA (prompt do sistema) do Omni Chat. Componente atomico
// usado dentro do bloco "Omni Chat" de OperationSidePanel.vue. So e renderizado
// para admin da plataforma (gating no pai). Le/grava o systemPrompt efetivo no
// banco via useOmniChatPersona (GET/PUT /v1/omni-chat/persona).
import { onMounted } from 'vue'
import {
  HISTORY_WINDOW_MAX,
  HISTORY_WINDOW_MIN,
  PERSONA_MAX_LENGTH,
  useOmniChatPersona,
} from '~/composables/useOmniChatPersona'

const emit = defineEmits<{ (event: 'close'): void }>()

const persona = useOmniChatPersona()

// Ao montar (editor recem-aberto), carrega o prompt vigente do banco e preenche
// o textarea. fetchPersona ja registra a mensagem de erro; o editor segue aberto
// para o admin tentar de novo.
onMounted(() => {
  void persona.fetchPersona().catch(() => {})
})

async function save() {
  await persona.savePersona(persona.draft.value)
}

function cancel() {
  persona.resetFeedback()
  emit('close')
}
</script>

<template>
  <div class="operation-persona">
    <p class="operation-persona__label">Persona do Omni (prompt do sistema)</p>
    <p v-if="persona.isDefault.value" class="operation-persona__note">
      Usando o texto padrão. Edite e salve para personalizar.
    </p>

    <p v-if="persona.loading.value" class="operation-persona__status">Carregando…</p>
    <textarea
      v-else
      v-model="persona.draft.value"
      class="operation-persona__field"
      rows="8"
      :maxlength="PERSONA_MAX_LENGTH"
      :disabled="persona.saving.value"
      placeholder="Descreva como o Omni deve se comportar, o tom e o que ele sabe…"
    ></textarea>

    <div v-if="!persona.loading.value" class="operation-persona__memory">
      <label class="operation-persona__memory-label" for="omni-history-window">
        Memória: últimas
        <input
          id="omni-history-window"
          v-model.number="persona.historyWindow.value"
          class="operation-persona__memory-input"
          type="number"
          :min="HISTORY_WINDOW_MIN"
          :max="HISTORY_WINDOW_MAX"
          :disabled="persona.saving.value"
        />
        interações (pergunta + resposta)
      </label>
      <p class="operation-persona__note">Quanto da conversa o Omni leva em conta para responder.</p>
    </div>

    <p v-if="persona.errorMessage.value" class="operation-persona__error" role="alert">
      {{ persona.errorMessage.value }}
    </p>
    <p
      v-else-if="persona.successMessage.value"
      class="operation-persona__success"
      aria-live="polite"
    >
      {{ persona.successMessage.value }}
    </p>

    <div class="operation-persona__actions">
      <button
        type="button"
        class="operation-persona__btn operation-persona__btn--ghost"
        :disabled="persona.saving.value"
        @click="cancel()"
      >
        Cancelar
      </button>
      <button
        type="button"
        class="operation-persona__btn operation-persona__btn--primary"
        :disabled="persona.saving.value || persona.loading.value || !persona.draft.value.trim()"
        @click="save()"
      >
        {{ persona.saving.value ? 'Salvando…' : 'Salvar' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.operation-persona {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border-radius: 12px;
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.35);
}

.operation-persona__label {
  margin: 0;
  font-size: 0.78rem;
  font-weight: 700;
  color: var(--text-main);
}

.operation-persona__note {
  margin: 0;
  font-size: 0.72rem;
  color: rgb(var(--muted));
}

.operation-persona__status {
  margin: 0;
  font-size: 0.76rem;
  font-style: italic;
  color: rgb(var(--muted));
}

.operation-persona__field {
  width: 100%;
  min-height: 0;
  resize: vertical;
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.4);
  color: var(--text-main);
  font-size: 0.8rem;
  line-height: 1.45;
  font-family: inherit;
}

.operation-persona__field:disabled {
  cursor: not-allowed;
  opacity: 0.7;
}

.operation-persona__memory {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.operation-persona__memory-label {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  font-size: 0.76rem;
  color: var(--text-main);
}

.operation-persona__memory-input {
  width: 56px;
  padding: 4px 8px;
  border-radius: 8px;
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.4);
  color: var(--text-main);
  font-size: 0.8rem;
  font-family: inherit;
  text-align: center;
}

.operation-persona__memory-input:disabled {
  cursor: not-allowed;
  opacity: 0.7;
}

.operation-persona__error {
  margin: 0;
  font-size: 0.74rem;
  color: rgb(var(--danger));
}

.operation-persona__success {
  margin: 0;
  font-size: 0.74rem;
  font-weight: 600;
  color: rgb(var(--success));
}

.operation-persona__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.operation-persona__btn {
  padding: 0.4rem 0.85rem;
  border-radius: 9px;
  font-size: 0.78rem;
  font-weight: 600;
  cursor: pointer;
  transition:
    opacity 0.15s ease,
    border-color 0.15s ease,
    background 0.15s ease;
}

.operation-persona__btn:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.operation-persona__btn--ghost {
  color: rgb(var(--muted));
  border: 1px solid var(--line-soft);
  background: transparent;
}

.operation-persona__btn--ghost:hover:not(:disabled) {
  border-color: rgb(var(--ring) / 0.42);
  color: var(--text-main);
}

.operation-persona__btn--primary {
  color: rgb(var(--primary));
  border: 1px solid rgb(var(--ring) / 0.42);
  background: rgb(var(--primary) / 0.16);
}

.operation-persona__btn--primary:hover:not(:disabled) {
  background: rgb(var(--primary) / 0.24);
}
</style>
