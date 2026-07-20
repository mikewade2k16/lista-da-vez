<script setup lang="ts">
import InboxChatSelectionToolbar from "./InboxChatSelectionToolbar.vue";
import InboxChatMessageRow from "./InboxChatMessageRow.vue";

const {
  activeConversation,
  loadingMessages,
  loadingOlderMessages,
  hasMoreMessages,
  showLoadOlderMessagesButton,
  showScrollToLatestButton,
  messageRenderItems,
  selectionMode,
  selectedMessageCount,
  canDeleteSelectedForAll,
  canForwardSelectedMessages,
  selectionStatusMessage,
  showStickyDate,
  stickyDateLabel,
  isGroupConversation,
  showOutboundOperatorLabel,
  canManageConversation,
  chatBodyRef,
  onChatScroll,
  onLoadOlderMessages,
  onScrollToLatest,
  onChatBodyClick,
  onSetReply,
  isMessageSelected,
  onStartMessageSelection,
  onToggleMessageSelection,
  onDeleteMessageForMe,
  onDeleteMessageForAll,
  onForwardMessage,
  onClearSelection,
  onDeleteSelectedForMe,
  onDeleteSelectedForAll,
  onForwardSelectedMessages,
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
  "activeConversation",
  "loadingMessages",
  "loadingOlderMessages",
  "hasMoreMessages",
  "showLoadOlderMessagesButton",
  "showScrollToLatestButton",
  "messageRenderItems",
  "selectionMode",
  "selectedMessageCount",
  "canDeleteSelectedForAll",
  "canForwardSelectedMessages",
  "selectionStatusMessage",
  "showStickyDate",
  "stickyDateLabel",
  "isGroupConversation",
  "showOutboundOperatorLabel",
  "canManageConversation",
  "chatBodyRef",
  "onChatScroll",
  "onLoadOlderMessages",
  "onScrollToLatest",
  "onChatBodyClick",
  "onSetReply",
  "isMessageSelected",
  "onStartMessageSelection",
  "onToggleMessageSelection",
  "onDeleteMessageForMe",
  "onDeleteMessageForAll",
  "onForwardMessage",
  "onClearSelection",
  "onDeleteSelectedForMe",
  "onDeleteSelectedForAll",
  "onForwardSelectedMessages",
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
</script>

<template>
  <div :ref="chatBodyRef" class="chat-page__chat-body" @scroll="onChatScroll($event)" @click="onChatBodyClick($event)">
    <div v-if="loadingMessages" class="chat-page__empty">Carregando mensagens...</div>
    <div v-else-if="!activeConversation" class="chat-page__empty">Selecione uma conversa para iniciar o atendimento.</div>

    <template v-else>
      <InboxChatSelectionToolbar
        v-if="selectionMode"
        :selected-count="selectedMessageCount"
        :can-delete-for-all="canDeleteSelectedForAll"
        :can-forward="canForwardSelectedMessages"
        :status-message="selectionStatusMessage"
        @clear="onClearSelection"
        @delete-mine="onDeleteSelectedForMe"
        @delete-all="onDeleteSelectedForAll"
        @forward="onForwardSelectedMessages"
      />

      <div v-if="showStickyDate && stickyDateLabel" class="chat-page__sticky-date">{{ stickyDateLabel }}</div>
      <div v-if="showLoadOlderMessagesButton && hasMoreMessages && !loadingOlderMessages" class="chat-page__older-action">
        <UButton
          size="xs"
          color="neutral"
          variant="outline"
          icon="i-lucide-history"
          @click.stop="onLoadOlderMessages()"
        >
          Carregar mais 50 mensagens
        </UButton>
      </div>
      <div v-if="loadingOlderMessages" class="chat-page__older-loading">Carregando historico...</div>

      <div v-for="item in messageRenderItems" :key="item.key">
        <div v-if="item.kind === 'date'" class="chat-date-separator">
          <span>{{ item.label }}</span>
        </div>

        <div v-else-if="item.kind === 'unread'" id="chat-unread-anchor" class="chat-unread-separator">
          <span>Nao lidas</span>
        </div>

        <InboxChatMessageRow
          v-else
          :item="item"
          :is-group-conversation="isGroupConversation"
          :show-outbound-operator-label="showOutboundOperatorLabel"
          :can-manage-conversation="canManageConversation"
          :selection-mode="selectionMode"
          :on-set-reply="onSetReply"
          :is-message-selected="isMessageSelected"
          :on-start-message-selection="onStartMessageSelection"
          :on-toggle-message-selection="onToggleMessageSelection"
          :on-delete-message-for-me="onDeleteMessageForMe"
          :on-delete-message-for-all="onDeleteMessageForAll"
          :on-forward-message="onForwardMessage"
          :message-row-id="messageRowId"
          :resolve-message-avatar-url="resolveMessageAvatarUrl"
          :resolve-message-author="resolveMessageAuthor"
          :get-initials="getInitials"
          :get-reply-preview="getReplyPreview"
          :has-reply-jump-target="hasReplyJumpTarget"
          :on-reply-preview-click="onReplyPreviewClick"
          :get-reply-author-label="getReplyAuthorLabel"
          :should-show-reply-type-meta="shouldShowReplyTypeMeta"
          :get-reply-type-icon="getReplyTypeIcon"
          :get-media-type-label="getMediaTypeLabel"
          :get-reply-preview-text="getReplyPreviewText"
          :has-unsupported-notice="hasUnsupportedNotice"
          :get-unsupported-label="getUnsupportedLabel"
          :build-unsupported-open-url="buildUnsupportedOpenUrl"
          :has-image-preview="hasImagePreview"
          :is-image-failed="isImageFailed"
          :get-message-media-url="getMessageMediaUrl"
          :get-message-audio-preview-url="getMessageAudioPreviewUrl"
          :get-message-image-preview-url="getMessageImagePreviewUrl"
          :is-image-preview-loading="isImagePreviewLoading"
          :is-audio-preview-loading="isAudioPreviewLoading"
          :request-image-preview="requestImagePreview"
          :request-audio-preview="requestAudioPreview"
          :mark-image-failed="markImageFailed"
          :is-media-action-loading="isMediaActionLoading"
          :open-message-media="openMessageMedia"
          :download-message-media="downloadMessageMedia"
          :has-video-preview="hasVideoPreview"
          :has-audio-preview="hasAudioPreview"
          :is-voice-note-message="isVoiceNoteMessage"
          :is-audio-file-message="isAudioFileMessage"
          :resolve-audio-avatar-url="resolveAudioAvatarUrl"
          :has-document-preview="hasDocumentPreview"
          :resolve-message-file-name="resolveMessageFileName"
          :has-pending-media-preview="hasPendingMediaPreview"
          :get-pending-media-label="getPendingMediaLabel"
          :get-message-contact-card="getMessageContactCard"
          :on-open-message-contact="onOpenMessageContact"
          :on-save-message-contact="onSaveMessageContact"
          :get-message-link-preview="getMessageLinkPreview"
          :should-render-message-text="shouldRenderMessageText"
          :render-message-html="renderMessageHtml"
          :get-rendered-message-text="getRenderedMessageText"
          :on-message-text-click="onMessageTextClick"
          :get-reaction-badges="getReactionBadges"
          :build-reaction-menu-items="buildReactionMenuItems"
          :toggle-reaction-badge="toggleReactionBadge"
          :format-time="formatTime"
          :is-mention-alert-message="isMentionAlertMessage"
          :resolve-outbound-operator-label="resolveOutboundOperatorLabel"
        />
      </div>

      <div v-if="showScrollToLatestButton" class="chat-page__scroll-latest">
        <UButton
          size="sm"
          color="primary"
          variant="solid"
          icon="i-lucide-chevron-down"
          class="chat-page__scroll-latest-btn"
          @click.stop="onScrollToLatest()"
        >
          Ultimas mensagens
        </UButton>
      </div>
    </template>
  </div>
</template>

<style scoped>
.chat-page__chat-body {
  flex: 1;
  min-height: 0 !important;
  overflow-y: auto;
  position: relative;
  scroll-behavior: smooth;
}

.chat-page__empty,
.chat-page__older-loading {
  color: rgb(var(--muted));
  font-size: 0.85rem;
}

.chat-page__older-action {
  display: flex;
  justify-content: center;
  margin: 0.25rem 0 0.45rem;
}

.chat-page__sticky-date {
  position: sticky;
  top: 0.35rem;
  z-index: 5;
  width: fit-content;
  margin: 0 auto 0.45rem;
  font-size: 0.74rem;
  color: rgb(var(--muted));
  background: rgb(var(--surface));
  border: 1px solid rgb(var(--border));
  border-radius: 999px;
  padding: 0.2rem 0.6rem;
}

.chat-page__scroll-latest {
  position: sticky;
  bottom: 0.9rem;
  display: flex;
  justify-content: center;
  padding-right: 0.35rem;
  margin-top: -2.75rem;
  z-index: 6;
 /* pointer-events: none; */
}

.chat-page__scroll-latest-btn {
  pointer-events: auto;
  box-shadow: var(--shadow-sm);

}

.chat-date-separator,
.chat-unread-separator {
  display: flex;
  justify-content: center;
  margin: 0.55rem 0;
}

.chat-date-separator > span,
.chat-unread-separator > span {
  font-size: 0.72rem;
  color: rgb(var(--muted));
  background: rgb(var(--surface));
  border: 1px solid rgb(var(--border));
  border-radius: 999px;
  padding: 0.16rem 0.58rem;
}

.chat-unread-separator > span {
  color: rgb(var(--primary));
  border-color: rgb(var(--primary) / 0.45);
}
</style>
