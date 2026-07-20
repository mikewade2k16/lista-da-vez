import type { ComputedRef, Ref } from "vue";
import type { Conversation, ConversationStatus } from "~/types";
import { UNASSIGNED_VALUE } from "~/composables/omnichannel/useOmnichannelInboxShared";

export function useOmnichannelInboxConversationActions(options: {
  canManageConversation: ComputedRef<boolean>;
  activeConversationId: Ref<string | null>;
  updatingStatus: Ref<boolean>;
  updatingAssignee: Ref<boolean>;
  assigneeModel: Ref<string>;
  sendError: Ref<string>;
  apiFetch: <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>;
  upsertConversation: (conversationEntry: Conversation) => void;
  selectConversation: (conversationId: string) => Promise<void>;
}) {
  async function updateConversationStatus(statusValue: ConversationStatus) {
    if (!options.canManageConversation.value) {
      options.sendError.value = "Seu perfil e somente leitura nesta inbox.";
      return;
    }

    if (!options.activeConversationId.value) {
      return;
    }

    options.updatingStatus.value = true;
    try {
      const updated = await options.apiFetch<Conversation>(`/conversations/${options.activeConversationId.value}/status`, {
        method: "PATCH",
        body: {
          status: statusValue
        }
      });

      options.upsertConversation(updated);
    } finally {
      options.updatingStatus.value = false;
    }
  }

  async function updateConversationAssignee(value: string) {
    if (!options.canManageConversation.value) {
      options.sendError.value = "Seu perfil e somente leitura nesta inbox.";
      return;
    }

    if (!options.activeConversationId.value) {
      return;
    }

    options.updatingAssignee.value = true;
    try {
      const updated = await options.apiFetch<Conversation>(`/conversations/${options.activeConversationId.value}/assign`, {
        method: "PATCH",
        body: {
          assignedToId: value === UNASSIGNED_VALUE ? null : value
        }
      });

      options.upsertConversation(updated);
      options.assigneeModel.value = updated.assignedToId ?? UNASSIGNED_VALUE;
    } finally {
      options.updatingAssignee.value = false;
    }
  }

  function closeConversation() {
    void updateConversationStatus("CLOSED");
  }

  async function openSandboxTestConversation() {
    if (!options.canManageConversation.value) {
      options.sendError.value = "Seu perfil e somente leitura nesta inbox.";
      return null;
    }

    const sandboxConversation = await options.apiFetch<Conversation>("/conversations/sandbox/test");
    options.upsertConversation(sandboxConversation);
    await options.selectConversation(sandboxConversation.id);
    return sandboxConversation;
  }

  return {
    closeConversation,
    updateConversationStatus,
    updateConversationAssignee,
    openSandboxTestConversation
  };
}
