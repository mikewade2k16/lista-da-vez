import { ref, type ComputedRef, type Ref } from "vue";
import type { Conversation, Message } from "~/types";

interface DeleteMessagesForMeResponse {
  deletedIds: string[];
  skippedIds: string[];
  conversation: Conversation | null;
}

interface DeleteMessagesForAllResponse {
  updatedIds: string[];
  skippedIds: string[];
  failedIds: string[];
  messages: Message[];
}

interface ForwardMessagesResponse {
  sourceConversationId: string;
  targetConversationId: string;
  createdCount: number;
  queuedCount: number;
  failedToQueueCount: number;
  failedToQueueIds: string[];
  messages: Message[];
}

function normalizeUniqueIds(messageIds: string[]) {
  return [...new Set(messageIds.map((entry) => entry.trim()).filter(Boolean))];
}

export function useOmnichannelInboxMessageActions(options: {
  canManageConversation: ComputedRef<boolean>;
  activeConversationId: Ref<string | null>;
  conversations: Ref<Conversation[]>;
  messages: Ref<Message[]>;
  apiFetch: <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>;
  upsertConversation: (conversationEntry: Conversation) => void;
  mergeMessages: (...chunks: Message[][]) => Message[];
  updateConversationPreviewFromMessage: (messageEntry: Message) => void;
  scheduleStickyDateRefresh: () => void;
  loadConversationMessages: (
    conversationId: string,
    loadOptions?: { forceRefresh?: boolean }
  ) => Promise<void>;
}) {
  const processingMessageAction = ref(false);

  function assertWritableConversation() {
    if (!options.canManageConversation.value) {
      throw new Error("Seu perfil e somente leitura nesta inbox.");
    }

    const conversationId = options.activeConversationId.value;
    if (!conversationId) {
      throw new Error("Selecione uma conversa antes de executar esta acao.");
    }

    return conversationId;
  }

  async function deleteMessagesForMe(messageIds: string[]) {
    const conversationId = assertWritableConversation();
    const normalizedIds = normalizeUniqueIds(messageIds);
    if (!normalizedIds.length) {
      return {
        deletedIds: [],
        skippedIds: [],
        conversation: null
      } satisfies DeleteMessagesForMeResponse;
    }

    processingMessageAction.value = true;
    try {
      const response = await options.apiFetch<DeleteMessagesForMeResponse>(
        `/conversations/${conversationId}/messages/delete-for-me`,
        {
          method: "POST",
          body: {
            messageIds: normalizedIds
          }
        }
      );

      if (response.deletedIds.length > 0) {
        const deletedSet = new Set(response.deletedIds);
        options.messages.value = options.messages.value.filter((entry) => !deletedSet.has(entry.id));
        options.scheduleStickyDateRefresh();
      }

      if (response.conversation) {
        options.upsertConversation(response.conversation);
      }

      return response;
    } finally {
      processingMessageAction.value = false;
    }
  }

  async function deleteMessagesForAll(messageIds: string[]) {
    const conversationId = assertWritableConversation();
    const normalizedIds = normalizeUniqueIds(messageIds);
    if (!normalizedIds.length) {
      return {
        updatedIds: [],
        skippedIds: [],
        failedIds: [],
        messages: []
      } satisfies DeleteMessagesForAllResponse;
    }

    processingMessageAction.value = true;
    try {
      const response = await options.apiFetch<DeleteMessagesForAllResponse>(
        `/conversations/${conversationId}/messages/delete-for-all`,
        {
          method: "POST",
          body: {
            messageIds: normalizedIds
          }
        }
      );

      if (response.messages.length > 0) {
        options.messages.value = options.mergeMessages(options.messages.value, response.messages);
        for (const messageEntry of response.messages) {
          options.updateConversationPreviewFromMessage(messageEntry);
        }
        options.scheduleStickyDateRefresh();
      }

      return response;
    } catch (error) {
      await options.loadConversationMessages(conversationId, { forceRefresh: true });
      throw error;
    } finally {
      processingMessageAction.value = false;
    }
  }

  async function forwardMessagesToConversation(messageIds: string[], targetConversationId: string) {
    const conversationId = assertWritableConversation();
    const normalizedIds = normalizeUniqueIds(messageIds);
    const normalizedTargetConversationId = targetConversationId.trim();

    if (!normalizedIds.length) {
      return {
        sourceConversationId: conversationId,
        targetConversationId: normalizedTargetConversationId,
        createdCount: 0,
        queuedCount: 0,
        failedToQueueCount: 0,
        failedToQueueIds: [],
        messages: []
      } satisfies ForwardMessagesResponse;
    }

    if (!normalizedTargetConversationId) {
      throw new Error("Selecione uma conversa destino para encaminhar.");
    }

    processingMessageAction.value = true;
    try {
      const response = await options.apiFetch<ForwardMessagesResponse>(
        `/conversations/${conversationId}/messages/forward`,
        {
          method: "POST",
          body: {
            messageIds: normalizedIds,
            targetConversationId: normalizedTargetConversationId
          }
        }
      );

      if (response.messages.length > 0) {
        const targetConversationExists = options.conversations.value.some(
          (entry) => entry.id === normalizedTargetConversationId
        );

        if (targetConversationExists) {
          for (const messageEntry of response.messages) {
            options.updateConversationPreviewFromMessage(messageEntry);
          }
        }

        if (normalizedTargetConversationId === options.activeConversationId.value) {
          options.messages.value = options.mergeMessages(options.messages.value, response.messages);
          options.scheduleStickyDateRefresh();
        }
      }

      return response;
    } finally {
      processingMessageAction.value = false;
    }
  }

  return {
    processingMessageAction,
    deleteMessagesForMe,
    deleteMessagesForAll,
    forwardMessagesToConversation
  };
}
