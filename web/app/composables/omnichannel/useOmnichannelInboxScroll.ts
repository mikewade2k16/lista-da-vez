import { nextTick, type ComputedRef, type Ref } from "vue";
import type { Conversation, Message } from "~/types";
import { UNASSIGNED_VALUE, isNearBottom } from "~/composables/omnichannel/useOmnichannelInboxShared";

export function useOmnichannelInboxScroll(options: {
  chatBodyRef: Ref<HTMLElement | null>;
  messages: Ref<Message[]>;
  hasMoreMessages: Ref<boolean>;
  loadingOlderMessages: Ref<boolean>;
  showLoadOlderMessagesButton: Ref<boolean>;
  showScrollToLatestButton: Ref<boolean>;
  showStickyDate: Ref<boolean>;
  stickyDateLabel: Ref<string>;
  firstUnreadMessageId: ComputedRef<string | null>;
  conversations: Ref<Conversation[]>;
  activeConversationId: Ref<string | null>;
  assigneeModel: Ref<string>;
  draft: Ref<string>;
  replyTarget: Ref<Message | null>;
  clearAttachment: () => void;
  loadConversationMessages: (conversationId: string) => Promise<void>;
  loadGroupParticipants: (conversationId: string) => Promise<void>;
  loadOlderMessages: () => Promise<void>;
  markConversationAsRead: (messageEntry?: Message) => void;
  messageRowId: (messageId: string) => string;
}) {
  let scrollRaf: number | null = null;
  const conversationScrollStateById = new Map<string, {
    anchorMessageId: string | null;
    anchorOffsetTop: number;
    scrollTop: number;
    nearBottom: boolean;
  }>();

  function scheduleStickyDateRefresh() {
    if (scrollRaf !== null) {
      cancelAnimationFrame(scrollRaf);
    }

    scrollRaf = requestAnimationFrame(() => {
      refreshStickyDate();
      scrollRaf = null;
    });
  }

  function cancelScheduledStickyRefresh() {
    if (scrollRaf !== null) {
      cancelAnimationFrame(scrollRaf);
      scrollRaf = null;
    }
  }

  function refreshStickyDate() {
    const container = options.chatBodyRef.value;
    if (!container) {
      options.showStickyDate.value = false;
      options.stickyDateLabel.value = "";
      options.showScrollToLatestButton.value = false;
      return;
    }

    const rows = container.querySelectorAll<HTMLElement>(".chat-message-row");
    if (!rows.length) {
      options.showStickyDate.value = false;
      options.stickyDateLabel.value = "";
      return;
    }

    const topLimit = container.getBoundingClientRect().top + 16;
    let selectedRow: HTMLElement | null = null;

    for (const row of rows) {
      const rect = row.getBoundingClientRect();
      if (rect.top <= topLimit) {
        selectedRow = row;
        continue;
      }

      break;
    }

    if (!selectedRow) {
      selectedRow = rows[0] ?? null;
    }

    if (!selectedRow) {
      options.showStickyDate.value = false;
      options.stickyDateLabel.value = "";
      return;
    }

    options.stickyDateLabel.value = selectedRow.dataset.dateLabel ?? "";
    options.showStickyDate.value = container.scrollTop > 24 && options.stickyDateLabel.value.length > 0;
  }

  function refreshScrollAffordances() {
    const container = options.chatBodyRef.value;
    if (!container) {
      options.showLoadOlderMessagesButton.value = false;
      options.showScrollToLatestButton.value = false;
      options.showStickyDate.value = false;
      options.stickyDateLabel.value = "";
      return;
    }

    if (!options.hasMoreMessages.value || options.loadingOlderMessages.value || !options.messages.value.length) {
      options.showLoadOlderMessagesButton.value = false;
    } else if (container.scrollTop < 96) {
      options.showLoadOlderMessagesButton.value = true;
    } else if (container.scrollTop > 160) {
      options.showLoadOlderMessagesButton.value = false;
    }

    options.showScrollToLatestButton.value = !isNearBottom(container) && container.scrollTop > 180;
    refreshStickyDate();
  }

  function extractMessageIdFromRow(row: HTMLElement) {
    const rowId = row.id.trim();
    const prefix = "chat-message-";
    if (!rowId.startsWith(prefix)) {
      return null;
    }

    const messageId = rowId.slice(prefix.length).trim();
    return messageId.length > 0 ? messageId : null;
  }

  function captureConversationScrollState(conversationId = options.activeConversationId.value) {
    const container = options.chatBodyRef.value;
    if (!container || !conversationId) {
      return;
    }

    const nearBottom = isNearBottom(container);
    if (nearBottom) {
      conversationScrollStateById.set(conversationId, {
        anchorMessageId: null,
        anchorOffsetTop: 0,
        scrollTop: container.scrollTop,
        nearBottom: true
      });
      return;
    }

    const containerRect = container.getBoundingClientRect();
    const rows = [...container.querySelectorAll<HTMLElement>(".chat-message-row")];
    const anchorRow = rows.find((row) => row.getBoundingClientRect().bottom > containerRect.top + 12) ?? rows[0] ?? null;
    const anchorMessageId = anchorRow ? extractMessageIdFromRow(anchorRow) : null;
    const anchorOffsetTop = anchorRow ? anchorRow.offsetTop - container.scrollTop : 0;

    conversationScrollStateById.set(conversationId, {
      anchorMessageId,
      anchorOffsetTop,
      scrollTop: container.scrollTop,
      nearBottom: false
    });
  }

  function restoreConversationScrollState(conversationId: string) {
    const container = options.chatBodyRef.value;
    const snapshot = conversationScrollStateById.get(conversationId);
    if (!container || !snapshot) {
      return false;
    }

    if (snapshot.nearBottom) {
      scrollToBottom();
      return true;
    }

    if (snapshot.anchorMessageId) {
      const anchorRow = document.getElementById(options.messageRowId(snapshot.anchorMessageId));
      if (anchorRow instanceof HTMLElement) {
        container.scrollTop = Math.max(anchorRow.offsetTop - snapshot.anchorOffsetTop, 0);
        return true;
      }
    }

    container.scrollTop = Math.max(snapshot.scrollTop, 0);
    return true;
  }

  function resolveLastMessageRow(container: HTMLElement) {
    const rows = container.querySelectorAll<HTMLElement>(".chat-message-row");
    return rows[rows.length - 1] ?? null;
  }

  function setContainerScrollTop(container: HTMLElement, top: number, behavior: ScrollBehavior = "auto") {
    const normalizedTop = Math.max(top, 0);

    if (typeof container.scrollTo === "function") {
      container.scrollTo({
        top: normalizedTop,
        behavior
      });
      return;
    }

    container.scrollTop = normalizedTop;
  }

  function scrollToBottom(config: { behavior?: ScrollBehavior } = {}) {
    const container = options.chatBodyRef.value;
    if (!container) {
      return;
    }

    const behavior = config.behavior ?? "auto";
    const lastRow = resolveLastMessageRow(container);

    if (lastRow && typeof lastRow.scrollIntoView === "function") {
      lastRow.scrollIntoView({
        block: "end",
        behavior
      });
    }

    setContainerScrollTop(container, container.scrollHeight, behavior);
    options.showScrollToLatestButton.value = false;
  }

  function scrollToLatest() {
    scrollToBottom({
      behavior: "smooth"
    });
    options.markConversationAsRead();
    options.showStickyDate.value = false;
    options.stickyDateLabel.value = "";

    void nextTick(() => {
      refreshScrollAffordances();
      scheduleStickyDateRefresh();

      requestAnimationFrame(() => {
        const container = options.chatBodyRef.value;
        if (!container) {
          return;
        }

        if (!isNearBottom(container)) {
          scrollToBottom({
            behavior: "smooth"
          });
        }

        window.setTimeout(() => {
          const currentContainer = options.chatBodyRef.value;
          if (!currentContainer) {
            return;
          }

          if (!isNearBottom(currentContainer)) {
            scrollToBottom();
          }

          options.markConversationAsRead();
          captureConversationScrollState();
          refreshScrollAffordances();
          scheduleStickyDateRefresh();
        }, 220);
      });
    });
  }

  async function positionConversationOnOpen(conversationId: string) {
    await nextTick();

    const container = options.chatBodyRef.value;
    if (!container) {
      return;
    }

    const unreadId = options.firstUnreadMessageId.value;
    const scrollState = conversationScrollStateById.get(conversationId) ?? null;

    if (unreadId && (!scrollState || scrollState.nearBottom)) {
      const marker = document.getElementById("chat-unread-anchor");
      if (marker) {
        container.scrollTop = Math.max(marker.offsetTop - 24, 0);
      } else {
        const messageRow = document.getElementById(options.messageRowId(unreadId));
        if (messageRow) {
          container.scrollTop = Math.max(messageRow.offsetTop - 24, 0);
        } else {
          scrollToBottom();
        }
      }
    } else if (restoreConversationScrollState(conversationId)) {
      if (scrollState?.nearBottom) {
        options.markConversationAsRead();
      }
    } else {
      scrollToBottom();
      options.markConversationAsRead();
    }

    captureConversationScrollState(conversationId);
    refreshScrollAffordances();
    scheduleStickyDateRefresh();
  }

  async function selectConversation(conversationId: string) {
    captureConversationScrollState();
    options.activeConversationId.value = conversationId;
    options.draft.value = "";
    options.replyTarget.value = null;
    options.showLoadOlderMessagesButton.value = false;
    options.showScrollToLatestButton.value = false;
    options.clearAttachment();
    options.assigneeModel.value = options.conversations.value.find((entry) => entry.id === conversationId)?.assignedToId ?? UNASSIGNED_VALUE;

    await options.loadConversationMessages(conversationId);
    void options.loadGroupParticipants(conversationId);
    await positionConversationOnOpen(conversationId);
  }

  function onChatScroll() {
    const container = options.chatBodyRef.value;
    if (!container) {
      return;
    }

    if (isNearBottom(container)) {
      options.markConversationAsRead();
    }

    captureConversationScrollState();
    refreshScrollAffordances();
    scheduleStickyDateRefresh();
  }

  function onChatBodyMounted(element: HTMLElement | null) {
    options.chatBodyRef.value = element;
    refreshScrollAffordances();
  }

  async function requestOlderMessages() {
    if (options.loadingOlderMessages.value || !options.hasMoreMessages.value) {
      return;
    }

    options.showLoadOlderMessagesButton.value = false;
    await options.loadOlderMessages();

    const container = options.chatBodyRef.value;
    if (
      container &&
      container.scrollTop < 96 &&
      options.hasMoreMessages.value &&
      !options.loadingOlderMessages.value &&
      options.messages.value.length > 0
    ) {
      options.showLoadOlderMessagesButton.value = true;
    }

    captureConversationScrollState();
    refreshScrollAffordances();
  }

  return {
    scheduleStickyDateRefresh,
    scrollToBottom,
    scrollToLatest,
    selectConversation,
    onChatScroll,
    onChatBodyMounted,
    requestOlderMessages,
    cancelScheduledStickyRefresh
  };
}
