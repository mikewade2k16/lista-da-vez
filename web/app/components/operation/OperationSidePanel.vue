<script setup lang="ts">
// Coluna lateral da operacao com dois blocos — Comunicados (topo, ainda PREVIA
// sem backend) e Omni Chat (rodape, agora LIGADO ao endpoint Go POST
// /v1/omni-chat/ask via useOmniChat). O bloco Comunicados segue stub visual ate
// ter dados/backend; mantido com badge "Previa".
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useOmniChat } from '~/composables/useOmniChat'
import { useAuthStore } from '~/stores/auth'
import OperationOmniPersonaEditor from '~/components/operation/OperationOmniPersonaEditor.vue'

// Conteudo de exemplo so para dar forma ao template. Nao vem de API.
const communications = [
  {
    id: 'campaign-progressiva',
    label: 'Campanha Progressiva',
    hint: '📅 Vigência: até 14/07',
    modalTitle: 'Campanha Progressiva',
    modalSubtitle: '📅 Vigência: até 14/07',
    modalBody: `Desconto progressivo válido para joias e relógios, conforme a quantidade de itens comprados dentro do mesmo segmento.

Segmentos válidos:

* Prata com Prata
* Ouro com Ouro
* Relógio com Relógio

Condições de pagamento:

À vista:
1 item = 10% OFF
2 itens = 20% OFF
3 itens ou mais = 30% OFF

Cartão:
1 item = 5% OFF
2 itens = 10% OFF
3 itens ou mais = 20% OFF

❗Importante: os itens não podem ser somados entre segmentos diferentes para aumentar o desconto.

Exemplo: 2 peças em Prata + 1 peça em Ouro não contam como 3 itens para desconto progressivo.`,
  },
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
const activeCommunication = ref<(typeof communications)[number] | null>(null)

function openPersonaEditor() {
  if (!canEditPersona.value) {
    return
  }
  isPersonaEditorOpen.value = true
}

function closePersonaEditor() {
  isPersonaEditorOpen.value = false
}

function openCommunication(item: (typeof communications)[number]) {
  activeCommunication.value = item
}

function closeCommunication() {
  activeCommunication.value = null
}

function syncBodyScrollLock(isOpen: boolean) {
  if (!import.meta.client) {
    return
  }

  document.body.style.overflow = isOpen ? 'hidden' : ''
}

function handleEscape(event: KeyboardEvent) {
  if (event.key === 'Escape' && activeCommunication.value) {
    closeCommunication()
  }
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

watch(activeCommunication, (item) => {
  syncBodyScrollLock(Boolean(item))
})

onMounted(() => {
  document.addEventListener('keydown', handleEscape)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleEscape)
  syncBodyScrollLock(false)
})
</script>

<template>
  <aside class="operation-side" data-testid="operation-side-panel">
    <!-- Comunicados -->
    <section class="operation-side__card operation-side__comms">
      <header class="operation-side__head">
        <h3 class="operation-side__title">Comunicados</h3>
      </header>
      <ul class="operation-side__comms-list">
        <li v-for="item in communications" :key="item.id">
          <button
            type="button"
            class="operation-side__comms-item operation-side__comms-button"
            @click="openCommunication(item)"
          >
            <span class="operation-side__comms-label">{{ item.label }}</span>
            <span class="operation-side__comms-hint">{{ item.hint }}</span>
          </button>
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

  <Teleport to="body">
    <Transition name="operation-side-modal-fade">
      <div
        v-if="activeCommunication"
        class="operation-side__modal-overlay"
        @click.self="closeCommunication()"
      >
        <Transition name="operation-side-modal-scale">
          <div
            v-if="activeCommunication"
            class="operation-side__modal"
            role="dialog"
            aria-modal="true"
            :aria-labelledby="`operation-side-comm-title-${activeCommunication.id}`"
          >
            <div class="operation-side__modal-head">
              <div class="operation-side__modal-copy">
                <h4
                  :id="`operation-side-comm-title-${activeCommunication.id}`"
                  class="operation-side__modal-title"
                >
                  {{ activeCommunication.modalTitle }}
                </h4>
                <p class="operation-side__modal-subtitle">
                  {{ activeCommunication.modalSubtitle }}
                </p>
              </div>
              <button
                type="button"
                class="operation-side__modal-close"
                aria-label="Fechar comunicado"
                @click="closeCommunication()"
              >
                <span class="material-icons-round">close</span>
              </button>
            </div>

            <div class="operation-side__modal-body">
              <p
                v-for="paragraph in activeCommunication.modalBody.split('\n\n')"
                :key="paragraph"
                class="operation-side__modal-paragraph"
              >
                {{ paragraph }}
              </p>
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>
