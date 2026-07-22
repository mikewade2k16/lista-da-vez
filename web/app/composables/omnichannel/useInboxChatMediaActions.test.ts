import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Message } from "~/types";
import { useInboxChatMediaActions } from "~/composables/omnichannel/useInboxChatMediaActions";

const { apiFetchMock } = vi.hoisted(() => ({
  apiFetchMock: vi.fn()
}));

vi.mock("~/composables/useApi", () => ({
  ApiClientError: class ApiClientError extends Error {
    statusCode = 500;
  },
  useApi: () => ({ apiFetch: apiFetchMock })
}));

function buildMediaMessage(mediaState: Message["mediaState"]): Message {
  return {
    id: "message-1",
    tenantId: "tenant-1",
    conversationId: "conversation-1",
    senderUserId: null,
    direction: "INBOUND",
    messageType: "IMAGE",
    senderName: "Contato",
    senderAvatarUrl: null,
    content: "",
    mediaUrl: null,
    mediaMimeType: "image/jpeg",
    mediaFileName: "foto.jpg",
    mediaFileSizeBytes: 100,
    mediaCaption: null,
    mediaDurationSeconds: null,
    metadataJson: null,
    status: "SENT",
    origin: "contact",
    replyTo: null,
    providerStatusAt: null,
    providerErrorCode: "",
    mediaState,
    canRetryMedia: true,
    externalMessageId: "external-1",
    createdAt: "2026-07-21T10:00:00.000Z",
    updatedAt: "2026-07-21T10:00:00.000Z"
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}

describe("useInboxChatMediaActions E1-R1", () => {
  beforeEach(() => {
    apiFetchMock.mockReset();
  });

  it("devolve a mensagem autoritativa e libera o pending local apos a resposta", async () => {
    const response = buildMediaMessage("pending");
    apiFetchMock.mockResolvedValue(response);
    const actions = useInboxChatMediaActions({
      getToken: () => null,
      getSelectedTenantSlug: () => null,
      resolveMessageType: () => "IMAGE"
    });
    const failedMessage = buildMediaMessage("failed");

    const updated = await actions.retryMessageMedia(failedMessage);

    expect(updated).toEqual(response);
    expect(actions.resolveMediaState(failedMessage)).toBe("failed");
    expect(actions.getMediaRetryError(failedMessage.id)).toBe("");
    expect(apiFetchMock).toHaveBeenCalledWith(
      "/conversations/conversation-1/messages/message-1/media/retry",
      { method: "POST" }
    );
    actions.disposeMediaActions();
  });

  it("ready ou failed de realtime deixam de ser mascarados por pending", async () => {
    const request = deferred<Message>();
    apiFetchMock.mockReturnValue(request.promise);
    const actions = useInboxChatMediaActions({
      getToken: () => null,
      getSelectedTenantSlug: () => null,
      resolveMessageType: () => "IMAGE"
    });
    const failedMessage = buildMediaMessage("failed");

    const retry = actions.retryMessageMedia(failedMessage);
    expect(actions.resolveMediaState({ ...failedMessage, mediaState: "pending" })).toBe("pending");

    const realtimeReady = { ...failedMessage, mediaState: "ready" as const };
    actions.reconcileMediaRetryState(realtimeReady);
    expect(actions.resolveMediaState(realtimeReady)).toBe("ready");

    request.resolve(realtimeReady);
    await retry;
    actions.disposeMediaActions();
  });

  it("mostra falha acionavel e nao deixa pending eterno quando o retry falha", async () => {
    apiFetchMock.mockRejectedValue(new Error("offline"));
    const actions = useInboxChatMediaActions({
      getToken: () => null,
      getSelectedTenantSlug: () => null,
      resolveMessageType: () => "IMAGE"
    });
    const failedMessage = buildMediaMessage("failed");

    await expect(actions.retryMessageMedia(failedMessage)).resolves.toBeNull();

    expect(actions.resolveMediaState(failedMessage)).toBe("failed");
    expect(actions.getMediaRetryError(failedMessage.id)).toBe("Nao foi possivel agendar a nova tentativa.");
    actions.disposeMediaActions();
  });
});
