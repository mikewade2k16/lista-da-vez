<script setup lang="ts">
import {
  UAvatar,
  UButton,
  UIcon
} from "#components";
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import InboxAudioMessagePlayer from "./InboxAudioMessagePlayer.vue";
import InboxMessageContactCard from "./InboxMessageContactCard.vue";
import InboxMessageActionMenu from "./InboxMessageActionMenu.vue";

const {
  item,
  isGroupConversation,
  showOutboundOperatorLabel,
  canManageConversation,
  selectionMode,
  isMessageSelected,
  onSetReply,
  onStartMessageSelection,
  onToggleMessageSelection,
  onDeleteMessageForMe,
  onDeleteMessageForAll,
  onForwardMessage,
  messageRowId,
  resolveMessageAvatarUrl,
  resolveMessageAuthor,
  getInitials,
  getReplyPreview,
  hasReplyJumpTarget,
  onReplyPreviewClick,
  getReplyAuthorLabel,
  shouldShowReplyTypeMeta,
  getReplyTypeIcon,
  getMediaTypeLabel,
  getReplyPreviewText,
  hasUnsupportedNotice,
  getUnsupportedLabel,
  buildUnsupportedOpenUrl,
  hasImagePreview,
  isImageFailed,
  getMessageMediaUrl,
  getMessageAudioPreviewUrl,
  getMessageImagePreviewUrl,
  isImagePreviewLoading,
  isAudioPreviewLoading,
  requestImagePreview,
  requestAudioPreview,
  markImageFailed,
  isMediaActionLoading,
  resolveMediaState,
  isMediaFailed,
  getMediaRetryError,
  retryMessageMedia,
  openMessageMedia,
  downloadMessageMedia,
  hasVideoPreview,
  hasAudioPreview,
  isVoiceNoteMessage,
  isAudioFileMessage,
  resolveAudioAvatarUrl,
  hasDocumentPreview,
  resolveMessageFileName,
  hasPendingMediaPreview,
  getPendingMediaLabel,
  getMessageContactCard,
  onOpenMessageContact,
  onSaveMessageContact,
  getMessageLinkPreview,
  shouldRenderMessageText,
  renderMessageHtml,
  getRenderedMessageText,
  onMessageTextClick,
  getReactionBadges,
  buildReactionMenuItems,
  toggleReactionBadge,
  formatTime,
  isMentionAlertMessage,
  resolveOutboundOperatorLabel
} = defineProps([
  "item",
  "isGroupConversation",
  "showOutboundOperatorLabel",
  "canManageConversation",
  "selectionMode",
  "isMessageSelected",
  "onSetReply",
  "onStartMessageSelection",
  "onToggleMessageSelection",
  "onDeleteMessageForMe",
  "onDeleteMessageForAll",
  "onForwardMessage",
  "messageRowId",
  "resolveMessageAvatarUrl",
  "resolveMessageAuthor",
  "getInitials",
  "getReplyPreview",
  "hasReplyJumpTarget",
  "onReplyPreviewClick",
  "getReplyAuthorLabel",
  "shouldShowReplyTypeMeta",
  "getReplyTypeIcon",
  "getMediaTypeLabel",
  "getReplyPreviewText",
  "hasUnsupportedNotice",
  "getUnsupportedLabel",
  "buildUnsupportedOpenUrl",
  "hasImagePreview",
  "isImageFailed",
  "getMessageMediaUrl",
  "getMessageAudioPreviewUrl",
  "getMessageImagePreviewUrl",
  "isImagePreviewLoading",
  "isAudioPreviewLoading",
  "requestImagePreview",
  "requestAudioPreview",
  "markImageFailed",
  "isMediaActionLoading",
  "resolveMediaState",
  "isMediaFailed",
  "getMediaRetryError",
  "retryMessageMedia",
  "openMessageMedia",
  "downloadMessageMedia",
  "hasVideoPreview",
  "hasAudioPreview",
  "isVoiceNoteMessage",
  "isAudioFileMessage",
  "resolveAudioAvatarUrl",
  "hasDocumentPreview",
  "resolveMessageFileName",
  "hasPendingMediaPreview",
  "getPendingMediaLabel",
  "getMessageContactCard",
  "onOpenMessageContact",
  "onSaveMessageContact",
  "getMessageLinkPreview",
  "shouldRenderMessageText",
  "renderMessageHtml",
  "getRenderedMessageText",
  "onMessageTextClick",
  "getReactionBadges",
  "buildReactionMenuItems",
  "toggleReactionBadge",
  "formatTime",
  "isMentionAlertMessage",
  "resolveOutboundOperatorLabel"
]);

const imagePreviewAnchorRef = ref<HTMLElement | null>(null);
const imagePreviewDecodedByMessageId = ref<Record<string, true>>({});
let imagePreviewObserver: IntersectionObserver | null = null;

const imagePreviewSrc = computed(() => getMessageImagePreviewUrl(item.message));
const audioPreviewSrc = computed(() => getMessageAudioPreviewUrl(item.message));
const isImagePreviewDecoded = computed(() => Boolean(imagePreviewDecodedByMessageId.value[item.message.id]));

function setImagePreviewDecoded(messageId: string, decoded: boolean) {
  const isAlreadyDecoded = Boolean(imagePreviewDecodedByMessageId.value[messageId]);

  if (decoded) {
    if (isAlreadyDecoded) {
      return;
    }

    imagePreviewDecodedByMessageId.value = {
      ...imagePreviewDecodedByMessageId.value,
      [messageId]: true
    };
    return;
  }

  if (!isAlreadyDecoded) {
    return;
  }

  const next = { ...imagePreviewDecodedByMessageId.value };
  delete next[messageId];
  imagePreviewDecodedByMessageId.value = next;
}

function disconnectImagePreviewObserver() {
  if (!imagePreviewObserver) {
    return;
  }

  imagePreviewObserver.disconnect();
  imagePreviewObserver = null;
}

function requestImagePreviewIfNeeded() {
  if (!hasImagePreview(item.message)) {
    return;
  }

  if (isImageFailed(item.message.id)) {
    return;
  }

  if (imagePreviewSrc.value || isImagePreviewLoading(item.message.id)) {
    return;
  }

  void requestImagePreview(item.message);
}

function requestAudioPreviewIfNeeded() {
  if (!hasAudioPreview(item.message)) {
    return;
  }

  if (audioPreviewSrc.value || isAudioPreviewLoading(item.message.id)) {
    return;
  }

  void requestAudioPreview(item.message);
}

function connectImagePreviewObserver() {
  disconnectImagePreviewObserver();

  if (!hasImagePreview(item.message)) {
    return;
  }

  const targetElement = imagePreviewAnchorRef.value;
  if (!import.meta.client || !targetElement || typeof IntersectionObserver !== "function") {
    requestImagePreviewIfNeeded();
    return;
  }

  const scrollRoot = targetElement.closest(".chat-page__chat-body");
  const rootElement = scrollRoot instanceof HTMLElement ? scrollRoot : null;

  imagePreviewObserver = new IntersectionObserver((entries) => {
    const shouldLoad = entries.some((entry) => entry.isIntersecting || entry.intersectionRatio > 0);
    if (!shouldLoad) {
      return;
    }

    requestImagePreviewIfNeeded();
    disconnectImagePreviewObserver();
  }, {
    root: rootElement,
    rootMargin: "320px 0px",
    threshold: 0.01
  });

  imagePreviewObserver.observe(targetElement);
}

function onImagePreviewLoad() {
  setImagePreviewDecoded(item.message.id, true);
}

function onImagePreviewError() {
  setImagePreviewDecoded(item.message.id, false);
  markImageFailed(item.message.id);
}

watch(
  () => imagePreviewSrc.value,
  (next, previous) => {
    if (next && next !== previous) {
      setImagePreviewDecoded(item.message.id, false);
    }
  }
);

watch(
  () => item.message.mediaUrl,
  () => {
    requestAudioPreviewIfNeeded();
  }
);

onMounted(() => {
  connectImagePreviewObserver();
  requestAudioPreviewIfNeeded();
});

onBeforeUnmount(() => {
  disconnectImagePreviewObserver();
});

function getStatusIcon(status: string): string {
  switch (status) {
    case 'PENDING':   return 'i-lucide-clock-3';
    case 'SENT':      return 'i-lucide-check';
    case 'FAILED':    return 'i-lucide-x';
    case 'DELIVERED': return 'i-lucide-check-check';
    case 'READ':      return 'i-lucide-check-check';
    case 'DELETED':   return 'i-lucide-trash-2';
    default:          return 'i-lucide-clock-3';
  }
}

function getStatusLabel(status: string): string {
  switch (status) {
    case 'PENDING': return 'Aguardando envio';
    case 'SENT': return 'Enviada ao provedor';
    case 'DELIVERED': return 'Entregue';
    case 'READ': return 'Lida';
    case 'FAILED': return 'Falha no envio';
    case 'DELETED': return 'Mensagem removida';
    default: return 'Status pendente';
  }
}

function resolveOutboundDisplayLabel(): string {
  if (item.message.origin === 'provider_device') {
    return 'Enviado pelo aparelho';
  }
  return resolveOutboundOperatorLabel(item.message);
}

function getStatusClass(status: string): string {
  if (status === 'FAILED') return 'msg-status-icon--failed';
  if (status === 'READ')   return 'msg-status-icon--read';
  return '';
}
</script>

<template>
  <div
    :id="messageRowId(item.message.id)"
    class="chat-message-row"
    :class="{ 'chat-message-row--selected': isMessageSelected(item.message.id) }"
    :data-date-key="item.dateKey"
    :data-date-label="item.dateLabel"
    :data-message-external-id="item.message.externalMessageId || undefined"
  >
    <div class="chat-message" :class="{ 'chat-message--out': item.message.direction === 'OUTBOUND' }">
      <div class="msg-bubble">
        <div
          v-if="
            (isGroupConversation && (item.message.direction === 'INBOUND' || showOutboundOperatorLabel)) ||
            (!isGroupConversation &&
              (
                item.message.direction === 'INBOUND' ||
                item.message.origin === 'provider_device' ||
                (showOutboundOperatorLabel && item.message.direction === 'OUTBOUND' && resolveOutboundOperatorLabel(item.message))
              )
            )
          "
          class="chat-message__author-row"
        >
          <UAvatar
            :src="resolveMessageAvatarUrl(item.message)"
            :alt="resolveMessageAuthor(item.message)"
            :text="getInitials(resolveMessageAuthor(item.message))"
            size="2xs"
          />
          <p class="chat-message__author">
            {{ item.message.direction === "OUTBOUND" ? resolveOutboundDisplayLabel() : resolveMessageAuthor(item.message) }}
          </p>
        </div>

        <div
          v-if="getReplyPreview(item.message)"
          class="chat-message__reply"
          :class="{ 'chat-message__reply--clickable': hasReplyJumpTarget(item.message) }"
          :role="hasReplyJumpTarget(item.message) ? 'button' : undefined"
          :tabindex="hasReplyJumpTarget(item.message) ? 0 : undefined"
          @click="onReplyPreviewClick(item.message)"
          @keydown.enter.prevent="onReplyPreviewClick(item.message)"
        >
          <div class="chat-message__reply-content">
            <p class="chat-message__reply-author">{{ getReplyAuthorLabel(item.message) }}</p>
            <div
              v-if="shouldShowReplyTypeMeta(getReplyPreview(item.message)?.messageType)"
              class="chat-message__reply-type-row"
            >
              <UIcon
                :name="getReplyTypeIcon(getReplyPreview(item.message)?.messageType)"
                class="chat-message__reply-icon"
              />
              <span class="chat-message__reply-type">
                {{ getMediaTypeLabel(getReplyPreview(item.message)?.messageType) }}
              </span>
            </div>
            <p v-if="getReplyPreviewText(getReplyPreview(item.message))" class="chat-message__reply-text">
              {{ getReplyPreviewText(getReplyPreview(item.message)) }}
            </p>
          </div>
        </div>

        <div v-if="hasUnsupportedNotice(item.message)" class="chat-message__unsupported">
          <div class="chat-message__unsupported-head">
            <UIcon name="i-lucide-triangle-alert" class="chat-message__unsupported-icon" />
            <div>
              <p class="chat-message__unsupported-title">Conteudo ainda nao suportado</p>
              <p class="chat-message__unsupported-label">{{ getUnsupportedLabel(item.message) }}</p>
            </div>
          </div>
          <div class="chat-message__media-actions">
            <a class="chat-message__media-link" :href="buildUnsupportedOpenUrl()" target="_blank" rel="noopener noreferrer">
              Abrir no WhatsApp
            </a>
          </div>
        </div>

        <div v-if="isMediaFailed(item.message)" class="chat-message__media-fallback chat-message__media-fallback--failed">
          <UIcon name="i-lucide-circle-alert" class="chat-message__media-fallback-icon" />
          <span>Nao foi possivel processar esta midia.</span>
          <button
            v-if="item.message.canRetryMedia"
            type="button"
            class="chat-message__media-link"
            :disabled="isMediaActionLoading(item.message.id)"
            @click="retryMessageMedia(item.message)"
          >
            {{ isMediaActionLoading(item.message.id) ? "Agendando..." : "Tentar novamente" }}
          </button>
          <span v-if="getMediaRetryError(item.message.id)" class="chat-message__media-error">
            {{ getMediaRetryError(item.message.id) }}
          </span>
        </div>

        <div v-else-if="hasImagePreview(item.message)" class="chat-message__media">
          <div ref="imagePreviewAnchorRef" class="chat-message__media-image-shell" @mouseenter="requestImagePreviewIfNeeded">
            <img
              v-if="!isImageFailed(item.message.id) && imagePreviewSrc"
              :src="imagePreviewSrc"
              alt="Imagem"
              class="chat-message__media-image"
              :class="{ 'chat-message__media-image--progressive': !isImagePreviewDecoded }"
              loading="lazy"
              decoding="async"
              fetchpriority="low"
              @load="onImagePreviewLoad"
              @error="onImagePreviewError"
            >
            <div v-else-if="!isImageFailed(item.message.id)" class="chat-message__media-loading">
              <UIcon
                :name="isImagePreviewLoading(item.message.id) ? 'i-lucide-loader-circle' : 'i-lucide-image'"
                class="chat-message__media-fallback-icon"
                :class="{ 'chat-message__media-fallback-icon--spin': isImagePreviewLoading(item.message.id) }"
              />
              <span>
                {{
                  isImagePreviewLoading(item.message.id)
                    ? "Carregando imagem..."
                    : "Preview em baixa resolucao. Carregando ao entrar na tela."
                }}
              </span>
            </div>
            <div v-else class="chat-message__media-fallback">
              <UIcon name="i-lucide-image-off" class="chat-message__media-fallback-icon" />
              <span>Nao foi possivel carregar o preview.</span>
            </div>
          </div>
          <div class="chat-message__media-actions">
            <button type="button" class="chat-message__media-link" :disabled="isMediaActionLoading(item.message.id)" @click="openMessageMedia(item.message)">
              Abrir
            </button>
            <button type="button" class="chat-message__media-link" :disabled="isMediaActionLoading(item.message.id)" @click="downloadMessageMedia(item.message)">
              Baixar
            </button>
          </div>
        </div>

        <div v-else-if="hasVideoPreview(item.message)" class="chat-message__media">
          <video :src="getMessageMediaUrl(item.message)" class="chat-message__media-video" controls preload="metadata"></video>
          <div class="chat-message__media-actions">
            <button type="button" class="chat-message__media-link" :disabled="isMediaActionLoading(item.message.id)" @click="openMessageMedia(item.message)">
              Abrir
            </button>
            <button type="button" class="chat-message__media-link" :disabled="isMediaActionLoading(item.message.id)" @click="downloadMessageMedia(item.message)">
              Baixar
            </button>
          </div>
        </div>

        <div v-else-if="hasAudioPreview(item.message) && isVoiceNoteMessage(item.message)" class="chat-message__media" @mouseenter="requestAudioPreviewIfNeeded">
          <InboxAudioMessagePlayer
            :src="audioPreviewSrc"
            :duration-seconds="item.message.mediaDurationSeconds ?? null"
            :direction="item.message.direction"
            :avatar-url="resolveAudioAvatarUrl(item.message)"
            :avatar-text="getInitials(resolveMessageAuthor(item.message))"
          />
          <div v-if="!audioPreviewSrc && isAudioPreviewLoading(item.message.id)" class="chat-message__media-loading">
            <UIcon name="i-lucide-loader-circle" class="chat-message__media-fallback-icon chat-message__media-fallback-icon--spin" />
            <span>Carregando audio...</span>
          </div>
        </div>

        <div v-else-if="hasAudioPreview(item.message) && isAudioFileMessage(item.message)" class="chat-message__document" @mouseenter="requestAudioPreviewIfNeeded">
          <div class="chat-message__audio-file-head">
            <UIcon name="i-lucide-file-audio-2" class="chat-message__document-icon" />
            <span class="chat-message__document-name">{{ resolveMessageFileName(item.message) || 'Arquivo de audio' }}</span>
          </div>
          <InboxAudioMessagePlayer
            :src="audioPreviewSrc"
            :duration-seconds="item.message.mediaDurationSeconds ?? null"
            :direction="item.message.direction"
            :avatar-url="resolveAudioAvatarUrl(item.message)"
            :avatar-text="getInitials(resolveMessageAuthor(item.message))"
            compact
          />
          <div v-if="!audioPreviewSrc && isAudioPreviewLoading(item.message.id)" class="chat-message__media-loading">
            <UIcon name="i-lucide-loader-circle" class="chat-message__media-fallback-icon chat-message__media-fallback-icon--spin" />
            <span>Carregando audio...</span>
          </div>
          <div class="chat-message__media-actions">
            <button type="button" class="chat-message__media-link" :disabled="isMediaActionLoading(item.message.id)" @click="openMessageMedia(item.message)">
              Abrir
            </button>
            <button type="button" class="chat-message__media-link" :disabled="isMediaActionLoading(item.message.id)" @click="downloadMessageMedia(item.message)">
              Baixar
            </button>
          </div>
        </div>

        <div v-else-if="hasDocumentPreview(item.message)" class="chat-message__document">
          <UIcon name="i-lucide-paperclip" class="chat-message__document-icon" />
          <span class="chat-message__document-name">{{ resolveMessageFileName(item.message) || 'Arquivo' }}</span>
          <div class="chat-message__media-actions">
            <button type="button" class="chat-message__media-link" :disabled="isMediaActionLoading(item.message.id)" @click="openMessageMedia(item.message)">
              Abrir
            </button>
            <button type="button" class="chat-message__media-link" :disabled="isMediaActionLoading(item.message.id)" @click="downloadMessageMedia(item.message)">
              Baixar
            </button>
          </div>
        </div>

        <div v-else-if="hasPendingMediaPreview(item.message) || resolveMediaState(item.message) === 'pending'" class="chat-message__media-fallback">
          <UIcon name="i-lucide-loader-circle" class="chat-message__media-fallback-icon chat-message__media-fallback-icon--spin" />
          <span>{{ getPendingMediaLabel(item.message) }}</span>
        </div>

        <InboxMessageContactCard
          v-if="getMessageContactCard(item.message)"
          :contact-card="getMessageContactCard(item.message)"
          :can-manage-conversation="canManageConversation"
          :get-initials="getInitials"
          :direction="item.message.direction"
          @open="onOpenMessageContact(item.message)"
          @save="onSaveMessageContact(item.message)"
        />

        <a
          v-if="getMessageLinkPreview(item.message)"
          :href="getMessageLinkPreview(item.message)?.url"
          target="_blank"
          rel="noopener noreferrer"
          class="chat-message__link-card"
          :class="{ 'chat-message__link-card--no-thumb': !getMessageLinkPreview(item.message)?.thumbnailUrl }"
        >
          <img
            v-if="getMessageLinkPreview(item.message)?.thumbnailUrl"
            :src="getMessageLinkPreview(item.message)?.thumbnailUrl || ''"
            alt="Preview do link"
            class="chat-message__link-thumb"
          >
          <div class="chat-message__link-content">
            <p class="chat-message__link-title">{{ getMessageLinkPreview(item.message)?.title }}</p>
            <p v-if="getMessageLinkPreview(item.message)?.description" class="chat-message__link-description">
              {{ getMessageLinkPreview(item.message)?.description }}
            </p>
            <p class="chat-message__link-host">
              {{ getMessageLinkPreview(item.message)?.host || getMessageLinkPreview(item.message)?.url }}
            </p>
          </div>
        </a>

        <p
          v-if="shouldRenderMessageText(item.message)"
          class="chat-message__text"
          v-html="renderMessageHtml(getRenderedMessageText(item.message), item.message)"
          @click="onMessageTextClick"
        />

        <div v-if="getReactionBadges(item.message).length > 0" class="chat-message__reactions">
          <button
            v-for="badge in getReactionBadges(item.message)"
            :key="`reaction-${item.message.id}-${badge.emoji}`"
            type="button"
            class="chat-message__reaction-badge"
            :class="{ 'chat-message__reaction-badge--active': badge.reactedByCurrentUser }"
            :title="badge.actors.join(', ')"
            :disabled="!canManageConversation"
            @click="toggleReactionBadge(item.message, badge)"
          >
            <span>{{ badge.emoji }}</span>
            <span>{{ badge.count }}</span>
          </button>
        </div>

        <div class="chat-message__meta">
          <time class="msg-time">{{ formatTime(item.message.createdAt) }}</time>
          <UIcon
            v-if="item.message.direction === 'OUTBOUND'"
            :name="getStatusIcon(item.message.status)"
            class="msg-status-icon"
            :class="getStatusClass(item.message.status)"
            :title="getStatusLabel(item.message.status)"
            :aria-label="getStatusLabel(item.message.status)"
          />
          <span v-if="isMentionAlertMessage(item.message)" class="chat-message__mention-indicator">
            <UIcon name="i-lucide-at-sign" />
            Mencao
          </span>
         <!-- <span v-if="resolveOutboundOperatorLabel(item.message)" class="chat-message__operator">
            {{ resolveOutboundOperatorLabel(item.message) }}
          </span>-->
        </div>
      </div>
      <div class="msg-bubble-actions">
        <InboxMessageActionMenu
          :can-manage-conversation="canManageConversation"
          :is-selected="isMessageSelected(item.message.id)"
          :can-delete-for-all="item.message.direction === 'OUTBOUND'"
          :reaction-items="buildReactionMenuItems(item.message)"
          @reply="onSetReply(item.message)"
          @toggle-select="selectionMode ? onToggleMessageSelection(item.message.id) : onStartMessageSelection(item.message.id)"
          @forward="onForwardMessage(item.message.id)"
          @delete-mine="onDeleteMessageForMe(item.message.id)"
          @delete-all="onDeleteMessageForAll(item.message.id)"
        />
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
// ─── Linha da mensagem (wrapper externo) ────────────────────────────────────
.chat-message-row {
  &--selected .msg-bubble {
    outline: 2px solid rgb(var(--primary) / 0.45);
    outline-offset: 1px;
  }

  &--reply-focus .msg-bubble {
    outline: 2px solid rgb(var(--primary) / 0.7);
    outline-offset: 1px;
    animation: chat-reply-focus-pulse 1.2s ease;
  }
}

// ─── Alinhamento INBOUND (esquerda) / OUTBOUND (direita) ────────────────────
.chat-message {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  margin-bottom: 0.25rem;
  overflow: hidden; // impede que a cauda ::before/::after cause rolagem lateral

  &--out {
    justify-content: flex-end;
  }
}

// ─── Bubble (nativo do painel: card liso, sem cauda WhatsApp) ─────────────────
// Card do design system da casa: raio --radius-md, borda --line-soft, fundo --surface.
// Recebido e enviado diferenciam-se pela COR (enviado = leve tom de --primary), não por
// cauda triangular — assim o chat parece parte do Omni, não um WhatsApp Web embutido.
.msg-bubble {
  position: relative;
  max-width: min(540px, 88%);
  border-radius: var(--radius-md);
  background: rgb(var(--surface));
  border: 1px solid rgb(var(--border));
  padding: 0.42rem 0.62rem 1.5rem; // bottom reservado para meta (hora+check)
}

// Enviado: tom leve de --primary para diferenciar de recebido, no estilo do painel.
.chat-message--out .msg-bubble {
  background: rgb(var(--primary) / 0.1);
  border-color: rgb(var(--primary) / 0.24);
}

// ─── Autor ──────────────────────────────────────────────────────────────────
.chat-message__author-row {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  margin-bottom: 0.35rem;
}

.chat-message__author {
  margin: 0;
  font-size: 0.74rem;
  font-weight: 600;
  color: rgb(var(--muted));
}

// ─── Texto da mensagem ───────────────────────────────────────────────────────
.chat-message__text {
  margin: 0;
  line-height: 1.45;
  white-space: normal;
  word-break: break-word;
}

// ─── Resposta citada ─────────────────────────────────────────────────────────
.chat-message__reply {
  margin-bottom: 0.4rem;
  border: 1px solid rgb(var(--border));
  border-left: 4px solid rgb(var(--primary));
  border-radius: var(--radius-xs);
  background: rgb(var(--surface-2));
  padding: 0.42rem 0.56rem;
  position: relative;
  overflow: hidden;

  &--clickable {
    cursor: pointer;
  }

  &-content {
    min-width: 0;
    display: grid;
    gap: 0.2rem;
  }

  &-author {
    margin: 0;
    font-size: 0.74rem;
    font-weight: 700;
    color: rgb(var(--primary));
  }

  &-type-row {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }

  &-icon {
    width: 0.95rem;
    height: 0.95rem;
  }

  &-type {
    font-size: 0.72rem;
    color: rgb(var(--muted));
    font-weight: 600;
  }

  &-text {
    margin: 0;
    font-size: 0.74rem;
    color: rgb(var(--muted));
  }
}

// ─── Conteúdo não suportado ──────────────────────────────────────────────────
.chat-message__unsupported {
  margin-bottom: 0.4rem;
  border: 1px solid rgb(var(--warning) / 0.5);
  border-radius: var(--radius-xs);
  background: rgb(var(--warning) / 0.08);
  padding: 0.45rem 0.55rem;

  &-head {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }

  &-icon {
    width: 0.95rem;
    height: 0.95rem;
  }

  &-title {
    margin: 0;
    font-size: 0.8rem;
    font-weight: 600;
  }

  &-label {
    margin: 0;
    font-size: 0.74rem;
    color: rgb(var(--muted));
  }
}

// ─── Mídia (imagem / vídeo / áudio / documento) ──────────────────────────────
.chat-message__media {
  display: grid;
  gap: 0.35rem;
  margin-bottom: 0.4rem;

  &-image-shell {
    max-width: min(360px, 100%);
    border-radius: var(--radius-sm);
    border: 1px solid rgb(var(--border) / 0.65);
    background: linear-gradient(130deg, rgb(var(--surface-2)), rgb(var(--surface-3)));
    overflow: hidden;
    min-height: 112px;
  }

  &-image {
    max-width: min(360px, 100%);
    max-height: 420px;
    width: 100%;
    border-radius: var(--radius-sm);
    display: block;
    transition: filter 0.28s ease, transform 0.28s ease, opacity 0.28s ease;

    &--progressive {
      filter: blur(14px) saturate(0.88);
      transform: scale(1.03);
      opacity: 0.72;
    }
  }

  &-video {
    max-width: min(360px, 100%);
    border-radius: var(--radius-sm);
    display: block;
  }

  &-loading {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    color: rgb(var(--muted));
    font-size: 0.74rem;
    min-height: 112px;
    padding: 0.5rem 0.55rem;
  }

  &-fallback {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    color: rgb(var(--muted));
    font-size: 0.78rem;
    padding: 0.45rem;
    margin-bottom: 0.4rem;

    &--failed {
      flex-wrap: wrap;
      border: 1px solid rgb(var(--error) / 0.45);
      border-radius: var(--radius-xs);
      background: rgb(var(--error) / 0.08);
      color: rgb(var(--text));
    }
  }

  &-error {
    flex-basis: 100%;
    color: rgb(var(--error));
    font-size: 0.72rem;
  }

  &-fallback-icon {
    width: 0.95rem;
    height: 0.95rem;

    &--spin {
      animation: chat-media-spin 0.9s linear infinite;
    }
  }

  &-actions {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }

  &-link {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 1.75rem;
    padding: 0.1rem 0.45rem;
    border: 1px solid rgb(var(--border));
    border-radius: var(--radius-xs);
    font-size: 0.72rem;
    text-decoration: none;
    color: rgb(var(--text));
    background: transparent;
    cursor: pointer;
  }
}

// ─── Documento ───────────────────────────────────────────────────────────────
.chat-message__document {
  display: grid;
  gap: 0.35rem;
  padding: 0.42rem 0.52rem;
  margin-bottom: 0.4rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-xs);
  background: rgb(var(--surface-2));

  &-icon {
    width: 0.95rem;
    height: 0.95rem;
  }

  &-name {
    font-size: 0.82rem;
    font-weight: 600;
    word-break: break-word;
  }
}

.chat-message__audio-file-head {
  display: flex;
  align-items: center;
  gap: 0.36rem;
}

// ─── Preview de link ─────────────────────────────────────────────────────────
.chat-message__link-card {
  display: grid;
  grid-template-columns: 88px minmax(0, 1fr);
  gap: 0.5rem;
  padding: 0.35rem;
  margin-bottom: 0.4rem;
  text-decoration: none;
  color: inherit;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-xs);
  background: rgb(var(--surface-2));

  &--no-thumb {
    grid-template-columns: minmax(0, 1fr);
  }
}

.chat-message__link-thumb {
  width: 88px;
  height: 88px;
  object-fit: cover;
  border-radius: calc(var(--radius-xs) - 2px);
}

.chat-message__link-content {
  min-width: 0;
  display: grid;
  gap: 0.2rem;
}

.chat-message__link-title {
  margin: 0;
  font-size: 0.8rem;
  font-weight: 700;
}

.chat-message__link-description,
.chat-message__link-host {
  margin: 0;
  font-size: 0.74rem;
  color: rgb(var(--muted));
}

// ─── Reações ─────────────────────────────────────────────────────────────────
.chat-message__reactions {
  margin-top: 0.45rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.28rem;
}

.chat-message__reaction-badge {
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface-2));
  color: rgb(var(--text));
  border-radius: 999px;
  min-height: 1.5rem;
  padding: 0 0.45rem;
  font-size: 0.72rem;
  display: inline-flex;
  align-items: center;
  gap: 0.22rem;
  cursor: pointer;

  &--active {
    border-color: rgb(var(--primary) / 0.55);
    background: rgb(var(--primary) / 0.14);
    color: rgb(var(--primary));
  }
}

// ─── Meta (hora, status, operador, menu) ─────────────────────────────────────
// position: absolute coloca hora+check no canto inferior direito do bubble
// sem ocupar linha própria — igual ao WhatsApp
.chat-message__meta {
  position: absolute;
  bottom: 0.3rem;
  right: 0.62rem;
  display: flex;
  align-items: center;
  gap: 0.28rem;
  font-size: 0.72rem;
  color: rgb(var(--muted));
  pointer-events: none; // não bloqueia cliques no texto
}

.msg-time {
  line-height: 1;
  white-space: nowrap;
}

.msg-status-icon {
  width: 0.95rem;
  height: 0.95rem;
  flex-shrink: 0;

  &--failed { color: rgb(var(--danger)); }
  &--read   { color: rgb(var(--primary)); } // lido: tom do painel (--primary), não o azul WhatsApp
}

.msg-bubble-actions {
  flex-shrink: 0;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.12s ease;
}

.chat-message--out .msg-bubble-actions {
  order: -1; // aparece à esquerda do bubble em mensagens enviadas
}

.chat-message:hover .msg-bubble-actions,
.chat-message:focus-within .msg-bubble-actions {
  opacity: 1;
  pointer-events: auto;
}

.chat-message__operator {
  font-weight: 600;
}

.chat-message__mention-indicator {
  display: inline-flex;
  align-items: center;
  gap: 0.18rem;
  padding: 0.06rem 0.38rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.15);
  color: rgb(var(--primary));
  font-weight: 600;
}

// ─── Conteúdo gerado via v-html (menções, links externos) ────────────────────
:deep(.chat-message__mention) {
  color: rgb(var(--primary));
  font-weight: 700;
  background: rgb(var(--primary) / 0.12);
  border-radius: 0.3rem;
  padding: 0 0.18rem;
}

:deep(.chat-message__mention-link) {
  text-decoration: none;
  cursor: pointer;
}

:deep(.chat-message__external-link) {
  color: rgb(var(--primary));
  text-decoration: underline;
  text-underline-offset: 2px;
  font-weight: 600;
}

// ─── Animações ───────────────────────────────────────────────────────────────
@keyframes chat-reply-focus-pulse {
  0%   { outline-color: rgb(var(--primary) / 0.0); }
  40%  { outline-color: rgb(var(--primary) / 0.8); }
  100% { outline-color: rgb(var(--primary) / 0.0); }
}

@keyframes chat-media-spin {
  from { transform: rotate(0deg); }
  to   { transform: rotate(360deg); }
}
</style>
