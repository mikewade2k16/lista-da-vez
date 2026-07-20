<script setup lang="ts">
import {
  UDashboardPanel
} from "#components";
import { computed, nextTick, onBeforeUnmount, onMounted, onUpdated, ref, watch } from "vue";
import { useInboxChatSelection } from "~/composables/omnichannel/useInboxChatSelection";
import { useInboxChatComposerDom } from "~/composables/omnichannel/useInboxChatComposerDom";
import { useInboxChatComposerControls } from "~/composables/omnichannel/useInboxChatComposerControls";
import { useInboxChatAudioRecorder } from "~/composables/omnichannel/useInboxChatAudioRecorder";
import { useInboxChatEmojiAssets } from "~/composables/omnichannel/useInboxChatEmojiAssets";
import { useInboxChatEmojiPanel } from "~/composables/omnichannel/useInboxChatEmojiPanel";
import { useInboxChatMentions } from "~/composables/omnichannel/useInboxChatMentions";
import { useInboxChatMessageHelpers } from "~/composables/omnichannel/useInboxChatMessageHelpers";
import { useInboxChatMessageIdentity } from "~/composables/omnichannel/useInboxChatMessageIdentity";
import { useInboxChatMessageContact } from "~/composables/omnichannel/useInboxChatMessageContact";
import { useInboxChatPresentation } from "~/composables/omnichannel/useInboxChatPresentation";
import { useInboxChatReactions } from "~/composables/omnichannel/useInboxChatReactions";
import { useInboxChatReplyMeta } from "~/composables/omnichannel/useInboxChatReplyMeta";
import { useInboxChatUtilities } from "~/composables/omnichannel/useInboxChatUtilities";
import { resolveMessageType } from "~/composables/omnichannel/useOmnichannelInboxShared";
import type { Contact, Conversation, GroupParticipant, Message, MessageType } from "~/types";
import type { InboxRenderItem } from "./types";
import InboxChatBody from "./InboxChatBody.vue";
import InboxChatFooter from "./InboxChatFooter.vue";
import InboxForwardMessagesModal from "./InboxForwardMessagesModal.vue";
import InboxChatHeader from "./InboxChatHeader.vue";

type AttachmentPickerMode = "document" | "media" | "camera" | "audio" | "voice" | "sticker" | "gif";
type MentionSendPayload = {
  mentionedJids: string[];
  linkPreviewEnabled?: boolean;
};
type MentionOpenPayload = {
  jid: string | null;
  phone: string | null;
  label: string | null;
};
type EmojiPanelTab = "emoji" | "gif" | "stickers";


const EMOJI_PANEL_TAB_ITEMS: Array<{ id: EmojiPanelTab; label: string }> = [
  { id: "emoji", label: "Emoji" },
  { id: "gif", label: "GIF" },
  { id: "stickers", label: "Figurinhas" }
];
const props = defineProps<{
  activeConversation: Conversation | null;
  activeConversationLabel: string | null;
  conversationOptions: Conversation[];
  savedContacts: Contact[];
  currentUserId: string | null;
  currentUserName: string | null;
  showOutboundOperatorLabel: boolean;
  userRole: "ADMIN" | "SUPERVISOR" | "AGENT" | "VIEWER" | null | undefined;
  canManageConversation: boolean;
  loadingMessages: boolean;
  loadingOlderMessages: boolean;
  hasMoreMessages: boolean;
  showLoadOlderMessagesButton: boolean;
  showScrollToLatestButton: boolean;
  messageRenderItems: InboxRenderItem[];
  groupParticipants: GroupParticipant[];
  loadingGroupParticipants: boolean;
  showStickyDate: boolean;
  stickyDateLabel: string;
  isGroupConversation: boolean;
  draft: string;
  hasAttachment: boolean;
  attachmentType: MessageType | null;
  attachmentName: string | null;
  attachmentMimeType: string | null;
  attachmentSizeBytes: number | null;
  attachmentDurationSeconds: number | null;
  attachmentPreviewUrl: string | null;
  sendingMessage: boolean;
  sendError: string;
  replyTarget: Message | null;
  deleteMessagesForMeAction: (messageIds: string[]) => Promise<{ deletedIds: string[] }>;
  deleteMessagesForAllAction: (messageIds: string[]) => Promise<{
    updatedIds: string[];
    skippedIds: string[];
    failedIds: string[];
  }>;
  forwardMessagesAction: (
    messageIds: string[],
    targetConversationId: string
  ) => Promise<{
    createdCount: number;
    queuedCount: number;
    failedToQueueCount: number;
  }>;
}>();

const emit = defineEmits<{
  (event: "body-mounted", element: HTMLElement | null): void;
  (event: "chat-scroll", payload: Event): void;
  (event: "load-older-messages"): void;
  (event: "scroll-to-latest"): void;
  (event: "send", payload?: MentionSendPayload): void;
  (event: "open-mention", payload: MentionOpenPayload): void;
  (event: "close-conversation"): void;
  (event: "open-whatsapp-session"): void;
  (event: "set-reply", messageEntry: Message): void;
  (event: "clear-reply"): void;
  (event: "set-reaction", value: { messageId: string; emoji: string | null }): void;
  (event: "send-contact", value: { name: string; phone: string; contactId?: string | null; avatarUrl?: string | null }): void;
  (event: "save-contact-card", value: { name: string; phone: string; contactId?: string | null; avatarUrl?: string | null }): void;
  (event: "open-contact-card", value: { name: string; phone: string; contactId?: string | null; avatarUrl?: string | null }): void;
  (event: "update:draft", value: string): void;
  (event: "pick-attachment", value: { file: File | null; mode: AttachmentPickerMode; durationSeconds?: number | null }): void;
  (event: "clear-attachment"): void;
  (event: "update:show-outbound-operator-label", value: boolean): void;
}>();
const { token, coreToken, tenantSlug } = useAdminSession();
const { apiFetch } = useApi();
const {
  fileInputRef,
  contactPickerRef,
  emojiPanelRef,
  setFileInputElement,
  setContactPickerElement,
  setComposerInputElement,
  setEmojiPanelElement,
  setEmojiSearchInputElement,
  setEmojiTriggerElement,
  resolveComposerTextareaElement,
  focusComposerTextarea,
  focusEmojiSearchInput,
  isInsideEmojiPanel,
  isInsideContactPicker
} = useInboxChatComposerDom();
const { getInitials, formatTime, getChannelLabel, getStatusLabel, getStatusColor } = useInboxChatPresentation();
const { formatContactDisplayPhone, getMessageContactCard, getMessageContactActionPayload } = useInboxChatMessageContact({
  getSavedContacts: () => props.savedContacts
});
const {
  escapeHtml,
  escapeHtmlAttribute,
  escapeRegExp,
  normalizeDigits,
  normalizeNameForComparison,
  normalizeJidForComparison,
  normalizeLinkUrl,
  extractFirstLinkFromText,
  extractLinksFromText,
  extractLinkHost,
  formatFileSize,
  asRecord,
  splitLegacyReplyContent,
  parseUnsupportedLabelFromPlaceholder,
  isMediaPlaceholder,
  sanitizeHumanLabel,
  normalizeMentionJid,
  normalizeMentionLabel
} = useInboxChatUtilities();
const {
  resolveMessageParticipantJid,
  resolveMessageAuthor,
  resolveOutboundOperatorLabel,
  resolveMessageAvatarUrl,
  resolveAudioAvatarUrl,
  messageRowId
} = useInboxChatMessageIdentity({
  getActiveConversation: () => props.activeConversation,
  getCurrentUserId: () => props.currentUserId,
  getCurrentUserName: () => props.currentUserName,
  getShowOutboundOperatorLabel: () => props.showOutboundOperatorLabel,
  getIsGroupConversation: () => props.isGroupConversation,
  getGroupParticipants: () => props.groupParticipants,
  getMessageRenderItems: () => props.messageRenderItems,
  asRecord,
  normalizeDigits,
  normalizeJidForComparison,
  normalizeNameForComparison,
  sanitizeHumanLabel
});
const {
  getReactionBadges,
  buildReactionMenuItems,
  toggleReactionBadge
} = useInboxChatReactions({
  getCurrentUserId: () => props.currentUserId,
  getCanManageConversation: () => props.canManageConversation,
  asRecord,
  onSetReaction: (payload) => emit("set-reaction", payload)
});
const chatBodyRef = ref<HTMLElement | null>(null);
let closeEmojiPanelDelegate: ((options?: { resetSearch?: boolean }) => void) | null = null;
function closeEmojiPanel(options: { resetSearch?: boolean } = {}) {
  closeEmojiPanelDelegate?.(options);
  if (options.resetSearch !== false) {
    gifError.value = "";
  }
}
const {
  isRecording,
  recordingElapsedLabel,
  recordingError,
  recordingWaveLevels,
  stopRecording,
  toggleRecording,
  cancelRecording,
  sendRecordedAudio,
  disposeRecording
} = useInboxChatAudioRecorder({
  getHasActiveConversation: () => Boolean(props.activeConversation),
  onPickAttachment: (payload) => emit("pick-attachment", payload),
  onAutoSend: () => emit("send")
});
const {
  pickerMode,
  linkPreviewEnabled,
  contactPickerOpen,
  contactPickerSearch,
  updateContactPickerSearch,
  filteredSavedContacts,
  composerHasContent,
  isComposerReadOnly,
  showLinkPreviewToggle,
  pickerAccept,
  pickerCapture,
  closeContactPicker,
  openAttachmentPicker,
  openContactPrompt,
  selectSavedContactForSend,
  toggleLinkPreview,
  attachmentMenuItems
} = useInboxChatComposerControls({
  getSavedContacts: () => props.savedContacts,
  getDraft: () => props.draft,
  getHasAttachment: () => props.hasAttachment,
  getCanManageConversation: () => props.canManageConversation,
  getActiveConversation: () => props.activeConversation,
  getIsRecording: () => isRecording.value,
  fileInputRef,
  extractFirstLinkFromText,
  stopRecordingForPicker: () => stopRecording(true),
  closeEmojiPanel,
  onSendContact: (payload) => emit("send-contact", payload)
});
const {
  recentEmojis,
  emojiPickerCategoryDefinitions,
  emojiPickerSearchIndex,
  emojiPickerLoading,
  emojiPickerError,
  gifSearch,
  gifLoading,
  gifError,
  gifResults,
  savedStickers,
  savedStickersLoading,
  stickerError,
  updateGifSearch,
  loadRecentEmojis,
  pushRecentEmoji,
  loadSavedStickers,
  saveStickerFromFile,
  selectSavedSticker,
  removeSavedSticker,
  ensureEmojiPickerLoaded,
  clearGifSearchTimer,
  queueGifSearch,
  pickGifResult
} = useInboxChatEmojiAssets({
  apiFetch,
  asRecord,
  normalizeNameForComparison,
  onPickAttachment: (payload) => emit("pick-attachment", payload),
  onClosePanel: () => closeEmojiPanel({ resetSearch: false })
});

function setChatBodyElement(element: Element | null) {
  chatBodyRef.value = element instanceof HTMLElement ? element : null;
  emit("body-mounted", chatBodyRef.value);
}

function canFocusComposer() {
  return Boolean(props.activeConversation && props.canManageConversation && !isRecording.value);
}

function focusComposerInput(options?: { cursorAtEnd?: boolean }) {
  if (!canFocusComposer()) {
    return false;
  }

  return focusComposerTextarea({
    cursorAtEnd: options?.cursorAtEnd ?? true
  });
}

function focusComposerInputSoon(options?: { cursorAtEnd?: boolean }) {
  const attemptFocus = () => {
    const focused = focusComposerInput(options);
    if (focused) {
      onComposerCursorUpdate();
    }

    return focused;
  };

  void nextTick(() => {
    if (attemptFocus()) {
      return;
    }

    if (!import.meta.client) {
      return;
    }

    requestAnimationFrame(() => {
      if (attemptFocus()) {
        return;
      }

      window.setTimeout(() => {
        if (attemptFocus()) {
          return;
        }

        window.setTimeout(() => {
          if (attemptFocus()) {
            return;
          }

          window.setTimeout(() => {
            attemptFocus();
          }, 180);
        }, 48);
      }, 0);
    });
  });
}

function onSetReplyWithFocus(messageEntry: Message) {
  emit("set-reply", messageEntry);
  focusComposerInputSoon();
}

function onClearReplyWithFocus() {
  emit("clear-reply");
  focusComposerInputSoon();
}

function isTypingShortcutKey(event: KeyboardEvent) {
  if (event.key === "Enter") {
    return true;
  }

  return event.key.length === 1 && event.key !== "Dead" && !event.isComposing;
}

function isTextEditingElement(element: Element | null) {
  if (!(element instanceof HTMLElement)) {
    return false;
  }

  if (element.isContentEditable) {
    return true;
  }

  if (
    element instanceof HTMLInputElement ||
    element instanceof HTMLTextAreaElement ||
    element instanceof HTMLSelectElement
  ) {
    return true;
  }

  const role = element.getAttribute("role")?.trim().toLowerCase();
  return role === "textbox" || role === "searchbox" || role === "combobox";
}

function isInsideInboxPanel(element: Element | null) {
  return element instanceof HTMLElement && Boolean(element.closest("#omni-inbox-center"));
}

function shouldHandleGlobalComposerKeydown(event: KeyboardEvent) {
  if (!import.meta.client || !canFocusComposer()) {
    return false;
  }

  if (event.defaultPrevented || event.ctrlKey || event.altKey || event.metaKey) {
    return false;
  }

  if (!isTypingShortcutKey(event)) {
    return false;
  }

  const activeElement = document.activeElement;
  return (
    !isTextEditingElement(activeElement) &&
    (
      !activeElement ||
      activeElement === document.body ||
      isInsideInboxPanel(activeElement)
    )
  );
}

function shouldHandleGlobalReplyEscape(event: KeyboardEvent) {
  if (!import.meta.client || !canFocusComposer()) {
    return false;
  }

  if (event.defaultPrevented || event.ctrlKey || event.altKey || event.metaKey) {
    return false;
  }

  if (event.key !== "Escape" || !props.replyTarget) {
    return false;
  }

  const activeElement = document.activeElement;
  return (
    !isTextEditingElement(activeElement) &&
    (
      !activeElement ||
      activeElement === document.body ||
      isInsideInboxPanel(activeElement)
    )
  );
}

function onGlobalComposerKeydown(event: KeyboardEvent) {
  if (shouldHandleGlobalReplyEscape(event)) {
    event.preventDefault();
    onClearReplyWithFocus();
    return;
  }

  if (!shouldHandleGlobalComposerKeydown(event)) {
    return;
  }

  if (event.key === "Enter") {
    event.preventDefault();
    emitSendFromComposer();
    focusComposerInputSoon();
    return;
  }

  event.preventDefault();
  emit("update:draft", `${props.draft}${event.key}`);
  void nextTick(() => {
    onComposerInput();
    focusComposerInput();
  });
}

function onChatBodyClick() {
  // Deliberadamente sem auto-foco no corpo do chat.
  // O foco deve ocorrer apenas por clique direto no input
  // ou quando o usuario comeca a digitar pelo atalho global.
}

const draftModel = computed({
  get: () => props.draft,
  set: (value: string) => emit("update:draft", value)
});
const {
  mentionQuery,
  mentionSelectedIndex,
  mentionOptions,
  isMentionPickerVisible,
  resetMentionContext,
  updateMentionContextFromCursor,
  pruneDraftMentionSelections,
  pickMentionOption,
  handleMentionKeydown,
  buildMentionSendPayload,
  getMentionDisplayFromMetadata,
  collectMentionDisplayEntries,
  resolveMentionRouteTarget,
  onMessageTextClick,
  clearMentionDraftState
} = useInboxChatMentions({
  getIsGroupConversation: () => props.isGroupConversation,
  getActiveConversation: () => props.activeConversation,
  getDraft: () => draftModel.value,
  setDraft: (value) => {
    draftModel.value = value;
  },
  getGroupParticipants: () => props.groupParticipants,
  getMessageRenderItems: () => props.messageRenderItems,
  resolveComposerTextareaElement,
  resolveMessageParticipantJid,
  asRecord,
  normalizeDigits,
  normalizeMentionJid: (value: string) => normalizeMentionJid(value) ?? "",
  normalizeMentionLabel: (value: string) => normalizeMentionLabel(value) ?? "",
  sanitizeHumanLabel,
  onOpenMention: (payload) => emit("open-mention", payload)
});
const {
  emojiPanelTab,
  emojiSearch,
  emojiCategory,
  emojiPanelOpen,
  emojiCategories,
  showEmojiCategoryTabs,
  filteredEmojiList,
  activeEmojiCategoryLabel,
  updateEmojiPanelTab,
  updateEmojiSearch,
  updateEmojiCategory,
  toggleEmojiPanel,
  closeEmojiPanel: closeEmojiPanelState,
  insertEmoji
} = useInboxChatEmojiPanel({
  getActiveConversation: () => props.activeConversation,
  getCanManageConversation: () => props.canManageConversation,
  getDraft: () => draftModel.value,
  setDraft: (value) => {
    draftModel.value = value;
  },
  normalizeNameForComparison,
  recentEmojis,
  emojiPickerCategoryDefinitions,
  emojiPickerSearchIndex,
  gifSearch,
  pushRecentEmoji,
  loadRecentEmojis,
  loadSavedStickers,
  ensureEmojiPickerLoaded,
  clearGifSearchTimer,
  queueGifSearch,
  closeContactPicker,
  focusEmojiSearchInput,
  isInsideEmojiPanel,
  resolveComposerTextareaElement,
  pruneDraftMentionSelections,
  updateMentionContextFromCursor
});
closeEmojiPanelDelegate = (options = {}) => {
  closeEmojiPanelState(options);
  if (options.resetSearch !== false) {
    gifError.value = "";
  }
};
const {
  getMediaTypeLabel,
  hasUnsupportedNotice,
  getUnsupportedLabel,
  buildUnsupportedOpenUrl,
  isMentionAlertMessage,
  getReplyPreview,
  getRenderedMessageText,
  getMessageLinkPreview,
  shouldRenderMessageText: shouldRenderMessageTextBase,
  hasImagePreview,
  hasVideoPreview,
  hasAudioPreview,
  isVoiceNoteMessage,
  isAudioFileMessage,
  hasDocumentPreview,
  hasPendingMediaPreview,
  getPendingMediaLabel,
  getReplyAuthorLabel,
  getReplyTargetText,
  getReplyTargetAuthorLabel,
  hasReplyJumpTarget,
  onReplyPreviewClick,
  renderMessageHtml,
  markImageFailed,
  isImageFailed,
  getMessageMediaUrl,
  getMessageAudioPreviewUrl,
  getMessageImagePreviewUrl,
  isImagePreviewLoading,
  isAudioPreviewLoading,
  requestImagePreview,
  requestAudioPreview,
  isMediaActionLoading,
  openMessageMedia,
  downloadMessageMedia,
  resolveMessageFileName,
  disposeMessageHelpers
} = useInboxChatMessageHelpers({
  getActiveConversation: () => props.activeConversation,
  getIsGroupConversation: () => props.isGroupConversation,
  getToken: () => coreToken.value || token.value,
  getSelectedTenantSlug: () => tenantSlug.value,
  resolveMessageAuthor,
  resolveOutboundOperatorLabel,
  sanitizeHumanLabel,
  requestOlderMessages: () => emit("load-older-messages"),
  hasMoreMessages: () => props.hasMoreMessages,
  messageRowId,
  asRecord,
  splitLegacyReplyContent,
  parseUnsupportedLabelFromPlaceholder,
  isMediaPlaceholder,
  extractFirstLinkFromText,
  extractLinkHost,
  normalizeLinkUrl,
  escapeHtml,
  escapeHtmlAttribute,
  escapeRegExp,
  getMentionDisplayFromMetadata,
  collectMentionDisplayEntries,
  resolveMentionRouteTarget: (messageEntry, mentionToken) => {
    const target = resolveMentionRouteTarget(messageEntry, mentionToken);
    return {
      jid: target.jid ?? null,
      phone: target.phone ?? null
    };
  }
});
const { shouldShowReplyTypeMeta, getReplyPreviewText, getReplyTypeIcon } = useInboxChatReplyMeta({
  getMediaTypeLabel,
  normalizeNameForComparison
});

function shouldRenderMessageText(messageEntry: Message) {
  return shouldRenderMessageTextBase(messageEntry, Boolean(getMessageContactCard(messageEntry)));
}

const {
  selectionMode,
  selectedMessageCount,
  canForwardSelectedMessages,
  canDeleteSelectedForAll,
  selectionStatusMessage,
  selectionActionPending,
  forwardModalOpen,
  forwardConversationOptions,
  isMessageSelected,
  startMessageSelection,
  toggleMessageSelection,
  clearSelection,
  closeForwardDialog,
  deleteMessagesForMe,
  deleteMessagesForAll,
  requestForwardMessages,
  forwardMessagesToConversation
} = useInboxChatSelection({
  getMessageRenderItems: () => props.messageRenderItems,
  getConversationOptions: () => props.conversationOptions,
  getActiveConversationId: () => props.activeConversation?.id ?? null,
  deleteMessagesForMeAction: props.deleteMessagesForMeAction,
  deleteMessagesForAllAction: props.deleteMessagesForAllAction,
  forwardMessagesAction: props.forwardMessagesAction
});

watch(
  () => props.draft,
  (value) => {
    if (!value.trim()) {
      linkPreviewEnabled.value = true;
      return;
    }

    if (!extractFirstLinkFromText(value)) {
      linkPreviewEnabled.value = true;
    }
  }
);

watch(
  () => props.activeConversation?.id,
  () => {
    closeEmojiPanel();
    linkPreviewEnabled.value = true;
    clearSelection();
    focusComposerInputSoon();
  }
);

watch(
  () => props.replyTarget?.id,
  (nextReplyId, previousReplyId) => {
    if (!nextReplyId || nextReplyId === previousReplyId) {
      return;
    }

    focusComposerInputSoon();
  },
  { flush: "post" }
);

function onContactPickerPointerDownOutside(event: MouseEvent) {
  if (!contactPickerOpen.value) {
    return;
  }

  const target = event.target as HTMLElement | null;
  if (!target) {
    return;
  }

  if (isInsideContactPicker(target)) {
    return;
  }

  closeContactPicker();
}

function openStickerFromEmojiPanel() {
  closeEmojiPanel({ resetSearch: false });
  openAttachmentPicker("sticker");
}

function emitSendFromComposer() {
  const uniqueMentionedJids = buildMentionSendPayload();
  const sendPayload: MentionSendPayload = {
    mentionedJids: uniqueMentionedJids
  };

  if (showLinkPreviewToggle.value) {
    sendPayload.linkPreviewEnabled = linkPreviewEnabled.value;
  }

  if (uniqueMentionedJids.length > 0 || showLinkPreviewToggle.value) {
    emit("send", sendPayload);
  } else {
    emit("send");
  }

  clearMentionDraftState();
  closeEmojiPanel({ resetSearch: false });
}

function onComposerInput() {
  pruneDraftMentionSelections();
  updateMentionContextFromCursor();

  if (!draftModel.value.trim()) {
    emojiCategory.value = "recents";
  }
}

function onComposerCursorUpdate() {
  updateMentionContextFromCursor();
}

function onComposerKeydown(event: KeyboardEvent) {
  if (handleMentionKeydown(event)) {
    return;
  }

  if (event.key === "Escape") {
    if (emojiPanelOpen.value) {
      event.preventDefault();
      closeEmojiPanel({ resetSearch: false });
      return;
    }

    if (contactPickerOpen.value) {
      event.preventDefault();
      closeContactPicker();
      return;
    }

    if (props.replyTarget) {
      event.preventDefault();
      onClearReplyWithFocus();
      return;
    }
  }

  if (event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();
    emitSendFromComposer();
  }
}

onMounted(() => {
  if (import.meta.client) {
    document.addEventListener("mousedown", onContactPickerPointerDownOutside);
    document.addEventListener("keydown", onGlobalComposerKeydown);
  }

  void nextTick(() => {
    stripChatPanelMinHeightClass();
    focusComposerInput();
  });
});

onBeforeUnmount(() => {
  disposeRecording();
  disposeMessageHelpers();
  clearMentionDraftState();
  closeEmojiPanel();
  closeContactPicker();
  if (import.meta.client) {
    document.removeEventListener("mousedown", onContactPickerPointerDownOutside);
    document.removeEventListener("keydown", onGlobalComposerKeydown);
  }
  emit("body-mounted", null);
});

function stripChatPanelMinHeightClass() {
  if (!import.meta.client) {
    return;
  }

  document
    .querySelectorAll<HTMLElement>(".chat-page__chat.min-h-svh")
    .forEach((panelElement) => {
      panelElement.classList.remove("min-h-svh");
    });
}

onUpdated(() => {
  stripChatPanelMinHeightClass();
});

function onSaveMessageContact(messageEntry: Message) {
  const payload = getMessageContactActionPayload(messageEntry);
  if (!payload) {
    return;
  }

  emit("save-contact-card", payload);
}

function onOpenMessageContact(messageEntry: Message) {
  const payload = getMessageContactActionPayload(messageEntry);
  if (!payload) {
    return;
  }

  emit("open-contact-card", payload);
}

function onForwardModalOpenChange(value: boolean) {
  if (!value) {
    closeForwardDialog();
  }
}

async function onFileChange(event: Event) {
  const target = event.target as HTMLInputElement | null;
  const file = target?.files?.[0] ?? null;

  if (file && pickerMode.value === "sticker") {
    void saveStickerFromFile(file);
  }

  emit("pick-attachment", {
    file,
    mode: pickerMode.value
  });

  if (target) {
    target.value = "";
  }
}

const headerBindings = computed(() => ({
  activeConversation: props.activeConversation,
  activeConversationLabel: props.activeConversationLabel,
  userRole: props.userRole,
  showOutboundOperatorLabel: props.showOutboundOperatorLabel,
  canManageConversation: props.canManageConversation,
  getInitials,
  getChannelLabel,
  getStatusColor,
  getStatusLabel,
  onOpenWhatsAppSession: () => emit("open-whatsapp-session"),
  onToggleShowOutboundOperatorLabel: (value: boolean) => emit("update:show-outbound-operator-label", value),
  onCloseConversation: () => emit("close-conversation")
}));

const bodyBindings = computed(() => ({
  activeConversation: props.activeConversation,
  loadingMessages: props.loadingMessages,
  loadingOlderMessages: props.loadingOlderMessages,
  hasMoreMessages: props.hasMoreMessages,
  showLoadOlderMessagesButton: props.showLoadOlderMessagesButton,
  showScrollToLatestButton: props.showScrollToLatestButton,
  messageRenderItems: props.messageRenderItems,
  selectionMode: selectionMode.value,
  selectedMessageCount: selectedMessageCount.value,
  canDeleteSelectedForAll: canDeleteSelectedForAll.value,
  canForwardSelectedMessages: canForwardSelectedMessages.value,
  selectionStatusMessage: selectionStatusMessage.value,
  showStickyDate: props.showStickyDate,
  stickyDateLabel: props.stickyDateLabel,
  isGroupConversation: props.isGroupConversation,
  showOutboundOperatorLabel: props.showOutboundOperatorLabel,
  canManageConversation: props.canManageConversation,
  chatBodyRef: setChatBodyElement,
  onChatScroll: (payload: Event) => emit("chat-scroll", payload),
  onLoadOlderMessages: () => emit("load-older-messages"),
  onScrollToLatest: () => emit("scroll-to-latest"),
  onChatBodyClick,
  onSetReply: onSetReplyWithFocus,
  isMessageSelected,
  onStartMessageSelection: startMessageSelection,
  onToggleMessageSelection: toggleMessageSelection,
  onDeleteMessageForMe: (messageId: string) => deleteMessagesForMe([messageId]),
  onDeleteMessageForAll: (messageId: string) => deleteMessagesForAll([messageId]),
  onForwardMessage: (messageId: string) => requestForwardMessages([messageId]),
  onClearSelection: clearSelection,
  onDeleteSelectedForMe: () => deleteMessagesForMe(),
  onDeleteSelectedForAll: () => deleteMessagesForAll(),
  onForwardSelectedMessages: () => requestForwardMessages(),
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
}));

const footerBindings = computed(() => ({
  activeConversation: props.activeConversation,
  canManageConversation: props.canManageConversation,
  draft: props.draft,
  hasAttachment: props.hasAttachment,
  attachmentType: props.attachmentType,
  attachmentName: props.attachmentName,
  attachmentMimeType: props.attachmentMimeType,
  attachmentSizeBytes: props.attachmentSizeBytes,
  attachmentDurationSeconds: props.attachmentDurationSeconds,
  attachmentPreviewUrl: props.attachmentPreviewUrl,
  sendingMessage: props.sendingMessage,
  sendError: props.sendError,
  replyTarget: props.replyTarget,
  isRecording: isRecording.value,
  recordingElapsedLabel: recordingElapsedLabel.value,
  recordingWaveLevels: recordingWaveLevels.value,
  recordingError: recordingError.value,
  isComposerReadOnly: isComposerReadOnly.value,
  composerInputRef: setComposerInputElement,
  fileInputRef: setFileInputElement,
  pickerAccept: pickerAccept.value,
  pickerCapture: pickerCapture.value,
  attachmentMenuItems: attachmentMenuItems.value,
  contactPickerOpen: contactPickerOpen.value,
  contactPickerRef: setContactPickerElement,
  contactPickerSearch: contactPickerSearch.value,
  onUpdateContactPickerSearch: updateContactPickerSearch,
  closeContactPicker,
  filteredSavedContacts: filteredSavedContacts.value,
  selectSavedContactForSend,
  formatContactDisplayPhone,
  getInitials,
  emojiPanelOpen: emojiPanelOpen.value,
  emojiPanelRef: setEmojiPanelElement,
  emojiSearchInputRef: setEmojiSearchInputElement,
  emojiTriggerRef: setEmojiTriggerElement,
  toggleEmojiPanel,
  EMOJI_PANEL_TAB_ITEMS,
  emojiPanelTab: emojiPanelTab.value,
  onUpdateEmojiPanelTab: updateEmojiPanelTab,
  emojiSearch: emojiSearch.value,
  onUpdateEmojiSearch: updateEmojiSearch,
  emojiPickerLoading: emojiPickerLoading.value,
  emojiPickerError: emojiPickerError.value,
  showEmojiCategoryTabs: showEmojiCategoryTabs.value,
  emojiCategories: emojiCategories.value,
  emojiCategory: emojiCategory.value,
  updateEmojiCategory,
  activeEmojiCategoryLabel: activeEmojiCategoryLabel.value,
  filteredEmojiList: filteredEmojiList.value,
  insertEmoji,
  gifSearch: gifSearch.value,
  onUpdateGifSearch: updateGifSearch,
  gifLoading: gifLoading.value,
  gifError: gifError.value,
  gifResults: gifResults.value,
  pickGifResult,
  savedStickersLoading: savedStickersLoading.value,
  savedStickers: savedStickers.value,
  stickerError: stickerError.value,
  selectSavedSticker,
  removeSavedSticker,
  openStickerFromEmojiPanel,
  isMentionPickerVisible: isMentionPickerVisible.value,
  mentionOptions: mentionOptions.value,
  mentionSelectedIndex: mentionSelectedIndex.value,
  pickMentionOption,
  loadingGroupParticipants: props.loadingGroupParticipants,
  showLinkPreviewToggle: showLinkPreviewToggle.value,
  linkPreviewEnabled: linkPreviewEnabled.value,
  toggleLinkPreview,
  composerHasContent: composerHasContent.value,
  onComposerKeydown,
  onComposerInput,
  onComposerCursorUpdate,
  emitSendFromComposer,
  toggleRecording,
  cancelRecording,
  sendRecordedAudio,
  onFileChange,
  onClearReply: onClearReplyWithFocus,
  onUpdateDraft: (value: string) => emit("update:draft", value),
  onClearAttachment: () => emit("clear-attachment"),
  getReplyTargetAuthorLabel,
  shouldShowReplyTypeMeta,
  resolveMessageType,
  getReplyTypeIcon,
  getMediaTypeLabel,
  getReplyTargetText,
  formatFileSize
}));
</script>
<template>
  <UDashboardPanel
    id="omni-inbox-center"
    :ui="{
      root: 'chat-page__chat !min-h-0 !h-full',
      body: 'chat-page__panel-body chat-page__chat-panel-body'
    }"
  >
    <template #header>
      <div class="chat-page__panel-header-slot">
        <InboxChatHeader v-bind="headerBindings" />
      </div>
    </template>

    <template #body>
      <InboxChatBody v-bind="bodyBindings" />
    </template>

    <template #footer>
      <div class="chat-page__panel-footer-slot">
        <InboxChatFooter v-bind="footerBindings" />
      </div>
    </template>
  </UDashboardPanel>

  <InboxForwardMessagesModal
    :open="forwardModalOpen"
    :loading="selectionActionPending"
    :selected-count="selectedMessageCount"
    :conversations="forwardConversationOptions"
    :active-conversation-id="activeConversation?.id ?? null"
    @update:open="onForwardModalOpenChange"
    @select-conversation="forwardMessagesToConversation"
  />
</template>

<style scoped>
.chat-page__chat {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  padding: 12px 4px;
}

.chat-page__panel-header-slot,
.chat-page__panel-footer-slot {
  padding: 0;
}

.chat-page__panel-body,
.chat-page__chat-panel-body {
  flex: 1;
  min-height: 0;
  padding: 8px 4px !important;
}

.chat-page__chat-panel-body {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 0 !important;
}
</style>
