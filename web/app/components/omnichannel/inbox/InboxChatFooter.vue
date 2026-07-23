<script setup lang="ts">
import InboxChatComposerActions from "./InboxChatComposerActions.vue";
import InboxChatComposerAttachmentMenu from "./InboxChatComposerAttachmentMenu.vue";
import InboxChatComposerEmojiMenu from "./InboxChatComposerEmojiMenu.vue";
import InboxChatComposerInput from "./InboxChatComposerInput.vue";
import InboxChatFooterStatus from "./InboxChatFooterStatus.vue";

const {
  activeConversation,
  canManageConversation,
  draft,
  hasAttachment,
  attachmentType,
  attachmentName,
  attachmentMimeType,
  attachmentSizeBytes,
  attachmentDurationSeconds,
  attachmentPreviewUrl,
  sendingMessage,
  sendError,
  replyTarget,
  isRecording,
  recordingElapsedLabel,
  recordingWaveLevels,
  recordingError,
  isComposerReadOnly,
  composerInputRef,
  fileInputRef,
  pickerAccept,
  pickerCapture,
  attachmentMenuItems,
  contactPickerOpen,
  contactPickerRef,
  contactPickerSearch,
  onUpdateContactPickerSearch,
  closeContactPicker,
  filteredSavedContacts,
  selectSavedContactForSend,
  formatContactDisplayPhone,
  getInitials,
  emojiPanelOpen,
  emojiPanelRef,
  emojiSearchInputRef,
  emojiTriggerRef,
  toggleEmojiPanel,
  EMOJI_PANEL_TAB_ITEMS,
  emojiPanelTab,
  onUpdateEmojiPanelTab,
  emojiSearch,
  onUpdateEmojiSearch,
  emojiPickerLoading,
  emojiPickerError,
  showEmojiCategoryTabs,
  emojiCategories,
  emojiCategory,
  updateEmojiCategory,
  activeEmojiCategoryLabel,
  filteredEmojiList,
  insertEmoji,
  gifSearch,
  onUpdateGifSearch,
  gifLoading,
  gifError,
  gifResults,
  pickGifResult,
  savedStickersLoading,
  savedStickers,
  stickerError,
  selectSavedSticker,
  removeSavedSticker,
  openStickerFromEmojiPanel,
  isMentionPickerVisible,
  mentionOptions,
  mentionSelectedIndex,
  pickMentionOption,
  loadingGroupParticipants,
  showLinkPreviewToggle,
  linkPreviewEnabled,
  toggleLinkPreview,
  composerHasContent,
  onComposerKeydown,
  onComposerInput,
  onComposerCursorUpdate,
  emitSendFromComposer,
  toggleRecording,
  cancelRecording,
  sendRecordedAudio,
  onFileChange,
  onClearReply,
  onUpdateDraft,
  onClearAttachment,
  getReplyTargetAuthorLabel,
  shouldShowReplyTypeMeta,
  resolveMessageType,
  getReplyTypeIcon,
  getMediaTypeLabel,
  getReplyTargetText,
  formatFileSize
} = defineProps([
  "activeConversation",
  "canManageConversation",
  "draft",
  "hasAttachment",
  "attachmentType",
  "attachmentName",
  "attachmentMimeType",
  "attachmentSizeBytes",
  "attachmentDurationSeconds",
  "attachmentPreviewUrl",
  "sendingMessage",
  "sendError",
  "replyTarget",
  "isRecording",
  "recordingElapsedLabel",
  "recordingWaveLevels",
  "recordingError",
  "isComposerReadOnly",
  "composerInputRef",
  "fileInputRef",
  "pickerAccept",
  "pickerCapture",
  "attachmentMenuItems",
  "contactPickerOpen",
  "contactPickerRef",
  "contactPickerSearch",
  "onUpdateContactPickerSearch",
  "closeContactPicker",
  "filteredSavedContacts",
  "selectSavedContactForSend",
  "formatContactDisplayPhone",
  "getInitials",
  "emojiPanelOpen",
  "emojiPanelRef",
  "emojiSearchInputRef",
  "emojiTriggerRef",
  "toggleEmojiPanel",
  "EMOJI_PANEL_TAB_ITEMS",
  "emojiPanelTab",
  "onUpdateEmojiPanelTab",
  "emojiSearch",
  "onUpdateEmojiSearch",
  "emojiPickerLoading",
  "emojiPickerError",
  "showEmojiCategoryTabs",
  "emojiCategories",
  "emojiCategory",
  "updateEmojiCategory",
  "activeEmojiCategoryLabel",
  "filteredEmojiList",
  "insertEmoji",
  "gifSearch",
  "onUpdateGifSearch",
  "gifLoading",
  "gifError",
  "gifResults",
  "pickGifResult",
  "savedStickersLoading",
  "savedStickers",
  "stickerError",
  "selectSavedSticker",
  "removeSavedSticker",
  "openStickerFromEmojiPanel",
  "isMentionPickerVisible",
  "mentionOptions",
  "mentionSelectedIndex",
  "pickMentionOption",
  "loadingGroupParticipants",
  "showLinkPreviewToggle",
  "linkPreviewEnabled",
  "toggleLinkPreview",
  "composerHasContent",
  "onComposerKeydown",
  "onComposerInput",
  "onComposerCursorUpdate",
  "emitSendFromComposer",
  "toggleRecording",
  "cancelRecording",
  "sendRecordedAudio",
  "onFileChange",
  "onClearReply",
  "onUpdateDraft",
  "onClearAttachment",
  "getReplyTargetAuthorLabel",
  "shouldShowReplyTypeMeta",
  "resolveMessageType",
  "getReplyTypeIcon",
  "getMediaTypeLabel",
  "getReplyTargetText",
  "formatFileSize"
]);
</script>

<template>
  <div class="chat-page__panel-footer">
    <InboxChatFooterStatus
      v-bind="{
        replyTarget,
        hasAttachment,
        attachmentType,
        attachmentName,
        attachmentMimeType,
        attachmentSizeBytes,
        attachmentDurationSeconds,
        attachmentPreviewUrl,
        isRecording,
        recordingElapsedLabel,
        recordingWaveLevels,
        recordingError,
        sendError,
        sendingMessage,
        isComposerReadOnly,
        onClearReply,
        onClearAttachment,
        cancelRecording,
        sendRecordedAudio,
        getReplyTargetAuthorLabel,
        shouldShowReplyTypeMeta,
        resolveMessageType,
        getReplyTypeIcon,
        getMediaTypeLabel,
        getReplyTargetText,
        formatFileSize
      }"
    />

    <div v-if="!isRecording" class="chat-composer">
      <InboxChatComposerAttachmentMenu
        v-bind="{
          activeConversation,
          canManageConversation,
          fileInputRef,
          pickerAccept,
          pickerCapture,
          attachmentMenuItems,
          contactPickerOpen,
          contactPickerRef,
          contactPickerSearch,
          onUpdateContactPickerSearch,
          closeContactPicker,
          filteredSavedContacts,
          selectSavedContactForSend,
          formatContactDisplayPhone,
          getInitials,
          onFileChange
        }"
      />

      <InboxChatComposerEmojiMenu
        v-bind="{
          activeConversation,
          canManageConversation,
          emojiPanelOpen,
          emojiPanelRef,
          emojiSearchInputRef,
          emojiTriggerRef,
          toggleEmojiPanel,
          EMOJI_PANEL_TAB_ITEMS,
          emojiPanelTab,
          onUpdateEmojiPanelTab,
          emojiSearch,
          onUpdateEmojiSearch,
          emojiPickerLoading,
          emojiPickerError,
          showEmojiCategoryTabs,
          emojiCategories,
          emojiCategory,
          updateEmojiCategory,
          activeEmojiCategoryLabel,
          filteredEmojiList,
          insertEmoji,
          gifSearch,
          onUpdateGifSearch,
          gifLoading,
          gifError,
          gifResults,
          pickGifResult,
          savedStickersLoading,
          savedStickers,
          stickerError,
          selectSavedSticker,
          removeSavedSticker,
          openStickerFromEmojiPanel
        }"
      />

      <InboxChatComposerInput
        v-bind="{
          activeConversation,
          canManageConversation,
          draft,
          onUpdateDraft,
          composerInputRef,
          isMentionPickerVisible,
          mentionOptions,
          mentionSelectedIndex,
          pickMentionOption,
          loadingGroupParticipants,
          getInitials,
          onComposerKeydown,
          onComposerInput,
          onComposerCursorUpdate
        }"
      />

      <InboxChatComposerActions
        v-bind="{
          activeConversation,
          canManageConversation,
          showLinkPreviewToggle,
          linkPreviewEnabled,
          toggleLinkPreview,
          composerHasContent,
          isRecording,
          emitSendFromComposer,
          toggleRecording
        }"
      />
    </div>
  </div>
</template>

<style scoped>
.chat-page__panel-footer {
  display: grid;
  gap: 0.45rem;
  width: 100%;
  align-items: stretch;
}

.chat-composer {
  display: grid;
  grid-template-columns: auto auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.35rem;
  padding: 0.35rem 0.45rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-md);
  background: rgb(var(--surface));
  width: 100%;
  transition: border-color 140ms ease;
}

.chat-composer:focus-within {
  border-color: rgb(var(--muted) / 0.7);
}
</style>
