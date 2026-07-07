<script setup lang="ts">
// Coluna lateral da operacao com dois blocos — Comunicados (topo, ainda PREVIA
// sem backend) e Omni Chat (rodape, agora LIGADO ao endpoint Go POST
// /v1/omni-chat/ask via useOmniChat). O bloco Comunicados segue stub visual ate
// ter dados/backend; mantido com badge "Previa".
import { computed, nextTick, ref, watch } from 'vue'
import { useOmniChat } from '~/composables/useOmniChat'
import { useAuthStore } from '~/stores/auth'
import OperationOmniPersonaEditor from '~/components/operation/OperationOmniPersonaEditor.vue'

// Conteudo de exemplo so para dar forma ao template. Nao vem de API.
const communications = [
  { id: 'campaigns', label: 'Campanhas ativas', hint: 'Nenhuma campanha conectada ainda' },
  //{ id: 'messages', label: 'Mensagens', hint: 'Sem mensagens no momento' },
  // { id: 'notices', label: 'Avisos', hint: 'Sem avisos publicados' },
]

const chatTopics = [
  'Atendimento',
  'Produtos',
  'Pesquisa',
  'Operacional',
  'Duvidas gerais',
  'Pesquisa de mercado',
]

const chat = useOmniChat()
const chatStreamRef = ref<HTMLElement | null>(null)

// Edicao da persona (prompt/sistema) do Omni Chat. So aparece para admin da
// plataforma; o chat em si continua para todos. Gating no mesmo padrao do
// CrmWorkspace (auth.role === 'platform_admin'). O editor inline propriamente
// dito (carregar/salvar) mora em OperationOmniPersonaEditor.vue.
const auth = useAuthStore()
const canEditPersona = computed(() => auth.role === 'platform_admin')
const isPersonaEditorOpen = ref(false)

function openPersonaEditor() {
  if (!canEditPersona.value) {
    return
  }
  isPersonaEditorOpen.value = true
}

function closePersonaEditor() {
  isPersonaEditorOpen.value = false
}

// Rola a conversa para a ultima mensagem assim que a lista muda. flush:'post'
// garante que o DOM ja renderizou a nova bolha antes de medir scrollHeight.
watch(
  () => chat.messages.value.length,
  () => {
    void nextTick(() => {
      const stream = chatStreamRef.value
      if (stream) {
        stream.scrollTop = stream.scrollHeight
      }
    })
  },
  { flush: 'post' },
)
</script>

<template>
  <aside class="operation-side" data-testid="operation-side-panel">
    <!-- Comunicados -->
    <section class="operation-side__card operation-side__comms">
      <header class="operation-side__head">
        <h3 class="operation-side__title">Comunicados</h3>
        <span class="operation-side__preview-tag">Prévia</span>
      </header>
      <ul class="operation-side__comms-list">
        <li v-for="item in communications" :key="item.id" class="operation-side__comms-item">
          <span class="operation-side__comms-label">{{ item.label }}</span>
          <span class="operation-side__comms-hint">{{ item.hint }}</span>
        </li>
      </ul>
    </section>

    <!-- Omni Chat -->
    <section class="operation-side__card operation-side__chat">
      <header class="operation-side__head">
        <h3 class="operation-side__title">Omni Chat</h3>
        <div class="operation-side__head-actions">
          <button
            type="button"
            class="operation-side__persona-toggle"
            :disabled="!chat.messages.value.length && !chat.sending.value"
            aria-label="Nova conversa"
            title="Nova conversa (limpa o histórico)"
            @click="chat.newConversation()"
          >
            <span class="material-icons-round">restart_alt</span>
          </button>
          <button
            v-if="canEditPersona"
            type="button"
            class="operation-side__persona-toggle"
            :class="{ 'operation-side__persona-toggle--is-active': isPersonaEditorOpen }"
            :aria-pressed="isPersonaEditorOpen"
            aria-label="Editar persona do Omni"
            title="Editar persona do Omni"
            @click="isPersonaEditorOpen ? closePersonaEditor() : openPersonaEditor()"
          >
            <span class="material-icons-round">tune</span>
          </button>
          <span class="operation-side__preview-tag">Prévia</span>
        </div>
      </header>

      <!-- Editor inline da persona (so admin). Abre via engrenagem no cabecalho. -->
      <OperationOmniPersonaEditor
        v-if="canEditPersona && isPersonaEditorOpen"
        @close="closePersonaEditor()"
      />

      <!--<div class="operation-side__chat-topics">
        <button
          v-for="topic in chatTopics"
          :key="topic"
          type="button"
          class="operation-side__chip"
          :class="{ 'operation-side__chip--is-selected': topic === chat.activeTopic.value }"
          @click="chat.selectTopic(topic)"
        >
          {{ topic }}
        </button>
      </div>-->

      <div ref="chatStreamRef" class="operation-side__chat-stream">
        <p v-if="!chat.messages.value.length" class="operation-side__chat-empty">
          O assistente Omni vai ajudar com atendimento, produtos, operação e pesquisa de mercado.
        </p>

        <template v-else>
          <div
            v-for="message in chat.messages.value"
            :key="message.id"
            class="operation-side__chat-msg"
            :class="[
              `operation-side__chat-msg--${message.role}`,
              { 'operation-side__chat-msg--wide': message.products && message.products.length },
            ]"
          >
            <span v-if="message.text" class="operation-side__chat-text">{{ message.text }}</span>
            <div
              v-if="message.products && message.products.length"
              class="operation-side__products"
            >
              <article
                v-for="(product, index) in message.products"
                :key="product.code || index"
                class="operation-side__product"
              >
                <img
                  v-if="product.image"
                  class="operation-side__product-img"
                  :src="chat.mediaUrl(product.image)"
                  :alt="product.name"
                  loading="lazy"
                />
                <div class="operation-side__product-info">
                  <span class="operation-side__product-name">{{ product.name }}</span>
                  <span v-if="product.brand" class="operation-side__product-brand">
                    {{ product.brand }}
                  </span>
                  <span
                    v-if="chat.formatPrice(product.price)"
                    class="operation-side__product-price"
                  >
                    {{ chat.formatPrice(product.price) }}
                  </span>
                </div>
              </article>
            </div>
          </div>
        </template>

        <div
          v-if="chat.sending.value"
          class="operation-side__chat-msg operation-side__chat-msg--assistant operation-side__chat-typing"
          aria-live="polite"
        >
          digitando…
        </div>
      </div>

      <p v-if="chat.errorMessage.value" class="operation-side__chat-error" role="alert">
        {{ chat.errorMessage.value }}
      </p>

      <div class="operation-side__chat-input">
        <input
          v-model="chat.draft.value"
          class="operation-side__chat-field"
          type="text"
          placeholder="Pergunte ao Omni…"
          @keydown.enter.prevent="chat.send()"
        />
        <button
          class="operation-side__chat-send"
          type="button"
          :disabled="chat.sending.value || !chat.draft.value.trim()"
          aria-label="Enviar"
          @click="chat.send()"
        >
          <span class="material-icons-round">send</span>
        </button>
      </div>
    </section>
  </aside>
</template>

<style scoped>
.operation-side {
  display: grid;
  /* Altura das duas faixas (comunicados x chat) — ajuste aqui se quiser. */
  grid-template-rows: var(--operation-side-comms-height, auto) minmax(0, 1fr);
  gap: 12px;
  min-height: 0;
  overflow: hidden;
}

.operation-side__card {
  display: flex;
  flex-direction: column;
  min-height: 0;
  border: 1px solid var(--line-soft);
  border-radius: 18px;
  background: var(--bg-panel);
  padding: 14px;
  gap: 10px;
}

.operation-side__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.operation-side__title {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 700;
  color: var(--text-main);
}

.operation-side__preview-tag {
  font-size: 0.62rem;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  padding: 0.15rem 0.45rem;
  border-radius: 999px;
  color: var(--accent-warning);
  border: 1px solid var(--accent-warning);
  background: rgb(var(--surface-2) / 0.5);
}

.operation-side__head-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.operation-side__persona-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 8px;
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.4);
  color: rgb(var(--muted));
  cursor: pointer;
  transition:
    color 0.15s ease,
    border-color 0.15s ease,
    background 0.15s ease;
}

.operation-side__persona-toggle:hover {
  border-color: rgb(var(--ring) / 0.42);
  color: var(--text-main);
}

.operation-side__persona-toggle--is-active {
  color: rgb(var(--primary));
  border-color: rgb(var(--ring) / 0.42);
  background: rgb(var(--primary) / 0.16);
}

.operation-side__persona-toggle .material-icons-round {
  font-size: 16px;
}

.operation-side__comms-list {
  display: grid;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.operation-side__comms-item {
  display: grid;
  gap: 2px;
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.4);
}

.operation-side__comms-label {
  font-size: 0.82rem;
  font-weight: 700;
  color: var(--text-main);
}

.operation-side__comms-hint {
  font-size: 0.72rem;
  color: rgb(var(--muted));
}

.operation-side__chat-topics {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.operation-side__chip {
  font-size: 0.68rem;
  font-weight: 600;
  padding: 0.2rem 0.5rem;
  border-radius: 999px;
  color: rgb(var(--muted));
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.4);
  cursor: pointer;
  transition:
    color 0.15s ease,
    border-color 0.15s ease,
    background 0.15s ease;
}

.operation-side__chip:hover {
  border-color: rgb(var(--ring) / 0.42);
  color: var(--text-main);
}

.operation-side__chip--is-selected {
  color: rgb(var(--primary));
  border-color: rgb(var(--ring) / 0.42);
  background: rgb(var(--primary) / 0.16);
}

.operation-side__chat-stream {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
  border-radius: 12px;
  border: 1px dashed var(--line-soft);
  background: rgb(var(--surface-2) / 0.25);
}

.operation-side__chat-empty {
  margin: auto;
  max-width: 24ch;
  text-align: center;
  font-size: 0.78rem;
  color: rgb(var(--muted));
}

.operation-side__chat-msg {
  max-width: 88%;
  padding: 8px 11px;
  border-radius: 12px;
  font-size: 0.8rem;
  line-height: 1.4;
  white-space: pre-wrap;
  word-break: break-word;
}

.operation-side__chat-msg--user {
  align-self: flex-end;
  color: rgb(var(--primary));
  border: 1px solid rgb(var(--ring) / 0.42);
  background: rgb(var(--primary) / 0.16);
  border-bottom-right-radius: 4px;
}

.operation-side__chat-msg--assistant {
  align-self: flex-start;
  color: var(--text-main);
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.6);
  border-bottom-left-radius: 4px;
}

.operation-side__chat-typing {
  color: rgb(var(--muted));
  font-style: italic;
}

.operation-side__chat-text {
  display: block;
}

/* Mensagem com produtos ocupa a largura cheia para os cards/imagens respirarem. */
.operation-side__chat-msg--wide {
  max-width: 100%;
  width: 100%;
}

.operation-side__products {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
  gap: 8px;
  margin-top: 8px;
}

.operation-side__product {
  display: grid;
  gap: 4px;
  padding: 7px;
  border-radius: 10px;
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.5);
}

.operation-side__product-img {
  width: 100%;
  height: 128px;
  object-fit: contain;
  border-radius: 8px;
  background: rgb(var(--surface-2) / 0.6);
}

.operation-side__product-info {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.operation-side__product-name {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--text-main);
  line-height: 1.25;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  overflow: hidden;
}

.operation-side__product-brand {
  font-size: 0.66rem;
  color: rgb(var(--muted));
}

.operation-side__product-price {
  font-size: 0.8rem;
  font-weight: 800;
  color: rgb(var(--primary));
}

.operation-side__chat-error {
  margin: 0;
  padding: 8px 11px;
  border-radius: 10px;
  font-size: 0.76rem;
  color: rgb(var(--danger));
  border: 1px solid rgb(var(--danger) / 0.25);
  background: rgb(var(--danger) / 0.08);
}

.operation-side__chat-input {
  display: flex;
  gap: 8px;
  align-items: center;
}

.operation-side__chat-field {
  flex: 1;
  min-width: 0;
  height: 38px;
  padding: 0 12px;
  border-radius: 10px;
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.4);
  color: var(--text-main);
  font-size: 0.82rem;
}

.operation-side__chat-field:disabled {
  cursor: not-allowed;
  opacity: 0.7;
}

.operation-side__chat-send {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border-radius: 10px;
  border: 1px solid rgb(var(--ring) / 0.42);
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
  cursor: pointer;
  transition: opacity 0.15s ease;
}

.operation-side__chat-send:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.operation-side__chat-send .material-icons-round {
  font-size: 18px;
}
</style>
