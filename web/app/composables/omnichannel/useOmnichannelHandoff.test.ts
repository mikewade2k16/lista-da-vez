import { describe, expect, it, vi } from "vitest";
import { useOmnichannelHandoff } from "~/composables/omnichannel/useOmnichannelHandoff";

const { apiFetchMock } = vi.hoisted(() => ({ apiFetchMock: vi.fn() }));
vi.mock("~/composables/useApi", () => ({
  useApi: () => ({ apiFetch: apiFetchMock })
}));

describe("useOmnichannelHandoff E5", () => {
  it("carrega filas, handoffs e eventos SLA pelo contrato do Go", async () => {
    apiFetchMock.mockImplementation(async (path: string) => {
      if (path === "/settings/queues") {
        return [{ id: "queue-1", name: "Comercial", isActive: true }];
      }
      if (path.endsWith("/handoffs")) {
        return [{ id: "handoff-1", status: "queued", reasonCode: "low_confidence" }];
      }
      return [{ id: "sla-1", eventType: "warning" }];
    });
    const handoff = useOmnichannelHandoff();

    await handoff.loadQueues();
    await handoff.loadConversationOperations("conversation-1");

    expect(handoff.queues.value).toHaveLength(1);
    expect(handoff.handoffs.value[0]?.id).toBe("handoff-1");
    expect(handoff.slaEvents.value[0]?.eventType).toBe("warning");
    expect(apiFetchMock).toHaveBeenCalledWith("/conversations/conversation-1/handoffs");
    apiFetchMock.mockReset();
  });

  it("transfere a conversa e falha fechado sem fila", async () => {
    const updated = { id: "conversation-1", status: "OPEN" };
    apiFetchMock
      .mockReset()
      .mockResolvedValueOnce(updated)
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([]);
    const handoff = useOmnichannelHandoff();

    expect(await handoff.transferConversation("conversation-1", "")).toBeNull();
    expect(await handoff.transferConversation("conversation-1", "queue-2")).toEqual(updated);
    expect(apiFetchMock).toHaveBeenNthCalledWith(1, "/conversations/conversation-1/queue", {
      method: "PATCH",
      body: { queueId: "queue-2" }
    });
    expect(handoff.transferringQueue.value).toBe(false);
    apiFetchMock.mockReset();
  });
});
