<script setup lang="ts">
interface Props {
  enabled: boolean
  savingEnabled: boolean
  connected: boolean
  connecting: boolean
  docsCount: number
}

defineProps<Props>()

const emit = defineEmits<{
  'toggle-enabled': []
  connect: []
  disconnect: []
}>()
</script>

<template>
  <div class="asb">
    <div class="asb__group">
      <span class="asb__dot" :class="enabled ? 'asb__dot--on' : 'asb__dot--off'"></span>
      <span class="asb__text">
        Robo
        <strong>{{ enabled ? 'ligado' : 'desligado' }}</strong>
      </span>
      <button
        type="button"
        class="asb__switch"
        :class="{ 'asb__switch--on': enabled }"
        role="switch"
        :aria-checked="enabled"
        :disabled="savingEnabled"
        @click="emit('toggle-enabled')"
      >
        <span class="asb__switch-thumb"></span>
      </button>
    </div>

    <span class="asb__sep"></span>

    <div class="asb__group">
      <span class="asb__dot" :class="connected ? 'asb__dot--on' : 'asb__dot--warn'"></span>
      <span class="asb__text">
        WhatsApp
        <strong>{{ connected ? 'conectado' : 'desconectado' }}</strong>
      </span>
    </div>

    <div class="asb__spacer"></div>

    <button
      v-if="!connected"
      type="button"
      class="asb__btn"
      :disabled="connecting"
      @click="emit('connect')"
    >
      {{ connecting ? 'Conectando...' : 'Conectar' }}
    </button>
    <button v-else type="button" class="asb__btn asb__btn--ghost" @click="emit('disconnect')">
      Desconectar
    </button>

    <span class="asb__sep"></span>

    <div class="asb__group">
      <span class="asb__dot asb__dot--info"></span>
      <span class="asb__text">
        <strong>{{ docsCount }}</strong>
        documentos
      </span>
    </div>
  </div>
</template>

<style scoped>
.asb {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.85rem 1.25rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.7);
  flex-wrap: wrap;
}

.asb__group {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}

.asb__dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  flex-shrink: 0;
}

.asb__dot--on {
  background: rgb(var(--success));
  box-shadow: 0 0 0 3px rgb(var(--success) / 0.18);
}

.asb__dot--off {
  background: rgb(var(--danger));
  box-shadow: 0 0 0 3px rgb(var(--danger) / 0.16);
}

.asb__dot--warn {
  background: var(--accent-warning);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent-warning) 18%, transparent);
}

.asb__dot--info {
  background: rgb(var(--primary));
  box-shadow: 0 0 0 3px rgb(var(--primary) / 0.18);
}

.asb__text {
  font-size: 0.92rem;
  color: var(--text-muted);
}

.asb__text strong {
  color: var(--text-main);
  font-weight: 600;
}

.asb__sep {
  width: 1px;
  align-self: stretch;
  background: var(--line-soft);
  margin: 0.15rem 0;
}

.asb__spacer {
  flex: 1;
  min-width: 1rem;
}

.asb__switch {
  width: 42px;
  height: 23px;
  border-radius: 999px;
  border: none;
  cursor: pointer;
  background: rgb(var(--surface-2));
  position: relative;
  transition: background 0.15s ease;
  flex-shrink: 0;
}

.asb__switch--on {
  background: rgb(var(--primary));
}

.asb__switch:disabled {
  opacity: 0.6;
  cursor: progress;
}

.asb__switch-thumb {
  position: absolute;
  top: 3px;
  left: 3px;
  width: 17px;
  height: 17px;
  border-radius: 50%;
  background: rgb(255 255 255);
  transition: transform 0.15s ease;
}

.asb__switch--on .asb__switch-thumb {
  transform: translateX(19px);
}

.asb__btn {
  font-size: 0.88rem;
  font-weight: 600;
  padding: 0.45rem 1.1rem;
  border-radius: 0.5rem;
  cursor: pointer;
  background: transparent;
  border: 1px solid rgb(var(--success) / 0.5);
  color: rgb(var(--success));
}

.asb__btn:hover {
  background: rgb(var(--success) / 0.1);
}

.asb__btn:disabled {
  opacity: 0.6;
  cursor: progress;
}

.asb__btn--ghost {
  border-color: var(--line-soft);
  color: var(--text-muted);
}

.asb__btn--ghost:hover {
  background: rgb(var(--surface-2) / 0.5);
  color: var(--text-main);
}
</style>
