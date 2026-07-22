import { computed, ref } from "vue";
import { describe, expect, it, vi } from "vitest";
import type { Conversation } from "~/types";
import { useOmnichannelInboxConversationActions } from "~/composables/omnichannel/useOmnichannelInboxConversationActions";

function buildConversation(overrides: Partial<Conversation> = {}): Conversation {
  return {
    id: "conversation-1",
    channel: "WHATSAPP",
    status: "OPEN",
    aiStatus: "transferring",
    externalId: "5511999999999@s.whatsapp.net",
    contactName: "Contato",
    contactAvatarUrl: null,
    contactPhone: "5511999999999",
    assignedToId: "user-1",
    createdAt: "2026-07-21T10:00:00.000Z",
    updatedAt: "2026-07-21T10:00:00.000Z",
    lastMessageAt: "2026-07-21T10:00:00.000Z",
    lastMessage: null,
    ...overrides
  };
}

function buildActions(apiFetch: (path: string, init?: Record<string, unknown>) => Promise<unknown>) {
  const sendError = ref("");
  const assigneeModel = ref("__unassigned__");
  const updatingStatus = ref(false);
  const updatingAssignee = ref(false);
  const updatingHandoff = ref(false);
  const upsertConversation = vi.fn();
  const selectConversation = vi.fn(async () => undefined);

  return {
    state: { sendError, assigneeModel, updatingHandoff },
    actions: useOmnichannelInboxConversationActions({
      canManageConversation: computed(() => true),
      activeConversationId: ref("conversation-1"),
      updatingStatus,
      updatingAssignee,
      updatingHandoff,
      assigneeModel,
      sendError,
      apiFetch: apiFetch as <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>,
      upsertConversation,
      selectConversation
    }),
    upsertConversation
  };
}

describe("useOmnichannelInboxConversationActions E5", () => {
  it("assume com chave de idempotencia e reidrata a conversa", async () => {
    const updated = buildConversation();
    const apiFetch = vi.fn().mockResolvedValue(updated);
    const harness = buildActions(apiFetch);

    await harness.actions.takeConversation();

    expect(apiFetch).toHaveBeenCalledWith(
      "/conversations/conversation-1/take",
      expect.objectContaining({
        method: "POST",
        body: expect.objectContaining({ idempotencyKey: expect.stringMatching(/^take:/) })
      })
    );
    expect(harness.upsertConversation).toHaveBeenCalledWith(updated);
    expect(harness.state.assigneeModel.value).toBe("user-1");
    expect(harness.state.updatingHandoff.value).toBe(false);
  });

  it("libera pelo endpoint autoritativo e mostra falha acionavel", async () => {
    const updated = buildConversation({ assignedToId: null, aiStatus: "idle" });
    const apiFetch = vi.fn()
      .mockResolvedValueOnce(updated)
      .mockRejectedValueOnce(new Error("conflito"));
    const harness = buildActions(apiFetch);

    await harness.actions.releaseConversation();
    expect(apiFetch).toHaveBeenNthCalledWith(1, "/conversations/conversation-1/release", { method: "POST" });
    expect(harness.state.assigneeModel.value).toBe("__unassigned__");

    await harness.actions.takeConversation();
    expect(harness.state.sendError.value).toBe("conflito");
    expect(harness.state.updatingHandoff.value).toBe(false);
  });
});
