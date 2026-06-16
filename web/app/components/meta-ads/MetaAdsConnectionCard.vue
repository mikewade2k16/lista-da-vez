<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMetaAdsStore } from '~/stores/meta-ads'

const store = useMetaAdsStore()

const token = ref('')

const canConnect = computed(() => token.value.trim().length > 0 && !store.connecting)

async function onConnect() {
  if (!canConnect.value) return
  await store.saveConnection(token.value.trim())
  // Nunca ecoar o token de volta: limpa o campo apos a tentativa.
  token.value = ''
}

async function onDisconnect() {
  await store.deleteConnection()
}

const expiresLabel = computed(() => {
  const raw = store.connection?.tokenExpiresAt
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleDateString('pt-BR')
})

// Status do assistente MCP (carregado pelo workspace via store.loadAssistant).
const assistantStatus = computed(() => {
  const health = store.assistantHealth
  if (!health) return { label: 'Verificando...', ok: false }
  if (health.ok) return { label: 'Assistente pronto', ok: true }
  return { label: health.detail.trim() || 'Assistente indisponivel', ok: false }
})
</script>

<template>
  <article class="ma-connection">
    <header class="ma-connection__head">
      <div class="ma-connection__head-text">
        <h2 class="ma-connection__title">Conexao com a Meta</h2>
        <p class="ma-connection__subtitle">
          System User token do Business Manager para puxar contas, campanhas e metricas.
        </p>
      </div>
      <span
        class="ma-connection__status"
        :class="store.connected ? 'ma-connection__status--on' : 'ma-connection__status--off'"
      >
        <span class="ma-connection__dot" aria-hidden="true"></span>
        {{ store.connected ? 'Conectado' : 'Desconectado' }}
      </span>
    </header>

    <hr class="ma-connection__divider" />

    <div v-if="store.connected" class="ma-connection__connected">
      <dl class="ma-connection__facts">
        <div class="ma-connection__fact">
          <dt class="ma-connection__fact-label">Negocio</dt>
          <dd class="ma-connection__fact-value">{{ store.connection?.name || '—' }}</dd>
        </div>
        <div class="ma-connection__fact">
          <dt class="ma-connection__fact-label">Business ID</dt>
          <dd class="ma-connection__fact-value">{{ store.connection?.metaBusinessId || '—' }}</dd>
        </div>
        <div v-if="expiresLabel" class="ma-connection__fact">
          <dt class="ma-connection__fact-label">Token expira em</dt>
          <dd class="ma-connection__fact-value">{{ expiresLabel }}</dd>
        </div>
        <div class="ma-connection__fact">
          <dt class="ma-connection__fact-label">Assistente</dt>
          <dd
            class="ma-connection__fact-value"
            :class="
              assistantStatus.ok
                ? 'ma-connection__fact-value--ok'
                : 'ma-connection__fact-value--muted'
            "
          >
            {{ assistantStatus.label }}
          </dd>
        </div>
      </dl>
      <button
        type="button"
        class="ma-connection__btn ma-connection__btn--ghost"
        @click="onDisconnect"
      >
        Desconectar
      </button>
    </div>

    <form v-else class="ma-connection__form" @submit.prevent="onConnect">
      <label class="ma-connection__field">
        <span class="ma-connection__label">System User token</span>
        <textarea
          v-model="token"
          class="ma-connection__textarea"
          rows="4"
          spellcheck="false"
          autocomplete="off"
          placeholder="Cole aqui o token de longa duracao do Business Manager"
          :disabled="store.connecting"
        ></textarea>
      </label>

      <p class="ma-connection__note">
        <span class="ma-connection__note-badge">Admin</span>
        O token e guardado cifrado no banco e nunca e exibido de volta. Trate como segredo.
      </p>

      <button
        type="submit"
        class="ma-connection__btn ma-connection__btn--primary"
        :disabled="token.trim().length === 0 || store.connecting"
      >
        <span v-if="store.connecting" class="ma-connection__spinner" aria-hidden="true"></span>
        {{ store.connecting ? 'Conectando...' : 'Conectar' }}
      </button>
    </form>
  </article>
</template>

<style scoped>
.ma-connection {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  padding: 1.6rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.7);
  box-shadow: var(--shadow-card);
}

.ma-connection__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.ma-connection__title {
  font-size: 1.3rem;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.ma-connection__subtitle {
  font-size: 0.88rem;
  color: var(--text-muted);
  margin-top: 0.25rem;
  max-width: 46ch;
}

.ma-connection__status {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  flex-shrink: 0;
  font-size: 0.8rem;
  font-weight: 600;
  padding: 0.35rem 0.75rem;
  border-radius: 999px;
  border: 1px solid var(--line-soft);
}

.ma-connection__status--on {
  color: rgb(var(--success));
  border-color: rgb(var(--success) / 0.4);
  background: rgb(var(--success) / 0.1);
}

.ma-connection__status--off {
  color: var(--text-muted);
  background: rgb(var(--surface-2) / 0.6);
}

.ma-connection__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: currentColor;
}

.ma-connection__divider {
  border: none;
  border-top: 1px solid var(--line-soft);
  margin: 0;
}

.ma-connection__connected {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1.25rem;
  flex-wrap: wrap;
}

.ma-connection__facts {
  display: flex;
  flex-wrap: wrap;
  gap: 1.75rem;
  margin: 0;
}

.ma-connection__fact {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.ma-connection__fact-label {
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.ma-connection__fact-value {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--text-main);
  margin: 0;
}

.ma-connection__fact-value--ok {
  color: rgb(var(--success));
}

.ma-connection__fact-value--muted {
  color: var(--text-muted);
  max-width: 36ch;
}

.ma-connection__form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.ma-connection__field {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.ma-connection__label {
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.ma-connection__textarea {
  width: 100%;
  padding: 0.7rem 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.6rem;
  background: rgb(var(--surface-2) / 0.5);
  color: var(--text-main);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.85rem;
  line-height: 1.5;
  resize: vertical;
  min-height: 96px;
}

.ma-connection__textarea:focus {
  outline: none;
  border-color: rgb(var(--ring) / 0.5);
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.16);
}

.ma-connection__textarea:disabled {
  opacity: 0.6;
}

.ma-connection__note {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.8rem;
  color: var(--text-muted);
  line-height: 1.5;
}

.ma-connection__note-badge {
  flex-shrink: 0;
  font-size: 0.62rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  padding: 0.1rem 0.4rem;
  border-radius: 0.3rem;
  background: rgb(var(--primary) / 0.15);
  color: rgb(var(--primary));
}

.ma-connection__btn {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  align-self: flex-start;
  font-size: 0.9rem;
  font-weight: 600;
  padding: 0.6rem 1.3rem;
  border-radius: 0.55rem;
  cursor: pointer;
  border: 1px solid transparent;
}

.ma-connection__btn:disabled {
  opacity: 0.6;
  cursor: progress;
}

.ma-connection__btn--primary {
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  color: rgb(255 255 255);
}

.ma-connection__btn--ghost {
  border-color: var(--line-soft);
  color: var(--text-muted);
  background: transparent;
}

.ma-connection__btn--ghost:hover {
  background: rgb(var(--surface-2) / 0.5);
  color: var(--text-main);
}

.ma-connection__spinner {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 2px solid rgb(255 255 255 / 0.4);
  border-top-color: rgb(255 255 255);
  animation: ma-connection-spin 0.7s linear infinite;
}

@keyframes ma-connection-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 720px) {
  .ma-connection__head {
    flex-direction: column;
  }
}
</style>
