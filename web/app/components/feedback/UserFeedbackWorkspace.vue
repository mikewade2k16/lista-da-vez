<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { storeToRefs } from "pinia";
import { ImagePlus, Send, X } from "lucide-vue-next";
import { useAuthStore } from "~/stores/auth";
import { useFeedbackStore } from "~/stores/feedback";
import { useUiStore } from "~/stores/ui";
import { compressFeedbackImage, formatFeedbackImageSize } from "~/utils/feedback-image";

const feedbackStore = useFeedbackStore();
const auth = useAuthStore();
const ui = useUiStore();
const route = useRoute();
const { user } = storeToRefs(auth);
const selectedFeedbackId = ref("");
const replyMessage = ref("");
const replyImage = ref<File | null>(null);
const replyImagePreviewUrl = ref("");
const replyTextarea = ref(null);
const messagesViewport = ref<HTMLElement | null>(null);
let feedbackPollingTimer = null;
let messagesPollingTimer = null;

const selectedFeedback = computed(() =>
  feedbackStore.myFeedbacks.find((feedback) => feedback.id === selectedFeedbackId.value) ||
  feedbackStore.myFeedbacks[0] ||
  null
);

const selectedMessages = computed(() => {
  if (!selectedFeedback.value?.id) {
    return [];
  }

  return feedbackStore.messagesByFeedbackId[selectedFeedback.value.id] || [];
});

const isSelectedFeedbackClosed = computed(() =>
  String(selectedFeedback.value?.status || "").trim() === "closed"
);

const ownUserId = computed(() => String(user.value?.id || "").trim());

const lastMyFeedbackSyncCursor = computed(() => {
  const timestamps = feedbackStore.myFeedbacks
    .map((feedback) =>
      Math.max(
        new Date(feedback.updated_at || feedback.created_at).getTime(),
        new Date(feedback.user_last_read_at || feedback.created_at).getTime()
      )
    )
    .filter((value) => Number.isFinite(value));

  if (!timestamps.length) {
    return "";
  }

  return new Date(Math.max(...timestamps)).toISOString();
});

const lastSelectedMessageCreatedAt = computed(() => {
  const timestamps = selectedMessages.value
    .map((message) => new Date(message.created_at).getTime())
    .filter((value) => Number.isFinite(value));

  if (!timestamps.length) {
    return "";
  }

  return new Date(Math.max(...timestamps)).toISOString();
});

function isDocumentVisible() {
  return !import.meta.client || document.visibilityState === "visible";
}

function statusLabel(status) {
  const labels = {
    open: "Aberto",
    in_progress: "Em analise",
    resolved: "Resolvido",
    closed: "Fechado"
  };

  return labels[String(status || "").trim()] || status || "-";
}

function kindLabel(kind) {
  const labels = {
    suggestion: "Sugestao",
    question: "Duvida",
    problem: "Problema"
  };

  return labels[String(kind || "").trim()] || kind || "-";
}

function formatDate(isoString) {
  try {
    return new Date(isoString).toLocaleDateString("pt-BR", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit"
    });
  } catch {
    return isoString || "";
  }
}

function getFeedbackMessages(feedbackId) {
  return feedbackStore.messagesByFeedbackId[String(feedbackId || "").trim()] || [];
}

function getFeedbackPreview(feedback) {
  const latestMessage = getFeedbackMessages(feedback.id).at(-1);
  if (!latestMessage) {
    return feedback.body || "";
  }

  return latestMessage.body || (latestMessage.image_url ? "Imagem anexada" : feedback.body || "");
}

function getUnreadCount(feedback) {
  if (!feedback?.id) {
    return 0;
  }

  const readAt = new Date(feedback.user_last_read_at || feedback.created_at).getTime();

  return getFeedbackMessages(feedback.id).filter((message) => {
    const authorUserId = String(message.author_user_id || "").trim();
    const createdAt = new Date(message.created_at).getTime();

    return authorUserId !== ownUserId.value && createdAt > readAt;
  }).length;
}

async function loadMyFeedbacks(options = {}) {
  if (!isDocumentVisible()) {
    return;
  }

  const result = await feedbackStore.fetchMyFeedbacks(options);
  if (!result.ok) {
    ui.error(result.message || "Erro ao carregar seus chamados");
    return;
  }

  await feedbackStore.syncMessagesForFeedbacks(feedbackStore.myFeedbacks.map((feedback) => feedback.id));

  const queryId = String(route.query.id || "").trim();
  if (queryId && feedbackStore.myFeedbacks.some((feedback) => feedback.id === queryId)) {
    selectedFeedbackId.value = queryId;
    return;
  }

  if (!selectedFeedbackId.value && feedbackStore.myFeedbacks[0]) {
    selectedFeedbackId.value = feedbackStore.myFeedbacks[0].id;
  }
}

async function loadMyFeedbackUpdates() {
  if (!lastMyFeedbackSyncCursor.value) {
    return;
  }

  await loadMyFeedbacks({
    since: lastMyFeedbackSyncCursor.value
  });
}

function hasUnreadMessages(feedback) {
  if (!feedback?.id) {
    return false;
  }

  const readAt = new Date(feedback.user_last_read_at || feedback.created_at).getTime();

  return selectedMessages.value.some((message) => {
    const authorUserId = String(message.author_user_id || "").trim();
    const createdAt = new Date(message.created_at).getTime();

    return authorUserId !== ownUserId.value && createdAt > readAt;
  });
}

async function markSelectedFeedbackAsRead() {
  if (!selectedFeedback.value?.id || !isDocumentVisible() || !hasUnreadMessages(selectedFeedback.value)) {
    return;
  }

  const result = await feedbackStore.markFeedbackAsRead(selectedFeedback.value.id);
  if (!result.ok) {
    ui.error(result.message || "Erro ao marcar chamado como lido");
  }
}

async function loadSelectedMessages(options = {}) {
  if (!selectedFeedback.value?.id || !isDocumentVisible()) {
    return;
  }

  const result = await feedbackStore.fetchMessages(selectedFeedback.value.id, {
    after: lastSelectedMessageCreatedAt.value
  });

  if (!result.ok) {
    ui.error(result.message || "Erro ao carregar conversa");
    return;
  }

  if (options.markRead) {
    await markSelectedFeedbackAsRead();
  }
  await scrollMessagesToBottom();
}

async function scrollMessagesToBottom() {
  await nextTick();
  if (messagesViewport.value) {
    messagesViewport.value.scrollTop = messagesViewport.value.scrollHeight;
  }
}

function selectFeedback(feedbackId) {
  selectedFeedbackId.value = feedbackId;
}

function setReplyImage(file: File | null) {
  if (import.meta.client && replyImagePreviewUrl.value) {
    URL.revokeObjectURL(replyImagePreviewUrl.value);
  }

  replyImage.value = file;
  replyImagePreviewUrl.value = file && import.meta.client ? URL.createObjectURL(file) : "";
}

function clearReplyImage() {
  setReplyImage(null);
}

function syncReplyTextareaHeight(reset = false) {
  const textarea = replyTextarea.value;
  if (!textarea) {
    return;
  }

  if (reset) {
    textarea.style.height = "";
    textarea.style.overflowY = "hidden";
    return;
  }

  textarea.style.height = "0px";
  const nextHeight = Math.min(textarea.scrollHeight, 176);
  textarea.style.height = `${Math.max(nextHeight, 44)}px`;
  textarea.style.overflowY = textarea.scrollHeight > 176 ? "auto" : "hidden";
}

function handleReplyKeydown(event) {
  if (event.key !== "Enter" || event.isComposing) {
    return;
  }

  if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) {
    nextTick(() => syncReplyTextareaHeight());
    return;
  }

  event.preventDefault();
  if (isSelectedFeedbackClosed.value || feedbackStore.loading) {
    return;
  }

  if (!replyMessage.value.trim() && !replyImage.value) {
    return;
  }

  void sendReply();
}

async function handleReplyImageChange(event: Event) {
  const target = event.target as HTMLInputElement;
  const file = target.files?.[0] || null;
  if (!file) {
    return;
  }

  try {
    const compressedImage = await compressFeedbackImage(file);
    setReplyImage(compressedImage);
  } catch (err) {
    ui.error(err instanceof Error ? err.message : "Nao foi possivel preparar a imagem.");
  } finally {
    target.value = "";
  }
}

async function sendReply() {
  if (!selectedFeedback.value?.id) {
    return;
  }

  if (isSelectedFeedbackClosed.value) {
    ui.error("Chamado encerrado. Nao e mais possivel enviar mensagens.");
    return;
  }

  const body = String(replyMessage.value || "").trim();
  const image = replyImage.value;
  if (!body && !image) {
    return;
  }

  const result = await feedbackStore.sendMessage(selectedFeedback.value.id, {
    body,
    image
  });

  if (!result.ok) {
    ui.error(result.message || "Erro ao enviar resposta");
    return;
  }

  replyMessage.value = "";
  clearReplyImage();
  syncReplyTextareaHeight(true);
  await scrollMessagesToBottom();
}

function startPolling() {
  stopPolling();
  feedbackPollingTimer = window.setInterval(loadMyFeedbackUpdates, 30000);
  messagesPollingTimer = window.setInterval(loadSelectedMessages, 8000);
}

function stopPolling() {
  if (feedbackPollingTimer) {
    window.clearInterval(feedbackPollingTimer);
    feedbackPollingTimer = null;
  }

  if (messagesPollingTimer) {
    window.clearInterval(messagesPollingTimer);
    messagesPollingTimer = null;
  }
}

function handleVisibilityChange() {
  if (isDocumentVisible()) {
    loadMyFeedbackUpdates();
    loadSelectedMessages();
  }
}

watch(selectedFeedbackId, (feedbackId) => {
  replyMessage.value = "";
  clearReplyImage();
  syncReplyTextareaHeight(true);
  feedbackStore.applyLocalReadState(feedbackId);
  loadSelectedMessages({ markRead: true });
});

watch(replyMessage, () => {
  nextTick(() => syncReplyTextareaHeight());
});

watch(selectedMessages, () => {
  scrollMessagesToBottom();
});

watch(
  () => route.query.id,
  (id) => {
    const normalizedId = String(id || "").trim();
    if (normalizedId) {
      selectedFeedbackId.value = normalizedId;
    }
  }
);

onMounted(async () => {
  await loadMyFeedbacks();
  await loadSelectedMessages({ markRead: true });
  syncReplyTextareaHeight(true);
  startPolling();
  document.addEventListener("visibilitychange", handleVisibilityChange);
});

onBeforeUnmount(() => {
  stopPolling();
  document.removeEventListener("visibilitychange", handleVisibilityChange);
  clearReplyImage();
});
</script>

<template>
  <section class="admin-panel user-feedback" data-testid="my-feedback-panel">
    <header class="admin-panel__header user-feedback__header">
      <h2 class="admin-panel__title">Meus chamados</h2>
      <p class="admin-panel__subtitle">Acompanhe as respostas do time e continue a conversa quando precisar.</p>
    </header>

    <div class="user-feedback__layout">
      <aside class="user-feedback__list" aria-label="Meus chamados">
        <button
          v-for="feedback in feedbackStore.myFeedbacks"
          :key="feedback.id"
          class="user-feedback__ticket"
          :class="{ 'is-active': selectedFeedback?.id === feedback.id, 'has-unread': getUnreadCount(feedback) > 0 }"
          type="button"
          @click="selectFeedback(feedback.id)"
        >
          <span class="user-feedback__ticket-line">
            <strong :title="feedback.subject">{{ feedback.subject }}</strong>
            <span class="user-feedback__ticket-badges">
              <small class="user-feedback__kind-tag" :class="`user-feedback__kind-tag--${feedback.kind}`">{{ kindLabel(feedback.kind) }}</small>
              <small class="user-feedback__ticket-time">{{ formatDate(feedback.updated_at || feedback.created_at) }}</small>
              <small class="user-feedback__status-tag" :class="`user-feedback__status-tag--${feedback.status}`">{{ statusLabel(feedback.status) }}</small>
            </span>
          </span>
          <small class="user-feedback__ticket-preview" :title="getFeedbackPreview(feedback)">{{ getFeedbackPreview(feedback) }}</small>
        </button>

        <div v-if="!feedbackStore.myFeedbacks.length" class="user-feedback__empty">
          <strong>Nenhum chamado enviado</strong>
          <span>Quando voce enviar feedback, ele aparece aqui.</span>
        </div>
      </aside>

      <article v-if="selectedFeedback" class="user-feedback__conversation">
        <header class="user-feedback__conversation-header">
          <div class="user-feedback__conversation-copy">
            <div class="user-feedback__conversation-meta">
              <span class="user-feedback__kind-tag" :class="`user-feedback__kind-tag--${selectedFeedback.kind}`">{{ kindLabel(selectedFeedback.kind) }}</span>
              <small>{{ formatDate(selectedFeedback.created_at) }}</small>
            </div>
            <h3 :title="selectedFeedback.subject">{{ selectedFeedback.subject }}</h3>
          </div>
          <strong class="user-feedback__status-pill">{{ statusLabel(selectedFeedback.status) }}</strong>
        </header>

        <div ref="messagesViewport" class="user-feedback__messages">
          <article
            v-for="message in selectedMessages"
            :key="message.id"
            class="user-feedback__message"
            :class="{ 'user-feedback__message--own': message.author_user_id === user?.id }"
          >
            <header>
              <strong>{{ message.author_name || "Usuario" }}</strong>
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
              <img :src="message.image_url" alt="Imagem anexada ao feedback" class="user-feedback__message-image">
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
              :placeholder="isSelectedFeedbackClosed ? 'Chamado encerrado' : 'Responder este chamado'"
              rows="1"
              :disabled="isSelectedFeedbackClosed || feedbackStore.loading"
              @input="syncReplyTextareaHeight()"
              @keydown="handleReplyKeydown"
            ></textarea>

            <button type="submit" :disabled="isSelectedFeedbackClosed || (!replyMessage.trim() && !replyImage) || feedbackStore.loading">
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
              >
              <ImagePlus :size="16" :stroke-width="2.1" />
              <span>{{ replyImage ? "Trocar imagem" : "Anexar imagem" }}</span>
            </label>
            <small class="user-feedback__upload-hint">
              A imagem e compactada no envio e apagada 7 dias apos o fechamento.
            </small>
          </div>

          <div v-if="replyImagePreviewUrl" class="user-feedback__reply-preview">
            <img :src="replyImagePreviewUrl" alt="Preview da imagem anexada" class="user-feedback__reply-preview-image">
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
  background: rgba(13, 18, 29, 0.9);
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
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 0.85rem;
  background: rgba(18, 25, 38, 0.72);
  color: var(--text-main);
  text-align: left;
  cursor: pointer;
}

.user-feedback__ticket.has-unread::after {
  content: "";
  position: absolute;
  top: 0.88rem;
  right: 0.88rem;
  width: 0.58rem;
  height: 0.58rem;
  border-radius: 999px;
  background: #ff6b6b;
  box-shadow: 0 0 0 0.2rem rgba(255, 107, 107, 0.16);
}

.user-feedback__ticket:hover,
.user-feedback__ticket.is-active {
  border-color: rgba(129, 140, 248, 0.36);
  background: rgba(79, 70, 229, 0.16);
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
  background: rgba(148, 163, 184, 0.12);
  color: #e2e8f0;
}

.user-feedback__kind-tag--problem {
  background: rgba(248, 113, 113, 0.16);
  color: #fecaca;
}

.user-feedback__kind-tag--question {
  background: rgba(96, 165, 250, 0.16);
  color: #bfdbfe;
}

.user-feedback__kind-tag--suggestion {
  background: rgba(52, 211, 153, 0.16);
  color: #a7f3d0;
}

.user-feedback__status-tag {
  background: rgba(148, 163, 184, 0.16);
  color: #e2e8f0;
}

.user-feedback__status-tag--open {
  background: rgba(59, 130, 246, 0.16);
  color: #bfdbfe;
}

.user-feedback__status-tag--in_progress {
  background: rgba(251, 191, 36, 0.16);
  color: #fde68a;
}

.user-feedback__status-tag--resolved {
  background: rgba(34, 197, 94, 0.16);
  color: #bbf7d0;
}

.user-feedback__status-tag--closed {
  background: rgba(148, 163, 184, 0.2);
  color: #cbd5e1;
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
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
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
  color: #ffffff;
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
  background: rgba(59, 130, 246, 0.16);
  color: #bfdbfe;
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
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 0.9rem;
  background: rgba(18, 25, 38, 0.9);
}

.user-feedback__message--own {
  justify-self: end;
  border-color: rgba(129, 140, 248, 0.26);
  background: rgba(79, 70, 229, 0.18);
}

.user-feedback__message header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  color: rgba(226, 232, 240, 0.68);
  font-size: 0.72rem;
}

.user-feedback__message header strong {
  color: #ffffff;
}

.user-feedback__message p {
  margin: 0;
  color: rgba(226, 232, 240, 0.92);
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
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.user-feedback__readonly {
  padding: 0.85rem 1rem 0;
  color: #fca5a5;
  font-size: 0.76rem;
}

.user-feedback__reply {
  display: grid;
  gap: 0.75rem;
  padding: 1rem;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
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
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 0.85rem;
  background: rgba(8, 12, 19, 0.65);
  color: #ffffff;
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
  background: #3b82f6;
  color: #ffffff;
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
  border: 1px solid rgba(96, 165, 250, 0.22);
  background: rgba(59, 130, 246, 0.12);
  color: #dbeafe;
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
  background: rgba(8, 12, 19, 0.6);
}

.user-feedback__reply-preview-image {
  width: 4.25rem;
  height: 4.25rem;
  object-fit: cover;
  border-radius: 0.7rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
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
  color: #ffffff;
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
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.04);
  color: rgba(226, 232, 240, 0.78);
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
