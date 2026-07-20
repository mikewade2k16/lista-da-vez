import type { Ref } from "vue";
import type { Conversation, Message } from "~/types";
import type { ReadStateEntry } from "~/composables/omnichannel/useOmnichannelInboxShared";

export function useOmnichannelInboxReadState(options: {
  readState: Ref<Record<string, ReadStateEntry>>;
  activeConversationId: Ref<string | null>;
  messages: Ref<Message[]>;
  conversations: Ref<Conversation[]>;
  readStateStorageKey: Ref<string>;
  clearMentionAlert: (conversationId: string) => void;
}) {
  function getReadAt(conversationId: string) {
    const entry = options.readState.value[conversationId];
    if (!entry?.lastReadAt) {
      return null;
    }

    const timestamp = Number(new Date(entry.lastReadAt));
    return Number.isFinite(timestamp) ? timestamp : null;
  }

  function loadReadState() {
    if (!import.meta.client) {
      return;
    }

    const raw = localStorage.getItem(options.readStateStorageKey.value);
    if (!raw) {
      options.readState.value = {};
      return;
    }

    try {
      const parsed = JSON.parse(raw) as Record<string, ReadStateEntry>;
      options.readState.value = parsed;
    } catch {
      options.readState.value = {};
    }
  }

  function saveReadState() {
    if (!import.meta.client) {
      return;
    }

    localStorage.setItem(options.readStateStorageKey.value, JSON.stringify(options.readState.value));
  }

  function bootstrapReadState() {
    if (Object.keys(options.readState.value).length > 0) {
      return;
    }

    const nextState: Record<string, ReadStateEntry> = {};

    for (const conversationEntry of options.conversations.value) {
      nextState[conversationEntry.id] = {
        lastReadAt: conversationEntry.lastMessageAt,
        lastReadMessageId: conversationEntry.lastMessage?.id ?? null
      };
    }

    options.readState.value = nextState;
    saveReadState();
  }

  function markConversationAsRead(messageEntry?: Message) {
    const conversationId = options.activeConversationId.value;
    if (!conversationId) {
      return;
    }

    const targetMessage = messageEntry ?? options.messages.value[options.messages.value.length - 1];
    if (!targetMessage) {
      return;
    }

    const currentReadAt = getReadAt(conversationId);
    const nextReadAt = Number(new Date(targetMessage.createdAt));

    if (currentReadAt && currentReadAt >= nextReadAt) {
      return;
    }

    options.readState.value = {
      ...options.readState.value,
      [conversationId]: {
        lastReadAt: targetMessage.createdAt,
        lastReadMessageId: targetMessage.id
      }
    };
    saveReadState();
    options.clearMentionAlert(conversationId);
  }

  function isConversationUnread(conversationEntry: Conversation) {
    const readAt = getReadAt(conversationEntry.id);

    if (!readAt) {
      return false;
    }

    return Number(new Date(conversationEntry.lastMessageAt)) > readAt;
  }

  return {
    getReadAt,
    loadReadState,
    saveReadState,
    bootstrapReadState,
    markConversationAsRead,
    isConversationUnread
  };
}
