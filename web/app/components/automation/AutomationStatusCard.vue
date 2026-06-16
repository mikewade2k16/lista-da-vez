<script setup lang="ts">
interface Props {
  connected: boolean
  whatsappStatus: string
  connectedPhone: string
  qr: string
  connecting: boolean
  enabled: boolean
  savingEnabled: boolean
  loading: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  connect: []
  disconnect: []
  toggleEnabled: []
}>()

const statusLabel = computed(() => {
  switch (props.whatsappStatus) {
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
  if (props.connected) return 'is-connected'
  if (props.whatsappStatus === 'SCAN_QR_CODE' || props.whatsappStatus === 'STARTING') {
    return 'is-pending'
  }
  return 'is-off'
})
</script>

<template>
  <div class="ast-grid">
    <!-- Conexao do WhatsApp -->
    <article class="ast-card">
      <header class="ast-card__head">
        <h2 class="ast-card__title">WhatsApp</h2>
        <span class="ast-card__status" :class="`ast-card__status--${statusModifier}`">
          {{ statusLabel }}
        </span>
      </header>

      <div class="ast-card__body">
        <p v-if="connected" class="ast-card__info">
          Conectado{{ connectedPhone ? ` como +${connectedPhone}` : '' }}.
        </p>

        <div v-else-if="qr" class="ast-card__qr">
          <img :src="qr" alt="QR code para conectar o WhatsApp" width="200" height="200" />
          <p class="ast-card__hint">Abra o WhatsApp no celular e escaneie o codigo.</p>
        </div>

        <p v-else class="ast-card__info ast-card__info--muted">
          Nenhum numero conectado. Use um numero dedicado ao robo, nao o seu pessoal.
        </p>
      </div>

      <footer class="ast-card__foot">
        <button
          v-if="connected"
          type="button"
          class="ast-btn ast-btn--ghost"
          @click="emit('disconnect')"
        >
          Desconectar
        </button>
        <button
          v-else
          type="button"
          class="ast-btn ast-btn--primary"
          :disabled="connecting"
          @click="emit('connect')"
        >
          {{ connecting ? 'Conectando...' : 'Conectar WhatsApp' }}
        </button>
      </footer>
    </article>

    <!-- Liga/desliga do robo -->
    <article class="ast-card">
      <header class="ast-card__head">
        <h2 class="ast-card__title">Robo</h2>
        <span
          class="ast-card__status"
          :class="enabled ? 'ast-card__status--is-connected' : 'ast-card__status--is-off'"
        >
          {{ enabled ? 'Ligado' : 'Desligado' }}
        </span>
      </header>

      <div class="ast-card__body">
        <button
          type="button"
          class="ast-switch"
          :class="{ 'ast-switch--on': enabled }"
          :disabled="savingEnabled || loading"
          role="switch"
          :aria-checked="enabled"
          @click="emit('toggleEnabled')"
        >
          <span class="ast-switch__track">
            <span class="ast-switch__thumb"></span>
          </span>
          <span class="ast-switch__label">{{ enabled ? 'Ligado' : 'Desligado' }}</span>
        </button>

        <p class="ast-card__hint">
          Ligado, o robo responde as mensagens recebidas. Desligado, ele ignora tudo.
        </p>
      </div>
    </article>
  </div>
</template>

<style scoped>
.ast-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 1rem;
}

.ast-card {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.25rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.9);
  box-shadow: var(--shadow-card);
}

.ast-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.ast-card__title {
  font-size: 1.05rem;
  font-weight: 600;
}

.ast-card__status {
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.15rem 0.5rem;
  border-radius: 999px;
}

.ast-card__status--is-connected {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.ast-card__status--is-pending {
  background: color-mix(in srgb, var(--accent-warning) 18%, transparent);
  color: var(--accent-warning);
}

.ast-card__status--is-off {
  background: rgb(var(--border) / 0.44);
  color: rgb(var(--muted));
}

.ast-card__body {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  align-items: flex-start;
}

.ast-card__info {
  font-size: 0.9rem;
}

.ast-card__info--muted {
  color: var(--text-muted);
}

.ast-card__qr {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  align-items: center;
}

.ast-card__qr img {
  border-radius: 0.5rem;
  border: 1px solid var(--line-soft);
}

.ast-card__hint {
  font-size: 0.8rem;
  color: var(--text-muted);
}

.ast-card__foot {
  margin-top: auto;
}

.ast-btn {
  font-size: 0.9rem;
  font-weight: 600;
  padding: 0.5rem 1rem;
  border-radius: 0.5rem;
  cursor: pointer;
  border: 1px solid transparent;
}

.ast-btn:disabled {
  opacity: 0.6;
  cursor: progress;
}

.ast-btn--primary {
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  color: rgb(255 255 255);
}

.ast-btn--ghost {
  background: transparent;
  border-color: var(--line-soft);
  color: var(--text-main);
}

.ast-switch {
  display: inline-flex;
  align-items: center;
  gap: 0.6rem;
  background: transparent;
  border: none;
  cursor: pointer;
  padding: 0;
}

.ast-switch:disabled {
  opacity: 0.6;
  cursor: progress;
}

.ast-switch__track {
  width: 42px;
  height: 24px;
  border-radius: 999px;
  background: rgb(var(--border));
  position: relative;
  transition: background 0.15s ease;
}

.ast-switch--on .ast-switch__track {
  background: rgb(var(--primary));
}

.ast-switch__thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: rgb(255 255 255);
  transition: transform 0.15s ease;
}

.ast-switch--on .ast-switch__thumb {
  transform: translateX(18px);
}

.ast-switch__label {
  font-size: 0.9rem;
  font-weight: 600;
}
</style>
