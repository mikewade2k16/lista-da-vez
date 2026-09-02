import { effectScope, ref } from "vue";
import { describe, expect, it, vi } from "vitest";
import type { Conversation, GroupParticipant, Message } from "~/types";
import type {
  ConversationsPageResponse,
  WhatsAppInstanceAccessState
} from "~/composables/omnichannel/useOmnichannelInboxShared";
import { useOmnichannelInboxHistory } from "./useOmnichannelInboxHistory";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}

function createScopeHistory(
  apiFetch: (path: string, init?: Record<string, unknown>) => Promise<unknown>,
  getScopeGeneration: () => number,
  accessState: WhatsAppInstanceAccessState = "resolved-nonempty",
  channelValue = "all"
) {
  const conversations = ref<Conversation[]>([]);
  const messages = ref<Message[]>([]);
  const scope = effectScope();
  const history = scope.run(() => useOmnichannelInboxHistory({
    apiFetch: apiFetch as <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>,
    conversations,
    messages,
    visibleMessagesConversationId: ref<string | null>(null),
    activeConversationId: ref<string | null>(null),
    selectedInstanceId: ref("all"),
    whatsappInstanceAccessState: ref(accessState),
    search: ref(""),
    channel: ref(channelValue),
    status: ref("all"),
    loadingWhatsAppStatus: ref(false),
    isWhatsAppConfigured: ref(true),
    isWhatsAppConnected: ref(true),
    loadingConversations: ref(false),
    loadingMoreConversations: ref(false),
    loadingMessages: ref(false),
    loadingOlderMessages: ref(false),
    loadingGroupParticipants: ref(false),
    conversationsError: ref(""),
    messagesError: ref(""),
    hasMoreConversations: ref(false),
    nextConversationsCursor: ref<string | null>(null),
    hasMoreMessages: ref(false),
    chatBodyRef: ref<HTMLElement | null>(null),
    mentionAlertState: ref<Record<string, number>>({}),
    groupParticipantsByConversation: {} as Record<string, GroupParticipant[]>,
    realtimeMessageHydrationLocks: new Set<string>(),
    groupParticipantsRefreshAtByConversation: new Map<string, number>(),
    groupParticipantsInFlightByConversation: new Set<string>(),
    historySyncAttemptAtByConversation: new Map<string, number>(),
    historySyncInFlightByConversation: new Set<string>(),
    sortConversations: vi.fn(),
    bootstrapReadState: vi.fn(),
    getReadAt: () => null,
    getSelectConversation: () => null,
    normalizeMessage: (message) => message,
    mergeMessages: (...chunks) => chunks.flat(),
    updateConversationPreviewFromMessage: vi.fn(),
    messageNeedsMediaHydration: () => false,
    getScopeGeneration
  }));
  if (!history) throw new Error("Falha ao criar historico de teste.");
  return { history, scope, conversations };
}

describe("sync-open por escopo de conta", () => {
  it("nao herda bootstrap, cooldown nem promise em voo ao trocar de conta", async () => {
    let generation = 0;
    const syncB = deferred<{ createdCount: number; updatedCount: number }>();
    const syncC = deferred<{ createdCount: number; updatedCount: number }>();
    const syncGenerations: number[] = [];
    const apiFetch = vi.fn((path: string) => {
      if (path === "/conversations/sync-open") {
        syncGenerations.push(generation);
        if (generation === 1) return syncB.promise;
        if (generation === 2) return syncC.promise;
        return Promise.resolve({ createdCount: 0, updatedCount: 0 });
      }
      return Promise.resolve({ conversations: [], hasMore: false } satisfies ConversationsPageResponse);
    });
    const state = createScopeHistory(apiFetch, () => generation);

    await state.history.loadConversations();
    await vi.waitFor(() => expect(syncGenerations).toEqual([0]));

    generation = 1;
    state.history.clearAllConversationCaches();
    await state.history.loadConversations();
    await vi.waitFor(() => expect(syncGenerations).toEqual([0, 1]));

    generation = 2;
    state.history.clearAllConversationCaches();
    await state.history.loadConversations();
    await vi.waitFor(() => expect(syncGenerations).toEqual([0, 1, 2]));

    syncC.resolve({ createdCount: 0, updatedCount: 0 });
    syncB.resolve({ createdCount: 0, updatedCount: 0 });
    await Promise.all([syncB.promise, syncC.promise]);
    state.scope.stop();
  });
});

describe("carregamento fail-closed por acesso WhatsApp", () => {
  it("consulta somente Instagram quando /instances/access terminou vazio ou em erro", async () => {
    for (const accessState of ["resolved-empty", "error"] as const) {
      const apiFetch = vi.fn(async () => ({ conversations: [], hasMore: false }));
      const state = createScopeHistory(apiFetch, () => 0, accessState);

      await state.history.loadConversations({ skipOpenSync: true });

      expect(apiFetch).toHaveBeenCalledTimes(1);
      const path = String(apiFetch.mock.calls[0]?.[0] ?? "");
      expect(path).toContain("channel=INSTAGRAM");
      expect(path).not.toContain("instanceId=");
      state.scope.stop();
    }
  });

  it("nao consulta conversas quando o filtro pede WhatsApp sem instancia autorizada", async () => {
    const apiFetch = vi.fn(async () => ({ conversations: [], hasMore: false }));
    const state = createScopeHistory(apiFetch, () => 0, "error", "WHATSAPP");
    state.conversations.value = [{ id: "stale", channel: "WHATSAPP" } as Conversation];

    await state.history.loadConversations({ skipOpenSync: true });

    expect(apiFetch).not.toHaveBeenCalled();
    expect(state.conversations.value).toEqual([]);
    state.scope.stop();
  });

  it("interpreta all como o conjunto autorizado sem enviar instanceId", async () => {
    const apiFetch = vi.fn(async () => ({ conversations: [], hasMore: false }));
    const state = createScopeHistory(apiFetch, () => 0, "resolved-nonempty");

    await state.history.loadConversations({ skipOpenSync: true });

    const path = String(apiFetch.mock.calls[0]?.[0] ?? "");
    expect(path).not.toContain("instanceId=");
    expect(path).not.toContain("channel=");
    state.scope.stop();
  });
});
