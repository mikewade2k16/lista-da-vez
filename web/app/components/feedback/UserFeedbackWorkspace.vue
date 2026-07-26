<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { ImagePlus, Send, X } from 'lucide-vue-next'
import { useFeedbackChat } from '~/composables/useFeedbackChat'
import { useAuthStore } from '~/stores/auth'
import { useFeedbackStore } from '~/stores/feedback'
import { useUiStore } from '~/stores/ui'
import { formatFeedbackImageSize } from '~/utils/feedback-image'
import {
  feedbackKindLabel as kindLabel,
  feedbackStatusLabel as statusLabel,
  formatFeedbackDate as formatDate,
} from '~/domain/utils/feedback-display'

const feedbackStore = useFeedbackStore()
const auth = useAuthStore()
const ui = useUiStore()
const route = useRoute()
const { user } = storeToRefs(auth)
const selectedFeedbackId = ref('')
const feedbackSyncCursor = ref('')

const selectedFeedback = computed(
  () =>
    feedbackStore.myFeedbacks.find((feedback) => feedback.id === selectedFeedbackId.value) ||
    feedbackStore.myFeedbacks[0] ||
    null,
)

const selectedMessages = computed(() => {
  if (!selectedFeedback.value?.id) {
    return []
  }

  return feedbackStore.messagesByFeedbackId[selectedFeedback.value.id] || []
})

const isSelectedFeedbackClosed = computed(
  () => String(selectedFeedback.value?.status || '').trim() === 'closed',
)

const ownUserId = computed(() => String(user.value?.id || '').trim())

// Nucleo de chat compartilhado com o workspace admin (useFeedbackWorkspace). Aqui
// a perspectiva de nao-lido eh do USUARIO: nao-lidas sao as mensagens que NAO sao
// dele. loadMyFeedbackUpdates eh hoisted (func declaration) e recarrega a lista.
const chat = useFeedbackChat({
  selectedFeedback,
  selectedMessages,
  isReadFromOwnerPerspective: (authorUserId: string) => authorUserId !== ownUserId.value,
  loadFeedbackUpdates: loadMyFeedbackUpdates,
  messagesLoadErrorMessage: 'Erro ao carregar conversa',
})
const {
  replyMessage,
  replyImage,
  replyImagePreviewUrl,
  replyTextarea,
  messagesViewport,
  isDocumentVisible,
  getFeedbackPreview,
  getUnreadCount,
  loadSelectedMessages,
  scrollMessagesToBottom,
  clearReplyImage,
  syncReplyTextareaHeight,
  handleReplyImageChange,
  startPolling,
} = chat

async function loadMyFeedbacks(options = {}) {
  if (!isDocumentVisible()) {
    return
  }

  const nextSince = Object.prototype.hasOwnProperty.call(options, 'since')
    ? options.since
    : feedbackSyncCursor.value
  const result = await feedbackStore.fetchMyFeedbacks(
    nextSince ? { ...options, since: nextSince } : options,
  )
  if (!result.ok) {
    ui.error(result.message || 'Erro ao carregar seus chamados')
    return
  }

  if (result.cursor) {
    feedbackSyncCursor.value = result.cursor
  }

  const queryId = String(route.query.id || '').trim()
  if (queryId && feedbackStore.myFeedbacks.some((feedback) => feedback.id === queryId)) {
    selectedFeedbackId.value = queryId
    return
  }

  if (!selectedFeedbackId.value && feedbackStore.myFeedbacks[0]) {
    selectedFeedbackId.value = feedbackStore.myFeedbacks[0].id
  }
}

async function loadMyFeedbackUpdates() {
  await loadMyFeedbacks()
}

function selectFeedback(feedbackId) {
  selectedFeedbackId.value = feedbackId
}

function handleReplyKeydown(event) {
  if (event.key !== 'Enter' || event.isComposing) {
    return
  }

  if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) {
    nextTick(() => syncReplyTextareaHeight())
    return
  }

  event.preventDefault()
  if (isSelectedFeedbackClosed.value || feedbackStore.loading) {
    return
  }

  if (!replyMessage.value.trim() && !replyImage.value) {
    return
  }

  void sendReply()
}

async function sendReply() {
  if (!selectedFeedback.value?.id) {
    return
  }

  if (isSelectedFeedbackClosed.value) {
    ui.error('Chamado encerrado. Nao e mais possivel enviar mensagens.')
    return
  }

  const body = String(replyMessage.value || '').trim()
  const image = replyImage.value
  if (!body && !image) {
    return
  }

  const result = await feedbackStore.sendMessage(selectedFeedback.value.id, {
    body,
    image,
  })

  if (!result.ok) {
    ui.error(result.message || 'Erro ao enviar resposta')
    return
  }

  replyMessage.value = ''
  clearReplyImage()
  syncReplyTextareaHeight(true)
  await scrollMessagesToBottom()
}

watch(selectedFeedbackId, (feedbackId) => {
  replyMessage.value = ''
  clearReplyImage()
  syncReplyTextareaHeight(true)
  feedbackStore.applyLocalReadState(feedbackId)
  loadSelectedMessages({ markRead: true })
})

watch(replyMessage, () => {
  nextTick(() => syncReplyTextareaHeight())
})

watch(selectedMessages, () => {
  scrollMessagesToBottom()
})

watch(
  () => route.query.id,
  (id) => {
    const normalizedId = String(id || '').trim()
    if (normalizedId) {
      selectedFeedbackId.value = normalizedId
    }
  },
)

onMounted(async () => {
  // O nucleo de chat (useFeedbackChat) cuida do tracking de visibilidade e da
  // limpeza (stopPolling/clearReplyImage) no onBeforeUnmount.
  await loadMyFeedbacks()
  await loadSelectedMessages({ markRead: true })
  syncReplyTextareaHeight(true)
  startPolling()
})
</script>

<template>
  <section class="admin-panel user-feedback" data-testid="my-feedback-panel">
    <header class="admin-panel__header user-feedback__header">
      <h2 class="admin-panel__title">Meus chamados</h2>
      <p class="admin-panel__subtitle">
        Acompanhe as respostas do time e continue a conversa quando precisar.
      </p>
    </header>

    <div class="user-feedback__layout">
      <aside class="user-feedback__list" aria-label="Meus chamados">
        <button
          v-for="feedback in feedbackStore.myFeedbacks"
          :key="feedback.id"
          class="user-feedback__ticket"
          :class="{
            'is-active': selectedFeedback?.id === feedback.id,
            'has-unread': getUnreadCount(feedback) > 0,
          }"
          type="button"
          @click="selectFeedback(feedback.id)"
        >
          <span class="user-feedback__ticket-line">
            <strong :title="feedback.subject">{{ feedback.subject }}</strong>
            <span class="user-feedback__ticket-badges">
              <small
                class="user-feedback__kind-tag"
                :class="`user-feedback__kind-tag--${feedback.kind}`"
              >
                {{ kindLabel(feedback.kind) }}
              </small>
              <small class="user-feedback__ticket-time">
                {{ formatDate(feedback.updated_at || feedback.created_at) }}
              </small>
              <small
                class="user-feedback__status-tag"
                :class="`user-feedback__status-tag--${feedback.status}`"
              >
                {{ statusLabel(feedback.status) }}
              </small>
            </span>
          </span>
          <small class="user-feedback__ticket-preview" :title="getFeedbackPreview(feedback)">
            {{ getFeedbackPreview(feedback) }}
          </small>
        </button>

        <div v-if="!feedbackStore.myFeedbacks.length" class="user-feedback__empty">
          <strong>Nenhum chamado enviado</strong>
          <span>Quando voce abrir um chamado, ele aparece aqui.</span>
        </div>
      </aside>

      <article v-if="selectedFeedback" class="user-feedback__conversation">
        <header class="user-feedback__conversation-header">
          <div class="user-feedback__conversation-copy">
            <div class="user-feedback__conversation-meta">
              <span
                class="user-feedback__kind-tag"
                :class="`user-feedback__kind-tag--${selectedFeedback.kind}`"
              >
                {{ kindLabel(selectedFeedback.kind) }}
              </span>
              <small>{{ formatDate(selectedFeedback.created_at) }}</small>
            </div>
            <h3 :title="selectedFeedback.subject">{{ selectedFeedback.subject }}</h3>
          </div>
          <strong class="user-feedback__status-pill">
            {{ statusLabel(selectedFeedback.status) }}
          </strong>
        </header>

        <div ref="messagesViewport" class="user-feedback__messages">
          <article
            v-for="message in selectedMessages"
            :key="message.id"
            class="user-feedback__message"
            :class="{ 'user-feedback__message--own': message.author_user_id === user?.id }"
          >
            <header>
              <strong>{{ message.author_name || 'Usuario' }}</strong>
              <span>{{ formatDate(message.created_at) }}</span>
            </header>
            <p v-if="message.body">{{ message.body }}</p>
            <a
              v-if="message.image_url"
              class="user-feedback__message-image-link"
              :href="message.image_url"
              target="_blank"
              rel="noopener noreferrer"
            >
              <img
                :src="message.image_url"
                alt="Imagem anexada ao chamado"
                class="user-feedback__message-image"
              />
            </a>
          </article>
        </div>

        <div v-if="isSelectedFeedbackClosed" class="user-feedback__readonly">
          Chamado encerrado. A conversa esta bloqueada para novas mensagens.
        </div>

        <form class="user-feedback__reply" @submit.prevent="sendReply">
          <div class="user-feedback__reply-input-row">
            <textarea
              ref="replyTextarea"
              v-model="replyMessage"
              :placeholder="
                isSelectedFeedbackClosed ? 'Chamado encerrado' : 'Responder este chamado'
              "
              rows="1"
              :disabled="isSelectedFeedbackClosed || feedbackStore.loading"
              @input="syncReplyTextareaHeight()"
              @keydown="handleReplyKeydown"
            ></textarea>

            <button
              type="submit"
              :disabled="
                isSelectedFeedbackClosed ||
                (!replyMessage.trim() && !replyImage) ||
                feedbackStore.loading
              "
            >
              <Send :size="16" :stroke-width="2.2" />
              <span>Enviar</span>
            </button>
          </div>

          <div class="user-feedback__reply-tools">
            <label
              class="user-feedback__upload-btn"
              :class="{ 'is-disabled': isSelectedFeedbackClosed || feedbackStore.loading }"
            >
              <input
                type="file"
                accept="image/png,image/jpeg,image/webp"
                hidden
                :disabled="isSelectedFeedbackClosed || feedbackStore.loading"
                @change="handleReplyImageChange"
              />
              <ImagePlus :size="16" :stroke-width="2.1" />
              <span>{{ replyImage ? 'Trocar imagem' : 'Anexar imagem' }}</span>
            </label>
            <small class="user-feedback__upload-hint">
              A imagem e compactada no envio e apagada 7 dias apos o fechamento.
            </small>
          </div>

          <div v-if="replyImagePreviewUrl" class="user-feedback__reply-preview">
            <img
              :src="replyImagePreviewUrl"
              alt="Preview da imagem anexada"
              class="user-feedback__reply-preview-image"
            />
            <div class="user-feedback__reply-preview-copy">
              <strong>{{ replyImage?.name }}</strong>
              <span>{{ formatFeedbackImageSize(replyImage?.size || 0) }}</span>
            </div>
            <button
              type="button"
              class="user-feedback__reply-preview-remove"
              :disabled="feedbackStore.loading"
              @click="clearReplyImage"
            >
              <X :size="14" :stroke-width="2.2" />
            </button>
          </div>
        </form>
      </article>
    </div>
  </section>
</template>

<style scoped>
.user-feedback {
  display: grid;
  gap: 1rem;
  min-height: 0;
}

.user-feedback__layout {
  display: grid;
  grid-template-columns: minmax(17rem, 22rem) minmax(0, 1fr);
  gap: 1rem;
  min-height: 0;
  flex: 1;
}

.user-feedback__list,
.user-feedback__conversation {
  min-height: 0;
  border: 1px solid var(--line-soft);
  border-radius: 1rem;
  background: rgb(var(--surface) / 0.9);
  box-shadow: var(--shadow-card);
}

.user-feedback__list {
  display: grid;
  align-content: start;
  gap: 0.55rem;
  padding: 0.75rem;
  max-height: 38rem;
  overflow: auto;
}

.user-feedback__ticket {
  position: relative;
  display: grid;
  gap: 0.28rem;
  width: 100%;
  padding: 0.78rem 1.1rem 0.78rem 0.82rem;
  border: 1px solid rgb(var(--border) / 0.74);
  border-radius: 0.85rem;
  background: rgb(var(--surface-2) / 0.72);
  color: var(--text-main);
  text-align: left;
  cursor: pointer;
}

.user-feedback__ticket.has-unread::after {
  content: '';
  position: absolute;
  top: 0.88rem;
  right: 0.88rem;
  width: 0.58rem;
  height: 0.58rem;
  border-radius: 999px;
  background: rgb(var(--danger));
  box-shadow: 0 0 0 0.2rem rgb(var(--danger) / 0.16);
}

.user-feedback__ticket:hover,
.user-feedback__ticket.is-active {
  border-color: rgb(var(--ring) / 0.36);
  background: rgb(var(--primary) / 0.16);
}

.user-feedback__ticket-line {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  min-width: 0;
}

.user-feedback__ticket-line strong {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.88rem;
}

.user-feedback__ticket-badges {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  flex-shrink: 0;
}

.user-feedback__kind-tag,
.user-feedback__status-tag {
  display: inline-flex;
  align-items: center;
  min-height: 1.2rem;
  padding: 0 0.42rem;
  border-radius: 999px;
  font-size: 0.64rem;
  font-weight: 800;
  letter-spacing: 0.02em;
  text-transform: uppercase;
}

.user-feedback__kind-tag {
  background: rgb(var(--border) / 0.42);
  color: rgb(var(--text));
}

.user-feedback__kind-tag--problem {
  background: rgb(var(--danger) / 0.16);
  color: rgb(var(--danger));
}

.user-feedback__kind-tag--question {
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.user-feedback__kind-tag--suggestion {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.user-feedback__status-tag {
  background: rgb(var(--border) / 0.44);
  color: rgb(var(--text));
}

.user-feedback__status-tag--open {
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.user-feedback__status-tag--in_progress {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary-600));
}

.user-feedback__status-tag--resolved {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.user-feedback__status-tag--closed {
  background: rgb(var(--border) / 0.56);
  color: rgb(var(--muted));
}

.user-feedback__ticket-time,
.user-feedback__ticket-preview {
  color: var(--text-muted);
  font-size: 0.72rem;
}

.user-feedback__ticket-preview {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-feedback__conversation {
  min-width: 0;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
}

.user-feedback__conversation-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.9rem;
  padding: 0.88rem 1rem;
  border-bottom: 1px solid rgb(var(--border) / 0.74);
}

.user-feedback__conversation-copy {
  min-width: 0;
  display: grid;
  gap: 0.28rem;
}

.user-feedback__conversation-meta {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
}

.user-feedback__conversation-header h3 {
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: rgb(var(--text));
  font-size: 0.98rem;
}

.user-feedback__conversation-meta small {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-muted);
  font-size: 0.72rem;
}

.user-feedback__status-pill {
  padding: 0.3rem 0.55rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
  font-size: 0.72rem;
}

.user-feedback__messages {
  display: grid;
  align-content: start;
  gap: 0.75rem;
  padding: 1rem;
  overflow: auto;
  min-height: 20rem;
  max-height: 30rem;
}

.user-feedback__message {
  max-width: min(36rem, 92%);
  justify-self: start;
  display: grid;
  gap: 0.45rem;
  padding: 0.78rem 0.9rem;
  border: 1px solid rgb(var(--border) / 0.74);
  border-radius: 0.9rem;
  background: rgb(var(--surface-2) / 0.9);
}

.user-feedback__message--own {
  justify-self: end;
  border-color: rgb(var(--ring) / 0.26);
  background: rgb(var(--primary) / 0.18);
}

.user-feedback__message header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  color: rgb(var(--muted) / 0.82);
  font-size: 0.72rem;
}

.user-feedback__message header strong {
  color: rgb(var(--text));
}

.user-feedback__message p {
  margin: 0;
  color: rgb(var(--text) / 0.92);
  line-height: 1.55;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.user-feedback__message-image-link {
  display: block;
}

.user-feedback__message-image {
  display: block;
  max-width: min(20rem, 100%);
  border-radius: 0.9rem;
  border: 1px solid rgb(var(--border) / 0.78);
}

.user-feedback__readonly {
  padding: 0.85rem 1rem 0;
  color: rgb(var(--danger));
  font-size: 0.76rem;
}

.user-feedback__reply {
  display: grid;
  gap: 0.75rem;
  padding: 1rem;
  border-top: 1px solid rgb(var(--border) / 0.74);
}

.user-feedback__reply-input-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 0.75rem;
  align-items: center;
  min-width: 0;
}

.user-feedback__reply textarea {
  min-width: 0;
  min-height: 2.75rem;
  max-height: 11rem;
  resize: none;
  border: 1px solid rgb(var(--border) / 0.84);
  border-radius: 0.85rem;
  background: rgb(var(--surface-2) / 0.76);
  color: rgb(var(--text));
  padding: 0.62rem 0.9rem;
  line-height: 1.5;
  font: inherit;
}

.user-feedback__reply-input-row > button {
  align-self: center;
  min-height: 2.75rem;
  height: 2.75rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.45rem;
  padding: 0 1rem;
  border: none;
  border-radius: 0.85rem;
  background: rgb(var(--primary));
  color: rgb(255 255 255);
  font-weight: 800;
  cursor: pointer;
}

.user-feedback__reply-input-row > button:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.user-feedback__reply-tools {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  flex-wrap: wrap;
}

.user-feedback__upload-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.55rem 0.8rem;
  border-radius: 0.75rem;
  border: 1px solid rgb(var(--ring) / 0.22);
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
  font-size: 0.78rem;
  font-weight: 700;
  cursor: pointer;
}

.user-feedback__upload-btn.is-disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.user-feedback__upload-hint {
  color: var(--text-muted);
  font-size: 0.72rem;
}

.user-feedback__reply-preview {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 0.75rem;
  align-items: center;
  padding: 0.6rem;
  border-radius: 0.8rem;
  background: rgb(var(--surface-2) / 0.76);
}

.user-feedback__reply-preview-image {
  width: 4.25rem;
  height: 4.25rem;
  object-fit: cover;
  border-radius: 0.7rem;
  border: 1px solid rgb(var(--border) / 0.78);
}

.user-feedback__reply-preview-copy {
  min-width: 0;
  display: grid;
  gap: 0.2rem;
}

.user-feedback__reply-preview-copy strong,
.user-feedback__reply-preview-copy span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-feedback__reply-preview-copy strong {
  color: rgb(var(--text));
  font-size: 0.8rem;
}

.user-feedback__reply-preview-copy span {
  color: var(--text-muted);
  font-size: 0.72rem;
}

.user-feedback__reply-preview-remove {
  width: 2rem;
  height: 2rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid rgb(var(--border) / 0.78);
  border-radius: 999px;
  background: rgb(var(--surface-2) / 0.7);
  color: rgb(var(--muted));
  cursor: pointer;
}

.user-feedback__reply-preview-remove:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.user-feedback__empty {
  display: grid;
  gap: 0.25rem;
  padding: 1rem;
  color: var(--text-muted);
  font-size: 0.82rem;
}

.user-feedback__empty strong {
  color: var(--text-main);
}

@media (max-width: 900px) {
  .user-feedback__layout {
    grid-template-columns: 1fr;
  }

  .user-feedback__conversation-header {
    display: grid;
    grid-template-columns: 1fr;
    align-items: start;
  }

  .user-feedback__reply-input-row {
    grid-template-columns: 1fr;
  }
}
</style>
