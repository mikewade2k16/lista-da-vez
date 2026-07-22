import { effectScope, ref } from "vue";
import { describe, expect, it, vi } from "vitest";
import type { Conversation, Message } from "~/types";
import type {
  ConversationsPageResponse,
  MessagesPageResponse
} from "~/composables/omnichannel/useOmnichannelInboxShared";
import { useOmnichannelInboxHistory } from "~/composables/omnichannel/useOmnichannelInboxHistory";

function buildConversation(id: string, lastMessageAt: string): Conversation {
  return {
    id,
    instanceId: "instance-1",
    instanceScopeKey: "instance-1",
    instanceName: "instance-1",
    instanceDisplayName: "Principal",
    channel: "WHATSAPP",
    status: "OPEN",
    externalId: `${id}@s.whatsapp.net`,
    contactId: null,
    contactName: id,
    contactAvatarUrl: null,
    contactPhone: "5511999999999",
    assignedToId: null,
    createdAt: lastMessageAt,
    updatedAt: lastMessageAt,
    lastMessageAt,
    lastMessage: null
  };
}

function buildMessage(id: string, conversationId: string, createdAt: string): Message {
  return {
    id,
    tenantId: "tenant-1",
    conversationId,
    senderUserId: null,
    direction: "INBOUND",
    messageType: "TEXT",
    senderName: "Contato",
    senderAvatarUrl: null,
    content: id,
    mediaUrl: null,
    mediaMimeType: null,
    mediaFileName: null,
    mediaFileSizeBytes: null,
    mediaCaption: null,
    mediaDurationSeconds: null,
    metadataJson: null,
    status: "SENT",
    origin: "contact",
    replyTo: null,
    providerStatusAt: null,
    providerErrorCode: "",
    mediaState: "",
    canRetryMedia: false,
    externalMessageId: id,
    createdAt,
    updatedAt: createdAt
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

function createHistory(apiFetch: (path: string, init?: Record<string, unknown>) => Promise<unknown>) {
  const conversations = ref<Conversation[]>([]);
  const messages = ref<Message[]>([]);
  const activeConversationId = ref<string | null>(null);
  const visibleMessagesConversationId = ref<string | null>(null);
  const loadingConversations = ref(false);
  const loadingMoreConversations = ref(false);
  const loadingMessages = ref(false);
  const loadingOlderMessages = ref(false);
  const conversationsError = ref("");
  const messagesError = ref("");
  const hasMoreConversations = ref(false);
  const nextConversationsCursor = ref<string | null>(null);
  const hasMoreMessages = ref(false);
  const search = ref("");
  const scope = effectScope();
  const history = scope.run(() => useOmnichannelInboxHistory({
    apiFetch: apiFetch as <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>,
    conversations,
    messages,
    visibleMessagesConversationId,
    activeConversationId,
    selectedInstanceId: ref("all"),
    search,
    channel: ref("all"),
    status: ref("all"),
    loadingWhatsAppStatus: ref(false),
    isWhatsAppConfigured: ref(false),
    isWhatsAppConnected: ref(false),
    loadingConversations,
    loadingMoreConversations,
    loadingMessages,
    loadingOlderMessages,
    loadingGroupParticipants: ref(false),
    conversationsError,
    messagesError,
    hasMoreConversations,
    nextConversationsCursor,
    hasMoreMessages,
    chatBodyRef: ref<HTMLElement | null>(null),
    mentionAlertState: ref<Record<string, number>>({}),
    groupParticipantsByConversation: {},
    realtimeMessageHydrationLocks: new Set<string>(),
    groupParticipantsRefreshAtByConversation: new Map<string, number>(),
    groupParticipantsInFlightByConversation: new Set<string>(),
    historySyncAttemptAtByConversation: new Map<string, number>(),
    historySyncInFlightByConversation: new Set<string>(),
    sortConversations: () => {
      conversations.value.sort((left, right) => {
        return Number(new Date(right.lastMessageAt)) - Number(new Date(left.lastMessageAt));
      });
    },
    bootstrapReadState: vi.fn(),
    getReadAt: () => null,
    getSelectConversation: () => null,
    normalizeMessage: (messageEntry) => messageEntry,
    mergeMessages: (...chunks) => {
      const byId = new Map<string, Message>();
      for (const chunk of chunks) {
        for (const messageEntry of chunk) {
          byId.set(messageEntry.id, messageEntry);
        }
      }
      return [...byId.values()].sort((left, right) => left.createdAt.localeCompare(right.createdAt));
    },
    updateConversationPreviewFromMessage: vi.fn(),
    messageNeedsMediaHydration: () => false
  }));

  if (!history) {
    throw new Error("Falha ao criar o composable de historico.");
  }

  return {
    scope,
    history,
    conversations,
    messages,
    activeConversationId,
    visibleMessagesConversationId,
    loadingConversations,
    loadingMoreConversations,
    conversationsError,
    messagesError,
    hasMoreConversations,
    nextConversationsCursor,
    search
  };
}

describe("useOmnichannelInboxHistory E1-R1", () => {
  it("concatena paginas sem duplicar e preserva a conversa ativa", async () => {
    const active = buildConversation("active", "2026-07-21T10:00:00.000Z");
    const first = buildConversation("first", "2026-07-21T09:00:00.000Z");
    const second = buildConversation("second", "2026-07-21T08:00:00.000Z");
    const apiFetch = vi.fn(async (path: string) => {
      if (path.includes("before=cursor-1")) {
        return {
          conversations: [{ ...first, contactName: "Atualizado" }, second],
          hasMore: false
        } satisfies ConversationsPageResponse;
      }

      return {
        conversations: [first],
        hasMore: true,
        nextCursor: "cursor-1"
      } satisfies ConversationsPageResponse;
    });
    const state = createHistory(apiFetch);
    state.conversations.value = [active];
    state.activeConversationId.value = active.id;

    await state.history.loadConversations({ skipOpenSync: true });
    expect(state.activeConversationId.value).toBe(active.id);
    expect(state.hasMoreConversations.value).toBe(true);
    expect(state.nextConversationsCursor.value).toBe("cursor-1");

    await state.history.loadMoreConversations();

    expect(state.activeConversationId.value).toBe(active.id);
    expect(state.conversations.value.map((entry) => entry.id)).toEqual(["active", "first", "second"]);
    expect(state.conversations.value.find((entry) => entry.id === "first")?.contactName).toBe("Atualizado");
    expect(state.hasMoreConversations.value).toBe(false);
    expect(state.nextConversationsCursor.value).toBeNull();
    expect(apiFetch).toHaveBeenCalledTimes(2);
    state.scope.stop();
  });

  it("ignora a resposta obsoleta depois que a busca muda", async () => {
    const alphaResponse = deferred<ConversationsPageResponse>();
    const betaResponse = deferred<ConversationsPageResponse>();
    const apiFetch = vi.fn((path: string) => {
      return path.includes("search=alpha") ? alphaResponse.promise : betaResponse.promise;
    });
    const state = createHistory(apiFetch);
    state.search.value = "alpha";
    const alphaRequest = state.history.loadConversations({ skipOpenSync: true });

    state.search.value = "beta";
    const betaRequest = state.history.loadConversations({ skipOpenSync: true });
    betaResponse.resolve({
      conversations: [buildConversation("beta", "2026-07-21T11:00:00.000Z")],
      hasMore: false
    });
    await betaRequest;

    alphaResponse.resolve({
      conversations: [buildConversation("alpha", "2026-07-21T12:00:00.000Z")],
      hasMore: false
    });
    await alphaRequest;

    expect(state.conversations.value.map((entry) => entry.id)).toEqual(["beta"]);
    expect(state.loadingConversations.value).toBe(false);
    expect(state.conversationsError.value).toBe("");
    state.scope.stop();
  });

  it("expoe erro acionavel sem transformar resposta invalida em lista vazia", async () => {
    const existing = buildConversation("existing", "2026-07-21T10:00:00.000Z");
    const apiFetch = vi.fn().mockResolvedValue({ conversations: null, hasMore: false });
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const state = createHistory(apiFetch);
    state.conversations.value = [existing];

    await state.history.loadConversations({ skipOpenSync: true });

    expect(state.conversations.value).toEqual([existing]);
    expect(state.conversationsError.value).toBe("Nao foi possivel carregar as conversas.");
    expect(state.loadingConversations.value).toBe(false);
    consoleError.mockRestore();
    state.scope.stop();
  });

  it("nao deixa historico atrasado substituir a conversa selecionada depois", async () => {
    const firstPage = deferred<MessagesPageResponse>();
    const apiFetch = vi.fn(async (path: string) => {
      if (path.includes("/first/messages")) {
        return firstPage.promise;
      }
      return {
        conversationId: "second",
        messages: [buildMessage("second-message", "second", "2026-07-21T12:00:00.000Z")],
        hasMore: false
      } satisfies MessagesPageResponse;
    });
    const state = createHistory(apiFetch);
    state.activeConversationId.value = "first";
    const firstRequest = state.history.loadConversationMessages("first");

    state.activeConversationId.value = "second";
    await state.history.loadConversationMessages("second");
    firstPage.resolve({
      conversationId: "first",
      messages: [buildMessage("first-message", "first", "2026-07-21T11:00:00.000Z")],
      hasMore: false
    });
    await firstRequest;

    expect(state.visibleMessagesConversationId.value).toBe("second");
    expect(state.messages.value.map((entry) => entry.id)).toEqual(["second-message"]);
    expect(state.messagesError.value).toBe("");
    state.scope.stop();
  });
});
