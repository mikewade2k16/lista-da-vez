<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { ImagePlus, Send, X } from "lucide-vue-next";
import { storeToRefs } from "pinia";
import { hasPermission, normalizeAppRole } from "~/domain/utils/permissions";
import AppSelectField from "~/components/ui/AppSelectField.vue";
import { useAuthStore } from "~/stores/auth";
import { useFeedbackStore } from "~/stores/feedback";
import { useUiStore } from "~/stores/ui";
import { compressFeedbackImage, formatFeedbackImageSize } from "~/utils/feedback-image";

const feedbackStore = useFeedbackStore();
const ui = useUiStore();
const auth = useAuthStore();
const route = useRoute();
const router = useRouter();
const { storeContext, user } = storeToRefs(auth);

const selectedFeedbackId = ref("");
const replyMessage = ref("");
const editingStatus = ref("");
const selectedKindFilter = ref("");
const selectedStatusFilter = ref("");
const searchValue = ref("");
const saving = ref(false);
const messagesViewport = ref<HTMLElement | null>(null);
const replyTextarea = ref<HTMLTextAreaElement | null>(null);
const replyImage = ref<File | null>(null);
const replyImagePreviewUrl = ref("");
const syncingStatusFromFeedback = ref(false);
let feedbackPollingTimer: number | null = null;
let messagesPollingTimer: number | null = null;

const kindOptions = [
  { value: "", label: "Todos" },
  { value: "suggestion", label: "Sugestao" },
  { value: "question", label: "Duvida" },
  { value: "problem", label: "Problema" }
];

const statusOptions = [
  { value: "", label: "Todos" },
  { value: "open", label: "Aberto" },
  { value: "in_progress", label: "Em analise" },
  { value: "resolved", label: "Resolvido" },
  { value: "closed", label: "Fechado" }
];

const detailStatusOptions = statusOptions.filter((option) => option.value);

const canEditFeedback = computed(() => {
  if (auth.permissionsResolved) {
    return hasPermission(auth.permissionKeys, "workspace.feedback.edit");
  }

  const normalizedRole = normalizeAppRole(auth.role);
  return normalizedRole === "platform_admin" || normalizedRole === "owner" || normalizedRole === "manager";
});

const storeLookup = computed(() =>
  new Map(
    (storeContext.value || []).map((store) => [String(store?.id || "").trim(), store])
  )
);

const filteredFeedbacks = computed(() => {
  const normalizedSearch = String(searchValue.value || "").trim().toLowerCase();

  return feedbackStore.feedbacks.filter((feedback) => {
    if (selectedKindFilter.value && feedback.kind !== selectedKindFilter.value) {
      return false;
    }

    if (selectedStatusFilter.value && feedback.status !== selectedStatusFilter.value) {
      return false;
    }

    if (!normalizedSearch) {
      return true;
    }

    const haystack = [
      feedback.subject,
      feedback.user_name,
      feedback.body,
      getStoreLabel(feedback.store_id),
      kindLabel(feedback.kind),
      statusLabel(feedback.status)
    ]
      .join(" ")
      .toLowerCase();

    return haystack.includes(normalizedSearch);
  });
});

const selectedFeedback = computed(() =>
  feedbackStore.feedbacks.find((feedback) => feedback.id === selectedFeedbackId.value) || null
);

const selectedMessages = computed(() => {
  if (!selectedFeedback.value?.id) {
    return [];
  }

  return feedbackStore.messagesByFeedbackId[selectedFeedback.value.id] || [];
});

const lastFeedbackSyncCursor = computed(() => {
  const timestamps = feedbackStore.feedbacks
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

const isSelectedFeedbackClosed = computed(() =>
  String(editingStatus.value || selectedFeedback.value?.status || "").trim() === "closed"
);

function isDocumentVisible() {
  return !import.meta.client || document.visibilityState === "visible";
}

function kindLabel(kind: string) {
  const labels = {
    suggestion: "Sugestao",
    question: "Duvida",
    problem: "Problema"
  };

  return labels[String(kind || "").trim()] || kind || "-";
}

function statusLabel(status: string) {
  const labels = {
    open: "Aberto",
    in_progress: "Em analise",
    resolved: "Resolvido",
    closed: "Fechado"
  };

  return labels[String(status || "").trim()] || status || "-";
}

function formatDate(isoString: string) {
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

function getFeedbackMessages(feedbackId: string) {
  return feedbackStore.messagesByFeedbackId[String(feedbackId || "").trim()] || [];
}

function getFeedbackPreview(feedback: { id: string; body: string }) {
  const latestMessage = getFeedbackMessages(feedback.id).at(-1);
  if (!latestMessage) {
    return feedback.body || "";
  }

  return latestMessage.body || (latestMessage.image_url ? "Imagem anexada" : feedback.body || "");
}

function getUnreadCount(feedback: { id: string; user_id: string; created_at: string; user_last_read_at: string }) {
  if (!feedback?.id) {
    return 0;
  }

  const feedbackOwnerId = String(feedback.user_id || "").trim();
  const readAt = new Date(feedback.user_last_read_at || feedback.created_at).getTime();

  return getFeedbackMessages(feedback.id).filter((message) => {
    const authorUserId = String(message.author_user_id || "").trim();
    const createdAt = new Date(message.created_at).getTime();

    return authorUserId === feedbackOwnerId && createdAt > readAt;
  }).length;
}

function getStoreLabel(storeId: string) {
  const store = storeLookup.value.get(String(storeId || "").trim());
  if (!store) {
    return "Loja nao informada";
  }

  return String(store.name || store.code || store.city || "Loja nao informada").trim();
}

function syncEditingStatus(status: string) {
  syncingStatusFromFeedback.value = true;
  editingStatus.value = String(status || "").trim();
  void nextTick(() => {
    syncingStatusFromFeedback.value = false;
  });
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

function handleReplyKeydown(event: KeyboardEvent) {
  if (event.key !== "Enter" || event.isComposing) {
    return;
  }

  if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) {
    void nextTick(() => syncReplyTextareaHeight());
    return;
  }

  event.preventDefault();
  if (saving.value || !canEditFeedback.value || isSelectedFeedbackClosed.value) {
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

function syncRouteWithFeedbackId(feedbackId: string) {
  if (!import.meta.client) {
    return;
  }

  const normalizedId = String(feedbackId || "").trim();
  const currentId = String(route.query.id || "").trim();

  if (normalizedId === currentId) {
    return;
  }

  const nextQuery = { ...route.query };
  if (normalizedId) {
    nextQuery.id = normalizedId;
  } else {
    delete nextQuery.id;
  }

  void router.replace({ query: nextQuery });
}

function syncSelectionFromRoute() {
  const routeId = String(route.query.id || "").trim();

  if (routeId && filteredFeedbacks.value.some((feedback) => feedback.id === routeId)) {
    selectedFeedbackId.value = routeId;
    return;
  }

  if (!filteredFeedbacks.value.length) {
    selectedFeedbackId.value = "";
    return;
  }

  if (!filteredFeedbacks.value.some((feedback) => feedback.id === selectedFeedbackId.value)) {
    selectedFeedbackId.value = filteredFeedbacks.value[0].id;
  }
}

async function loadFeedbacks(options: { since?: string } = {}) {
  if (!isDocumentVisible()) {
    return;
  }

  const result = await feedbackStore.fetchFeedbacks({
    kind: selectedKindFilter.value,
    status: selectedStatusFilter.value,
    ...options
  });

  if (!result.ok) {
    ui.error(result.message || "Erro ao carregar feedbacks");
    return;
  }

  await feedbackStore.syncMessagesForFeedbacks(feedbackStore.feedbacks.map((feedback) => feedback.id));
  syncSelectionFromRoute();
}

async function loadFeedbackUpdates() {
  if (!lastFeedbackSyncCursor.value) {
    return;
  }

  await loadFeedbacks({ since: lastFeedbackSyncCursor.value });
}

function hasUnreadMessages(feedback: { id: string; user_id: string; created_at: string; user_last_read_at: string }) {
  if (!feedback?.id) {
    return false;
  }

  const feedbackOwnerId = String(feedback.user_id || "").trim();
  const readAt = new Date(feedback.user_last_read_at || feedback.created_at).getTime();

  return selectedMessages.value.some((message) => {
    const authorUserId = String(message.author_user_id || "").trim();
    const createdAt = new Date(message.created_at).getTime();

    return authorUserId === feedbackOwnerId && createdAt > readAt;
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

async function loadSelectedMessages(options: { markRead?: boolean } = {}) {
  if (!selectedFeedback.value?.id || !isDocumentVisible()) {
    return;
  }

  const result = await feedbackStore.fetchMessages(selectedFeedback.value.id, {
    after: lastSelectedMessageCreatedAt.value
  });

  if (!result.ok) {
    ui.error(result.message || "Erro ao carregar mensagens");
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

function selectFeedback(feedbackId: string) {
  selectedFeedbackId.value = String(feedbackId || "").trim();
}

async function persistStatusIfNeeded() {
  if (!selectedFeedback.value?.id || !canEditFeedback.value) {
    return { ok: true, changed: false };
  }

  if (editingStatus.value === selectedFeedback.value.status) {
    return { ok: true, changed: false };
  }

  const result = await feedbackStore.updateFeedback(selectedFeedback.value.id, {
    status: editingStatus.value
  });

  if (!result.ok) {
    ui.error(result.message || "Erro ao atualizar status");
    return { ok: false, changed: false };
  }

  return { ok: true, changed: true };
}

async function saveStatus() {
  if (!selectedFeedback.value?.id || !canEditFeedback.value) {
    return;
  }

  saving.value = true;
  try {
    const result = await persistStatusIfNeeded();
    if (!result.ok) {
      syncEditingStatus(String(selectedFeedback.value?.status || ""));
      return;
    }
    if (result.ok && result.changed) {
      ui.success("Status atualizado.");
    }
  } finally {
    saving.value = false;
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

  if (!canEditFeedback.value) {
    ui.error("Seu acesso ao feedback esta em modo somente leitura.");
    return;
  }

  const body = String(replyMessage.value || "").trim();
  const image = replyImage.value;
  saving.value = true;

  try {
    const statusResult = await persistStatusIfNeeded();
    if (!statusResult.ok) {
      return;
    }

    if (!body && !image) {
      if (statusResult.changed) {
        ui.success("Status atualizado.");
      }
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
    ui.success(statusResult.changed ? "Status atualizado e resposta enviada." : "Resposta enviada.");
  } finally {
    saving.value = false;
  }
}

function startPolling() {
  stopPolling();
  feedbackPollingTimer = window.setInterval(loadFeedbackUpdates, 30000);
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
    void loadFeedbackUpdates();
    void loadSelectedMessages();
  }
}

watch(filteredFeedbacks, () => {
  syncSelectionFromRoute();
}, { immediate: true });

watch(selectedFeedbackId, (feedbackId) => {
  syncRouteWithFeedbackId(feedbackId);
  replyMessage.value = "";
  clearReplyImage();
  syncReplyTextareaHeight(true);
  feedbackStore.applyLocalReadState(feedbackId);
  void loadSelectedMessages({ markRead: true });
});

watch(
  () => route.query.id,
  () => {
    syncSelectionFromRoute();
  }
);

watch(
  () => selectedFeedback.value?.status,
  (status) => {
    syncEditingStatus(String(status || ""));
  },
  { immediate: true }
);

watch(editingStatus, (status) => {
  if (
    syncingStatusFromFeedback.value ||
    !selectedFeedback.value?.id ||
    !canEditFeedback.value ||
    !status ||
    status === selectedFeedback.value.status
  ) {
    return;
  }

  void saveStatus();
});

watch(replyMessage, () => {
  void nextTick(() => syncReplyTextareaHeight());
});

watch(selectedMessages, () => {
  void scrollMessagesToBottom();
});

watch([selectedKindFilter, selectedStatusFilter], () => {
  void loadFeedbacks();
});

onMounted(async () => {
  await loadFeedbacks();
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
  <section class="admin-panel admin-feedback" data-testid="feedback-panel">
    <header class="admin-panel__header admin-feedback__header">
      <h2 class="admin-panel__title">Feedback</h2>
      <p class="admin-panel__subtitle">Acompanhe a conversa dos chamados, responda no mesmo fio e ajuste o status sem sair da tela.</p>
    </header>

    <div class="admin-feedback__toolbar">
      <input
        v-model="searchValue"
        class="admin-feedback__search"
        type="search"
        placeholder="Buscar por assunto, usuario, loja ou conteudo..."
      >

      <div class="admin-feedback__toolbar-filters">
        <AppSelectField
          v-model="selectedKindFilter"
          :options="kindOptions"
          compact
        />
        <AppSelectField
          v-model="selectedStatusFilter"
          :options="statusOptions"
          compact
        />
      </div>
    </div>

    <div class="admin-feedback__layout">
      <aside class="admin-feedback__list" aria-label="Chamados de feedback">
        <button
          v-for="feedback in filteredFeedbacks"
          :key="feedback.id"
          class="admin-feedback__ticket"
          :class="{ 'is-active': selectedFeedback?.id === feedback.id, 'has-unread': getUnreadCount(feedback) > 0 }"
          type="button"
          @click="selectFeedback(feedback.id)"
        >
          <span class="admin-feedback__ticket-line">
            <strong :title="feedback.subject || 'Chamado sem assunto'">{{ feedback.subject || "Chamado sem assunto" }}</strong>
            <span class="admin-feedback__ticket-badges">
              <small class="admin-feedback__kind-tag" :class="`admin-feedback__kind-tag--${feedback.kind}`">{{ kindLabel(feedback.kind) }}</small>
              <small class="admin-feedback__status-tag" :class="`admin-feedback__status-tag--${feedback.status}`">{{ statusLabel(feedback.status) }}</small>
            </span>
          </span>
          <span class="admin-feedback__ticket-meta-row">
            <span class="admin-feedback__ticket-meta">{{ feedback.user_name || "Usuario" }} · {{ getStoreLabel(feedback.store_id) }}</span>
            <small class="admin-feedback__ticket-time">{{ formatDate(feedback.updated_at || feedback.created_at) }}</small>
          </span>
          <small class="admin-feedback__ticket-preview" :title="getFeedbackPreview(feedback)">{{ getFeedbackPreview(feedback) }}</small>
        </button>

        <div v-if="!filteredFeedbacks.length" class="admin-feedback__empty-list">
          <strong>Nenhum feedback encontrado</strong>
          <span>Ajuste os filtros ou aguarde novos chamados.</span>
        </div>
      </aside>

      <article v-if="selectedFeedback" class="admin-feedback__conversation">
        <header class="admin-feedback__conversation-header">
          <div class="admin-feedback__conversation-copy">
            <div class="admin-feedback__conversation-meta">
              <span class="admin-feedback__kind-tag" :class="`admin-feedback__kind-tag--${selectedFeedback.kind}`">{{ kindLabel(selectedFeedback.kind) }}</span>
              <p>
                {{ selectedFeedback.user_name || "Usuario" }} ·
                {{ getStoreLabel(selectedFeedback.store_id) }} ·
                {{ formatDate(selectedFeedback.created_at) }}
              </p>
            </div>
            <h3 :title="selectedFeedback.subject">{{ selectedFeedback.subject }}</h3>
          </div>

          <AppSelectField
            v-model="editingStatus"
            class="admin-feedback__status-select"
            :options="detailStatusOptions"
            compact
            :disabled="!canEditFeedback || saving"
          />
        </header>

        <div ref="messagesViewport" class="admin-feedback__messages">
          <article
            v-for="message in selectedMessages"
            :key="message.id"
            class="admin-feedback__message"
            :class="{ 'admin-feedback__message--own': message.author_user_id === user?.id }"
          >
            <header>
              <strong>{{ message.author_name || "Usuario" }}</strong>
              <span>{{ formatDate(message.created_at) }}</span>
            </header>
            <p v-if="message.body">{{ message.body }}</p>
            <a
              v-if="message.image_url"
              class="admin-feedback__message-image-link"
              :href="message.image_url"
              target="_blank"
              rel="noopener noreferrer"
            >
              <img :src="message.image_url" alt="Imagem anexada ao feedback" class="admin-feedback__message-image">
            </a>
          </article>

          <article v-if="!selectedMessages.length" class="admin-feedback__message">
            <header>
              <strong>{{ selectedFeedback.user_name || "Usuario" }}</strong>
              <span>{{ formatDate(selectedFeedback.created_at) }}</span>
            </header>
            <p>{{ selectedFeedback.body }}</p>
          </article>
        </div>

        <div v-if="isSelectedFeedbackClosed" class="admin-feedback__readonly admin-feedback__readonly--closed">
          Chamado encerrado. A conversa esta bloqueada para novas mensagens.
        </div>
        <div v-else-if="!canEditFeedback" class="admin-feedback__readonly">
          Seu acesso ao feedback esta em modo somente leitura.
        </div>

        <form class="admin-feedback__reply" @submit.prevent="sendReply">
          <div class="admin-feedback__reply-input-row">
            <textarea
              ref="replyTextarea"
              v-model="replyMessage"
              :placeholder="isSelectedFeedbackClosed ? 'Chamado encerrado' : 'Responder este chamado'"
              rows="1"
              :disabled="!canEditFeedback || saving || isSelectedFeedbackClosed"
              @input="syncReplyTextareaHeight()"
              @keydown="handleReplyKeydown"
            ></textarea>

            <button
              type="submit"
              :disabled="saving || !canEditFeedback || isSelectedFeedbackClosed || (!replyMessage.trim() && !replyImage)"
            >
              <Send :size="16" :stroke-width="2.2" />
              <span>{{ saving ? "Enviando..." : "Enviar" }}</span>
            </button>
          </div>

          <div class="admin-feedback__reply-tools">
            <label
              class="admin-feedback__upload-btn"
              :class="{ 'is-disabled': !canEditFeedback || saving || isSelectedFeedbackClosed }"
            >
              <input
                type="file"
                accept="image/png,image/jpeg,image/webp"
                hidden
                :disabled="!canEditFeedback || saving || isSelectedFeedbackClosed"
                @change="handleReplyImageChange"
              >
              <ImagePlus :size="16" :stroke-width="2.1" />
              <span>{{ replyImage ? "Trocar imagem" : "Anexar imagem" }}</span>
            </label>
            <small class="admin-feedback__upload-hint">
              A imagem e compactada no envio e apagada 7 dias apos o fechamento.
            </small>
          </div>

          <div v-if="replyImagePreviewUrl" class="admin-feedback__reply-preview">
            <img :src="replyImagePreviewUrl" alt="Preview da imagem anexada" class="admin-feedback__reply-preview-image">
            <div class="admin-feedback__reply-preview-copy">
              <strong>{{ replyImage?.name }}</strong>
              <span>{{ formatFeedbackImageSize(replyImage?.size || 0) }}</span>
            </div>
            <button
              type="button"
              class="admin-feedback__reply-preview-remove"
              :disabled="saving"
              @click="clearReplyImage"
            >
              <X :size="14" :stroke-width="2.2" />
            </button>
          </div>
        </form>
      </article>

      <article v-else class="admin-feedback__placeholder">
        <strong>Selecione um chamado</strong>
        <span>Quando voce abrir um feedback, a conversa aparece aqui.</span>
      </article>
    </div>
  </section>
</template>

<style scoped>
.admin-feedback {
  display: grid;
  gap: 1rem;
  min-height: 0;
}

.admin-feedback__toolbar {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 0.85rem;
  align-items: center;
}

.admin-feedback__search {
  width: 100%;
  min-width: 0;
  padding: 0.78rem 0.95rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.95rem;
  background: rgba(13, 18, 29, 0.9);
  color: var(--text-main);
  font: inherit;
}

.admin-feedback__toolbar-filters {
  display: flex;
  align-items: center;
  gap: 0.65rem;
}

.admin-feedback__layout {
  display: grid;
  grid-template-columns: minmax(18rem, 23rem) minmax(0, 1fr);
  gap: 1rem;
  min-height: 0;
  flex: 1;
}

.admin-feedback__list,
.admin-feedback__conversation,
.admin-feedback__placeholder {
  min-height: 0;
  border: 1px solid var(--line-soft);
  border-radius: 1rem;
  background: rgba(13, 18, 29, 0.9);
  box-shadow: var(--shadow-card);
}

.admin-feedback__list {
  display: grid;
  align-content: start;
  gap: 0.55rem;
  padding: 0.75rem;
  max-height: 42rem;
  overflow: auto;
}

.admin-feedback__ticket {
  position: relative;
  display: grid;
  gap: 0.28rem;
  width: 100%;
  padding: 0.78rem 1.1rem 0.78rem 0.82rem;
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 0.9rem;
  background: rgba(18, 25, 38, 0.72);
  color: var(--text-main);
  text-align: left;
  cursor: pointer;
}

.admin-feedback__ticket.has-unread::after {
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

.admin-feedback__ticket:hover,
.admin-feedback__ticket.is-active {
  border-color: rgba(129, 140, 248, 0.36);
  background: rgba(79, 70, 229, 0.16);
}

.admin-feedback__ticket-line {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  min-width: 0;
}

.admin-feedback__ticket-line strong {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.88rem;
}

.admin-feedback__ticket-badges {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  flex-shrink: 0;
}

.admin-feedback__kind-tag,
.admin-feedback__status-tag {
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

.admin-feedback__kind-tag {
  background: rgba(148, 163, 184, 0.12);
  color: #e2e8f0;
}

.admin-feedback__kind-tag--problem {
  background: rgba(248, 113, 113, 0.16);
  color: #fecaca;
}

.admin-feedback__kind-tag--question {
  background: rgba(96, 165, 250, 0.16);
  color: #bfdbfe;
}

.admin-feedback__kind-tag--suggestion {
  background: rgba(52, 211, 153, 0.16);
  color: #a7f3d0;
}

.admin-feedback__status-tag {
  background: rgba(148, 163, 184, 0.16);
  color: #e2e8f0;
}

.admin-feedback__status-tag--open {
  background: rgba(59, 130, 246, 0.16);
  color: #bfdbfe;
}

.admin-feedback__status-tag--in_progress {
  background: rgba(251, 191, 36, 0.16);
  color: #fde68a;
}

.admin-feedback__status-tag--resolved {
  background: rgba(34, 197, 94, 0.16);
  color: #bbf7d0;
}

.admin-feedback__status-tag--closed {
  background: rgba(148, 163, 184, 0.2);
  color: #cbd5e1;
}

.admin-feedback__ticket-meta-row {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
}

.admin-feedback__ticket-time,
.admin-feedback__ticket-meta,
.admin-feedback__ticket-preview {
  color: var(--text-muted);
  font-size: 0.72rem;
}

.admin-feedback__ticket-meta {
  min-width: 0;
  flex: 1;
}

.admin-feedback__ticket-meta,
.admin-feedback__ticket-preview {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.admin-feedback__ticket-time {
  flex-shrink: 0;
}

.admin-feedback__empty-list,
.admin-feedback__placeholder {
  display: grid;
  place-items: center;
  gap: 0.35rem;
  padding: 1.5rem;
  color: var(--text-muted);
  text-align: center;
}

.admin-feedback__placeholder strong,
.admin-feedback__empty-list strong {
  color: var(--text-main);
}

.admin-feedback__conversation {
  min-width: 0;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto auto;
}

.admin-feedback__conversation-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.9rem;
  padding: 0.88rem 1rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.admin-feedback__conversation-copy {
  min-width: 0;
  display: grid;
  gap: 0.28rem;
}

.admin-feedback__conversation-meta {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
}

.admin-feedback__conversation-copy h3 {
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #ffffff;
  font-size: 0.98rem;
}

.admin-feedback__conversation-copy p {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-muted);
  font-size: 0.74rem;
}

.admin-feedback__status-select {
  width: 10rem;
  flex-shrink: 0;
}

.admin-feedback__messages {
  display: grid;
  align-content: start;
  gap: 0.75rem;
  padding: 1rem;
  overflow: auto;
  min-height: 20rem;
  max-height: 32rem;
}

.admin-feedback__message {
  max-width: min(38rem, 92%);
  justify-self: start;
  display: grid;
  gap: 0.45rem;
  padding: 0.78rem 0.9rem;
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 0.95rem;
  background: rgba(18, 25, 38, 0.9);
}

.admin-feedback__message--own {
  justify-self: end;
  border-color: rgba(129, 140, 248, 0.26);
  background: rgba(79, 70, 229, 0.18);
}

.admin-feedback__message header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  color: rgba(226, 232, 240, 0.68);
  font-size: 0.72rem;
}

.admin-feedback__message header strong {
  color: #ffffff;
}

.admin-feedback__message p {
  margin: 0;
  color: rgba(226, 232, 240, 0.92);
  line-height: 1.55;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.admin-feedback__message-image-link {
  display: block;
}

.admin-feedback__message-image {
  display: block;
  max-width: min(20rem, 100%);
  border-radius: 0.9rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.admin-feedback__readonly {
  padding: 0.85rem 1rem 0;
  color: #fcd34d;
  font-size: 0.76rem;
}

.admin-feedback__readonly--closed {
  color: #fca5a5;
}

.admin-feedback__reply {
  display: grid;
  gap: 0.75rem;
  padding: 1rem;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}

.admin-feedback__reply-input-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
  min-width: 0;
}

.admin-feedback__reply textarea {
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

.admin-feedback__reply textarea:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}

.admin-feedback__reply-tools {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  flex-wrap: wrap;
}

.admin-feedback__upload-btn {
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

.admin-feedback__upload-btn.is-disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.admin-feedback__upload-hint {
  color: var(--text-muted);
  font-size: 0.72rem;
}

.admin-feedback__reply-preview {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 0.75rem;
  align-items: center;
  padding: 0.6rem;
  border-radius: 0.8rem;
  background: rgba(8, 12, 19, 0.6);
}

.admin-feedback__reply-preview-image {
  width: 4.25rem;
  height: 4.25rem;
  object-fit: cover;
  border-radius: 0.7rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.admin-feedback__reply-preview-copy {
  min-width: 0;
  display: grid;
  gap: 0.2rem;
}

.admin-feedback__reply-preview-copy strong,
.admin-feedback__reply-preview-copy span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.admin-feedback__reply-preview-copy strong {
  color: #ffffff;
  font-size: 0.8rem;
}

.admin-feedback__reply-preview-copy span {
  color: var(--text-muted);
  font-size: 0.72rem;
}

.admin-feedback__reply-preview-remove {
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

.admin-feedback__reply-preview-remove:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.admin-feedback__reply-input-row > button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.45rem;
  min-width: 7rem;
  height: 2.75rem;
  padding: 0 1rem;
  border: none;
  border-radius: 0.85rem;
  background: linear-gradient(135deg, #3b82f6, #2563eb);
  color: #ffffff;
  font-weight: 800;
  cursor: pointer;
  align-self: center;
}

.admin-feedback__reply-input-row > button:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

@media (max-width: 980px) {
  .admin-feedback__toolbar {
    grid-template-columns: 1fr;
  }

  .admin-feedback__toolbar-filters {
    width: 100%;
    flex-wrap: wrap;
  }

  .admin-feedback__layout {
    grid-template-columns: 1fr;
  }

  .admin-feedback__conversation-header {
    display: grid;
    grid-template-columns: 1fr;
    align-items: start;
  }

  .admin-feedback__status-select {
    width: 100%;
  }

  .admin-feedback__reply-input-row {
    grid-template-columns: 1fr;
  }
}
</style>
