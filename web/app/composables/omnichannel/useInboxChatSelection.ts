import { computed, ref } from "vue";
import type { Conversation } from "~/types";
import type { InboxRenderItem } from "~/components/omnichannel/inbox/types";

function toErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }

  return fallback;
}

export function useInboxChatSelection(options: {
  getMessageRenderItems: () => InboxRenderItem[];
  getConversationOptions: () => Conversation[];
  getActiveConversationId: () => string | null;
  deleteMessagesForMeAction: (messageIds: string[]) => Promise<{ deletedIds: string[] }>;
  deleteMessagesForAllAction: (messageIds: string[]) => Promise<{
    updatedIds: string[];
    skippedIds: string[];
    failedIds: string[];
  }>;
  forwardMessagesAction: (messageIds: string[], targetConversationId: string) => Promise<{
    createdCount: number;
    queuedCount: number;
    failedToQueueCount: number;
  }>;
}) {
  const selectedMessageIds = ref<string[]>([]);
  const selectionStatusMessage = ref("");
  const selectionActionPending = ref(false);
  const forwardModalOpen = ref(false);

  const selectedMessageItems = computed(() => {
    const selectedIds = new Set(selectedMessageIds.value);
    return options.getMessageRenderItems()
      .filter((item): item is Extract<InboxRenderItem, { kind: "message" }> => item.kind === "message")
      .filter((item) => selectedIds.has(item.message.id));
  });

  const selectionMode = computed(() => selectedMessageIds.value.length > 0);
  const selectedMessageCount = computed(() => selectedMessageItems.value.length);
  const canForwardSelectedMessages = computed(() => selectedMessageCount.value > 0);
  const canDeleteSelectedForAll = computed(() => {
    if (selectedMessageItems.value.length === 0) {
      return false;
    }

    return selectedMessageItems.value.every((item) => item.message.direction === "OUTBOUND");
  });

  const forwardConversationOptions = computed(() => {
    const activeConversationId = options.getActiveConversationId();
    return options.getConversationOptions().filter((conversationEntry) => {
      return conversationEntry.id !== activeConversationId;
    });
  });

  function clearSelectionStatus() {
    selectionStatusMessage.value = "";
  }

  function closeForwardDialog() {
    forwardModalOpen.value = false;
  }

  function clearSelection() {
    selectedMessageIds.value = [];
    clearSelectionStatus();
    closeForwardDialog();
  }

  function isMessageSelected(messageId: string) {
    return selectedMessageIds.value.includes(messageId);
  }

  function setSelectedMessageIds(nextIds: string[]) {
    selectedMessageIds.value = [...new Set(nextIds.filter((entry) => entry.trim().length > 0))];
  }

  function startMessageSelection(messageId: string) {
    if (!messageId.trim()) {
      return;
    }

    setSelectedMessageIds([...selectedMessageIds.value, messageId]);
    clearSelectionStatus();
  }

  function toggleMessageSelection(messageId: string) {
    if (!messageId.trim()) {
      return;
    }

    if (isMessageSelected(messageId)) {
      setSelectedMessageIds(selectedMessageIds.value.filter((entry) => entry !== messageId));
    } else {
      setSelectedMessageIds([...selectedMessageIds.value, messageId]);
    }

    if (!selectedMessageIds.value.length) {
      clearSelectionStatus();
      closeForwardDialog();
    }
  }

  function resolveTargetMessageIds(targetIds?: string[]) {
    if (Array.isArray(targetIds) && targetIds.length > 0) {
      return [...new Set(targetIds.filter((entry) => entry.trim().length > 0))];
    }

    return [...selectedMessageIds.value];
  }

  async function deleteMessagesForMe(targetIds?: string[]) {
    if (selectionActionPending.value) {
      return;
    }

    const ids = resolveTargetMessageIds(targetIds);
    if (!ids.length) {
      return;
    }

    selectionActionPending.value = true;
    clearSelectionStatus();

    try {
      const response = await options.deleteMessagesForMeAction(ids);
      const deletedIds = response.deletedIds.filter((entry) => ids.includes(entry));
      const nextSelectedIds = selectedMessageIds.value.filter((entry) => !deletedIds.includes(entry));
      setSelectedMessageIds(nextSelectedIds);
      closeForwardDialog();

      if (!deletedIds.length) {
        selectionStatusMessage.value = "Nenhuma mensagem foi removida.";
        return;
      }

      selectionStatusMessage.value =
        deletedIds.length === 1
          ? "Mensagem removida apenas para voce."
          : `${deletedIds.length} mensagens removidas apenas para voce.`;
    } catch (error) {
      selectionStatusMessage.value = toErrorMessage(error, "Nao foi possivel apagar a mensagem para voce.");
    } finally {
      selectionActionPending.value = false;
    }
  }

  async function deleteMessagesForAll(targetIds?: string[]) {
    if (selectionActionPending.value) {
      return;
    }

    const ids = resolveTargetMessageIds(targetIds);
    if (!ids.length) {
      return;
    }

    if (!canDeleteSelectedForAll.value && !targetIds?.length) {
      selectionStatusMessage.value = "So mensagens enviadas podem ser apagadas para todos.";
      return;
    }

    selectionActionPending.value = true;
    clearSelectionStatus();

    try {
      const response = await options.deleteMessagesForAllAction(ids);
      const updatedIds = response.updatedIds.filter((entry) => ids.includes(entry));
      setSelectedMessageIds(selectedMessageIds.value.filter((entry) => !updatedIds.includes(entry)));
      closeForwardDialog();

      if (!updatedIds.length) {
        selectionStatusMessage.value = "Nenhuma mensagem foi apagada para todos.";
        return;
      }

      const failedInfo =
        response.failedIds.length > 0 || response.skippedIds.length > 0
          ? ` (${response.failedIds.length} falharam, ${response.skippedIds.length} ignoradas)`
          : "";

      selectionStatusMessage.value =
        updatedIds.length === 1
          ? `Mensagem apagada para todos${failedInfo}.`
          : `${updatedIds.length} mensagens apagadas para todos${failedInfo}.`;
    } catch (error) {
      selectionStatusMessage.value = toErrorMessage(error, "Nao foi possivel apagar para todos.");
    } finally {
      selectionActionPending.value = false;
    }
  }

  function requestForwardMessages(targetIds?: string[]) {
    if (selectionActionPending.value) {
      return;
    }

    const ids = resolveTargetMessageIds(targetIds);
    if (!ids.length) {
      return;
    }

    setSelectedMessageIds(ids);
    clearSelectionStatus();
    forwardModalOpen.value = true;
  }

  async function forwardMessagesToConversation(targetConversationId: string) {
    if (selectionActionPending.value) {
      return;
    }

    const ids = resolveTargetMessageIds();
    if (!ids.length) {
      return;
    }

    selectionActionPending.value = true;
    clearSelectionStatus();

    try {
      const response = await options.forwardMessagesAction(ids, targetConversationId);
      setSelectedMessageIds([]);
      closeForwardDialog();

      const queuedInfo =
        response.failedToQueueCount > 0
          ? ` (${response.failedToQueueCount} ficaram pendentes de fila)`
          : "";

      selectionStatusMessage.value =
        response.createdCount === 1
          ? `Mensagem encaminhada${queuedInfo}.`
          : `${response.createdCount} mensagens encaminhadas${queuedInfo}.`;
    } catch (error) {
      selectionStatusMessage.value = toErrorMessage(error, "Nao foi possivel encaminhar as mensagens.");
    } finally {
      selectionActionPending.value = false;
    }
  }

  return {
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
  };
}
