<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useMetaAdsStore } from '~/stores/meta-ads'

const store = useMetaAdsStore()

const draft = ref('')
const listEl = ref<HTMLElement | null>(null)
const inputEl = ref<HTMLTextAreaElement | null>(null)

type HealthTone = 'online' | 'offline' | 'unconfigured' | 'unknown'

// ok → online; !ok com Claude autenticado → problema de configuracao;
// !ok sem Claude autenticado → runner fora do ar (ou auth pendente — o detail diz).
const healthTone = computed<HealthTone>(() => {
  const health = store.assistantHealth
  if (!health) return 'unknown'
  if (health.ok) return 'online'
  return health.claudeAuth ? 'unconfigured' : 'offline'
})

const HEALTH_LABELS: Record<HealthTone, string> = {
  online: 'Online',
  offline: 'Offline',
  unconfigured: 'Desconfigurado',
  unknown: 'Verificando...',
}

const healthLabel = computed(() => HEALTH_LABELS[healthTone.value])

const healthHint = computed(() => {
  const health = store.assistantHealth
  if (!health || health.ok) return ''
  const detail = health.detail.trim()
  if (healthTone.value === 'offline') {
    return detail
      ? `${detail} — Runner do assistente nao esta rodando? Veja meta-ads-assistant/README.`
      : 'Runner do assistente nao esta rodando — veja meta-ads-assistant/README.'
  }
  return detail || 'Assistente indisponivel no momento.'
})

const hasMessages = computed(() => store.assistantMessages.length > 0)

const canSend = computed(
  () =>
    draft.value.trim().length > 0 && !store.assistantSending && Boolean(store.selectedAdAccountId),
)

function autoGrow() {
  const el = inputEl.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 140)}px`
}

async function onSend() {
  if (!canSend.value) return
  const text = draft.value.trim()
  draft.value = ''
  await nextTick()
  autoGrow()
  const sent = await store.sendAssistant(text)
  if (!sent && !draft.value) {
    // Falhou/cancelou: devolve o comando pro campo para nao perder o texto.
    draft.value = text
    await nextTick()
    autoGrow()
  }
}

async function onClear() {
  if (store.assistantSending) return
  await store.clearAssistant()
}

// Mantem a conversa colada no fim quando chegam mensagens ou o "pensando..." liga.
watch(
  () => [store.assistantMessages.length, store.assistantSending] as const,
  async () => {
    await nextTick()
    const el = listEl.value
    if (el) {
      el.scrollTop = el.scrollHeight
    }
  },
)
</script>

<template>
  <article class="ma-assistant" aria-label="Assistente Meta Ads">
    <header class="ma-assistant__head">
      <div class="ma-assistant__head-text">
        <h3 class="ma-assistant__title">Assistente</h3>
        <p class="ma-assistant__subtitle">
          Comande campanhas por texto. As acoes rodam no MCP oficial da Meta.
        </p>
      </div>
      <div class="ma-assistant__head-actions">
        <button
          v-if="hasMessages"
          type="button"
          class="ma-assistant__clear"
          :disabled="store.assistantSending"
          @click="onClear"
        >
          Limpar
        </button>
        <span class="ma-assistant__status" :class="`ma-assistant__status--${healthTone}`">
          <span class="ma-assistant__dot" aria-hidden="true"></span>
          {{ healthLabel }}
        </span>
      </div>
    </header>

    <p v-if="healthHint" class="ma-assistant__hint">{{ healthHint }}</p>

    <div ref="listEl" class="ma-assistant__list" aria-live="polite">
      <p v-if="!hasMessages && !store.assistantSending" class="ma-assistant__empty">
        Peça em texto: criar campanha, ajustar orcamento, pausar, ver criativos... Acoes de escrita
        pedem sua confirmacao e campanhas novas nascem PAUSADAS.
      </p>

      <MetaAdsAssistantMessage
        v-for="message in store.assistantMessages"
        :key="message.id"
        :message="message"
      />

      <div v-if="store.assistantSending" class="ma-assistant__pending">
        <span class="ma-assistant__spinner" aria-hidden="true"></span>
        pensando... acoes na Meta podem levar 1-2 minutos
      </div>
    </div>

    <p v-if="store.assistantError" class="ma-assistant__error">{{ store.assistantError }}</p>

    <form class="ma-assistant__form" @submit.prevent="onSend">
      <textarea
        ref="inputEl"
        v-model="draft"
        rows="1"
        class="ma-assistant__input"
        placeholder="Ex.: cria uma campanha de trafego com R$50/dia"
        :disabled="store.assistantSending"
        @input="autoGrow"
        @keydown.enter.exact.prevent="onSend"
      ></textarea>
      <button
        v-if="store.assistantSending"
        type="button"
        class="ma-assistant__send ma-assistant__send--cancel"
        @click="store.cancelAssistant()"
      >
        Cancelar
      </button>
      <button v-else type="submit" class="ma-assistant__send" :disabled="!canSend">Enviar</button>
    </form>
    <p class="ma-assistant__footnote">Enter envia · Shift+Enter quebra linha</p>
  </article>
</template>

<style scoped>
.ma-assistant {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  padding: 1.4rem 1.5rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.7);
  box-shadow: var(--shadow-card);
}

.ma-assistant__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.ma-assistant__title {
  font-size: 1.1rem;
  font-weight: 700;
}

.ma-assistant__subtitle {
  font-size: 0.85rem;
  color: var(--text-muted);
  margin-top: 0.2rem;
  max-width: 52ch;
}

.ma-assistant__head-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}

.ma-assistant__clear {
  font: inherit;
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--text-muted);
  background: transparent;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  padding: 0.3rem 0.7rem;
  cursor: pointer;
}

.ma-assistant__clear:hover:not(:disabled) {
  color: var(--text-main);
  border-color: var(--line-strong);
}

.ma-assistant__clear:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ma-assistant__status {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  flex-shrink: 0;
  font-size: 0.78rem;
  font-weight: 600;
  padding: 0.3rem 0.7rem;
  border-radius: 999px;
  border: 1px solid var(--line-soft);
}

.ma-assistant__status--online {
  color: rgb(var(--success));
  border-color: rgb(var(--success) / 0.4);
  background: rgb(var(--success) / 0.1);
}

.ma-assistant__status--offline {
  color: rgb(var(--danger));
  border-color: rgb(var(--danger) / 0.35);
  background: rgb(var(--danger) / 0.1);
}

.ma-assistant__status--unconfigured {
  color: rgb(var(--primary));
  border-color: rgb(var(--primary) / 0.35);
  background: rgb(var(--primary) / 0.1);
}

.ma-assistant__status--unknown {
  color: var(--text-muted);
  background: rgb(var(--surface-2) / 0.6);
}

.ma-assistant__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: currentColor;
}

.ma-assistant__hint,
.ma-assistant__empty {
  font-size: 0.85rem;
  color: var(--text-muted);
  line-height: 1.6;
  padding: 0.7rem 0.95rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.4);
}

.ma-assistant__list {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
  max-height: 380px;
  min-height: 120px;
  overflow-y: auto;
  padding: 0.25rem 0.1rem;
}

.ma-assistant__pending {
  align-self: flex-start;
  display: flex;
  align-items: center;
  gap: 0.55rem;
  max-width: 78%;
  padding: 0.65rem 0.9rem;
  border-radius: var(--radius-soft);
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.55);
  color: var(--text-muted);
  font-style: italic;
  font-size: 0.88rem;
}

.ma-assistant__error {
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.16);
  padding: 0.6rem 0.85rem;
  border-radius: var(--radius-soft);
  font-size: 0.88rem;
}

.ma-assistant__form {
  display: flex;
  align-items: flex-end;
  gap: 0.6rem;
}

.ma-assistant__input {
  flex: 1;
  padding: 0.6rem 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.6rem;
  background: rgb(var(--surface-2) / 0.5);
  color: var(--text-main);
  font: inherit;
  font-size: 0.88rem;
  line-height: 1.5;
  resize: none;
  min-height: 42px;
  max-height: 140px;
  overflow-y: auto;
}

.ma-assistant__input:focus {
  outline: none;
  border-color: rgb(var(--ring) / 0.5);
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.16);
}

.ma-assistant__input:disabled,
.ma-assistant__send:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.ma-assistant__send {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
  font-size: 0.9rem;
  font-weight: 600;
  padding: 0.6rem 1.3rem;
  border-radius: 0.55rem;
  cursor: pointer;
  border: 1px solid transparent;
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  color: rgb(255 255 255);
}

.ma-assistant__send--cancel {
  background: transparent;
  color: rgb(var(--danger));
  border-color: rgb(var(--danger) / 0.4);
}

.ma-assistant__footnote {
  font-size: 0.7rem;
  color: var(--text-muted);
}

/* currentColor: branco no botao primario, muted na bolha "pensando...". */
.ma-assistant__spinner {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 2px solid currentColor;
  border-top-color: transparent;
  opacity: 0.85;
  animation: ma-assistant-spin 0.7s linear infinite;
}

@keyframes ma-assistant-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 720px) {
  .ma-assistant__head {
    flex-direction: column;
  }
}
</style>
