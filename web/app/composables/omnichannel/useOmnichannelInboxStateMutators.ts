import type { ComputedRef, Ref } from "vue";
import type { Message } from "~/types";
import type {
  AttachmentSelectionPayload,
  InboxSidebarView
} from "~/composables/omnichannel/useOmnichannelInboxShared";

export function useOmnichannelInboxStateMutators(options: {
  replyTarget: Ref<Message | null>;
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
  internalNotes: ComputedRef<string>;
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
    updateInternalNotes
  };
}
