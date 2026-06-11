<script setup lang="ts">
import { useAutomation } from '~/composables/useAutomation'

const {
  qr,
  loading,
  connecting,
  savingEnabled,
  errorMessage,
  connected,
  whatsappStatus,
  connectedPhone,
  enabled,
  load,
  connect,
  disconnect,
  setEnabled,
  personaName,
  personaPrompt,
  personaLoading,
  savingPersona,
  personaSavedAt,
  loadPersona,
  savePersona,
} = useAutomation()

const personaSaved = computed(() => personaSavedAt.value > 0)

const ctxPreview = ref<{ refresh: () => void } | null>(null)

const statusLabel = computed(() => {
  switch (whatsappStatus.value) {
    case 'WORKING':
      return 'Conectado'
    case 'SCAN_QR_CODE':
      return 'Escaneie o QR'
    case 'STARTING':
      return 'Iniciando...'
    case 'FAILED':
      return 'Falhou'
    default:
      return 'Desconectado'
  }
})

const statusModifier = computed(() => {
  if (connected.value) return 'is-connected'
  if (whatsappStatus.value === 'SCAN_QR_CODE' || whatsappStatus.value === 'STARTING') {
    return 'is-pending'
  }
  return 'is-off'
})

function onToggleEnabled() {
  if (savingEnabled.value) return
  void setEnabled(!enabled.value)
}

onMounted(() => {
  void load()
  void loadPersona()
})
</script>

<template>
  <section class="automation-workspace">
    <header class="automation-workspace__header">
      <div class="automation-workspace__title">
        <h1>Automacao</h1>
        <span class="automation-workspace__badge">BETA</span>
      </div>
      <p class="automation-workspace__subtitle">
        Assistente de WhatsApp/IA. Conecte o numero e ligue/desligue o robo.
      </p>
    </header>

    <p v-if="errorMessage" class="automation-workspace__error">{{ errorMessage }}</p>

    <div class="automation-workspace__grid">
      <!-- Conexao do WhatsApp -->
      <article class="automation-card">
        <header class="automation-card__head">
          <h2 class="automation-card__title">WhatsApp</h2>
          <span
            class="automation-card__status"
            :class="`automation-card__status--${statusModifier}`"
          >
            {{ statusLabel }}
          </span>
        </header>

        <div class="automation-card__body">
          <p v-if="connected" class="automation-card__info">
            Conectado{{ connectedPhone ? ` como +${connectedPhone}` : '' }}.
          </p>

          <div v-else-if="qr" class="automation-card__qr">
            <img :src="qr" alt="QR code para conectar o WhatsApp" width="240" height="240" />
            <p class="automation-card__hint">Abra o WhatsApp no celular e escaneie o codigo.</p>
          </div>

          <p v-else class="automation-card__info automation-card__info--muted">
            Nenhum numero conectado. Use um numero dedicado ao robo, nao o seu pessoal.
          </p>
        </div>

        <footer class="automation-card__foot">
          <button
            v-if="connected"
            type="button"
            class="automation-btn automation-btn--ghost"
            @click="disconnect"
          >
            Desconectar
          </button>
          <button
            v-else
            type="button"
            class="automation-btn automation-btn--primary"
            :disabled="connecting"
            @click="connect"
          >
            {{ connecting ? 'Conectando...' : 'Conectar WhatsApp' }}
          </button>
        </footer>
      </article>

      <!-- Liga/desliga do robo -->
      <article class="automation-card">
        <header class="automation-card__head">
          <h2 class="automation-card__title">Robo</h2>
          <span
            class="automation-card__status"
            :class="
              enabled ? 'automation-card__status--is-connected' : 'automation-card__status--is-off'
            "
          >
            {{ enabled ? 'Ligado' : 'Desligado' }}
          </span>
        </header>

        <div class="automation-card__body">
          <button
            type="button"
            class="automation-switch"
            :class="{ 'automation-switch--on': enabled }"
            :disabled="savingEnabled || loading"
            role="switch"
            :aria-checked="enabled"
            @click="onToggleEnabled"
          >
            <span class="automation-switch__track">
              <span class="automation-switch__thumb"></span>
            </span>
            <span class="automation-switch__label">{{ enabled ? 'Ligado' : 'Desligado' }}</span>
          </button>

          <p class="automation-card__hint">
            Ligado, o robo responde as mensagens recebidas. Desligado, ele ignora tudo.
          </p>
        </div>
      </article>
    </div>

    <!-- Comportamento (persona) -->
    <article class="automation-card">
      <header class="automation-card__head">
        <h2 class="automation-card__title">Comportamento (persona)</h2>
        <span
          v-if="personaSaved"
          class="automation-card__status automation-card__status--is-connected"
        >
          Salvo
        </span>
      </header>

      <div class="automation-card__body automation-card__body--stretch">
        <label class="automation-field">
          <span class="automation-field__label">Nome</span>
          <input
            v-model="personaName"
            type="text"
            class="automation-field__input"
            :disabled="personaLoading"
          />
        </label>

        <label class="automation-field">
          <span class="automation-field__label">Instrucoes (comportamento)</span>
          <textarea
            v-model="personaPrompt"
            class="automation-field__textarea"
            rows="18"
            spellcheck="false"
            :disabled="personaLoading"
          ></textarea>
        </label>

        <p class="automation-card__hint">
          Escreva o comportamento, tom e personalidade do assistente. Conhecimento (catalogo,
          precos, FAQs) vai nos documentos abaixo. Guardrails de WhatsApp sao aplicados
          automaticamente no runtime.
        </p>
      </div>

      <footer class="automation-card__foot">
        <button
          type="button"
          class="automation-btn automation-btn--primary"
          :disabled="savingPersona || personaLoading"
          @click="savePersona"
        >
          {{ savingPersona ? 'Salvando...' : 'Salvar comportamento' }}
        </button>
      </footer>
    </article>

    <!-- Conhecimento por documento (M3+) -->
    <AutomationKnowledgeCard @change="ctxPreview?.refresh()" />

    <!-- Contexto do bot — fluxo visual + preview do systemMessage -->
    <AutomationContextPreview ref="ctxPreview" />
  </section>
</template>

<style scoped>
.automation-workspace {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  padding: 1.5rem;
  /* rolagem padrao das paginas: ocupa a altura e scrolla o proprio conteudo
     (o layout module-workspace-full e overflow:hidden). */
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.automation-workspace__title {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.automation-workspace__title h1 {
  font-size: 1.5rem;
  font-weight: 600;
}

.automation-workspace__badge {
  font-size: 0.65rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  padding: 0.1rem 0.4rem;
  border-radius: 0.35rem;
  background: color-mix(in srgb, var(--accent-warning) 22%, transparent);
  color: var(--accent-warning);
}

.automation-workspace__subtitle {
  color: var(--text-muted);
  font-size: 0.9rem;
}

.automation-workspace__error {
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.16);
  padding: 0.6rem 0.85rem;
  border-radius: 0.5rem;
  font-size: 0.9rem;
}

.automation-workspace__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 1rem;
}

.automation-card {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.25rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.9);
  box-shadow: var(--shadow-card);
}

.automation-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.automation-card__title {
  font-size: 1.05rem;
  font-weight: 600;
}

.automation-card__status {
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.15rem 0.5rem;
  border-radius: 999px;
}

.automation-card__status--is-connected {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.automation-card__status--is-pending {
  background: color-mix(in srgb, var(--accent-warning) 18%, transparent);
  color: var(--accent-warning);
}

.automation-card__status--is-off {
  background: rgb(var(--border) / 0.44);
  color: rgb(var(--muted));
}

.automation-card__body {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  align-items: flex-start;
}

.automation-card__info {
  font-size: 0.9rem;
}

.automation-card__info--muted {
  color: var(--text-muted);
}

.automation-card__qr {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  align-items: center;
}

.automation-card__qr img {
  border-radius: 0.5rem;
  border: 1px solid var(--line-soft);
}

.automation-card__hint {
  font-size: 0.8rem;
  color: var(--text-muted);
}

.automation-card__body--stretch {
  align-items: stretch;
}

.automation-field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  width: 100%;
}

.automation-field__label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-muted);
}

.automation-field__input,
.automation-field__textarea {
  width: 100%;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.5rem;
  background: rgb(var(--surface-2) / 0.76);
  color: var(--text-main);
  font: inherit;
}

.automation-field__input:focus,
.automation-field__textarea:focus {
  outline: none;
  border-color: rgb(var(--ring) / 0.5);
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.16);
}

.automation-field__textarea {
  resize: vertical;
  min-height: 220px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.85rem;
  line-height: 1.5;
  white-space: pre-wrap;
}

.automation-btn {
  font-size: 0.9rem;
  font-weight: 600;
  padding: 0.5rem 1rem;
  border-radius: 0.5rem;
  cursor: pointer;
  border: 1px solid transparent;
}

.automation-btn:disabled {
  opacity: 0.6;
  cursor: progress;
}

.automation-btn--primary {
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  color: rgb(255 255 255);
}

.automation-btn--ghost {
  background: transparent;
  border-color: var(--line-soft);
  color: var(--text-main);
}

.automation-switch {
  display: inline-flex;
  align-items: center;
  gap: 0.6rem;
  background: transparent;
  border: none;
  cursor: pointer;
  padding: 0;
}

.automation-switch:disabled {
  opacity: 0.6;
  cursor: progress;
}

.automation-switch__track {
  width: 42px;
  height: 24px;
  border-radius: 999px;
  background: rgb(var(--border));
  position: relative;
  transition: background 0.15s ease;
}

.automation-switch--on .automation-switch__track {
  background: rgb(var(--primary));
}

.automation-switch__thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: rgb(255 255 255);
  transition: transform 0.15s ease;
}

.automation-switch--on .automation-switch__thumb {
  transform: translateX(18px);
}

.automation-switch__label {
  font-size: 0.9rem;
  font-weight: 600;
}
</style>
