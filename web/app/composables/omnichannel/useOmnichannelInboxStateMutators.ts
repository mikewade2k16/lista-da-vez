import type { Ref, WritableComputedRef } from "vue";
import type { Conversation, Message } from "~/types";
import type {
  AttachmentSelectionPayload,
  InboxSidebarView
} from "~/composables/omnichannel/useOmnichannelInboxShared";

export function useOmnichannelInboxStateMutators(options: {
  replyTarget: Ref<Message | null>;
  conversations: Ref<Conversation[]>;
  messages: Ref<Message[]>;
  activeConversationId: Ref<string | null>;
  visibleMessagesConversationId: Ref<string | null>;
  hasMoreMessages: Ref<boolean>;
  showLoadOlderMessagesButton: Ref<boolean>;
  showScrollToLatestButton: Ref<boolean>;
  mentionAlertState: Ref<Record<string, number>>;
  draft: Ref<string>;
  search: Ref<string>;
  channel: Ref<string>;
  status: Ref<string>;
  instanceId: Ref<string>;
  sidebarView: Ref<InboxSidebarView>;
  showFilters: Ref<boolean>;
  leftCollapsed: Ref<boolean>;
  rightCollapsed: Ref<boolean>;
  assigneeModel: Ref<string>;
  contactActionError: Ref<string>;
  internalNotes: WritableComputedRef<string>;
  setAttachmentFromFile: (
    file: File | null,
    mode: AttachmentSelectionPayload["mode"],
    durationSeconds?: number | null
  ) => void;
}) {
  function setReplyTarget(messageEntry: Message) {
    // Keep reply target replacement explicit to support quick switch between messages.
    options.replyTarget.value = { ...messageEntry };
  }

  function clearReplyTarget() {
    options.replyTarget.value = null;
  }

  function updateDraft(value: string) {
    options.draft.value = value;
  }

  function updateAttachment(payload: AttachmentSelectionPayload) {
    options.setAttachmentFromFile(payload.file, payload.mode, payload.durationSeconds);
  }

  function updateSearch(value: string) {
    options.search.value = value;
  }

  function updateChannel(value: string) {
    options.channel.value = value;
  }

  function updateStatus(value: string) {
    options.status.value = value;
  }

  function updateInstanceId(value: string) {
    options.instanceId.value = value;
  }

  function updateSidebarView(value: InboxSidebarView) {
    options.sidebarView.value = value;
    options.contactActionError.value = "";
  }

  function updateShowFilters(value: boolean) {
    options.showFilters.value = value;
  }

  function updateLeftCollapsed(value: boolean) {
    options.leftCollapsed.value = value;
  }

  function updateRightCollapsed(value: boolean) {
    options.rightCollapsed.value = value;
  }

  function updateAssigneeModel(value: string) {
    options.assigneeModel.value = value;
  }

  function updateInternalNotes(value: string) {
    options.internalNotes.value = value;
  }

  function resetActiveConversationProjection() {
    options.activeConversationId.value = null;
    options.messages.value = [];
    options.visibleMessagesConversationId.value = null;
    options.hasMoreMessages.value = false;
    options.showLoadOlderMessagesButton.value = false;
    options.showScrollToLatestButton.value = false;
    options.replyTarget.value = null;
  }

  function clearConversationProjections(conversationIds: string[]) {
    const removedIds = new Set(conversationIds);
    if (removedIds.size < 1) {
      return;
    }

    options.conversations.value = options.conversations.value.filter(
      (entry) => !removedIds.has(entry.id)
    );
    options.mentionAlertState.value = Object.fromEntries(
      Object.entries(options.mentionAlertState.value).filter(
        ([conversationId]) => !removedIds.has(conversationId)
      )
    );

    const activeId = options.activeConversationId.value;
    const visibleId = options.visibleMessagesConversationId.value;
    if ((activeId && removedIds.has(activeId)) || (visibleId && removedIds.has(visibleId))) {
      resetActiveConversationProjection();
    }
  }

  function clearInstanceConversationProjection(instanceId: string, instanceScopeKey: string) {
    const conversationIds = options.conversations.value
      .filter((entry) => {
        if (entry.channel !== "WHATSAPP") {
          return false;
        }
        if (entry.instanceId === instanceId) {
          return true;
        }
        const hasLegacyInstanceId = entry.instanceId === null || entry.instanceId === undefined;
        return hasLegacyInstanceId && entry.instanceScopeKey === instanceScopeKey;
      })
      .map((entry) => entry.id);
    clearConversationProjections(conversationIds);
    return conversationIds;
  }

  function clearAllConversationProjections() {
    const conversationIds = options.conversations.value.map((entry) => entry.id);
    options.conversations.value = [];
    options.mentionAlertState.value = {};
    resetActiveConversationProjection();
    return conversationIds;
  }

  return {
    setReplyTarget,
    clearReplyTarget,
    updateDraft,
    updateAttachment,
    updateSearch,
    updateChannel,
    updateStatus,
    updateInstanceId,
    updateSidebarView,
    updateShowFilters,
    updateLeftCollapsed,
    updateRightCollapsed,
    updateAssigneeModel,
    updateInternalNotes,
    clearActiveConversationProjection: resetActiveConversationProjection,
    clearInstanceConversationProjection,
    clearAllConversationProjections
  };
}
