<script setup lang="ts">
// registerType=prompt evita recarregar o painel no meio de uma operacao.
// O modulo nao injeta $pwa em ambientes onde esta desligado (como Vitest).
const { $pwa } = useNuxtApp()
</script>

<template>
  <div v-if="$pwa?.needRefresh" class="pwa-reload" role="alert" aria-live="polite">
    <span class="pwa-reload__text">Nova versão do Omni disponível.</span>
    <button type="button" class="pwa-reload__btn" @click="$pwa.updateServiceWorker(true)">
      Atualizar agora
    </button>
    <button
      type="button"
      class="pwa-reload__btn pwa-reload__btn--ghost"
      @click="$pwa.cancelPrompt()"
    >
      Depois
    </button>
  </div>
</template>

<style scoped>
.pwa-reload {
  position: fixed;
  right: 1rem;
  bottom: 1rem;
  z-index: 1000;
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.7rem 0.9rem;
  border: 1px solid var(--line-soft, rgba(255, 255, 255, 0.15));
  border-radius: 0.6rem;
  background: var(--bg-panel, #0d121d);
  color: var(--text-strong, #fff);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35);
}

.pwa-reload__text {
  font-size: 0.85rem;
}

.pwa-reload__btn {
  padding: 0.4rem 0.8rem;
  border: none;
  border-radius: 0.45rem;
  background: var(--accent-info, #6366f1);
  color: #fff;
  font-size: 0.8rem;
  font-weight: 600;
  cursor: pointer;
}

.pwa-reload__btn--ghost {
  background: transparent;
  color: var(--text-muted, #9aa3b5);
}
</style>
